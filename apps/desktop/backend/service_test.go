// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/metadata/cover"
	"animeportable/adapters/player/mpv"
	"animeportable/core"
)

type fakeStore struct {
	mu        sync.Mutex
	anime     map[core.AnimeID]core.Anime
	refs      map[core.AnimeID][]core.SourceRef
	mappings  map[core.AnimeID][]core.EpisodeMapping
	following map[core.AnimeID]bool
	history   []core.HistoryEntry
	metadata  map[core.AnimeID]core.AnimeMetadata
	settings  core.Settings
	setErr    error
	block     bool
	started   chan struct{}
	closed    chan struct{}
}

func newFakeStore() *fakeStore {
	return &fakeStore{anime: map[core.AnimeID]core.Anime{}, refs: map[core.AnimeID][]core.SourceRef{}, mappings: map[core.AnimeID][]core.EpisodeMapping{}, following: map[core.AnimeID]bool{}, metadata: map[core.AnimeID]core.AnimeMetadata{}, settings: core.DefaultSettings()}
}
func (s *fakeStore) wait(ctx context.Context) error {
	if s.block {
		select {
		case <-s.started:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return ctx.Err()
}
func (s *fakeStore) SaveAnime(ctx context.Context, a core.Anime) error {
	if err := s.wait(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.anime[a.ID] = a
	return nil
}
func (s *fakeStore) Anime(ctx context.Context, id core.AnimeID) (core.Anime, error) {
	if err := s.wait(ctx); err != nil {
		return core.Anime{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.anime[id]
	if !ok {
		return core.Anime{}, core.ErrNotFound
	}
	return a, nil
}
func (s *fakeStore) ListAnime(ctx context.Context) ([]core.Anime, error) {
	if err := s.wait(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := make([]core.Anime, 0, len(s.anime))
	for _, a := range s.anime {
		r = append(r, a)
	}
	return r, nil
}
func (s *fakeStore) SaveSourceRef(ctx context.Context, id core.AnimeID, r core.SourceRef) error {
	if err := s.wait(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.refs[id] {
		if x == r {
			return nil
		}
	}
	s.refs[id] = append(s.refs[id], r)
	return nil
}

func (s *fakeStore) IngestSourceAnime(ctx context.Context, candidate core.Anime, ref core.SourceRef) (core.Anime, error) {
	if err := s.wait(ctx); err != nil {
		return core.Anime{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, refs := range s.refs {
		for _, existing := range refs {
			if existing != ref {
				continue
			}
			anime, ok := s.anime[id]
			if !ok {
				return core.Anime{}, core.ErrNotFound
			}
			anime.Title = candidate.Title
			anime.NativeTitle = candidate.NativeTitle
			s.anime[id] = anime
			return anime, nil
		}
	}
	if _, exists := s.anime[candidate.ID]; exists {
		return core.Anime{}, errors.New("identity conflict")
	}
	s.anime[candidate.ID] = candidate
	s.refs[candidate.ID] = append(s.refs[candidate.ID], ref)
	return candidate, nil
}
func (s *fakeStore) SourceRefs(ctx context.Context, id core.AnimeID) ([]core.SourceRef, error) {
	if err := s.wait(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]core.SourceRef(nil), s.refs[id]...), nil
}
func (s *fakeStore) SaveEpisodeMapping(ctx context.Context, m core.EpisodeMapping) error {
	if err := s.wait(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.mappings[m.AnimeID] {
		if x.Ref == m.Ref {
			return nil
		}
	}
	s.mappings[m.AnimeID] = append(s.mappings[m.AnimeID], m)
	return nil
}
func (s *fakeStore) EpisodeMappings(ctx context.Context, id core.AnimeID) ([]core.EpisodeMapping, error) {
	if err := s.wait(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]core.EpisodeMapping(nil), s.mappings[id]...), nil
}
func (s *fakeStore) SaveMetadata(ctx context.Context, id core.AnimeID, m core.AnimeMetadata) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[id] = m
	return nil
}
func (s *fakeStore) Metadata(ctx context.Context, id core.AnimeID) (core.AnimeMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.metadata[id]
	if !ok {
		return core.AnimeMetadata{}, core.ErrNotFound
	}
	return m, nil
}
func (s *fakeStore) SetFollowing(ctx context.Context, id core.AnimeID, v bool) error {
	if _, e := s.Anime(ctx, id); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.following[id] = v
	return nil
}
func (s *fakeStore) Following(ctx context.Context) ([]core.AnimeID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := []core.AnimeID{}
	for id, v := range s.following {
		if v {
			r = append(r, id)
		}
	}
	return r, nil
}
func (s *fakeStore) AddHistory(ctx context.Context, e core.HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, e)
	return nil
}
func (s *fakeStore) History(ctx context.Context) ([]core.HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]core.HistoryEntry(nil), s.history...), nil
}
func (s *fakeStore) RemoveHistory(context.Context, core.AnimeID) error               { return nil }
func (s *fakeStore) SaveProgress(context.Context, core.PlaybackProgress) error       { return nil }
func (s *fakeStore) SavePlaybackCheckpoint(context.Context, core.HistoryEntry) error { return nil }
func (s *fakeStore) Progress(context.Context, core.AnimeID, core.EpisodeID) (core.PlaybackProgress, error) {
	return core.PlaybackProgress{}, core.ErrNotFound
}
func (s *fakeStore) SaveSettings(ctx context.Context, v core.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.setErr != nil {
		return s.setErr
	}
	s.settings = v
	return nil
}
func (s *fakeStore) Settings(context.Context) (core.Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, nil
}
func (s *fakeStore) Close() error { return nil }

type fakeSource struct {
	mu       sync.Mutex
	calls    int
	catalog  func(context.Context) ([]core.SourceAnime, error)
	episodes []core.SourceEpisode
}

func (s *fakeSource) Catalog(ctx context.Context) ([]core.SourceAnime, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.catalog != nil {
		return s.catalog(ctx)
	}
	return nil, nil
}
func (s *fakeSource) Search(context.Context, string) ([]core.SourceAnime, error) { return nil, nil }
func (s *fakeSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.episodes, nil
}
func (s *fakeSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.NewPlaybackSource("", nil), nil
}
func (s *fakeSource) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return nil, nil
}

func testService() (*Service, *fakeStore, *fakeSource) {
	st := newFakeStore()
	src := &fakeSource{}
	return newWithDependencies(dependencies{store: st, source: src}), st, src
}
func sourceAnime() core.SourceAnime {
	return core.SourceAnime{Ref: core.SourceRef{Provider: "p", ID: "1"}, Title: "  Title  ", NativeTitle: "  Native  "}
}

func TestLifecycle(t *testing.T) {
	s, _, src := testService()
	started := make(chan struct{})
	release := make(chan struct{})
	closeErr := errors.New("close failed")
	s.closeDeps = func() error { <-release; return closeErr }
	src.catalog = func(ctx context.Context) ([]core.SourceAnime, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := make(chan error, 1)
	go func() { _, e := s.Catalog(context.Background()); done <- e }()
	<-started
	shutdown := make(chan error, 1)
	go func() { shutdown <- s.Close() }()
	select {
	case <-shutdown:
		t.Fatal("shutdown did not wait")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-shutdown:
		t.Fatal("shutdown did not wait")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	firstShutdown := <-shutdown
	if firstShutdown != ErrUnavailable {
		t.Fatalf("shutdown error %v", firstShutdown)
	}
	if e := <-done; !errors.Is(e, context.Canceled) {
		t.Fatalf("operation error %v", e)
	}
	if _, _, e := s.begin(context.Background()); e != ErrClosed {
		t.Fatalf("begin after shutdown: %v", e)
	}
	if e := s.Close(); e != firstShutdown {
		t.Fatalf("second shutdown error %v", e)
	}
}

func TestValidationAndNormalization(t *testing.T) {
	for _, v := range []string{"", strings.Repeat("x", 129), "   "} {
		if _, e := localID(v); e != ErrInvalidInput {
			t.Errorf("localID %q: %v", v, e)
		}
		if _, e := episodeID(v); e != ErrInvalidInput {
			t.Errorf("episodeID %q: %v", v, e)
		}
	}
	for _, in := range []core.SourceAnime{{Title: "x"}, {Ref: core.SourceRef{ID: "x"}, Title: "x"}, {Ref: core.SourceRef{Provider: "p"}, Title: "x"}, {Ref: core.SourceRef{Provider: "p", ID: "x"}}} {
		if _, e := normalizeSourceAnime(in); e == nil {
			t.Error("expected invalid source")
		}
	}
	v, e := normalizeSourceAnime(sourceAnime())
	if e != nil || v.Title != "Title" || v.NativeTitle != "Native" {
		t.Fatalf("normalized=%+v err=%v", v, e)
	}
}
func TestSafeError(t *testing.T) {
	for _, x := range []struct{ in, want error }{
		{context.Canceled, context.Canceled},
		{core.ErrNotFound, core.ErrNotFound},
		{mpv.ErrNotFound, mpv.ErrNotFound},
		{fmt.Errorf("secret path: %w", mpv.ErrNotFound), mpv.ErrNotFound},
		{mpv.ErrInvalidPath, mpv.ErrInvalidPath},
		{fmt.Errorf("secret path: %w", mpv.ErrInvalidPath), mpv.ErrInvalidPath},
		{errors.New("x"), ErrUnavailable},
	} {
		if e := safeError(x.in); e != x.want {
			t.Errorf("%v => %v", x.in, e)
		}
	}
}

func TestIngestConcurrentAndEpisodes(t *testing.T) {
	s, st, _ := testService()
	items := []core.SourceAnime{sourceAnime()}
	a, e := s.ingestAnime(context.Background(), st, items)
	if e != nil || len(a) != 1 || !strings.HasPrefix(string(a[0].anime.ID), "anime:") {
		t.Fatalf("%+v %v", a, e)
	}
	b, _ := s.ingestAnime(context.Background(), st, items)
	if b[0].anime.ID != a[0].anime.ID {
		t.Error("known ref not reused")
	}
	var wg sync.WaitGroup
	ids := make(chan core.AnimeID, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			x, e := s.ingestAnime(context.Background(), st, items)
			if e == nil {
				ids <- x[0].anime.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	for id := range ids {
		if id != a[0].anime.ID {
			t.Error("duplicate ID")
		}
	}
	parent := a[0].ref
	ep := core.SourceEpisode{Ref: core.EpisodeRef{Anime: parent, ID: "e1"}, Number: "1", Title: "E"}
	out, e := s.ingestEpisodes(context.Background(), st, a[0].anime.ID, parent, []core.SourceEpisode{ep})
	if e != nil || len(out) != 1 || !strings.HasPrefix(out[0].ID, "episode:") {
		t.Fatalf("episode %v %v", out, e)
	}
	again, _ := s.ingestEpisodes(context.Background(), st, a[0].anime.ID, parent, []core.SourceEpisode{ep})
	if again[0].ID != out[0].ID {
		t.Error("episode not reused")
	}
	if _, e = s.ingestEpisodes(context.Background(), st, a[0].anime.ID, parent, []core.SourceEpisode{{Ref: core.EpisodeRef{Anime: core.SourceRef{Provider: "other"}, ID: "e"}, Number: "1"}}); e != ErrInvalidInput {
		t.Error("parent mismatch")
	}
	if _, e = s.ingestEpisodes(context.Background(), st, a[0].anime.ID, parent, []core.SourceEpisode{{Ref: core.EpisodeRef{Anime: parent, ID: "e"}}}); e != ErrInvalidInput {
		t.Error("empty number")
	}
}

func TestFollowFollowingHistorySettingsSchedule(t *testing.T) {
	s, st, src := testService()
	st.anime["a"] = core.Anime{ID: "a", Title: "A"}
	if e := s.Follow(context.Background(), " "); e != ErrInvalidInput {
		t.Error(e)
	}
	if e := s.Follow(context.Background(), "missing"); e != core.ErrNotFound {
		t.Error(e)
	}
	st.following["a"] = true
	parent := core.SourceRef{Provider: "p", ID: "a"}
	st.mappings["a"] = []core.EpisodeMapping{
		{AnimeID: "a", EpisodeID: "old", Ref: core.EpisodeRef{Anime: parent, ID: "old"}},
		{AnimeID: "a", EpisodeID: "new", Ref: core.EpisodeRef{Anime: parent, ID: "new"}},
	}
	playedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	st.history = []core.HistoryEntry{
		{Progress: core.PlaybackProgress{AnimeID: "a", EpisodeID: "old"}, LastPlayedAt: playedAt},
		{Progress: core.PlaybackProgress{AnimeID: "a", EpisodeID: "new"}, LastPlayedAt: playedAt.Add(time.Hour)},
	}
	got, e := s.Following(context.Background())
	if e != nil || len(got) != 1 || got[0].HasAvailable || got[0].LatestAvailable != "" || !got[0].HasWatched || got[0].LatestWatched != "new" || got[0].NewEpisode {
		t.Fatalf("following %+v %v", got, e)
	}
	src.mu.Lock()
	if src.calls != 0 {
		t.Error("following called source")
	}
	src.mu.Unlock()
	settings := Settings{Appearance: "dark", MPVPath: "x", AutoplayNext: "enabled", ResumePlayback: "disabled", Language: "zh-TW"}
	if e = s.SaveSettings(context.Background(), settings); e != nil {
		t.Fatal(e)
	}
	saved, e := s.Settings(context.Background())
	if e != nil || saved != settings {
		t.Fatalf("settings %+v %v", saved, e)
	}
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	h := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: "a", EpisodeID: "e", Position: 1500 * time.Millisecond, Duration: 3 * time.Second, Completed: true, UpdatedAt: now}, LastPlayedAt: now}
	st.history = []core.HistoryEntry{h}
	hs, e := s.History(context.Background())
	if e != nil || hs[0].Position != 1500 || hs[0].UpdatedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("history %+v %v", hs, e)
	}
	if _, e = s.Schedule(context.Background(), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); e != ErrInvalidInput {
		t.Error("invalid schedule range")
	}
}

func TestPlayAndCoverValidation(t *testing.T) {
	s, st, _ := testService()
	calls := 0
	s.newPlayer = func(string) (core.Player, error) { calls++; return nil, errors.New("mpv") }
	for _, in := range []PlayRequest{{EpisodeID: "e"}, {AnimeID: "a", StartAt: -1}, {AnimeID: "a", EpisodeID: "e"}} {
		if e := s.Play(context.Background(), in); e == nil {
			t.Error("invalid play accepted")
		}
	}
	if calls != 0 {
		t.Error("player used for invalid play")
	}
	st.anime["a"] = core.Anime{ID: "a"}
	st.mappings["a"] = []core.EpisodeMapping{{AnimeID: "a", EpisodeID: "e", Ref: core.EpisodeRef{Anime: core.SourceRef{Provider: "p", ID: "a"}, ID: "e"}}}
	if e := s.Play(context.Background(), PlayRequest{AnimeID: "a", EpisodeID: "e"}); e != ErrUnavailable {
		t.Errorf("play error %v", e)
	}
	s.cover, _ = cover.New()
	if _, e := s.GetCover(context.Background(), "a"); e != core.ErrNotFound {
		t.Errorf("cover metadata error %v", e)
	}
	s.cover = nil
	st.metadata["a"] = core.AnimeMetadata{}
	if _, e := s.GetCover(context.Background(), "a"); e != ErrUnavailable {
		t.Errorf("nil loader error %v", e)
	}
}

func TestSortAnime(t *testing.T) {
	items := []Anime{{ID: "b"}, {ID: "a"}, {ID: "c"}}
	sortAnime(items)
	if items[0].ID != "a" || items[1].ID != "b" || items[2].ID != "c" {
		t.Fatal(items)
	}
}
