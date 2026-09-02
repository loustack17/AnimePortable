// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

var (
	fakeAnimeRef = core.SourceRef{Provider: "fake-source", ID: "anime"}
	fakeEpisode1 = core.EpisodeRef{Anime: fakeAnimeRef, ID: "episode-1"}
	fakeEpisode2 = core.EpisodeRef{Anime: fakeAnimeRef, ID: "episode-2"}
	fakeMetaRef  = core.MetadataRef{Provider: "fake-metadata", ID: "anime"}
)

type fakeAnimeSource struct{}

func (*fakeAnimeSource) Catalog(ctx context.Context) ([]core.SourceAnime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []core.SourceAnime{{Ref: fakeAnimeRef, Title: "Anime"}}, nil
}

func (*fakeAnimeSource) Search(ctx context.Context, _ string) ([]core.SourceAnime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []core.SourceAnime{{Ref: fakeAnimeRef, Title: "Anime"}}, nil
}

func (*fakeAnimeSource) Episodes(ctx context.Context, ref core.SourceRef) ([]core.SourceEpisode, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ref != fakeAnimeRef {
		return nil, core.ErrNotFound
	}
	return []core.SourceEpisode{{Ref: fakeEpisode1, Number: "1"}, {Ref: fakeEpisode2, Number: "2"}}, nil
}

func (*fakeAnimeSource) Resolve(ctx context.Context, ref core.EpisodeRef) (core.PlaybackSource, error) {
	if err := ctx.Err(); err != nil {
		return core.PlaybackSource{}, err
	}
	if ref != fakeEpisode1 {
		return core.PlaybackSource{}, core.ErrNotFound
	}
	return core.NewPlaybackSource("https://media.example/episode?token=raw-secret", http.Header{"Cookie": {"raw-secret"}}), nil
}

func (*fakeAnimeSource) Schedule(ctx context.Context, _ core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	start := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	return []core.SourceScheduleItem{
		{Anime: core.SourceAnime{Ref: fakeAnimeRef, Title: "Anime"}, Episode: core.SourceEpisode{Ref: fakeEpisode1, Number: "1"}, AirsAt: start, Precision: core.SchedulePrecisionTime},
		{Anime: core.SourceAnime{Ref: fakeAnimeRef, Title: "Anime"}, Episode: core.SourceEpisode{Ref: fakeEpisode2, Number: "2"}, AirsAt: start.Add(time.Hour), Precision: core.SchedulePrecisionTime},
	}, nil
}

type unsupportedAnimeSource struct{}

func (*unsupportedAnimeSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return nil, core.ErrUnsupported
}
func (*unsupportedAnimeSource) Search(context.Context, string) ([]core.SourceAnime, error) {
	return nil, core.ErrUnsupported
}
func (*unsupportedAnimeSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	return nil, core.ErrUnsupported
}
func (*unsupportedAnimeSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.PlaybackSource{}, core.ErrUnsupported
}
func (*unsupportedAnimeSource) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return nil, core.ErrUnsupported
}

type fakeMetadataProvider struct{}

type unsupportedMetadataProvider struct{}

func (*unsupportedMetadataProvider) Search(context.Context, core.MetadataQuery) ([]core.MetadataCandidate, error) {
	return nil, core.ErrUnsupported
}

func (*unsupportedMetadataProvider) Get(context.Context, core.MetadataRef) (core.AnimeMetadata, error) {
	return core.AnimeMetadata{}, core.ErrUnsupported
}

func (*fakeMetadataProvider) Search(ctx context.Context, _ core.MetadataQuery) ([]core.MetadataCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return []core.MetadataCandidate{{Ref: fakeMetaRef, Title: "Anime", Year: 2026, EpisodeCount: 12}}, nil
}

func (*fakeMetadataProvider) Get(ctx context.Context, ref core.MetadataRef) (core.AnimeMetadata, error) {
	if err := ctx.Err(); err != nil {
		return core.AnimeMetadata{}, err
	}
	if ref != fakeMetaRef {
		return core.AnimeMetadata{}, core.ErrNotFound
	}
	return fakeMetadata(), nil
}

