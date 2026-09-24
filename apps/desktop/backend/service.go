// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"animeportable/adapters/metadata/cover"
	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/persistence/sqlite"
	"animeportable/adapters/player/mpv"
	"animeportable/adapters/source/anime1"
	"animeportable/core"
	metadata "animeportable/internal/metadata"
)

var (
	ErrClosed       = errors.New("application is closed")
	ErrUnavailable  = errors.New("application unavailable")
	ErrInvalidInput = errors.New("invalid input")
)

type dependencies struct {
	store     core.Store
	source    core.AnimeSource
	cover     *cover.Loader
	newPlayer func(string) (core.Player, error)
	close     func() error
}

type Service struct {
	mu              sync.Mutex
	identity        sync.Mutex
	playMu          sync.Mutex
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
	closeDeps       func() error
	newPlayer       func(string) (core.Player, error)
	app             *core.App
	player          core.Player
	session         core.PlaybackSession
}

func New() *Service {
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &Service{operationSlots: make(chan struct{}, 16), shutdownDone: make(chan struct{}), lifecycleCtx: lifecycleCtx, lifecycleCancel: lifecycleCancel, newPlayer: newMPVPlayer}
}

func newWithDependencies(deps dependencies) *Service {
	service := New()
	service.store = deps.store
	service.source = deps.source
	service.cover = deps.cover
	service.closeDeps = deps.close
	if deps.newPlayer != nil {
		service.newPlayer = deps.newPlayer
	}
	service.app = core.NewApp(service.source, nil, service.store)
	return service
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
	deps, err := openProduction(startupCtx)
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
	service.store = deps.store
	service.source = deps.source
	service.cover = deps.cover
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
	if errors.Is(err, ErrInvalidInput) {
		return ErrInvalidInput
	}
	if errors.Is(err, mpv.ErrNotFound) {
		return mpv.ErrNotFound
	}
	if errors.Is(err, mpv.ErrInvalidPath) {
		return mpv.ErrInvalidPath
	}
	if errors.Is(err, mpv.ErrPlayerClosed) || errors.Is(err, mpv.ErrPlayerFailed) || errors.Is(err, mpv.ErrIPCClosed) {
		return ErrUnavailable
	}
	return ErrUnavailable
}

func newMPVPlayer(path string) (core.Player, error) {
	executable, err := mpv.Find(path)
	if err != nil {
		return nil, err
	}
	return mpv.NewPlayer(executable), nil
}

func openProduction(ctx context.Context) (dependencies, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return dependencies{}, ErrUnavailable
	}
	databasePath := filepath.Join(configDir, "AnimePortable", "animeportable.db")
	store, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		return dependencies{}, err
	}
	client, err := securehttp.New(securehttp.Config{AllowedOrigins: anime1.AllowedOrigins()})
	if err != nil {
		_ = store.Close()
		return dependencies{}, err
	}
	loader, err := cover.New()
	if err != nil {
		client.CloseIdleConnections()
		_ = store.Close()
		return dependencies{}, err
	}
	return dependencies{
		store:     store,
		source:    anime1.New(client),
		cover:     loader,
		newPlayer: newMPVPlayer,
		close: func() error {
			client.CloseIdleConnections()
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

func settingValues(settings core.Settings) Settings {
	return Settings{Appearance: appearanceName(settings.Appearance), MPVPath: settings.MPVPath, AutoplayNext: toggleName(settings.AutoplayNext), ResumePlayback: toggleName(settings.ResumePlayback), Language: languageName(settings.Language)}
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
func languageName(value core.Language) string {
	if value == core.LanguageTraditionalChinese {
		return "zh-TW"
	}
	if value == core.LanguageEnglish {
		return "en"
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
func parseLanguage(value string) (core.Language, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unspecified":
		return core.LanguageUnspecified, nil
	case "zh-tw", "zh_tw":
		return core.LanguageTraditionalChinese, nil
	case "en", "en-us":
		return core.LanguageEnglish, nil
	default:
		return 0, ErrInvalidInput
	}
}

func sortAnime(items []Anime) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
}
