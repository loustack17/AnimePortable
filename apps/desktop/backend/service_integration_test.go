// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/adapters/player/mpv"
	"animeportable/core"
	metadata "animeportable/internal/metadata"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type persistentSource struct {
	mu       sync.Mutex
	item     core.SourceAnime
	searches int
}

func (source *persistentSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return []core.SourceAnime{source.item}, nil
}

func (source *persistentSource) Search(context.Context, string) ([]core.SourceAnime, error) {
	source.mu.Lock()
	source.searches++
	source.mu.Unlock()
	return []core.SourceAnime{source.item}, nil
}

func (*persistentSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	return nil, nil
}

func (*persistentSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.NewPlaybackSource("https://media.example/video", nil), nil
}

func (*persistentSource) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return nil, nil
}

func TestCanonicalIdentityPersistsAcrossSQLiteRestartAndConcurrentIngest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	item := core.SourceAnime{Ref: core.SourceRef{Provider: "anime1", ID: "show-42"}, Title: "Show 42"}
	ctx := context.Background()

	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	source := &persistentSource{item: item}
	service := newWithDependencies(dependencies{store: store, source: source})
	first, err := service.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].ID == "" {
		t.Fatalf("first catalog = %#v", first)
	}
	canonicalID := first[0].ID

	const concurrentCalls = 8
	results := make(chan []Anime, concurrentCalls)
	for range concurrentCalls {
		go func() {
			items, callErr := service.Search(ctx, "show")
			if callErr != nil {
				results <- []Anime{{Description: callErr.Error()}}
				return
			}
			results <- items
		}()
	}
	for range concurrentCalls {
		items := <-results
		if len(items) != 1 || items[0].ID != canonicalID || items[0].Description != "" {
			t.Fatalf("concurrent identity result = %#v", items)
		}
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := newWithDependencies(dependencies{store: reopened, source: &persistentSource{item: item}})
	t.Cleanup(func() { _ = restarted.ServiceShutdown() })
	afterRestart, err := restarted.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(afterRestart) != 1 || afterRestart[0].ID != canonicalID {
		t.Fatalf("restart identity = %#v, want %q", afterRestart, canonicalID)
	}
}

type playbackTestPlayer struct {
	mu       sync.Mutex
	starts   int
	startErr error
	session  *playbackTestSession
}

func (player *playbackTestPlayer) Start(ctx context.Context, request core.PlayRequest) (core.PlaybackSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	player.mu.Lock()
	defer player.mu.Unlock()
	player.starts++
	if player.startErr != nil {
		return nil, player.startErr
	}
	if player.session == nil || player.session.isClosed() {
		player.session = newPlaybackTestSession()
	}
	player.session.mu.Lock()
	player.session.request = request
	player.session.mu.Unlock()
	return player.session, nil
}

func (player *playbackTestPlayer) startCount() int {
	player.mu.Lock()
	defer player.mu.Unlock()
	return player.starts
}

type playbackTestSession struct {
	mu       sync.Mutex
	events   chan core.PlaybackEvent
	request  core.PlayRequest
	loads    []core.PlayRequest
	loadErr  error
	closed   bool
	closeLog *[]string
	logMu    *sync.Mutex
}

func newPlaybackTestSession() *playbackTestSession {
	return &playbackTestSession{events: make(chan core.PlaybackEvent)}
}

func (session *playbackTestSession) Load(ctx context.Context, request core.PlayRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	session.mu.Lock()
	session.request = request
	session.loads = append(session.loads, request)
	loadErr := session.loadErr
	session.mu.Unlock()
	return loadErr
}

func (session *playbackTestSession) Events() <-chan core.PlaybackEvent {
	return session.events
}

func (session *playbackTestSession) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	session.mu.Lock()
	startAt := session.request.StartAt
	session.mu.Unlock()
	return core.PlaybackSnapshot{Position: startAt}, nil
}

func (session *playbackTestSession) Close() error {
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return nil
	}
	session.closed = true
	close(session.events)
	log := session.closeLog
	logMu := session.logMu
	session.mu.Unlock()
	if log != nil && logMu != nil {
		logMu.Lock()
		*log = append(*log, "session")
		logMu.Unlock()
	}
	return nil
}

func (session *playbackTestSession) loadCount() int {
	session.mu.Lock()
	defer session.mu.Unlock()
	return len(session.loads)
}

func (session *playbackTestSession) isClosed() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closed
}

func (session *playbackTestSession) setLoadError(err error) {
	session.mu.Lock()
	session.loadErr = err
	session.mu.Unlock()
}

