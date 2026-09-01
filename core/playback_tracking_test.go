package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type manualPlaybackTicker struct {
	ticks chan time.Time
	once  sync.Once
	done  chan struct{}
}

func newManualPlaybackTicker() *manualPlaybackTicker {
	return &manualPlaybackTicker{ticks: make(chan time.Time, 8), done: make(chan struct{})}
}

func (ticker *manualPlaybackTicker) C() <-chan time.Time { return ticker.ticks }
func (ticker *manualPlaybackTicker) Stop() {
	ticker.once.Do(func() { close(ticker.done) })
}

type trackingRawSession struct {
	mu          sync.Mutex
	events      chan PlaybackEvent
	snapshot    PlaybackSnapshot
	snapshotErr error
	snapshotFn  func(context.Context) (PlaybackSnapshot, error)
	loads       []PlayRequest
	loadErr     error
	closeCount  int
	closeErr    error
	closed      bool
	onSnapshot  func(PlaybackSnapshot)
	onLoad      func(PlayRequest)
}

func newTrackingRawSession() *trackingRawSession {
	return &trackingRawSession{events: make(chan PlaybackEvent, 256)}
}

func (session *trackingRawSession) Load(ctx context.Context, request PlayRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	session.mu.Lock()
	session.loads = append(session.loads, request)
	err := session.loadErr
	hook := session.onLoad
	if err == nil {
		session.snapshot = PlaybackSnapshot{Position: request.StartAt}
	}
	session.mu.Unlock()
	if hook != nil {
		hook(request)
	}
	return err
}

func (session *trackingRawSession) Events() <-chan PlaybackEvent { return session.events }

func (session *trackingRawSession) Snapshot(ctx context.Context) (PlaybackSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return PlaybackSnapshot{}, err
	}
	session.mu.Lock()
	snapshot := session.snapshot
	err := session.snapshotErr
	callback := session.snapshotFn
	hook := session.onSnapshot
	session.mu.Unlock()
	if callback != nil {
		return callback(ctx)
	}
	if hook != nil {
		hook(snapshot)
	}
	return snapshot, err
}

func (session *trackingRawSession) Close() error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if !session.closed {
		session.closed = true
		session.closeCount++
	}
	return session.closeErr
}

func (session *trackingRawSession) emit(event PlaybackEvent) {
	session.mu.Lock()
	session.snapshot.Position = event.Position
	session.snapshot.Duration = event.Duration
	session.snapshot.Paused = event.Kind == PlaybackEventPaused
	session.mu.Unlock()
	session.events <- event
}

func (session *trackingRawSession) setSnapshot(snapshot PlaybackSnapshot) {
	session.mu.Lock()
	session.snapshot = snapshot
	session.mu.Unlock()
}

func (session *trackingRawSession) loadCount() int {
	session.mu.Lock()
	defer session.mu.Unlock()
	return len(session.loads)
}

func (session *trackingRawSession) closes() int {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.closeCount
}

type trackingStore struct {
	*fakeStore
	mu          sync.Mutex
	checkpoints []HistoryEntry
	err         error
	onSave      func(HistoryEntry)
}

func newTrackingStore() *trackingStore {
	return &trackingStore{fakeStore: &fakeStore{}}
}

func (store *trackingStore) SavePlaybackCheckpoint(_ context.Context, entry HistoryEntry) error {
	store.mu.Lock()
	store.checkpoints = append(store.checkpoints, entry)
	err := store.err
	hook := store.onSave
	store.mu.Unlock()
	if hook != nil {
		hook(entry)
	}
	return err
}

func (store *trackingStore) entries() []HistoryEntry {
	store.mu.Lock()
	defer store.mu.Unlock()
	return append([]HistoryEntry(nil), store.checkpoints...)
}

func newTrackingTestApp(source AnimeSource, raw *trackingRawSession, store *trackingStore, ticker *manualPlaybackTicker) *App {
	app := NewApp(source, &fakePlayer{session: raw}, store)
	app.newTicker = func(time.Duration) playbackTicker { return ticker }
	app.checkpointInterval = time.Hour
	app.checkpointTimeout = time.Second
	return app
}

func requireTrackingEventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition timed out")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestAppResumePolicyAndAnimePreflight(t *testing.T) {
	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "episode"}
	progress := PlaybackProgress{
		AnimeID: "anime", EpisodeID: "episode", Position: 7 * time.Minute,
		Duration: 24 * time.Minute, UpdatedAt: time.Now(),
	}
	tests := []struct {
		name       string
		startAt    time.Duration
		settings   Settings
		progress   PlaybackProgress
		animeErr   error
		wantStart  time.Duration
		wantErr    error
		wantSource bool
	}{
		{name: "enabled resume", settings: Settings{ResumePlayback: ToggleEnabled}, progress: progress, wantStart: progress.Position, wantSource: true},
		{name: "explicit override", startAt: time.Minute, settings: Settings{ResumePlayback: ToggleEnabled}, progress: progress, wantStart: time.Minute, wantSource: true},
		{name: "disabled", settings: Settings{ResumePlayback: ToggleDisabled}, progress: progress, wantSource: true},
		{name: "unspecified", progress: progress, wantSource: true},
		{name: "completed", settings: Settings{ResumePlayback: ToggleEnabled}, progress: func() PlaybackProgress { value := progress; value.Completed = true; return value }(), wantSource: true},
		{name: "threshold", settings: Settings{ResumePlayback: ToggleEnabled}, progress: func() PlaybackProgress { value := progress; value.Position = 22 * time.Minute; return value }(), wantSource: true},
		{name: "negative", startAt: -time.Nanosecond, wantErr: ErrInvalidPlayback},
		{name: "missing anime", animeErr: ErrNotFound, wantErr: ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := newTrackingRawSession()
			store := newTrackingStore()
			store.settings = test.settings
			store.progress = test.progress
			store.animeErr = test.animeErr
			source := &fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}
			app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
			session, err := app.PlayEpisode(context.Background(), "anime", "episode", ref, test.startAt)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if source.called != test.wantSource {
				t.Fatalf("source called = %t", source.called)
			}
			if err == nil {
				if got := raw.loads; got != nil {
					t.Fatalf("raw loads before start = %#v", got)
				}
				if playerRequest := app.player.(*fakePlayer).request; playerRequest.StartAt != test.wantStart {
					t.Fatalf("startAt = %s, want %s", playerRequest.StartAt, test.wantStart)
				}
				if err := session.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestTrackedSessionThrottlesProgressAndPersistsPause(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	ticker := newManualPlaybackTicker()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, ticker)
	session, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 100; index++ {
		raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress, Position: time.Duration(index) * time.Second, Duration: 24 * time.Minute})
	}
	time.Sleep(20 * time.Millisecond)
	if got := len(store.entries()); got != 0 {
		t.Fatalf("progress writes before tick = %d", got)
	}
	ticker.ticks <- time.Now()
	requireTrackingEventually(t, func() bool { return len(store.entries()) == 1 })
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventPaused, Position: 101 * time.Second, Duration: 24 * time.Minute})
	requireTrackingEventually(t, func() bool { return len(store.entries()) == 2 })
	entries := store.entries()
	if entries[0].Progress.Position != 100*time.Second || entries[1].Progress.Position != 101*time.Second {
		t.Fatalf("checkpoint positions = %#v", entries)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ticker.done:
	case <-time.After(time.Second):
		t.Fatal("ticker was not stopped")
	}
}

func TestAppSwitchUsesDoubleCheckpointBeforeLoad(t *testing.T) {
	raw := newTrackingRawSession()
	raw.setSnapshot(PlaybackSnapshot{Position: 10 * time.Second, Duration: 24 * time.Minute})
	store := newTrackingStore()
	var callsMu sync.Mutex
	calls := []string{}
	appendCall := func(value string) {
		callsMu.Lock()
		calls = append(calls, value)
		callsMu.Unlock()
	}
	raw.onSnapshot = func(snapshot PlaybackSnapshot) { appendCall("snapshot:" + snapshot.Position.String()) }
	raw.onLoad = func(PlayRequest) { appendCall("load") }
	store.onSave = func(entry HistoryEntry) { appendCall("save:" + entry.Progress.Position.String()) }
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/two", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	source.resolve = func(context.Context, EpisodeRef) (PlaybackSource, error) {
		appendCall("resolve")
		raw.setSnapshot(PlaybackSnapshot{Position: 20 * time.Second, Duration: 24 * time.Minute})
		return source.resolved, nil
	}
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0); err != nil {
		t.Fatal(err)
	}
	callsMu.Lock()
	actual := append([]string(nil), calls...)
	callsMu.Unlock()
	want := []string{"snapshot:10s", "save:10s", "resolve", "snapshot:20s", "save:20s", "load"}
	if len(actual) != len(want) {
		t.Fatalf("switch calls = %#v, want %#v", actual, want)
	}
	for index := range want {
		if actual[index] != want[index] {
			t.Fatalf("switch calls = %#v, want %#v", actual, want)
		}
	}
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchPersistenceFailurePreventsResolveAndLoad(t *testing.T) {
	expected := errors.New("checkpoint failed")
	raw := newTrackingRawSession()
	raw.setSnapshot(PlaybackSnapshot{Position: time.Second})
	store := newTrackingStore()
	store.err = expected
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/two", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0)
	if !errors.Is(err, expected) || source.resolveRef.ID == "two" || raw.loadCount() != 0 {
		t.Fatalf("switch error = %v, resolved = %#v, loads = %d", err, source.resolveRef, raw.loadCount())
	}
	store.err = nil
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedTerminalSurvivesBackpressureAndCompletes(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 100; index++ {
		raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress, Position: time.Duration(index) * time.Second, Duration: 100 * time.Second})
	}
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventEnded, Position: 100 * time.Second, Duration: 100 * time.Second})
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-started.Events():
			if event.Kind == PlaybackEventEnded {
				goto terminalReceived
			}
		case <-deadline:
			t.Fatal("terminal event was dropped")
		}
	}

