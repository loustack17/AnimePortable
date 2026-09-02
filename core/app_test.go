// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeSource struct {
	context     context.Context
	calls       *[]string
	searchQuery string
	searchItems []SourceAnime
	searchErr   error
	resolveRef  EpisodeRef
	resolved    PlaybackSource
	resolveErr  error
	resolve     func(context.Context, EpisodeRef) (PlaybackSource, error)
	episodes    []SourceEpisode
	episodesErr error
	episodesFn  func(context.Context, SourceRef) ([]SourceEpisode, error)
	called      bool
}

func (source *fakeSource) Catalog(context.Context) ([]SourceAnime, error) {
	source.called = true
	return nil, source.searchErr
}

func (source *fakeSource) Search(ctx context.Context, query string) ([]SourceAnime, error) {
	source.called = true
	source.context = ctx
	source.searchQuery = query
	return source.searchItems, source.searchErr
}

func (source *fakeSource) Episodes(ctx context.Context, ref SourceRef) ([]SourceEpisode, error) {
	source.called = true
	source.context = ctx
	if source.episodesFn != nil {
		return source.episodesFn(ctx, ref)
	}
	return source.episodes, source.episodesErr
}

func (source *fakeSource) Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error) {
	source.called = true
	source.context = ctx
	source.resolveRef = ref
	if source.calls != nil {
		*source.calls = append(*source.calls, "resolve")
	}
	if source.resolve != nil {
		return source.resolve(ctx, ref)
	}
	return source.resolved, source.resolveErr
}

func (source *fakeSource) Schedule(context.Context, ScheduleQuery) ([]SourceScheduleItem, error) {
	source.called = true
	return nil, source.searchErr
}

type fakePlayer struct {
	context context.Context
	calls   *[]string
	request PlayRequest
	session PlaybackSession
	err     error
	started bool
}

func (player *fakePlayer) Start(ctx context.Context, request PlayRequest) (PlaybackSession, error) {
	player.started = true
	player.context = ctx
	player.request = request
	if player.calls != nil {
		*player.calls = append(*player.calls, "start")
	}
	return player.session, player.err
}

type fakeMetadataProvider struct{}

func (*fakeMetadataProvider) Search(context.Context, MetadataQuery) ([]MetadataCandidate, error) {
	return nil, nil
}

func (*fakeMetadataProvider) Get(context.Context, MetadataRef) (AnimeMetadata, error) {
	return AnimeMetadata{}, nil
}

type fakeSession struct {
	events   chan PlaybackEvent
	context  context.Context
	request  PlayRequest
	snapshot PlaybackSnapshot
	loads    int
	err      error
}

func (session *fakeSession) Load(ctx context.Context, request PlayRequest) error {
	session.context = ctx
	session.request = request
	session.snapshot.Position = request.StartAt
	session.loads++
	return session.err
}

func (session *fakeSession) Events() <-chan PlaybackEvent {
	return session.events
}

func (session *fakeSession) Snapshot(ctx context.Context) (PlaybackSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return PlaybackSnapshot{}, err
	}
	return session.snapshot, nil
}

func (*fakeSession) Close() error {
	return nil
}

func trackFakeSession(t *testing.T, session *fakeSession, store *fakeStore) *trackedPlaybackSession {
	t.Helper()
	tracked, err := newTrackedPlaybackSession(session, store, PlayRequest{AnimeID: "old-anime", EpisodeID: "old-episode"}, playbackTrackingConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tracked.Close() })
	return tracked
}

type fakeStore struct {
	context            context.Context
	anime              []Anime
	animeErr           error
	err                error
	settings           Settings
	settingsErr        error
	progress           PlaybackProgress
	progressErr        error
	checkpoints        []HistoryEntry
	following          []AnimeID
	followingErr       error
	setFollowingErr    error
	sourceRefs         map[AnimeID][]SourceRef
	sourceRefsErr      error
	saveSourceRefErr   error
	savedSourceRefs    []savedSourceRef
	episodeMappings    map[AnimeID][]EpisodeMapping
	episodeMappingsErr error
	saveMappingErr     error
	savedMappings      []EpisodeMapping
	calls              *[]string
	history            []HistoryEntry
	historyErr         error
}