func (session *playbackTestSession) emit(event core.PlaybackEvent) {
	session.events <- event
}

func TestPlayReusesTrackedSessionAfterCallerCancellation(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime", Title: "Anime"}
	ref1 := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode-1"}
	ref2 := core.EpisodeRef{Anime: ref1.Anime, ID: "episode-2"}
	store.mappings["anime"] = []core.EpisodeMapping{
		{AnimeID: "anime", EpisodeID: "episode-1", Ref: ref1},
		{AnimeID: "anime", EpisodeID: "episode-2", Ref: ref2},
	}
	player := &playbackTestPlayer{}
	service := newWithDependencies(dependencies{
		store:     store,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref1.Anime, Title: "Anime"}},
		newPlayer: func(string) (core.Player, error) { return player, nil },
	})
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	requestCtx, cancel := context.WithCancel(context.Background())
	if err := service.Play(requestCtx, PlayRequest{AnimeID: "anime", EpisodeID: "episode-1"}); err != nil {
		t.Fatal(err)
	}
	cancel()
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode-2", StartAt: 2500}); err != nil {
		t.Fatal(err)
	}
	if got := player.startCount(); got != 1 {
		t.Fatalf("player starts = %d, want one reused session", got)
	}
	player.mu.Lock()
	session := player.session
	player.mu.Unlock()
	if session == nil {
		t.Fatal("player did not retain a playback session")
	}
	if loads := session.loadCount(); loads != 1 {
		t.Fatalf("session loads = %d, want one switch", loads)
	}
	if session.isClosed() {
		t.Fatal("tracked session closed when initiating context was canceled")
	}
}

func TestPlayFailureCanRecoverAndRejectsMillisecondOverflow(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime"}
	ref := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode"}
	store.mappings["anime"] = []core.EpisodeMapping{{AnimeID: "anime", EpisodeID: "episode", Ref: ref}}
	player := &playbackTestPlayer{startErr: errors.New("player secret https://internal.invalid")}
	service := newWithDependencies(dependencies{
		store:     store,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: "Anime"}},
		newPlayer: func(string) (core.Player, error) { return player, nil },
	})
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode", StartAt: math.MaxInt64}); err != ErrInvalidInput {
		t.Fatalf("overflow error = %v, want ErrInvalidInput", err)
	}
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode"}); err != ErrUnavailable {
		t.Fatalf("first player error = %v, want ErrUnavailable", err)
	}
	player.mu.Lock()
	player.startErr = nil
	player.mu.Unlock()
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode"}); err != nil {
		t.Fatalf("recovery error = %v", err)
	}
}

func TestPlayPreservesSafePlayerLaunchErrors(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime"}
	ref := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode"}
	store.mappings["anime"] = []core.EpisodeMapping{{AnimeID: "anime", EpisodeID: "episode", Ref: ref}}
	for _, test := range []struct {
		name  string
		cause error
		want  error
	}{
		{name: "missing", cause: mpv.ErrNotFound, want: mpv.ErrNotFound},
		{name: "invalid path", cause: mpv.ErrInvalidPath, want: mpv.ErrInvalidPath},
		{name: "generic", cause: errors.New("player secret https://internal.invalid/path"), want: ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := newWithDependencies(dependencies{
				store:  store,
				source: &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: "Anime"}},
				newPlayer: func(string) (core.Player, error) {
					return nil, fmt.Errorf("launch secret C:/private/mpv.exe: %w", test.cause)
				},
			})
			t.Cleanup(func() { _ = service.ServiceShutdown() })
			err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode"})
			if err != test.want {
				t.Fatalf("play error identity = %v, want %v", err, test.want)
			}
			if test.want == mpv.ErrNotFound || test.want == mpv.ErrInvalidPath {
				if err.Error() != test.want.Error() {
					t.Fatalf("play error message = %q, want %q", err.Error(), test.want.Error())
				}
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "internal.invalid") {
				t.Fatalf("play error leaked launch detail: %q", err)
			}
		})
	}
}