func fakeMetadata() core.AnimeMetadata {
	return core.AnimeMetadata{Ref: fakeMetaRef, Title: "Anime", NativeTitle: "Anime", Description: "Plain text", Year: 2026, EpisodeCount: 12}
}

type fakePlayerProbe struct {
	mu       sync.Mutex
	starts   int
	loads    int
	active   int
	events   int
	clean    bool
	requests []core.PlayRequest
}

func (probe *fakePlayerProbe) Starts() int {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.starts
}
func (probe *fakePlayerProbe) Loads() int {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.loads
}
func (probe *fakePlayerProbe) ActiveSessions() int {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.active
}
func (probe *fakePlayerProbe) EventCount() int {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.events
}
func (probe *fakePlayerProbe) Clean() bool {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return probe.clean
}
func (probe *fakePlayerProbe) Requests() []core.PlayRequest {
	probe.mu.Lock()
	defer probe.mu.Unlock()
	return append([]core.PlayRequest(nil), probe.requests...)
}

type fakePlayer struct {
	probe *fakePlayerProbe
}

func (player *fakePlayer) Start(ctx context.Context, request core.PlayRequest) (core.PlaybackSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	player.probe.mu.Lock()
	player.probe.starts++
	player.probe.active++
	player.probe.clean = false
	player.probe.requests = append(player.probe.requests, request)
	player.probe.events++
	player.probe.mu.Unlock()
	events := make(chan core.PlaybackEvent, 1)
	events <- core.PlaybackEvent{AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: core.PlaybackEventProgress}
	return &fakePlaybackSession{probe: player.probe, events: events}, nil
}

type fakePlaybackSession struct {
	probe  *fakePlayerProbe
	events chan core.PlaybackEvent
	once   sync.Once
}

func (session *fakePlaybackSession) Load(ctx context.Context, request core.PlayRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	session.probe.mu.Lock()
	session.probe.loads++
	session.probe.requests = append(session.probe.requests, request)
	session.probe.mu.Unlock()
	return nil
}
func (session *fakePlaybackSession) Events() <-chan core.PlaybackEvent { return session.events }
func (session *fakePlaybackSession) Close() error {
	session.once.Do(func() {
		session.probe.mu.Lock()
		session.probe.active--
		session.probe.clean = true
		session.probe.mu.Unlock()
		close(session.events)
	})
	return nil
}

type memoryStore struct {
	anime      map[core.AnimeID]core.Anime
	sourceRefs map[core.AnimeID][]core.SourceRef
	metadata   map[core.AnimeID]core.AnimeMetadata
	following  map[core.AnimeID]bool
	mappings   map[core.AnimeID][]core.EpisodeMapping
	history    []core.HistoryEntry
	progress   map[[2]string]core.PlaybackProgress
	settings   *core.Settings
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		anime:      map[core.AnimeID]core.Anime{},
		sourceRefs: map[core.AnimeID][]core.SourceRef{},
		metadata:   map[core.AnimeID]core.AnimeMetadata{},
		following:  map[core.AnimeID]bool{},
		mappings:   map[core.AnimeID][]core.EpisodeMapping{},
		progress:   map[[2]string]core.PlaybackProgress{},
	}
}

