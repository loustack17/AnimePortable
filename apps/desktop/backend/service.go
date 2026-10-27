// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"animeportable/adapters/metadata/anilist"
	"animeportable/adapters/metadata/aniskip"
	"animeportable/adapters/metadata/bangumi"
	"animeportable/adapters/metadata/cover"
	"animeportable/adapters/network/forwardproxy"
	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/persistence/sqlite"
	"animeportable/adapters/source/anime1"
	"animeportable/adapters/update/github"
	"animeportable/core"
	metadata "animeportable/internal/metadata"
)

var (
	ErrClosed       = errors.New("application is closed")
	ErrUnavailable  = errors.New("application unavailable")
	ErrInvalidInput = errors.New("invalid input")
	// ErrOffline means a source could not be reached at all, as when the network is off.
	// It is still ErrUnavailable for callers that do not tell the two apart.
	ErrOffline = fmt.Errorf("network unreachable: %w", ErrUnavailable)
)

type dependencies struct {
	store    core.Store
	source   core.AnimeSource
	cover    *cover.Loader
	metadata []core.MetadataProvider
	schedule core.ScheduleProvider
	updates  releaseSource
	// myAnimeList and openings find opening times; either nil means no opening is ever found.
	myAnimeList core.MyAnimeListSource
	openings    core.OpeningSource
	newPlayer   func() (core.Player, error)
	close       func() error
}

type Service struct {
	mu sync.Mutex
	// settingsMu makes reading and then saving the settings row one step.
	settingsMu sync.Mutex
	identity   sync.Mutex
	playMu     sync.Mutex
	locationMu sync.Mutex
	// dataMoved is set when the data folder was moved: the open database is then the old
	// copy and store writes are refused until the app restarts (see writeGuard).
	dataMoved       atomic.Bool
	startupMu       sync.Mutex
	closed          bool
	operationsWG    sync.WaitGroup
	operationSlots  chan struct{}
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	lifecycleStop   func() bool
	shutdownOnce    sync.Once
	shutdownDone    chan struct{}
	shutdownErr     error
	store           core.Store
	source          core.AnimeSource
	cover           *cover.Loader
	metadata        []core.MetadataProvider
	schedule        core.ScheduleProvider
	updates         releaseSource
	myAnimeList     core.MyAnimeListSource
	openings        core.OpeningSource
	// myAnimeListIDs keeps each anime's MyAnimeList id for the app run, 0 for none (owner,
	// 2026-10-22: memory only, nothing stored).
	myAnimeListIDs map[core.AnimeID]int
	proxy          *forwardproxy.Source
	probeProxy     proxyProbe
	closeDeps      func() error
	newPlayer      func() (core.Player, error)
	databasePath   string
	// plan is the portable layout the app started with; moving the data folder updates it.
	plan     PortablePlan
	app      *core.App
	player   core.Player
	session  core.PlaybackSession
	failures *FailureLog
	timeline *StartTimeline
}

func NewAt(databasePath string) *Service {
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &Service{operationSlots: make(chan struct{}, 16), shutdownDone: make(chan struct{}), lifecycleCtx: lifecycleCtx, lifecycleCancel: lifecycleCancel, newPlayer: func() (core.Player, error) { return nil, ErrUnavailable }, databasePath: databasePath, proxy: forwardproxy.New()}
}

// NewAtWithPlayback builds a Service with a player factory and a playback failure log.
// The caller keeps using the log to report player-side failures; a nil log disables
// failure logging. proxy is the forward-proxy source the Service keeps in step with the
// saved settings; the caller gives the same source to every other client that needs it.
func NewAtWithPlayback(databasePath string, proxy *forwardproxy.Source, failures *FailureLog, factory func() (core.Player, error)) *Service {
	service := NewAt(databasePath)
	if proxy != nil {
		service.proxy = proxy
	}
	if factory != nil {
		service.newPlayer = factory
	}
	service.failures = failures
	return service
}

// NewAtWithPlan is NewAtWithPlayback for a portable plan, which also lets the Service move
// the data folder. timeline (nil allowed) times each Resolve.
func NewAtWithPlan(plan PortablePlan, proxy *forwardproxy.Source, failures *FailureLog, timeline *StartTimeline, factory func() (core.Player, error)) *Service {
	service := NewAtWithPlayback(plan.DatabasePath, proxy, failures, factory)
	service.plan = plan
	service.timeline = timeline
	return service
}