type savedSourceRef struct {
	animeID AnimeID
	ref     SourceRef
}

func (*fakeStore) SaveAnime(context.Context, Anime) error {
	return nil
}

func (store *fakeStore) Anime(ctx context.Context, id AnimeID) (Anime, error) {
	store.context = ctx
	if store.animeErr != nil {
		return Anime{}, store.animeErr
	}
	return Anime{ID: id}, nil
}

func (store *fakeStore) ListAnime(ctx context.Context) ([]Anime, error) {
	store.context = ctx
	return store.anime, store.err
}

func (store *fakeStore) SaveSourceRef(ctx context.Context, animeID AnimeID, ref SourceRef) error {
	store.context = ctx
	if store.saveSourceRefErr != nil {
		return store.saveSourceRefErr
	}
	if store.calls != nil {
		*store.calls = append(*store.calls, "save-source-ref")
	}
	store.savedSourceRefs = append(store.savedSourceRefs, savedSourceRef{animeID: animeID, ref: ref})
	return nil
}

func (store *fakeStore) SourceRefs(ctx context.Context, id AnimeID) ([]SourceRef, error) {
	store.context = ctx
	if store.sourceRefsErr != nil {
		return nil, store.sourceRefsErr
	}
	return store.sourceRefs[id], nil
}

func (store *fakeStore) SaveEpisodeMapping(ctx context.Context, mapping EpisodeMapping) error {
	store.context = ctx
	if store.saveMappingErr != nil {
		return store.saveMappingErr
	}
	if store.calls != nil {
		*store.calls = append(*store.calls, "save-mapping")
	}
	store.savedMappings = append(store.savedMappings, mapping)
	return nil
}

func (store *fakeStore) EpisodeMappings(ctx context.Context, id AnimeID) ([]EpisodeMapping, error) {
	store.context = ctx
	if store.episodeMappingsErr != nil {
		return nil, store.episodeMappingsErr
	}
	return store.episodeMappings[id], nil
}

func (*fakeStore) SaveMetadata(context.Context, AnimeID, AnimeMetadata) error {
	return nil
}

func (*fakeStore) Metadata(context.Context, AnimeID) (AnimeMetadata, error) {
	return AnimeMetadata{}, ErrNotFound
}

func (store *fakeStore) SetFollowing(ctx context.Context, id AnimeID, following bool) error {
	store.context = ctx
	if store.setFollowingErr != nil {
		return store.setFollowingErr
	}
	for index, existing := range store.following {
		if existing != id {
			continue
		}
		if following {
			return nil
		}
		store.following = append(store.following[:index], store.following[index+1:]...)
		return nil
	}
	if following {
		store.following = append(store.following, id)
	}
	return nil
}

func (store *fakeStore) Following(ctx context.Context) ([]AnimeID, error) {
	store.context = ctx
	if store.followingErr != nil {
		return nil, store.followingErr
	}
	return store.following, nil
}

func (*fakeStore) AddHistory(context.Context, HistoryEntry) error {
	return nil
}

func (store *fakeStore) History(ctx context.Context) ([]HistoryEntry, error) {
	store.context = ctx
	if store.historyErr != nil {
		return nil, store.historyErr
	}
	return store.history, nil
}

func (*fakeStore) RemoveHistory(context.Context, AnimeID) error {
	return nil
}

func (*fakeStore) SaveProgress(context.Context, PlaybackProgress) error {
	return nil
}

func (store *fakeStore) SavePlaybackCheckpoint(_ context.Context, entry HistoryEntry) error {
	store.checkpoints = append(store.checkpoints, entry)
	return nil
}

func (store *fakeStore) Progress(context.Context, AnimeID, EpisodeID) (PlaybackProgress, error) {
	if store.progressErr != nil {
		return PlaybackProgress{}, store.progressErr
	}
	if store.progress.AnimeID == "" {
		return PlaybackProgress{}, ErrNotFound
	}
	return store.progress, nil
}

func (*fakeStore) SaveSettings(context.Context, Settings) error {
	return nil
}

func (store *fakeStore) Settings(context.Context) (Settings, error) {
	if store.settingsErr != nil {
		return Settings{}, store.settingsErr
	}
	return store.settings, nil
}