func (store *memoryStore) SaveAnime(_ context.Context, anime core.Anime) error {
	store.anime[anime.ID] = anime
	return nil
}
func (store *memoryStore) Anime(_ context.Context, id core.AnimeID) (core.Anime, error) {
	anime, ok := store.anime[id]
	if !ok {
		return core.Anime{}, core.ErrNotFound
	}
	return anime, nil
}
func (store *memoryStore) ListAnime(context.Context) ([]core.Anime, error) {
	items := make([]core.Anime, 0, len(store.anime))
	for _, item := range store.anime {
		items = append(items, item)
	}
	return items, nil
}
func (store *memoryStore) SaveSourceRef(_ context.Context, id core.AnimeID, ref core.SourceRef) error {
	store.sourceRefs[id] = append(store.sourceRefs[id], ref)
	return nil
}
func (store *memoryStore) SourceRefs(_ context.Context, id core.AnimeID) ([]core.SourceRef, error) {
	return append([]core.SourceRef(nil), store.sourceRefs[id]...), nil
}
func (store *memoryStore) SaveEpisodeMapping(_ context.Context, mapping core.EpisodeMapping) error {
	for _, existing := range store.mappings[mapping.AnimeID] {
		if existing.Ref == mapping.Ref {
			if existing.EpisodeID == mapping.EpisodeID {
				return nil
			}
			return errors.New("identity conflict")
		}
	}
	store.mappings[mapping.AnimeID] = append(store.mappings[mapping.AnimeID], mapping)
	return nil
}
func (store *memoryStore) EpisodeMappings(_ context.Context, animeID core.AnimeID) ([]core.EpisodeMapping, error) {
	items := make([]core.EpisodeMapping, 0, len(store.mappings[animeID]))
	return append(items, store.mappings[animeID]...), nil
}
func (store *memoryStore) SaveMetadata(_ context.Context, id core.AnimeID, metadata core.AnimeMetadata) error {
	store.metadata[id] = metadata
	return nil
}
func (store *memoryStore) Metadata(_ context.Context, id core.AnimeID) (core.AnimeMetadata, error) {
	metadata, ok := store.metadata[id]
	if !ok {
		return core.AnimeMetadata{}, core.ErrNotFound
	}
	return metadata, nil
}
func (store *memoryStore) SetFollowing(_ context.Context, id core.AnimeID, following bool) error {
	store.following[id] = following
	return nil
}
func (store *memoryStore) Following(context.Context) ([]core.AnimeID, error) {
	items := []core.AnimeID{}
	for id, following := range store.following {
		if following {
			items = append(items, id)
		}
	}
	return items, nil
}
func (store *memoryStore) AddHistory(_ context.Context, entry core.HistoryEntry) error {
	store.history = append(store.history, entry)
	return nil
}
func (store *memoryStore) History(context.Context) ([]core.HistoryEntry, error) {
	return append([]core.HistoryEntry(nil), store.history...), nil
}
func (store *memoryStore) RemoveHistory(_ context.Context, id core.AnimeID) error {
	kept := store.history[:0]
	for _, entry := range store.history {
		if entry.Progress.AnimeID != id {
			kept = append(kept, entry)
		}
	}
	store.history = kept
	return nil
}
func progressKey(animeID core.AnimeID, episodeID core.EpisodeID) [2]string {
	return [2]string{string(animeID), string(episodeID)}
}
func (store *memoryStore) SaveProgress(_ context.Context, progress core.PlaybackProgress) error {
	store.progress[progressKey(progress.AnimeID, progress.EpisodeID)] = progress
	return nil
}
func (store *memoryStore) SavePlaybackCheckpoint(_ context.Context, entry core.HistoryEntry) error {
	store.progress[progressKey(entry.Progress.AnimeID, entry.Progress.EpisodeID)] = entry.Progress
	store.history = append(store.history, entry)
	return nil
}
func (store *memoryStore) Progress(_ context.Context, animeID core.AnimeID, episodeID core.EpisodeID) (core.PlaybackProgress, error) {
	progress, ok := store.progress[progressKey(animeID, episodeID)]
	if !ok {
		return core.PlaybackProgress{}, core.ErrNotFound
	}
	return progress, nil
}
func (store *memoryStore) SaveSettings(_ context.Context, settings core.Settings) error {
	store.settings = &settings
	return nil
}
func (store *memoryStore) Settings(context.Context) (core.Settings, error) {
	if store.settings == nil {
		return core.DefaultSettings(), nil
	}
	return *store.settings, nil
}

