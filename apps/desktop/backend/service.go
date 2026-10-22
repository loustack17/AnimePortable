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
	store     core.Store
	source    core.AnimeSource
	cover     *cover.Loader
	metadata  []core.MetadataProvider
	schedule  core.ScheduleProvider
	updates   releaseSource
	newPlayer func() (core.Player, error)
	close     func() error
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
	proxy           *forwardproxy.Source
	probeProxy      proxyProbe
	closeDeps       func() error
	newPlayer       func() (core.Player, error)
	databasePath    string
	// plan is the portable layout the app started with; moving the data folder updates it.
	plan     PortablePlan
	app      *core.App
	player   core.Player
	session  core.PlaybackSession
	failures *FailureLog
}

func New() *Service {
	return NewAt("")
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
// the data folder.
func NewAtWithPlan(plan PortablePlan, proxy *forwardproxy.Source, failures *FailureLog, factory func() (core.Player, error)) *Service {
	service := NewAtWithPlayback(plan.DatabasePath, proxy, failures, factory)
	service.plan = plan
	return service
}

func newWithDependencies(deps dependencies) *Service {
	service := New()
	service.store = service.guardedStore(deps.store)
	service.source = stageSource{deps.source}
	service.cover = deps.cover
	service.metadata = deps.metadata
	service.schedule = deps.schedule
	service.updates = deps.updates
	service.closeDeps = deps.close
	if deps.newPlayer != nil {
		service.newPlayer = deps.newPlayer
	}
	service.app = core.NewApp(service.source, nil, service.store)
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
		_ = deps.close()
		if deps.cover != nil {
			deps.cover.CloseIdleConnections()
		}
		if closer, ok := deps.store.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return ErrClosed
	}
	service.store = service.guardedStore(deps.store)
	service.source = stageSource{deps.source}
	service.cover = deps.cover
	service.metadata = deps.metadata
	service.schedule = deps.schedule
	service.updates = deps.updates
	service.closeDeps = deps.close
	service.app = core.NewApp(service.source, nil, service.store)
	service.mu.Unlock()
	service.bindLifecycle(ctx)
	if err := ctx.Err(); err != nil {
		service.lifecycleCancel()
		return safeError(err)
	}
	return nil
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
		if closeDeps != nil {
			if err := closeDeps(); first == nil {
				first = err
			}
		}
		if coverLoader != nil {
			coverLoader.CloseIdleConnections()
		}
		if store != nil {
			if closer, ok := store.(interface{ Close() error }); ok {
				if err := closer.Close(); first == nil {
					first = err
				}
			}
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
		plan, err := PlanPortable(executable, "")
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
	client, err := securehttp.New(securehttp.Config{AllowedOrigins: anime1.AllowedOrigins(), Proxy: proxy})
	if err != nil {
		_ = store.Close()
		return dependencies{}, err
	}
	metadataClient, err := securehttp.New(securehttp.Config{AllowedOrigins: append(anilist.AllowedOrigins(), bangumi.AllowedOrigins()...), Proxy: proxy})
	if err != nil {
		client.CloseIdleConnections()
		_ = store.Close()
		return dependencies{}, err
	}
	updateClient, err := securehttp.New(securehttp.Config{AllowedOrigins: github.AllowedOrigins(), MaxResponseBytes: github.MaxResponseBytes, Proxy: proxy})
	if err != nil {
		client.CloseIdleConnections()
		metadataClient.CloseIdleConnections()
		_ = store.Close()
		return dependencies{}, err
	}
	loader, err := cover.New(proxy)
	if err != nil {
		client.CloseIdleConnections()
		metadataClient.CloseIdleConnections()
		updateClient.CloseIdleConnections()
		_ = store.Close()
		return dependencies{}, err
	}
	bangumiClient := bangumi.New(metadataClient)
	return dependencies{
		store:    store,
		source:   anime1.New(client),
		cover:    loader,
		metadata: []core.MetadataProvider{anilist.New(metadataClient), bangumiClient},
		schedule: bangumiClient,
		updates:  github.New(updateClient),
		close: func() error {
			client.CloseIdleConnections()
			metadataClient.CloseIdleConnections()
			updateClient.CloseIdleConnections()
			return nil
		},
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

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, ErrInvalidInput
	}
	return parsed, nil
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
	return Settings{Appearance: appearanceName(settings.Appearance), AutoplayNext: toggleName(settings.AutoplayNext), AutoPlay: toggleName(settings.AutoPlay), ResumePlayback: toggleName(settings.ResumePlayback), Language: languageName(settings.Language),
		PlaybackSpeed:    int(settings.PlaybackSpeed),
		HardwareDecoding: toggleName(settings.HardwareDecoding), ProxyMode: proxyModeName(settings.ProxyMode), ProxyAddress: settings.ProxyAddress}
}

// settingsFromInput validates every field; zero values stay unspecified so the core resolves them.
func settingsFromInput(input Settings) (core.Settings, error) {
	var settings core.Settings
	var err error
	if settings.Appearance, err = parseAppearance(input.Appearance); err != nil {
		return core.Settings{}, err
	}
	if settings.AutoplayNext, err = parseToggle(input.AutoplayNext); err != nil {
		return core.Settings{}, err
	}
	if settings.AutoPlay, err = parseToggle(input.AutoPlay); err != nil {
		return core.Settings{}, err
	}
	if settings.ResumePlayback, err = parseToggle(input.ResumePlayback); err != nil {
		return core.Settings{}, err
	}
	if settings.Language, err = parseLanguage(input.Language); err != nil {
		return core.Settings{}, err
	}
	if settings.HardwareDecoding, err = parseToggle(input.HardwareDecoding); err != nil {
		return core.Settings{}, err
	}
	if settings.ProxyMode, err = parseProxyMode(input.ProxyMode); err != nil {
		return core.Settings{}, err
	}
	settings.ProxyAddress = strings.TrimSpace(input.ProxyAddress)
	if !core.ValidProxyAddress(settings.ProxyAddress) {
		return core.Settings{}, ErrInvalidInput
	}
	return settings, nil
}

func appearanceName(value core.Appearance) string {
	switch value {
	case core.AppearanceSystem:
		return "system"
	case core.AppearanceLight:
		return "light"
	case core.AppearanceDark:
		return "dark"
	default:
		return "unspecified"
	}
}
func toggleName(value core.Toggle) string {
	if value == core.ToggleEnabled {
		return "enabled"
	}
	if value == core.ToggleDisabled {
		return "disabled"
	}
	return "unspecified"
}
func proxyModeName(value core.ProxyMode) string {
	switch value {
	case core.ProxyModeNone:
		return "none"
	case core.ProxyModeSystem:
		return "system"
	case core.ProxyModeManual:
		return "manual"
	default:
		return "unspecified"
	}
}
func languageName(value core.Language) string {
	if value == core.LanguageTraditionalChinese {
		return "zh-TW"
	}
	if value == core.LanguageEnglish {
		return "en"
	}
	if value == core.LanguageJapanese {
		return "ja"
	}
	return "unspecified"
}

func parseAppearance(value string) (core.Appearance, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unspecified":
		return core.AppearanceUnspecified, nil
	case "system":
		return core.AppearanceSystem, nil
	case "light":
		return core.AppearanceLight, nil
	case "dark":
		return core.AppearanceDark, nil
	default:
		return 0, ErrInvalidInput
	}
}
func parseToggle(value string) (core.Toggle, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unspecified":
		return core.ToggleUnspecified, nil
	case "enabled":
		return core.ToggleEnabled, nil
	case "disabled":
		return core.ToggleDisabled, nil
	default:
		return 0, ErrInvalidInput
	}
}
func parseProxyMode(value string) (core.ProxyMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unspecified":
		return core.ProxyModeUnspecified, nil
	case "none":
		return core.ProxyModeNone, nil
	case "system":
		return core.ProxyModeSystem, nil
	case "manual":
		return core.ProxyModeManual, nil
	default:
		return 0, ErrInvalidInput
	}
}
func parseLanguage(value string) (core.Language, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unspecified":
		return core.LanguageUnspecified, nil
	case "zh-tw", "zh_tw":
		return core.LanguageTraditionalChinese, nil
	case "en", "en-us":
		return core.LanguageEnglish, nil
	case "ja", "ja-jp":
		return core.LanguageJapanese, nil
	default:
		return 0, ErrInvalidInput
	}
}

func sortAnime(items []Anime) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