func TestConcurrentInitialPlayUsesOnePlayerStart(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime"}
	ref := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode"}
	store.mappings["anime"] = []core.EpisodeMapping{{AnimeID: "anime", EpisodeID: "episode", Ref: ref}}
	player := &playbackTestPlayer{}
	service := newWithDependencies(dependencies{
		store:     store,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: "Anime"}},
		newPlayer: func(string) (core.Player, error) { return player, nil },
	})
	t.Cleanup(func() { _ = service.ServiceShutdown() })

	const callers = 8
	results := make(chan error, callers)
	for range callers {
		go func() {
			results <- service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode"})
		}()
	}
	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("concurrent play error = %v", err)
		}
	}
	if starts := player.startCount(); starts != 1 {
		t.Fatalf("player starts = %d, want one", starts)
	}
	player.mu.Lock()
	session := player.session
	player.mu.Unlock()
	if session == nil {
		t.Fatal("player did not retain session")
	}
	if loads := session.loadCount(); loads != callers-1 {
		t.Fatalf("session switches = %d, want %d", loads, callers-1)
	}
}

func TestTerminalPlaybackEventAllowsNextPlayOnTrackedSession(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime"}
	first := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode-1"}
	second := core.EpisodeRef{Anime: first.Anime, ID: "episode-2"}
	store.mappings["anime"] = []core.EpisodeMapping{
		{AnimeID: "anime", EpisodeID: "episode-1", Ref: first},
		{AnimeID: "anime", EpisodeID: "episode-2", Ref: second},
	}
	player := &playbackTestPlayer{}
	service := newWithDependencies(dependencies{
		store:     store,
		source:    &persistentSource{item: core.SourceAnime{Ref: first.Anime, Title: "Anime"}},
		newPlayer: func(string) (core.Player, error) { return player, nil },
	})
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode-1"}); err != nil {
		t.Fatal(err)
	}
	player.mu.Lock()
	session := player.session
	player.mu.Unlock()
	if session == nil {
		t.Fatal("player did not retain session")
	}
	session.emit(core.PlaybackEvent{AnimeID: "anime", EpisodeID: "episode-1", Kind: core.PlaybackEventStopped, Position: time.Second, Duration: time.Minute})
	session.setLoadError(mpv.ErrPlayerClosed)
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode-2"}); err != nil {
		t.Fatalf("play after stopped event = %v", err)
	}
	if starts := player.startCount(); starts != 2 {
		t.Fatalf("player starts after terminal event = %d, want recovery start", starts)
	}
	if loads := session.loadCount(); loads != 1 {
		t.Fatalf("loads after terminal event = %d, want one", loads)
	}
	if !session.isClosed() {
		t.Fatal("failed old session was not closed before recovery")
	}
	player.mu.Lock()
	recovered := player.session
	player.mu.Unlock()
	if recovered == nil || recovered == session || recovered.isClosed() {
		t.Fatal("recovery did not create a live replacement session")
	}
}

func TestCachedAnimeTextIsNormalizedAtBindingBoundary(t *testing.T) {
	service, store, _ := testService()
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	store.anime["unsafe"] = core.Anime{
		ID:          "unsafe",
		Title:       "<b>Title</b>",
		NativeTitle: "[Native](https://example.invalid)",
		Description: "<script>secret</script>Visible",
	}
	items, err := service.Library(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("library = %#v", items)
	}
	wantTitle, _ := metadata.NormalizePlainText(store.anime["unsafe"].Title, metadata.TitleLimits())
	wantNative, _ := metadata.NormalizePlainText(store.anime["unsafe"].NativeTitle, metadata.TitleLimits())
	wantDescription, _ := metadata.NormalizePlainText(store.anime["unsafe"].Description, metadata.DescriptionLimits())
	if items[0].Title != wantTitle || items[0].NativeTitle != wantNative || items[0].Description != wantDescription {
		t.Fatalf("normalized anime = %#v, want title=%q native=%q description=%q", items[0], wantTitle, wantNative, wantDescription)
	}
	if strings.Contains(items[0].Title+items[0].NativeTitle+items[0].Description, "<") {
		t.Fatalf("unsafe markup reached DTO: %#v", items[0])
	}
}

type fallbackEpisodeSource struct {
	item        core.SourceAnime
	first       core.SourceRef
	second      core.SourceRef
	firstCalls  int
	secondCalls int
}

func (source *fallbackEpisodeSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return []core.SourceAnime{source.item}, nil
}

func (*fallbackEpisodeSource) Search(context.Context, string) ([]core.SourceAnime, error) {
	return nil, nil
}

func (source *fallbackEpisodeSource) Episodes(_ context.Context, ref core.SourceRef) ([]core.SourceEpisode, error) {
	if ref == source.first {
		source.firstCalls++
		return nil, errors.New("first source unavailable")
	}
	if ref != source.second {
		return nil, errors.New("unexpected source ref")
	}
	source.secondCalls++
	return []core.SourceEpisode{{Ref: core.EpisodeRef{Anime: source.second, ID: "episode"}, Number: "1", Title: "Episode"}}, nil
}

