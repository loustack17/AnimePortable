// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

// followingSource serves a fixed episode list, or fails when err is set.
type followingSource struct {
	core.AnimeSource
	mu       sync.Mutex
	episodes []core.SourceEpisode
	err      error
	calls    int
}

func (source *followingSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.calls++
	return source.episodes, source.err
}

func TestFollowingSurvivesRestartAndSourceOutage(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "following.sqlite")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	anime := core.Anime{ID: "local-anime", Title: "追蹤測試"}
	parent := core.SourceRef{Provider: "anime1", ID: "show"}
	if err := store.SaveAnime(ctx, anime); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceRef(ctx, anime.ID, parent); err != nil {
		t.Fatal(err)
	}
	episode := func(number string) core.SourceEpisode {
		return core.SourceEpisode{Ref: core.EpisodeRef{Anime: parent, ID: "e" + number}, Number: number}
	}
	// Newest first, as Anime1 lists some series.
	source := &followingSource{episodes: []core.SourceEpisode{episode("2"), episode("1")}}
	service := newWithDependencies(dependencies{store: store, source: source})

	if err := service.Follow(ctx, string(anime.ID)); err != nil {
		t.Fatal(err)
	}
	if detail, err := service.Detail(ctx, string(anime.ID)); err != nil || !detail.Following {
		t.Fatalf("detail after follow = %#v, %v", detail, err)
	}
	if _, err := service.Episodes(ctx, string(anime.ID)); err != nil {
		t.Fatal(err)
	}
	mappings, err := store.EpisodeMappings(ctx, anime.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]core.EpisodeID)
	for _, mapping := range mappings {
		ids[mapping.Number] = mapping.EpisodeID
	}
	played := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	if err := store.SavePlaybackCheckpoint(ctx, core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: anime.ID, EpisodeID: ids["2"], Position: time.Minute, Duration: 24 * time.Minute, UpdatedAt: played}, LastPlayedAt: played}); err != nil {
		t.Fatal(err)
	}
	upToDate := Following{AnimeID: string(anime.ID), LatestAvailable: string(ids["2"]), LatestAvailableNumber: "2", LatestWatched: string(ids["2"]), LatestWatchedNumber: "2", HasAvailable: true, HasWatched: true}
	got, err := service.Following(ctx)
	if err != nil || len(got) != 1 || got[0].LatestSeenAt == "" {
		t.Fatalf("following = %#v, %v", got, err)
	}
	// The seen time is stamped at ingest; the rest is exact.
	upToDate.LatestSeenAt = got[0].LatestSeenAt
	if got[0] != upToDate {
		t.Fatalf("following = %#v, want %#v", got[0], upToDate)
	}

	// A new episode airs.
	source.mu.Lock()
	source.episodes = []core.SourceEpisode{episode("3"), episode("2"), episode("1")}
	source.mu.Unlock()
	if _, err := service.Episodes(ctx, string(anime.ID)); err != nil {
		t.Fatal(err)
	}
	// Episode 3 appeared after the anime was followed and is not watched: a recent update.
	got, err = service.Following(ctx)
	if err != nil || len(got) != 1 || !got[0].NewEpisode || !got[0].RecentlyUpdated || got[0].LatestAvailableNumber != "3" || got[0].LatestWatchedNumber != "2" {
		t.Fatalf("following after new episode = %#v, %v", got, err)
	}
	withNewEpisode := got[0]
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Restart with the source unreachable.
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	offline := &followingSource{err: errors.New("anime1 unreachable")}
	service = newWithDependencies(dependencies{store: reopened, source: offline})
	t.Cleanup(func() { _ = service.Close() })
	if got, err := service.Following(ctx); err != nil || len(got) != 1 || got[0] != withNewEpisode {
		t.Fatalf("offline following = %#v, %v, want %#v", got, err, withNewEpisode)
	}
	if offline.calls != 0 {
		t.Fatalf("Following called the source %d times", offline.calls)
	}
	if _, err := service.Episodes(ctx, string(anime.ID)); err == nil {
		t.Fatal("episode refresh with the source down reported success")
	}
	if got, err := service.Following(ctx); err != nil || len(got) != 1 || got[0] != withNewEpisode {
		t.Fatalf("following after failed refresh = %#v, %v, want %#v", got, err, withNewEpisode)
	}

	if err := service.Unfollow(ctx, string(anime.ID)); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Following(ctx); err != nil || len(got) != 0 {
		t.Fatalf("following after unfollow = %#v, %v", got, err)
	}
	if detail, err := service.Detail(ctx, string(anime.ID)); err != nil || detail.Following {
		t.Fatalf("detail after unfollow = %#v, %v", detail, err)
	}
}