// guardedStore wraps a store so it refuses writes once the data folder has moved. A nil store
// stays nil, which the Service reads as "not open yet".
func (service *Service) guardedStore(store core.Store) core.Store {
	if store == nil {
		return nil
	}
	return writeGuard{Store: store, refuse: &service.dataMoved}
}

func (service *Service) Start(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidInput
	}
	service.startupMu.Lock()
	defer service.startupMu.Unlock()
	if err := ctx.Err(); err != nil {
		return safeError(err)
	}
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return ErrClosed
	}
	ready := service.store != nil && service.source != nil
	service.mu.Unlock()
	if ready {
		service.bindLifecycle(ctx)
		if err := ctx.Err(); err != nil {
			service.lifecycleCancel()
			return safeError(err)
		}
		return nil
	}
	service.mu.Lock()
	startupCtx, startupCancel := context.WithCancel(ctx)
	stopLifecycle := context.AfterFunc(service.lifecycleCtx, startupCancel)
	service.mu.Unlock()
	deps, err := openProduction(startupCtx, service.databasePath, service.proxy)
	stopLifecycle()
	startupCancel()
	if err != nil {
		return safeError(err)
	}
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		_ = closeDependencies(deps.close, deps.cover, deps.store)
		return ErrClosed
	}
	service.adopt(deps)
	service.mu.Unlock()
	service.bindLifecycle(ctx)
	if err := ctx.Err(); err != nil {
		service.lifecycleCancel()
		return safeError(err)
	}
	return nil
}

// stageSource tags Resolve failures so Play can tell resolve from player failures, and
// times each Resolve for the start timeline.
type stageSource struct {
	core.AnimeSource
	timeline *StartTimeline
}

func (source stageSource) Resolve(ctx context.Context, ref core.EpisodeRef) (core.PlaybackSource, error) {
	began := time.Now()
	resolved, err := source.AnimeSource.Resolve(ctx, ref)
	source.timeline.resolved(time.Since(began), err)
	if err != nil {
		return resolved, &stageError{stage: stageResolve, err: err}
	}
	return resolved, nil
}

// adopt makes deps the Service's dependencies. The caller holds service.mu or owns the
// Service exclusively.
func (service *Service) adopt(deps dependencies) {
	service.store = service.guardedStore(deps.store)
	service.source = stageSource{AnimeSource: deps.source, timeline: service.timeline}
	service.cover = deps.cover
	service.metadata = deps.metadata
	service.schedule = deps.schedule
	service.updates = deps.updates
	service.myAnimeList = deps.myAnimeList
	service.openings = deps.openings
	service.closeDeps = deps.close
	if deps.newPlayer != nil {
		service.newPlayer = deps.newPlayer
	}
	service.app = core.NewApp(service.source, nil, service.store)
}

// closeDependencies releases what adopt took over, in order: the HTTP clients, the cover
// loader's connections, then the store. It returns the first error.
func closeDependencies(closeClients func() error, coverLoader *cover.Loader, store core.Store) error {
	var first error
	if closeClients != nil {
		first = closeClients()
	}
	if coverLoader != nil {
		coverLoader.CloseIdleConnections()
	}
	if closer, ok := store.(interface{ Close() error }); ok {
		if err := closer.Close(); first == nil {
			first = err
		}
	}
	return first
}

func (service *Service) bindLifecycle(ctx context.Context) {
	service.mu.Lock()
	if service.lifecycleStop == nil {
		service.lifecycleStop = context.AfterFunc(ctx, service.lifecycleCancel)
	}
	service.mu.Unlock()
}

func (service *Service) Close() error {
	if service == nil {
		return nil
	}
	service.shutdownOnce.Do(func() {
		service.mu.Lock()
		service.closed = true
		if service.lifecycleStop != nil {
			service.lifecycleStop()
		}
		service.lifecycleCancel()
		service.mu.Unlock()
		service.startupMu.Lock()
		defer service.startupMu.Unlock()
		service.operationsWG.Wait()
		service.mu.Lock()
		session := service.session
		service.session = nil
		closeDeps := service.closeDeps
		store := service.store
		coverLoader := service.cover
		service.mu.Unlock()
		var first error
		if session != nil {
			first = session.Close()
		}
		if err := closeDependencies(closeDeps, coverLoader, store); first == nil {
			first = err
		}
		service.shutdownErr = safeError(first)
		close(service.shutdownDone)
	})
	<-service.shutdownDone
	return service.shutdownErr
}

