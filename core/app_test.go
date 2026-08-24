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

func (source *fakeSource) Episodes(context.Context, SourceRef) ([]SourceEpisode, error) {
	source.called = true
	return nil, source.searchErr
}

func (source *fakeSource) Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error) {
	source.called = true
	source.context = ctx
	source.resolveRef = ref
	if source.calls != nil {
		*source.calls = append(*source.calls, "resolve")
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
	events chan PlaybackEvent
}

func (*fakeSession) Load(context.Context, PlayRequest) error {
	return nil
}

func (session *fakeSession) Events() <-chan PlaybackEvent {
	return session.events
}

func (*fakeSession) Close() error {
	return nil
}

type fakeStore struct {
	context context.Context
	anime   []Anime
	err     error
}

func (*fakeStore) SaveAnime(context.Context, Anime) error {
	return nil
}

func (*fakeStore) Anime(context.Context, AnimeID) (Anime, error) {
	return Anime{}, ErrNotFound
}

func (store *fakeStore) ListAnime(ctx context.Context) ([]Anime, error) {
	store.context = ctx
	return store.anime, store.err
}

func (*fakeStore) SaveSourceRef(context.Context, AnimeID, SourceRef) error {
	return nil
}

func (*fakeStore) SourceRefs(context.Context, AnimeID) ([]SourceRef, error) {
	return nil, nil
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

func (*fakeStore) Following(context.Context) ([]AnimeID, error) {
	return nil, nil
}

func (*fakeStore) AddHistory(context.Context, HistoryEntry) error {
	return nil
}

func (*fakeStore) History(context.Context) ([]HistoryEntry, error) {
	return nil, nil
}

func (*fakeStore) RemoveHistory(context.Context, AnimeID) error {
	return nil
}

func (*fakeStore) SaveProgress(context.Context, PlaybackProgress) error {
	return nil
}

func (*fakeStore) Progress(context.Context, AnimeID, EpisodeID) (PlaybackProgress, error) {
	return PlaybackProgress{}, ErrNotFound
}

func (*fakeStore) SaveSettings(context.Context, Settings) error {
	return nil
}

func (*fakeStore) Settings(context.Context) (Settings, error) {
	return Settings{}, ErrNotFound
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
	app := NewApp(source, player, &fakeStore{})
	ctx := context.WithValue(context.Background(), contextKey{}, "play")

	actual, err := app.PlayEpisode(ctx, AnimeID("local-anime"), EpisodeID("local-episode"), ref, 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if actual != session {
		t.Fatal("unexpected playback session")
	}
	if source.resolveRef != ref {
		t.Fatalf("resolved ref = %#v", source.resolveRef)
	}
	if !reflect.DeepEqual(calls, []string{"resolve", "start"}) {
		t.Fatalf("playback calls = %#v", calls)
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
	app := NewApp(&fakeSource{resolveErr: expected}, player, &fakeStore{})

	_, err := app.PlayEpisode(context.Background(), "anime", "episode", EpisodeRef{}, 0)
	if !errors.Is(err, expected) {
		t.Fatalf("error = %v", err)
	}
	if player.started {
		t.Fatal("player started after resolution failure")
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