terminalReceived:
	requireTrackingEventually(t, func() bool {
		entries := store.entries()
		return len(entries) > 0 && entries[len(entries)-1].Progress.Completed
	})
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedCloseIsConcurrentIdempotentAndAlwaysCleansRaw(t *testing.T) {
	expected := errors.New("persist failed")
	raw := newTrackingRawSession()
	raw.setSnapshot(PlaybackSnapshot{Position: time.Minute, Duration: 24 * time.Minute})
	store := newTrackingStore()
	store.err = expected
	ticker := newManualPlaybackTicker()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, ticker)
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 16)
	var group sync.WaitGroup
	for range 16 {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- started.Close()
		}()
	}
	group.Wait()
	close(results)
	for err := range results {
		if !errors.Is(err, expected) {
			t.Fatalf("close error = %v", err)
		}
	}
	if raw.closes() != 1 {
		t.Fatalf("raw close count = %d", raw.closes())
	}
	select {
	case <-ticker.done:
	case <-time.After(time.Second):
		t.Fatal("ticker was not stopped")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-started.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("events did not close")
		}
	}
}

func TestTrackedStagesTargetTerminalEmittedInsideLoad(t *testing.T) {
	raw := newTrackingRawSession()
	raw.setSnapshot(PlaybackSnapshot{Position: 10 * time.Second, Duration: 24 * time.Minute})
	store := newTrackingStore()
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw.onLoad = func(request PlayRequest) {
		raw.emit(PlaybackEvent{
			AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: PlaybackEventEnded,
			Position: 24 * time.Minute, Duration: 24 * time.Minute,
		})
		requireTrackingEventually(t, func() bool {
			tracked := started.(*trackedPlaybackSession)
			tracked.stateMu.Lock()
			defer tracked.stateMu.Unlock()
			return tracked.state.pending != nil && tracked.state.pending.terminal
		})
	}
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0); err != nil {
		t.Fatal(err)
	}
	requireTrackingEventually(t, func() bool {
		for _, entry := range store.entries() {
			if entry.Progress.EpisodeID == "episode-2" && entry.Progress.Completed {
				return true
			}
		}
		return false
	})
	raw.onLoad = nil
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedStagesTerminalWhenReloadingSameEpisode(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw.onLoad = func(request PlayRequest) {
		raw.emit(PlaybackEvent{
			AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: PlaybackEventEnded,
			Position: 24 * time.Minute, Duration: 24 * time.Minute,
		})
		requireTrackingEventually(t, func() bool {
			tracked := started.(*trackedPlaybackSession)
			tracked.stateMu.Lock()
			defer tracked.stateMu.Unlock()
			return tracked.state.pending != nil && tracked.state.pending.terminal
		})
	}
	if err := started.Load(context.Background(), PlayRequest{
		AnimeID: "anime", EpisodeID: "episode", Source: NewPlaybackSource("https://media.example/video", nil),
	}); err != nil {
		t.Fatal(err)
	}
	requireTrackingEventually(t, func() bool {
		for _, entry := range store.entries() {
			if entry.Progress.Completed {
				return true
			}
		}
		return false
	})
	raw.onLoad = nil
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedTerminalCheckpointRemainsFinalWhenPropertiesDisappear(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode-1", Kind: PlaybackEventEnded, Position: 24 * time.Minute, Duration: 24 * time.Minute})
	requireTrackingEventually(t, func() bool { return len(store.entries()) == 1 })
	raw.mu.Lock()
	raw.snapshotErr = errors.New("property unavailable")
	raw.mu.Unlock()
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0); err != nil {
		t.Fatalf("switch after terminal = %v", err)
	}
	raw.mu.Lock()
	raw.snapshotErr = nil
	raw.mu.Unlock()
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedSnapshotInFlightPreservesConcurrentTerminalState(t *testing.T) {
	tests := []struct {
		name        string
		rawSnapshot PlaybackSnapshot
		rawErr      error
	}{
		{name: "stale snapshot", rawSnapshot: PlaybackSnapshot{}},
		{name: "property unavailable", rawErr: errors.New("property unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := newTrackingRawSession()
			store := newTrackingStore()
			app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
			started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
			if err != nil {
				t.Fatal(err)
			}
			entered := make(chan struct{})
			release := make(chan struct{})
			raw.mu.Lock()
			raw.snapshotFn = func(context.Context) (PlaybackSnapshot, error) {
				close(entered)
				<-release
				return test.rawSnapshot, test.rawErr
			}
			raw.mu.Unlock()
			result := make(chan struct {
				snapshot PlaybackSnapshot
				err      error
			}, 1)
			go func() {
				snapshot, snapshotErr := started.(PlaybackSnapshotter).Snapshot(context.Background())
				result <- struct {
					snapshot PlaybackSnapshot
					err      error
				}{snapshot: snapshot, err: snapshotErr}
			}()
			<-entered
			raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventEnded, Position: 24 * time.Minute, Duration: 24 * time.Minute})
			requireTrackingEventually(t, func() bool { return len(store.entries()) == 1 })
			close(release)
			actual := <-result
			if actual.err != nil || actual.snapshot.Position != 24*time.Minute || actual.snapshot.Duration != 24*time.Minute {
				t.Fatalf("snapshot = %+v, error = %v", actual.snapshot, actual.err)
			}
			raw.mu.Lock()
			raw.snapshotFn = nil
			raw.mu.Unlock()
			if err := started.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTrackedIgnoresEventsAfterTerminalCheckpoint(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventEnded, Position: 24 * time.Minute, Duration: 24 * time.Minute})
	requireTrackingEventually(t, func() bool { return len(store.entries()) == 1 })
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress})
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
	entries := store.entries()
	if len(entries) != 1 || entries[0].Progress.Position != 24*time.Minute || entries[0].Progress.Duration != 24*time.Minute || !entries[0].Progress.Completed {
		t.Fatalf("terminal checkpoint regressed: %#v", entries)
	}
}