var (
	_ AnimeSource      = (*fakeSource)(nil)
	_ MetadataProvider = (*fakeMetadataProvider)(nil)
	_ Player           = (*fakePlayer)(nil)
	_ PlaybackSession  = (*fakeSession)(nil)
	_ Store            = (*fakeStore)(nil)
)

func TestAppSearchUsesSource(t *testing.T) {
	expected := []SourceAnime{{Ref: SourceRef{Provider: "source", ID: "anime"}, Title: "Anime"}}
	source := &fakeSource{searchItems: expected}
	app := NewApp(source, &fakePlayer{}, &fakeStore{})
	ctx := context.WithValue(context.Background(), contextKey{}, "search")

	actual, err := app.Search(ctx, "query")
	if err != nil {
		t.Fatal(err)
	}
	if source.searchQuery != "query" {
		t.Fatalf("query = %q", source.searchQuery)
	}
	if source.context != ctx {
		t.Fatal("search context was not forwarded")
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("results = %#v", actual)
	}
}

func TestAppPlayEpisodeResolvesBeforeStartingPlayer(t *testing.T) {
	headers := http.Header{"Referer": {"https://source.example/"}}
	resolved := NewPlaybackSource("https://media.example/episode.m3u8?token=secret", headers)
	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "episode"}
	session := &fakeSession{events: make(chan PlaybackEvent)}
	calls := []string{}
	source := &fakeSource{resolved: resolved, calls: &calls}
	player := &fakePlayer{session: session, calls: &calls}
	store := &fakeStore{calls: &calls}
	app := NewApp(source, player, store)
	ctx := context.WithValue(context.Background(), contextKey{}, "play")

	actual, err := app.PlayEpisode(ctx, AnimeID("local-anime"), EpisodeID("local-episode"), ref, 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	tracked, ok := actual.(*trackedPlaybackSession)
	if !ok || tracked.raw != session {
		t.Fatalf("unexpected playback session %T", actual)
	}
	if source.resolveRef != ref {
		t.Fatalf("resolved ref = %#v", source.resolveRef)
	}
	if !reflect.DeepEqual(calls, []string{"resolve", "save-source-ref", "save-mapping", "start"}) {
		t.Fatalf("playback calls = %#v", calls)
	}
	if !reflect.DeepEqual(store.savedSourceRefs, []savedSourceRef{{animeID: "local-anime", ref: ref.Anime}}) {
		t.Fatalf("saved source refs = %#v", store.savedSourceRefs)
	}
	if !reflect.DeepEqual(store.savedMappings, []EpisodeMapping{{AnimeID: "local-anime", EpisodeID: "local-episode", Ref: ref}}) {
		t.Fatalf("saved mappings = %#v", store.savedMappings)
	}
	if source.context != ctx || player.context != ctx {
		t.Fatal("playback context was not forwarded")
	}
	if player.request.AnimeID != AnimeID("local-anime") || player.request.EpisodeID != EpisodeID("local-episode") {
		t.Fatalf("canonical IDs = %q, %q", player.request.AnimeID, player.request.EpisodeID)
	}
	if player.request.StartAt != 90*time.Second {
		t.Fatalf("start position = %s", player.request.StartAt)
	}
	if player.request.Source.URL() != resolved.URL() || !reflect.DeepEqual(player.request.Source.Headers(), resolved.Headers()) {
		t.Fatal("resolved playback source was not forwarded")
	}
}

func TestAppPlayEpisodeDoesNotStartPlayerWhenResolveFails(t *testing.T) {
	expected := errors.New("resolve failed")
	player := &fakePlayer{}
	store := &fakeStore{}
	app := NewApp(&fakeSource{resolveErr: expected}, player, store)

	_, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{}, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if player.started {
		t.Fatal("player started after resolution failure")
	}
	if len(store.savedSourceRefs) != 0 || len(store.savedMappings) != 0 {
		t.Fatalf("stores changed after resolution failure: refs=%#v mappings=%#v", store.savedSourceRefs, store.savedMappings)
	}
}

