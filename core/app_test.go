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
	context    context.Context
	calls      *[]string
	resolveRef EpisodeRef
	resolved   PlaybackSource
	resolveErr error
	resolve    func(context.Context, EpisodeRef) (PlaybackSource, error)
	called     bool
}

func (*fakeSource) Catalog(context.Context) ([]SourceAnime, error) {
	return nil, nil
}

func (*fakeSource) Search(context.Context, string) ([]SourceAnime, error) {
	return nil, nil
}

func (*fakeSource) Episodes(context.Context, SourceRef) ([]SourceEpisode, error) {
	return nil, nil
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
	tracked, err := newTrackedPlaybackSession(session, store, PlayRequest{AnimeID: "old-anime", EpisodeID: "old-episode"}, NewApp(nil, nil, nil).tracking)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tracked.Close() })
	return tracked
}

type fakeStore struct {
	animeErr           error
	settings           Settings
	progress           PlaybackProgress
	checkpoints        []HistoryEntry
	following          []AnimeID
	followedAt         time.Time
	followingErr       error
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
	if store.animeErr != nil {
		return Anime{}, store.animeErr
	}
	return Anime{ID: id}, nil
}

func (*fakeStore) ListAnime(context.Context) ([]Anime, error) {
	return nil, nil
}

func (store *fakeStore) SaveSourceRef(ctx context.Context, animeID AnimeID, ref SourceRef) error {
	if store.saveSourceRefErr != nil {
		return store.saveSourceRefErr
	}
	if store.calls != nil {
		*store.calls = append(*store.calls, "save-source-ref")
	}
	store.savedSourceRefs = append(store.savedSourceRefs, savedSourceRef{animeID: animeID, ref: ref})
	return nil
}

func (*fakeStore) SourceRefs(context.Context, AnimeID) ([]SourceRef, error) {
	return nil, nil
}

func (store *fakeStore) SaveEpisodeMapping(ctx context.Context, mapping EpisodeMapping) error {
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

func (*fakeStore) SetFollowing(context.Context, AnimeID, bool) error {
	return nil
}

func (store *fakeStore) Following(ctx context.Context) ([]FollowedAnime, error) {
	if store.followingErr != nil {
		return nil, store.followingErr
	}
	followed := make([]FollowedAnime, 0, len(store.following))
	for _, id := range store.following {
		followed = append(followed, FollowedAnime{AnimeID: id, FollowedAt: store.followedAt})
	}
	return followed, nil
}

func (store *fakeStore) History(ctx context.Context) ([]HistoryEntry, error) {
	if store.historyErr != nil {
		return nil, store.historyErr
	}
	return store.history, nil
}

func (*fakeStore) RemoveHistory(context.Context, AnimeID) error {
	return nil
}

func (store *fakeStore) SavePlaybackCheckpoint(_ context.Context, entry HistoryEntry) error {
	store.checkpoints = append(store.checkpoints, entry)
	return nil
}

func (store *fakeStore) Progress(context.Context, AnimeID, EpisodeID) (PlaybackProgress, error) {
	if store.progress.AnimeID == "" {
		return PlaybackProgress{}, ErrNotFound
	}
	return store.progress, nil
}

func (*fakeStore) SaveSettings(context.Context, Settings) error {
	return nil
}

func (*fakeStore) ReplaceSchedule(context.Context, []ScheduleEntry) error {
	return nil
}

func (*fakeStore) Schedule(context.Context) ([]ScheduleEntry, error) {
	return nil, nil
}

func (store *fakeStore) Settings(context.Context) (Settings, error) {
	return store.settings, nil
}

var (
	_ AnimeSource     = (*fakeSource)(nil)
	_ Player          = (*fakePlayer)(nil)
	_ PlaybackSession = (*fakeSession)(nil)
	_ Store           = (*fakeStore)(nil)
)

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

	actual, err := app.PlayEpisode(ctx, AnimeID("local-anime"), EpisodeID("local-episode"), ref, 90*time.Second, false)
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

	_, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{}, 0, false)
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
	_, err := app.PlayEpisode(context.Background(), "anime", "episode", ref, 0, false)
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

	_, err := app.PlayEpisode(context.Background(), "anime", "episode", ref, 0, false)
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

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), AnimeID("local-anime"), EpisodeID("local-episode"), ref, 90*time.Second, false)
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

func TestAppSwitchEpisodeForwardsPausedToTheLoad(t *testing.T) {
	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "episode"}
	for _, paused := range []bool{true, false} {
		session := &fakeSession{events: make(chan PlaybackEvent)}
		store := &fakeStore{}
		app := NewApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/video", nil)}, &fakePlayer{}, store)
		if err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", ref, 0, paused); err != nil {
			t.Fatal(err)
		}
		if session.request.Paused != paused {
			t.Fatalf("load paused = %t, want %t", session.request.Paused, paused)
		}
	}
}

func TestAppSwitchEpisodeDoesNotLoadWhenMappingFails(t *testing.T) {
	expected := errors.New("mapping failed")
	session := &fakeSession{events: make(chan PlaybackEvent)}
	store := &fakeStore{saveMappingErr: expected}
	app := NewApp(&fakeSource{resolved: NewPlaybackSource("https://media.example/episode.m3u8", nil)}, &fakePlayer{}, store)

	ref := EpisodeRef{Anime: SourceRef{Provider: "source", ID: "anime"}, ID: "provider-episode"}
	err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", ref, 0, false)
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

	err := app.SwitchEpisode(context.Background(), trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0, false)
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

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0, false)
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

	err := app.SwitchEpisode(ctx, trackFakeSession(t, session, store), "anime", "episode", EpisodeRef{ID: "episode"}, 0, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if session.loads != 0 {
		t.Fatalf("load calls = %d", session.loads)
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

func TestSettingsResolvedFillsEveryUnchosenField(t *testing.T) {
	want := Settings{
		Appearance: AppearanceSystem, AutoplayNext: ToggleEnabled, AutoPlay: ToggleEnabled, ResumePlayback: ToggleEnabled,
		Language: LanguageTraditionalChinese, PlaybackSpeed: 100, HardwareDecoding: ToggleEnabled, ProxyMode: ProxyModeNone,
	}
	if got := (Settings{}).Resolved(); got != want {
		t.Fatalf("resolved zero settings = %#v, want %#v", got, want)
	}
}

func TestSettingsResolvedKeepsChosenValues(t *testing.T) {
	chosen := Settings{
		Appearance: AppearanceDark, AutoplayNext: ToggleDisabled, AutoPlay: ToggleDisabled, ResumePlayback: ToggleDisabled, Language: LanguageEnglish,
		PlaybackSpeed: 75, HardwareDecoding: ToggleDisabled, ProxyMode: ProxyModeManual, ProxyAddress: "proxy.example:8080",
	}
	if got := chosen.Resolved(); got != chosen {
		t.Fatalf("resolved chosen settings = %#v, want %#v", got, chosen)
	}
	light := Settings{
		Appearance: AppearanceLight, AutoplayNext: ToggleEnabled, AutoPlay: ToggleEnabled, ResumePlayback: ToggleEnabled,
		Language: LanguageJapanese, PlaybackSpeed: 200, HardwareDecoding: ToggleEnabled, ProxyMode: ProxyModeSystem,
	}
	if got := light.Resolved(); got != light {
		t.Fatalf("resolved light settings = %#v, want %#v", got, light)
	}
}
