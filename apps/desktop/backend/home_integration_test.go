// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

func TestHomeBindingsReadSQLiteAfterRestartWithoutSource(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "home.sqlite")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	anime := core.Anime{ID: "local-anime", Title: "本機動畫"}
	if err := store.SaveAnime(ctx, anime); err != nil {
		t.Fatal(err)
	}
	if err := store.SetFollowing(ctx, anime.ID, true); err != nil {
		t.Fatal(err)
	}
	played := time.Date(2026, time.September, 12, 12, 0, 0, 0, time.UTC)
	entry := core.HistoryEntry{
		Progress: core.PlaybackProgress{
			AnimeID: anime.ID, EpisodeID: "opaque-episode", Position: 125500 * time.Millisecond,
			Duration: 24 * time.Minute, UpdatedAt: played,
		},
		LastPlayedAt: played,
	}
	if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	uncallableSource := &struct{ core.AnimeSource }{}
	service := newWithDependencies(dependencies{store: reopened, source: uncallableSource})
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	library, err := service.Library(ctx)
	if err != nil || len(library) != 1 || library[0].ID != string(anime.ID) || library[0].Title != anime.Title {
		t.Fatalf("cached library = %#v, error = %v", library, err)
	}
	history, err := service.History(ctx)
	wantHistory := History{
		AnimeID: "local-anime", EpisodeID: "opaque-episode", Position: 125500, Duration: 1440000,
		UpdatedAt: "2026-09-12T12:00:00Z", LastPlayed: "2026-09-12T12:00:00Z",
	}
	if err != nil || len(history) != 1 || history[0] != wantHistory {
		t.Fatalf("cached history = %#v, error = %v", history, err)
	}
	following, err := service.Following(ctx)
	if err != nil || len(following) != 1 {
		t.Fatalf("cached following = %#v, error = %v", following, err)
	}
	want := Following{AnimeID: string(anime.ID), HasWatched: true, LatestWatched: string(entry.Progress.EpisodeID)}
	if following[0] != want {
		t.Fatalf("cached following = %#v, want %#v", following[0], want)
	}
}