func TestAppPlayEpisodeDoesNotStartPlayerWhenMappingFails(t *testing.T) {
	expected := errors.New("mapping failed")
	player := &fakePlayer{}
	store := &fakeStore{saveMappingErr: expected}
	app := NewApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}, player, store)

	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "provider-episode"}
	_, err := app.PlayEpisode(context.Background(), "anime", "episode", ref, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if player.started {
		t.Fatal("player started after mapping failure")
	}
	if len(store.savedMappings) != 0 {
		t.Fatalf("saved mappings after failure = %#v", store.savedMappings)
	}
	if len(store.savedSourceRefs) != 1 || store.savedSourceRefs[0] != (savedSourceRef{animeID: "anime", ref: ref.Anime}) {
		t.Fatalf("saved source refs after mapping failure = %#v", store.savedSourceRefs)
	}
}

func TestAppPlayEpisodeDoesNotSaveMappingOrStartWhenSourceRefFails(t *testing.T) {
	expected := errors.New("source ref conflict")
	player := &fakePlayer{}
	store := &fakeStore{saveSourceRefErr: expected}
	app := NewApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}, player, store)
	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "provider-episode"}

	_, err := app.PlayEpisode(context.Background(), "anime", "episode", ref, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if player.started {
		t.Fatal("player started after source ref failure")
	}
	if len(store.savedMappings) != 0 {
		t.Fatalf("saved mappings after source ref failure = %#v", store.savedMappings)
	}
}

func TestAppSwitchEpisodeResolvesAndLoadsCanonicalRequest(t *testing.T) {
	headers := http.Header{"Referer": {"https://source.example/"}}
	resolved := NewPlaybackSource("https://media.example/episode.m3u8?token=secret", headers)
	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "episode"}
	session := &fakeSession{events: make(chan PlaybackEvent)}
	source := &fakeSource{resolved: resolved}
	store := &fakeStore{}
	app := NewApp(source, &fakePlayer{}, store)
	ctx := context.WithValue(context.Background(), contextKey{}, "switch")

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), AnimeID("local-anime"), EpisodeID("local-episode"), ref, 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if source.resolveRef != ref {
		t.Fatalf("resolved ref = %#v", source.resolveRef)
	}
	if session.context == nil || session.context.Value(contextKey{}) != "switch" {
		t.Fatal("switch context was not forwarded")
	}
	if session.loads != 1 {
		t.Fatalf("load calls = %d", session.loads)
	}
	if session.request.AnimeID != AnimeID("local-anime") || session.request.EpisodeID != EpisodeID("local-episode") {
		t.Fatalf("canonical IDs = %q, %q", session.request.AnimeID, session.request.EpisodeID)
	}
	if session.request.StartAt != 90*time.Second {
		t.Fatalf("start position = %s", session.request.StartAt)
	}
	if session.request.Source.URL() != resolved.URL() || !reflect.DeepEqual(session.request.Source.Headers(), resolved.Headers()) {
		t.Fatal("resolved playback source was not forwarded")
	}
}

func TestAppSwitchEpisodeDoesNotLoadWhenResolveFails(t *testing.T) {
	expected := errors.New("resolve failed")
	session := &fakeSession{events: make(chan PlaybackEvent)}
	store := &fakeStore{}
	app := NewApp(&fakeSource{resolveErr: expected}, &fakePlayer{}, store)

	err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{}, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if session.loads != 0 {
		t.Fatalf("load calls = %d", session.loads)
	}
}

func TestAppSwitchEpisodeDoesNotLoadWhenMappingFails(t *testing.T) {
	expected := errors.New("mapping failed")
	session := &fakeSession{events: make(chan PlaybackEvent)}
	store := &fakeStore{saveMappingErr: expected}
	app := NewApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}, &fakePlayer{}, store)

	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "provider-episode"}
	err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", ref, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if session.loads != 0 {
		t.Fatalf("load calls = %d", session.loads)
	}
	if len(store.savedSourceRefs) != 1 {
		t.Fatalf("saved source refs = %#v", store.savedSourceRefs)
	}
}

func TestAppSwitchEpisodeReturnsLoadFailureWithoutSourceDetails(t *testing.T) {
	loadErr := errors.New("load failed")
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8?token=secret", nil)}
	session := &fakeSession{events: make(chan PlaybackEvent), err: loadErr}
	store := &fakeStore{}
	app := NewApp(source, &fakePlayer{}, store)

	err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if !errors.Is(err, loadErr) {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), source.resolved.URL()) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaked playback source: %v", err)
	}
}