func (*fallbackEpisodeSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.NewPlaybackSource("https://media.example/video", nil), nil
}

func (*fallbackEpisodeSource) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return nil, nil
}

func TestEpisodesFallsBackToTheNextPersistedSourceRef(t *testing.T) {
	store := newFakeStore()
	store.anime["anime"] = core.Anime{ID: "anime", Title: "Anime"}
	first := core.SourceRef{Provider: "anime1", ID: "first"}
	second := core.SourceRef{Provider: "anime1", ID: "second"}
	store.refs["anime"] = []core.SourceRef{first, second}
	source := &fallbackEpisodeSource{
		item:   core.SourceAnime{Ref: first, Title: "Anime"},
		first:  first,
		second: second,
	}
	service := newWithDependencies(dependencies{store: store, source: source})
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	episodes, err := service.Episodes(context.Background(), "anime")
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].Number != "1" {
		t.Fatalf("episodes = %#v", episodes)
	}
	if source.firstCalls != 1 || source.secondCalls != 1 {
		t.Fatalf("source calls = first:%d second:%d", source.firstCalls, source.secondCalls)
	}
	mappings := store.mappings["anime"]
	if len(mappings) != 1 || mappings[0].Ref.Anime != second {
		t.Fatalf("episode mapping = %#v, want fallback source", mappings)
	}
}

func TestCanceledStartupDoesNotLeavePartiallyInitializedService(t *testing.T) {
	service := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.ServiceStartup(ctx, application.ServiceOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("startup error = %v, want context.Canceled", err)
	}
	if _, _, err := service.begin(context.Background()); err != ErrUnavailable {
		t.Fatalf("begin after canceled startup = %v, want ErrUnavailable", err)
	}
	if err := service.ServiceShutdown(); err != nil {
		t.Fatalf("shutdown after canceled startup = %v", err)
	}
}

func TestAdmissionIsBoundedAndCancellationUnblocksShutdown(t *testing.T) {
	service, _, source := testService()
	started := make(chan struct{}, cap(service.operationSlots))
	source.catalog = func(ctx context.Context) ([]core.SourceAnime, error) {
		started <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	const operations = 16
	results := make(chan error, operations)
	for range operations {
		go func() {
			_, err := service.Catalog(context.Background())
			results <- err
		}()
	}
	for range operations {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("operation did not reach bounded source")
		}
	}
	if len(service.operationSlots) != operations {
		t.Fatalf("admitted slots = %d, want %d", len(service.operationSlots), operations)
	}
	if _, err := service.Catalog(context.Background()); err != ErrUnavailable {
		t.Fatalf("saturated admission error = %v, want ErrUnavailable", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := service.Catalog(ctx); err != ErrUnavailable {
		t.Fatalf("saturated timed admission error = %v, want ErrUnavailable", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.Catalog(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled admission error = %v, want cancellation", err)
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- service.ServiceShutdown() }()
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatalf("shutdown error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel admitted operations")
	}
	for range operations {
		if err := <-results; !errors.Is(err, context.Canceled) {
			t.Fatalf("admitted operation error = %v, want cancellation", err)
		}
	}
}

type scheduleResponseSource struct {
	items []core.SourceScheduleItem
}

func (source *scheduleResponseSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return nil, nil
}

func (*scheduleResponseSource) Search(context.Context, string) ([]core.SourceAnime, error) {
	return nil, nil
}

func (*scheduleResponseSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	return nil, nil
}

func (*scheduleResponseSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.NewPlaybackSource("https://media.example/video", nil), nil
}

func (source *scheduleResponseSource) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return source.items, nil
}

func TestScheduleRejectsMalformedResponsesBeforePersisting(t *testing.T) {
	from := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	anime := core.SourceAnime{Ref: core.SourceRef{Provider: "anime1", ID: "show"}, Title: "Show"}
	tests := []struct {
		name string
		item core.SourceScheduleItem
	}{
		{
			name: "before range",
			item: core.SourceScheduleItem{Anime: anime, AirsAt: from.Add(-time.Minute), Precision: core.SchedulePrecisionTime},
		},
		{
			name: "after range",
			item: core.SourceScheduleItem{Anime: anime, AirsAt: to.Add(time.Minute), Precision: core.SchedulePrecisionTime},
		},
		{
			name: "year out of range",
			item: core.SourceScheduleItem{Anime: anime, AirsAt: time.Date(10000, time.January, 1, 0, 0, 0, 0, time.UTC), Precision: core.SchedulePrecisionTime},
		},
		{
			name: "invalid precision",
			item: core.SourceScheduleItem{Anime: anime, AirsAt: from.Add(time.Hour), Precision: core.SchedulePrecision(99)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeStore()
			service := newWithDependencies(dependencies{store: store, source: &scheduleResponseSource{items: []core.SourceScheduleItem{test.item}}})
			t.Cleanup(func() { _ = service.ServiceShutdown() })
			_, err := service.Schedule(context.Background(), from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano))
			if err != ErrInvalidInput {
				t.Fatalf("schedule error = %v, want ErrInvalidInput", err)
			}
			if len(store.anime) != 0 || len(store.refs) != 0 || len(store.mappings) != 0 {
				t.Fatalf("malformed schedule persisted data: anime=%#v refs=%#v mappings=%#v", store.anime, store.refs, store.mappings)
			}
		})
	}
}

type closeTrackingStore struct {
	core.Store
	log   *[]string
	logMu *sync.Mutex
}

func (store *closeTrackingStore) Close() error {
	store.logMu.Lock()
	*store.log = append(*store.log, "store")
	store.logMu.Unlock()
	return nil
}

func TestShutdownClosesSessionBeforeDependenciesAndStore(t *testing.T) {
	base := newFakeStore()
	base.anime["anime"] = core.Anime{ID: "anime"}
	ref := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode"}
	base.mappings["anime"] = []core.EpisodeMapping{{AnimeID: "anime", EpisodeID: "episode", Ref: ref}}
	log := []string{}
	logMu := &sync.Mutex{}
	store := &closeTrackingStore{Store: base, log: &log, logMu: logMu}
	player := &playbackTestPlayer{}
	service := newWithDependencies(dependencies{
		store:     store,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: "Anime"}},
		newPlayer: func(string) (core.Player, error) { return player, nil },
		close: func() error {
			logMu.Lock()
			log = append(log, "dependencies")
			logMu.Unlock()
			return nil
		},
	})
	if err := service.Play(context.Background(), PlayRequest{AnimeID: "anime", EpisodeID: "episode"}); err != nil {
		t.Fatal(err)
	}
	player.mu.Lock()
	player.session.closeLog = &log
	player.session.logMu = logMu
	player.mu.Unlock()
	if err := service.ServiceShutdown(); err != nil {
		t.Fatal(err)
	}
	logMu.Lock()
	got := append([]string(nil), log...)
	logMu.Unlock()
	if len(got) != 3 || got[0] != "session" || got[1] != "dependencies" || got[2] != "store" {
		t.Fatalf("shutdown order = %#v", got)
	}
}