var (
	_ core.AnimeSource      = (*fakeAnimeSource)(nil)
	_ core.AnimeSource      = (*unsupportedAnimeSource)(nil)
	_ core.MetadataProvider = (*fakeMetadataProvider)(nil)
	_ core.MetadataProvider = (*unsupportedMetadataProvider)(nil)
	_ core.Player           = (*fakePlayer)(nil)
	_ core.PlaybackSession  = (*fakePlaybackSession)(nil)
	_ core.Store            = (*memoryStore)(nil)
)

func TestAnimeSourceContract(t *testing.T) {
	RunAnimeSource(t, AnimeSourceSuite{
		New:      func(*testing.T) core.AnimeSource { return &fakeAnimeSource{} },
		Catalog:  SourceListCase{Supported: true, Expected: []core.SourceRef{fakeAnimeRef}},
		Search:   SourceSearchCase{Supported: true, Query: "Anime", Expected: []core.SourceRef{fakeAnimeRef}},
		Episodes: SourceEpisodesCase{Supported: true, Anime: fakeAnimeRef, Expected: []core.EpisodeRef{fakeEpisode1, fakeEpisode2}},
		Resolve:  SourceResolveCase{Supported: true, Episode: fakeEpisode1},
		Schedule: SourceScheduleCase{Supported: true, Expected: []core.SourceScheduleItem{
			{Anime: core.SourceAnime{Ref: fakeAnimeRef, Title: "Anime"}, Episode: core.SourceEpisode{Ref: fakeEpisode1, Number: "1"}, AirsAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC), Precision: core.SchedulePrecisionTime},
			{Anime: core.SourceAnime{Ref: fakeAnimeRef, Title: "Anime"}, Episode: core.SourceEpisode{Ref: fakeEpisode2, Number: "2"}, AirsAt: time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC), Precision: core.SchedulePrecisionTime},
		}},
		ForbiddenStrings: []string{"raw-secret"},
	})
}

func TestUnsupportedAnimeSourceContract(t *testing.T) {
	RunAnimeSource(t, AnimeSourceSuite{New: func(*testing.T) core.AnimeSource { return &unsupportedAnimeSource{} }})
}

func TestMetadataProviderContract(t *testing.T) {
	RunMetadataProvider(t, MetadataProviderSuite{
		New:              func(*testing.T) core.MetadataProvider { return &fakeMetadataProvider{} },
		Search:           MetadataSearchCase{Supported: true, Query: core.MetadataQuery{Title: "Anime"}, Expected: []core.MetadataRef{fakeMetaRef}},
		Get:              MetadataGetCase{Supported: true, Ref: fakeMetaRef, Expected: fakeMetadata()},
		Missing:          &MetadataMissingCase{Ref: core.MetadataRef{Provider: fakeMetaRef.Provider, ID: "missing"}, Expected: core.ErrNotFound},
		ForbiddenStrings: []string{"raw-secret", "<script>"},
	})
}

func TestUnsupportedMetadataProviderContract(t *testing.T) {
	RunMetadataProvider(t, MetadataProviderSuite{New: func(*testing.T) core.MetadataProvider { return &unsupportedMetadataProvider{} }})
}

func TestPlayerContract(t *testing.T) {
	first := core.PlayRequest{AnimeID: "anime", EpisodeID: "episode-1", Source: core.NewPlaybackSource("http://127.0.0.1/one", nil)}
	second := core.PlayRequest{AnimeID: "anime", EpisodeID: "episode-2", Source: core.NewPlaybackSource("http://127.0.0.1/two", nil), StartAt: time.Minute}
	RunPlayer(t, PlayerSuite{New: func(*testing.T) (core.Player, PlayerProbe) {
		probe := &fakePlayerProbe{}
		return &fakePlayer{probe: probe}, probe
	}, First: first, Second: second, CheckCancellation: true, Timeout: 200 * time.Millisecond})
}

func TestStoreContract(t *testing.T) {
	RunStore(t, StoreSuite{New: func(*testing.T) core.Store { return newMemoryStore() }})
}