func TestAppSwitchEpisodeCancellationDoesNotLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	session := &fakeSession{events: make(chan PlaybackEvent)}
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}
	store := &fakeStore{}
	app := NewApp(source, &fakePlayer{}, store)

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if source.called {
		t.Fatal("resolver called after cancellation")
	}
	if session.loads != 0 {
		t.Fatalf("load calls = %d", session.loads)
	}
}

func TestAppSwitchEpisodeCancellationAfterResolveDoesNotLoad(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}
	source.resolve = func(context.Context, EpisodeRef) (PlaybackSource, error) {
		cancel()
		return source.resolved, nil
	}
	session := &fakeSession{events: make(chan PlaybackEvent)}
	store := &fakeStore{}
	app := NewApp(source, &fakePlayer{}, store)

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if session.loads != 0 {
		t.Fatalf("load calls = %d", session.loads)
	}
}

func TestAppLibraryDoesNotUseSource(t *testing.T) {
	expected := []Anime{{ID: "local-anime", Title: "Cached"}}
	source := &fakeSource{searchErr: errors.New("source unavailable")}
	store := &fakeStore{anime: expected}
	app := NewApp(source, &fakePlayer{}, store)
	ctx := context.WithValue(context.Background(), contextKey{}, "library")

	actual, err := app.Library(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if source.called {
		t.Fatal("library accessed remote source")
	}
	if store.context != ctx {
		t.Fatal("library context was not forwarded")
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("library = %#v", actual)
	}
}

func TestAppFollowAndUnfollow(t *testing.T) {
	store := &fakeStore{}
	app := NewApp(&fakeSource{}, &fakePlayer{}, store)
	ctx := context.WithValue(context.Background(), contextKey{}, "following")

	if err := app.Follow(ctx, "anime"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.following, []AnimeID{"anime"}) {
		t.Fatalf("following after follow = %#v", store.following)
	}
	if err := app.Unfollow(ctx, "anime"); err != nil {
		t.Fatal(err)
	}
	if len(store.following) != 0 {
		t.Fatalf("following after unfollow = %#v", store.following)
	}
	if store.context != ctx {
		t.Fatal("following context was not forwarded")
	}
}

func TestAppFollowAndUnfollowRejectMissingAnime(t *testing.T) {
	for _, operation := range []struct {
		name string
		call func(*App) error
	}{
		{name: "follow", call: func(app *App) error { return app.Follow(context.Background(), "missing") }},
		{name: "unfollow", call: func(app *App) error { return app.Unfollow(context.Background(), "missing") }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			store := &fakeStore{animeErr: ErrNotFound, following: []AnimeID{"existing"}}
			app := NewApp(&fakeSource{}, &fakePlayer{}, store)

			err := operation.call(app)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(store.following, []AnimeID{"existing"}) {
				t.Fatalf("following after rejected operation = %#v", store.following)
			}
		})
	}
}