func TestTrackedLoadRejectsUnknownAnimeBeforeRawLoad(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	store.animeErr = ErrNotFound
	err = started.Load(context.Background(), PlayRequest{
		AnimeID: "missing", EpisodeID: "episode-2", Source: NewPlaybackSource("https://media.example/two", nil),
	})
	if !errors.Is(err, ErrNotFound) || raw.loadCount() != 0 {
		t.Fatalf("load error = %v, raw loads = %d", err, raw.loadCount())
	}
	store.animeErr = nil
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedCloseCancelsInflightSnapshotBeforeCleanup(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	app.checkpointTimeout = 50 * time.Millisecond
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	var once sync.Once
	raw.mu.Lock()
	raw.snapshotFn = func(ctx context.Context) (PlaybackSnapshot, error) {
		once.Do(func() { close(entered) })
		<-ctx.Done()
		return PlaybackSnapshot{}, ctx.Err()
	}
	raw.mu.Unlock()
	snapshotDone := make(chan error, 1)
	go func() {
		_, snapshotErr := started.(PlaybackSnapshotter).Snapshot(context.Background())
		snapshotDone <- snapshotErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("snapshot did not start")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- started.Close() }()
	select {
	case closeErr := <-closeDone:
		if !errors.Is(closeErr, ErrPlaybackTracking) {
			t.Fatalf("close error = %v", closeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not reach cleanup")
	}
	if raw.closes() != 1 {
		t.Fatalf("raw close count = %d", raw.closes())
	}
	select {
	case snapshotErr := <-snapshotDone:
		if snapshotErr == nil {
			t.Fatal("snapshot unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot did not cancel")
	}
}

func TestTrackedCloseWaitsForOwnedRunAfterCheckpointTimeout(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	checkpointEntered := make(chan struct{})
	releaseCheckpoint := make(chan struct{})
	var once sync.Once
	store.onSave = func(HistoryEntry) {
		once.Do(func() {
			close(checkpointEntered)
			<-releaseCheckpoint
		})
	}
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	app.checkpointTimeout = 10 * time.Millisecond
	started, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventPaused, Position: time.Minute, Duration: 24 * time.Minute})
	select {
	case <-checkpointEntered:
	case <-time.After(time.Second):
		t.Fatal("checkpoint did not start")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- started.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("close returned before owned run stopped: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if raw.closes() != 1 {
		t.Fatalf("raw close count before blocked checkpoint release = %d", raw.closes())
	}
	close(releaseCheckpoint)
	select {
	case err := <-closeDone:
		if !errors.Is(err, ErrPlaybackTracking) {
			t.Fatalf("close error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not finish")
	}
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-started.Events():
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("events did not close")
		}
	}
}

func TestTrackedFailedLoadDiscardsStagedEvents(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	app := newTrackingTestApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	expected := errors.New("load failed")
	raw.loadErr = expected
	raw.onLoad = func(request PlayRequest) {
		raw.emit(PlaybackEvent{AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: PlaybackEventEnded})
	}
	err = started.Load(context.Background(), PlayRequest{
		AnimeID: "anime", EpisodeID: "episode-2", Source: NewPlaybackSource("https://media.example/two", nil),
	})
	if !errors.Is(err, expected) {
		t.Fatalf("load error = %v", err)
	}
	select {
	case event := <-started.Events():
		t.Fatalf("failed target event leaked: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	raw.loadErr = nil
	raw.onLoad = nil
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedRelayPreservesFirstTerminalAndBoundaryOrder(t *testing.T) {
	stop := make(chan struct{})
	relay := newTrackedEventRelay(stop)
	progress := PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress, Position: time.Second}
	ended := PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventEnded, Position: 2 * time.Second}
	failed := PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventFailed, Position: 2 * time.Second}
	relay.publish(progress, 1)
	relay.publish(ended, 1)
	relay.publish(failed, 1)
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-relay.events:
			if isTerminalPlaybackEvent(event.Kind) {
				if event.Kind != PlaybackEventEnded {
					t.Fatalf("terminal = %+v", event)
				}
				goto firstTerminal
			}
		case <-deadline:
			t.Fatal("first terminal was not delivered")
		}
	}

firstTerminal:
	relay.publish(progress, 1)
	relay.publish(PlaybackEvent{AnimeID: "anime", EpisodeID: "next", Kind: PlaybackEventStopped}, 2)
	select {
	case event := <-relay.events:
		if event.Kind != PlaybackEventStopped || event.EpisodeID != "next" {
			t.Fatalf("next terminal = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("next owner terminal was not delivered")
	}
	close(stop)
	relay.shutdown()
}

func TestTrackedRelayDropsQueuedOldOwnerEventsBeforeTargetTerminal(t *testing.T) {
	stop := make(chan struct{})
	relay := newTrackedEventRelay(stop)
	oldProgress := PlaybackEvent{AnimeID: "anime", EpisodeID: "old", Kind: PlaybackEventProgress, Position: time.Second}
	relay.publish(oldProgress, 1)
	requireTrackingEventually(t, func() bool { return len(relay.events) == 1 })
	relay.publish(PlaybackEvent{AnimeID: "anime", EpisodeID: "old", Kind: PlaybackEventPaused, Position: time.Second}, 1)
	relay.publish(PlaybackEvent{AnimeID: "anime", EpisodeID: "target", Kind: PlaybackEventEnded, Position: 2 * time.Second}, 2)
	if event := <-relay.events; event != oldProgress {
		t.Fatalf("first event = %+v", event)
	}
	select {
	case event := <-relay.events:
		if event.Kind != PlaybackEventEnded || event.EpisodeID != "target" {
			t.Fatalf("target terminal = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("target terminal was not delivered")
	}
	select {
	case event := <-relay.events:
		t.Fatalf("old owner event delivered after target terminal: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	close(stop)
	relay.shutdown()
}

func TestTrackedRelayDoesNotCoalesceAcrossPause(t *testing.T) {
	stop := make(chan struct{})
	relay := newTrackedEventRelay(stop)
	events := []PlaybackEvent{
		{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress, Position: 10 * time.Second},
		{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventPaused, Position: 10 * time.Second},
		{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress, Position: 11 * time.Second},
	}
	for _, event := range events {
		relay.publish(event, 1)
	}
	for index, expected := range events {
		select {
		case actual := <-relay.events:
			if actual.Kind != expected.Kind || actual.Position != expected.Position {
				t.Fatalf("event %d = %+v, want %+v", index, actual, expected)
			}
		case <-time.After(time.Second):
			t.Fatalf("event %d timed out", index)
		}
	}
	close(stop)
	relay.shutdown()
}