func TestConcurrentShutdownIsIdempotent(t *testing.T) {
	service, _, _ := testService()
	const callers = 8
	results := make(chan error, callers)
	for range callers {
		go func() { results <- service.ServiceShutdown() }()
	}
	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("concurrent shutdown error = %v", err)
		}
	}
	if _, _, err := service.begin(context.Background()); err != ErrClosed {
		t.Fatalf("begin after concurrent shutdown = %v", err)
	}
}

func TestRemoteAndStoreErrorsDoNotLeakDetails(t *testing.T) {
	service, store, source := testService()
	source.catalog = func(context.Context) ([]core.SourceAnime, error) {
		return nil, errors.New("remote token https://secret.invalid/path")
	}
	err := func() error {
		_, callErr := service.Catalog(context.Background())
		return callErr
	}()
	if err != ErrUnavailable || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "https") {
		t.Fatalf("remote error = %v", err)
	}
	store.setErr = errors.New("sqlite secret https://secret.invalid/path")
	err = service.SaveSettings(context.Background(), Settings{Appearance: "dark"})
	if err != ErrUnavailable || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "https") {
		t.Fatalf("store error = %v", err)
	}
}

func TestDTOJSONContainsOnlyExplicitSafeFields(t *testing.T) {
	payload, err := json.Marshal(struct {
		Anime    Anime
		Detail   Detail
		Schedule ScheduleItem
		Play     PlayRequest
	}{
		Anime:    Anime{ID: "anime", Title: "Title"},
		Detail:   Detail{Anime: Anime{ID: "anime", Title: "Title"}},
		Schedule: ScheduleItem{AnimeID: "anime", EpisodeID: "episode"},
		Play:     PlayRequest{AnimeID: "anime", EpisodeID: "episode"},
	})
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(payload)
	for _, forbidden := range []string{"Provider", "provider", "CoverURL", "coverUrl", "PlaybackSource", "password", "token"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("unsafe field %q in DTO JSON: %s", forbidden, serialized)
		}
	}
}