func TestAppFollowAndUnfollowPropagateSetFollowingError(t *testing.T) {
	expected := errors.New("following write failed")
	for _, operation := range []struct {
		name string
		call func(*App) error
	}{
		{name: "follow", call: func(app *App) error { return app.Follow(context.Background(), "anime") }},
		{name: "unfollow", call: func(app *App) error { return app.Unfollow(context.Background(), "anime") }},
	} {
		t.Run(operation.name, func(t *testing.T) {
			store := &fakeStore{following: []AnimeID{"existing"}, setFollowingErr: expected}
			app := NewApp(&fakeSource{}, &fakePlayer{}, store)

			if err := operation.call(app); !errors.Is(err, expected) {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(store.following, []AnimeID{"existing"}) {
				t.Fatalf("following after failed operation = %#v", store.following)
			}
		})
	}
}

func TestAppListFollowingEmptyIsNonNil(t *testing.T) {
	app := NewApp(&fakeSource{}, &fakePlayer{}, &fakeStore{})

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if entries == nil {
		t.Fatal("following entries are nil")
	}
	if len(entries) != 0 {
		t.Fatalf("following entries = %#v", entries)
	}
}

func TestAppListFollowingNewEpisode(t *testing.T) {
	latest := SourceEpisode{Ref: EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "latest"}}
	older := EpisodeRef{Anime: latest.Ref.Anime, ID: "older"}
	tests := []struct {
		name       string
		history    []HistoryEntry
		hasWatched bool
		newEpisode bool
	}{
		{
			name:       "watched latest",
			history:    []HistoryEntry{{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "latest"}, LastPlayedAt: time.Unix(1, 0)}},
			hasWatched: true,
			newEpisode: false,
		},
		{
			name:       "watched older",
			history:    []HistoryEntry{{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "older"}, LastPlayedAt: time.Unix(1, 0)}},
			hasWatched: true,
			newEpisode: true,
		},
		{
			name:       "never watched",
			hasWatched: false,
			newEpisode: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{
				following: []AnimeID{"anime"},
				sourceRefs: map[AnimeID][]SourceRef{
					"anime": {{Provider: "source", ID: "anime"}},
				},
				episodeMappings: map[AnimeID][]EpisodeMapping{
					"anime": {
						{AnimeID: "anime", EpisodeID: "latest", Ref: latest.Ref},
						{AnimeID: "anime", EpisodeID: "older", Ref: older},
					},
				},
				history: test.history,
			}
			source := &fakeSource{episodes: []SourceEpisode{latest}}
			app := NewApp(source, &fakePlayer{}, store)

			entries, err := app.ListFollowing(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("following entries = %#v", entries)
			}
			entry := entries[0]
			if entry.AnimeID != "anime" || entry.LatestAvailable != latest.Ref || !entry.HasAvailable {
				t.Fatalf("available episode = %#v", entry)
			}
			if entry.HasWatched != test.hasWatched || entry.NewEpisode != test.newEpisode {
				t.Fatalf("following entry = %#v", entry)
			}
		})
	}
}

func TestAppListFollowingUsesLatestWatchedBySourceOrder(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	older := EpisodeRef{Anime: animeRef, ID: "older-in-source"}
	latest := EpisodeRef{Anime: animeRef, ID: "latest-in-source"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {animeRef},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {
				{AnimeID: "anime", EpisodeID: "older-in-time", Ref: older},
				{AnimeID: "anime", EpisodeID: "latest-in-time", Ref: latest},
			},
		},
		history: []HistoryEntry{
			{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "latest-in-time"}, LastPlayedAt: time.Unix(2, 0)},
			{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "tie-wins"}, LastPlayedAt: time.Unix(2, 0)},
			{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "older-in-time"}, LastPlayedAt: time.Unix(1, 0)},
		},
	}
	source := &fakeSource{episodes: []SourceEpisode{{Ref: older}, {Ref: latest}}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].LatestWatched != "latest-in-time" {
		t.Fatalf("following entries = %#v", entries)
	}
}

func TestAppListFollowingSourceErrorDegradesGracefully(t *testing.T) {
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {{Provider: "source", ID: "anime"}},
		},
	}
	source := &fakeSource{episodesErr: errors.New("source unavailable")}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("following entries = %#v", entries)
	}
	if entries[0].HasAvailable || entries[0].NewEpisode {
		t.Fatalf("following entry = %#v", entries[0])
	}
}

func TestAppListFollowingUsesCanonicalMapping(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	latestRef := EpisodeRef{Anime: animeRef, ID: "provider-latest"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {animeRef},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {{AnimeID: "anime", EpisodeID: "canonical-latest", Ref: latestRef}},
		},
		history: []HistoryEntry{{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "canonical-latest"}}},
	}
	source := &fakeSource{episodes: []SourceEpisode{{Ref: latestRef}}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("following entries = %#v", entries)
	}
	entry := entries[0]
	if entry.LatestWatched != "canonical-latest" || !entry.HasWatched || entry.NewEpisode {
		t.Fatalf("following entry = %#v", entry)
	}
}

func TestAppListFollowingRewatchingOlderDoesNotRegressLatest(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	olderRef := EpisodeRef{Anime: animeRef, ID: "provider-older"}
	latestRef := EpisodeRef{Anime: animeRef, ID: "provider-latest"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {animeRef},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {
				{AnimeID: "anime", EpisodeID: "canonical-older", Ref: olderRef},
				{AnimeID: "anime", EpisodeID: "canonical-latest", Ref: latestRef},
			},
		},
		history: []HistoryEntry{
			{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "canonical-latest"}, LastPlayedAt: time.Unix(1, 0)},
			{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "canonical-older"}, LastPlayedAt: time.Unix(2, 0)},
		},
	}
	source := &fakeSource{episodes: []SourceEpisode{{Ref: olderRef}, {Ref: latestRef}}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("following entries = %#v", entries)
	}
	entry := entries[0]
	if entry.LatestWatched != "canonical-latest" || entry.NewEpisode {
		t.Fatalf("following entry = %#v", entry)
	}
}