func (service *Service) begin(ctx context.Context) (context.Context, func(), error) {
	if ctx == nil {
		return nil, nil, ErrInvalidInput
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	select {
	case service.operationSlots <- struct{}{}:
	default:
		return nil, nil, ErrUnavailable
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed || service.lifecycleCtx.Err() != nil {
		<-service.operationSlots
		return nil, nil, ErrClosed
	}
	if service.store == nil || service.source == nil {
		<-service.operationSlots
		return nil, nil, ErrUnavailable
	}
	operationCtx, cancel := context.WithCancel(ctx)
	stopLifecycle := context.AfterFunc(service.lifecycleCtx, cancel)
	service.operationsWG.Add(1)
	return operationCtx, func() {
		cancel()
		stopLifecycle()
		<-service.operationSlots
		service.operationsWG.Done()
	}, nil
}

func (service *Service) appSnapshot() (*core.App, core.Store, core.AnimeSource, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.closed {
		return nil, nil, nil, ErrClosed
	}
	if service.app == nil || service.store == nil || service.source == nil {
		return nil, nil, nil, ErrUnavailable
	}
	return service.app, service.store, service.source, nil
}

func safeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, core.ErrNotFound) {
		return core.ErrNotFound
	}
	if errors.Is(err, ErrClosed) {
		return ErrClosed
	}
	if errors.Is(err, ErrRestartRequired) {
		return ErrRestartRequired
	}
	if errors.Is(err, ErrInvalidInput) {
		return ErrInvalidInput
	}
	var secure *securehttp.Error
	if errors.As(err, &secure) && (secure.Kind == securehttp.KindNetwork || secure.Kind == securehttp.KindTimeout || secure.Kind == securehttp.KindProxy) {
		return ErrOffline
	}
	return ErrUnavailable
}

func openProduction(ctx context.Context, databasePath string, proxySource *forwardproxy.Source) (dependencies, error) {
	if databasePath == "" {
		executable, err := os.Executable()
		if err != nil {
			return dependencies{}, ErrUnavailable
		}
		plan, err := PlanPortable(executable)
		if err != nil {
			return dependencies{}, err
		}
		databasePath = plan.DatabasePath
	}
	store, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		return dependencies{}, err
	}
	// Before any client exists, so the first request already uses the stored proxy. Unreadable
	// settings leave the proxy off.
	stored, _ := store.Settings(ctx)
	stored = stored.Resolved()
	proxySource.Set(stored.ProxyMode, stored.ProxyAddress)
	proxy := proxySource.Address
	// clients are closed in creation order, on a failure below and by the returned close.
	var clients []*securehttp.Client
	closeClients := func() error {
		for _, client := range clients {
			client.CloseIdleConnections()
		}
		return nil
	}
	fail := func(err error) (dependencies, error) {
		_ = closeClients()
		_ = store.Close()
		return dependencies{}, err
	}
	newClient := func(config securehttp.Config) (*securehttp.Client, error) {
		config.Proxy = proxy
		created, err := securehttp.New(config)
		if err == nil {
			clients = append(clients, created)
		}
		return created, err
	}
	client, err := newClient(securehttp.Config{AllowedOrigins: anime1.AllowedOrigins()})
	if err != nil {
		return fail(err)
	}
	metadataClient, err := newClient(securehttp.Config{AllowedOrigins: append(anilist.AllowedOrigins(), bangumi.AllowedOrigins()...)})
	if err != nil {
		return fail(err)
	}
	updateClient, err := newClient(securehttp.Config{AllowedOrigins: github.AllowedOrigins(), MaxResponseBytes: github.MaxResponseBytes})
	if err != nil {
		return fail(err)
	}
	openingClient, err := newClient(securehttp.Config{AllowedOrigins: aniskip.AllowedOrigins(), MaxResponseBytes: aniskip.MaxResponseBytes})
	if err != nil {
		return fail(err)
	}
	loader, err := cover.New(proxy)
	if err != nil {
		return fail(err)
	}
	anilistClient := anilist.New(metadataClient)
	bangumiClient := bangumi.New(metadataClient)
	return dependencies{
		store:       store,
		source:      anime1.New(client),
		cover:       loader,
		metadata:    []core.MetadataProvider{anilistClient, bangumiClient},
		schedule:    bangumiClient,
		updates:     github.New(updateClient),
		myAnimeList: anilistClient,
		openings:    aniskip.New(openingClient),
		close:       closeClients,
	}, nil
}

func localID(value string) (core.AnimeID, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return "", ErrInvalidInput
	}
	return core.AnimeID(value), nil
}

func episodeID(value string) (core.EpisodeID, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return "", ErrInvalidInput
	}
	return core.EpisodeID(value), nil
}