func TestAppListFollowingMarksUnmappedLatestAsNew(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	olderRef := EpisodeRef{Anime: animeRef, ID: "provider-older"}
	latestRef := EpisodeRef{Anime: animeRef, ID: "provider-latest"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {animeRef},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {{AnimeID: "anime", EpisodeID: "canonical-older", Ref: olderRef}},
		},
		history: []HistoryEntry{{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "canonical-older"}}},
	}
	source := &fakeSource{episodes: []SourceEpisode{{Ref: olderRef}, {Ref: latestRef}}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[0]
	if entry.LatestWatched != "canonical-older" || !entry.NewEpisode {
		t.Fatalf("following entry = %#v", entry)
	}
}

func TestAppListFollowingTriesSourceRefsInOrder(t *testing.T) {
	first := SourceRef{Provider: "first", ID: "anime"}
	second := SourceRef{Provider: "second", ID: "anime"}
	latest := EpisodeRef{Anime: second, ID: "latest"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {first, second},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {{AnimeID: "anime", EpisodeID: "canonical-latest", Ref: latest}},
		},
	}
	var calls []SourceRef
	source := &fakeSource{episodesFn: func(_ context.Context, ref SourceRef) ([]SourceEpisode, error) {
		calls = append(calls, ref)
		if ref == first {
			return nil, errors.New("first source unavailable")
		}
		return []SourceEpisode{{Ref: latest}}, nil
	}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []SourceRef{first, second}) {
		t.Fatalf("source calls = %#v", calls)
	}
	if len(entries) != 1 || !entries[0].HasAvailable || entries[0].LatestAvailable != latest {
		t.Fatalf("following entries = %#v", entries)
	}
}

func TestAppListFollowingEmptySourceResultIsAuthoritative(t *testing.T) {
	first := SourceRef{Provider: "first", ID: "anime"}
	second := SourceRef{Provider: "second", ID: "anime"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {first, second},
		},
	}
	var calls []SourceRef
	source := &fakeSource{episodesFn: func(_ context.Context, ref SourceRef) ([]SourceEpisode, error) {
		calls = append(calls, ref)
		return nil, nil
	}}
	app := NewApp(source, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []SourceRef{first}) {
		t.Fatalf("source calls = %#v", calls)
	}
	if len(entries) != 1 || entries[0].HasAvailable {
		t.Fatalf("following entries = %#v", entries)
	}
}

func TestAppListFollowingSourceFailurePreservesWatchedFallback(t *testing.T) {
	animeRef := SourceRef{Provider: "source", ID: "anime"}
	latestRef := EpisodeRef{Anime: animeRef, ID: "provider-latest"}
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {animeRef},
		},
		episodeMappings: map[AnimeID][]EpisodeMapping{
			"anime": {{AnimeID: "anime", EpisodeID: "canonical-latest", Ref: latestRef}},
		},
		history: []HistoryEntry{{Progress: PlaybackProgress{AnimeID: "anime", EpisodeID: "canonical-latest"}}},
	}
	app := NewApp(&fakeSource{episodesErr: errors.New("source unavailable")}, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].LatestWatched != "canonical-latest" || !entries[0].HasWatched || entries[0].NewEpisode {
		t.Fatalf("following entries = %#v", entries)
	}
}

func TestAppListFollowingPropagatesLocalErrors(t *testing.T) {
	expected := errors.New("local failure")
	tests := []struct {
		name  string
		store *fakeStore
	}{
		{name: "anime", store: &fakeStore{animeErr: expected, following: []AnimeID{"anime"}}},
		{name: "source refs", store: &fakeStore{sourceRefsErr: expected, following: []AnimeID{"anime"}}},
		{name: "episode mappings", store: &fakeStore{episodeMappingsErr: expected, following: []AnimeID{"anime"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewApp(&fakeSource{}, &fakePlayer{}, test.store)
			_, err := app.ListFollowing(context.Background())
			if !errors.Is(err, expected) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestAppListFollowingPropagatesCallerCancellationFromSource(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeStore{
		following: []AnimeID{"anime"},
		sourceRefs: map[AnimeID][]SourceRef{
			"anime": {{Provider: "source", ID: "anime"}},
		},
	}
	source := &fakeSource{episodesFn: func(context.Context, SourceRef) ([]SourceEpisode, error) {
		cancel()
		return nil, errors.New("source canceled")
	}}
	app := NewApp(source, &fakePlayer{}, store)

	_, err := app.ListFollowing(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}

func TestAppListFollowingPreservesStoreOrder(t *testing.T) {
	store := &fakeStore{
		following: []AnimeID{"first", "second", "third"},
		sourceRefs: map[AnimeID][]SourceRef{
			"first":  {{Provider: "source", ID: "first"}},
			"second": {{Provider: "source", ID: "second"}},
			"third":  {{Provider: "source", ID: "third"}},
		},
	}
	app := NewApp(&fakeSource{}, &fakePlayer{}, store)

	entries, err := app.ListFollowing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]AnimeID, len(entries))
	for index, entry := range entries {
		got[index] = entry.AnimeID
	}
	if !reflect.DeepEqual(got, store.following) {
		t.Fatalf("following order = %#v, want %#v", got, store.following)
	}
}

func TestAppFollowingCancellationPropagates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store := &fakeStore{following: []AnimeID{"anime"}}
	app := NewApp(&fakeSource{}, &fakePlayer{}, store)

	if err := app.Follow(ctx, "anime"); !errors.Is(err, context.Canceled) {
		t.Fatalf("follow error = %v", err)
	}
	if len(store.following) != 1 {
		t.Fatalf("following after canceled follow = %#v", store.following)
	}

	store.followingErr = context.Canceled
	if _, err := app.ListFollowing(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("list error = %v", err)
	}
}

func TestAppListFollowingPropagatesHistoryError(t *testing.T) {
	expected := errors.New("history read failed")
	store := &fakeStore{historyErr: expected}
	app := NewApp(&fakeSource{}, &fakePlayer{}, store)

	if _, err := app.ListFollowing(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
}

type contextKey struct{}

func TestPlaybackSourceClonesAndRedactsSensitiveData(t *testing.T) {
	headers := http.Header{"Cookie": {"session=secret"}, "Referer": {"https://source.example/"}}
	source := NewPlaybackSource("https://media.example/video?token=secret", headers)
	headers.Set("Cookie", "changed")

	if source.Headers().Get("Cookie") != "session=secret" {
		t.Fatal("constructor retained caller header storage")
	}
	copy := source.Headers()
	copy.Set("Cookie", "changed again")
	if source.Headers().Get("Cookie") != "session=secret" {
		t.Fatal("accessor exposed internal header storage")
	}
	for _, output := range []string{fmt.Sprint(source), fmt.Sprintf("%#v", source)} {
		if output != "PlaybackSource{redacted}" {
			t.Fatalf("formatted source = %q", output)
		}
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "media.example") {
		t.Fatalf("serialized source leaked sensitive data: %s", encoded)
	}
}

func TestSettingsDefaults(t *testing.T) {
	var zero Settings
	if zero.Appearance != AppearanceUnspecified || zero.AutoplayNext != ToggleUnspecified || zero.ResumePlayback != ToggleUnspecified || zero.Language != LanguageUnspecified {
		t.Fatalf("zero settings = %#v", zero)
	}
	defaults := DefaultSettings()
	if defaults.AutoplayNext != ToggleEnabled {
		t.Fatalf("autoplay default = %d", defaults.AutoplayNext)
	}
	if defaults.ResumePlayback != ToggleUnspecified {
		t.Fatalf("undocumented resume default = %d", defaults.ResumePlayback)
	}
}

func TestPlaybackEventIdentifiesCanonicalEpisode(t *testing.T) {
	event := PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: PlaybackEventProgress}
	if event.AnimeID == "" || event.EpisodeID == "" {
		t.Fatalf("event = %#v", event)
	}
}