func newLocalID(prefix string) (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", ErrUnavailable
	}
	return prefix + ":" + hex.EncodeToString(random[:]), nil
}

func normalizeSourceAnime(item core.SourceAnime) (core.SourceAnime, error) {
	if strings.TrimSpace(item.Ref.Provider) == "" || strings.TrimSpace(item.Ref.ID) == "" {
		return core.SourceAnime{}, ErrInvalidInput
	}
	title, ok := metadata.NormalizePlainText(item.Title, metadata.TitleLimits())
	if !ok || title == "" {
		return core.SourceAnime{}, ErrInvalidInput
	}
	native := item.NativeTitle
	if native != "" {
		native, ok = metadata.NormalizePlainText(native, metadata.TitleLimits())
		if !ok {
			return core.SourceAnime{}, ErrInvalidInput
		}
	}
	item.Title, item.NativeTitle = title, native
	return item, nil
}

func animeDTO(anime core.Anime) (Anime, error) {
	title, ok := metadata.NormalizePlainText(anime.Title, metadata.TitleLimits())
	if !ok {
		return Anime{}, ErrUnavailable
	}
	native := anime.NativeTitle
	if native != "" {
		native, ok = metadata.NormalizePlainText(native, metadata.TitleLimits())
		if !ok {
			return Anime{}, ErrUnavailable
		}
	}
	description := anime.Description
	if description != "" {
		description, ok = metadata.NormalizePlainText(description, metadata.DescriptionLimits())
		if !ok {
			return Anime{}, ErrUnavailable
		}
	}
	return Anime{ID: string(anime.ID), Title: title, NativeTitle: native, Description: description}, nil
}

func historyDTO(entry core.HistoryEntry) History {
	return History{AnimeID: string(entry.Progress.AnimeID), EpisodeID: string(entry.Progress.EpisodeID), Position: entry.Progress.Position.Milliseconds(), Duration: entry.Progress.Duration.Milliseconds(), Completed: entry.Progress.Completed, UpdatedAt: entry.Progress.UpdatedAt.UTC().Format(time.RFC3339Nano), LastPlayed: entry.LastPlayedAt.UTC().Format(time.RFC3339Nano)}
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func validMillis(value int64) bool {
	return value >= 0 && value <= math.MaxInt64/int64(time.Millisecond)
}

// DefaultSettings is what a user who never chose anything gets; the UI holds it when the stored
// settings cannot be read, so the defaults stay in core.
func DefaultSettings() Settings {
	return settingValues(core.Settings{}.Resolved())
}

func settingValues(settings core.Settings) Settings {
	return Settings{Appearance: appearanceNames.name(settings.Appearance), AutoplayNext: toggleNames.name(settings.AutoplayNext), AutoPlay: toggleNames.name(settings.AutoPlay), ResumePlayback: toggleNames.name(settings.ResumePlayback), Language: languageNames.name(settings.Language),
		PlaybackSpeed:    int(settings.PlaybackSpeed),
		HardwareDecoding: toggleNames.name(settings.HardwareDecoding), ProxyMode: proxyModeNames.name(settings.ProxyMode), ProxyAddress: settings.ProxyAddress}
}

// settingsFromInput validates every field; zero values stay unspecified so the core resolves them.
func settingsFromInput(input Settings) (core.Settings, error) {
	var settings core.Settings
	var err error
	if settings.Appearance, err = appearanceNames.parse(input.Appearance); err != nil {
		return core.Settings{}, err
	}
	if settings.AutoplayNext, err = toggleNames.parse(input.AutoplayNext); err != nil {
		return core.Settings{}, err
	}
	if settings.AutoPlay, err = toggleNames.parse(input.AutoPlay); err != nil {
		return core.Settings{}, err
	}
	if settings.ResumePlayback, err = toggleNames.parse(input.ResumePlayback); err != nil {
		return core.Settings{}, err
	}
	if settings.Language, err = languageNames.parse(input.Language); err != nil {
		return core.Settings{}, err
	}
	if settings.HardwareDecoding, err = toggleNames.parse(input.HardwareDecoding); err != nil {
		return core.Settings{}, err
	}
	if settings.ProxyMode, err = proxyModeNames.parse(input.ProxyMode); err != nil {
		return core.Settings{}, err
	}
	settings.ProxyAddress = strings.TrimSpace(input.ProxyAddress)
	if !core.ValidProxyAddress(settings.ProxyAddress) {
		return core.Settings{}, ErrInvalidInput
	}
	return settings, nil
}

func sortAnime(items []Anime) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
