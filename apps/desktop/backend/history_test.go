// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

func TestHistorySurvivesRestartAndRemovalWorksOffline(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "history.sqlite")
	store, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	parent := core.SourceRef{Provider: "anime1", ID: "show"}
	for _, anime := range []core.Anime{{ID: "kept", Title: "保留"}, {ID: "removed", Title: "移除"}} {
		if err := store.SaveAnime(ctx, anime); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveSourceRef(ctx, "kept", parent); err != nil {
		t.Fatal(err)
	}
	source := &followingSource{episodes: []core.SourceEpisode{{Ref: core.EpisodeRef{Anime: parent, ID: "e5"}, Number: "5"}}}
	service := newWithDependencies(dependencies{store: store, source: source})
	if _, err := service.Episodes(ctx, "kept"); err != nil {
		t.Fatal(err)
	}
	mappings, err := store.EpisodeMappings(ctx, "kept")
	if err != nil || len(mappings) != 1 {
		t.Fatalf("mappings = %#v, %v", mappings, err)
	}
	keptEpisode := mappings[0].EpisodeID
	played := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	checkpoints := []core.HistoryEntry{
		{Progress: core.PlaybackProgress{AnimeID: "kept", EpisodeID: keptEpisode, Position: 90 * time.Second, Duration: 24 * time.Minute, UpdatedAt: played}, LastPlayedAt: played},
		{Progress: core.PlaybackProgress{AnimeID: "removed", EpisodeID: "episode", Position: time.Minute, Duration: 24 * time.Minute, UpdatedAt: played}, LastPlayedAt: played.Add(-time.Hour)},
	}
	for _, entry := range checkpoints {
		if err := store.SavePlaybackCheckpoint(ctx, entry); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	// Restart with the source unreachable: history is local only.
	reopened, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	offline := &followingSource{err: errors.New("anime1 unreachable")}
	service = newWithDependencies(dependencies{store: reopened, source: offline})
	t.Cleanup(func() { _ = service.Close() })
	if err := service.RemoveHistory(ctx, "removed"); err != nil {
		t.Fatal(err)
	}
	want := History{
		AnimeID: "kept", EpisodeID: string(keptEpisode), EpisodeNumber: "5", Position: 90000, Duration: 1440000,
		UpdatedAt: "2026-10-19T12:00:00Z", LastPlayed: "2026-10-19T12:00:00Z",
	}
	if got, err := service.History(ctx); err != nil || len(got) != 1 || got[0] != want {
		t.Fatalf("history after restart = %#v, %v, want %#v", got, err, want)
	}
	if _, err := reopened.Progress(ctx, "removed", "episode"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("removed progress error = %v", err)
	}
	if progress, err := reopened.Progress(ctx, "kept", keptEpisode); err != nil || progress.Position != 90*time.Second {
		t.Fatalf("kept progress = %#v, %v", progress, err)
	}
	if offline.calls != 0 {
		t.Fatalf("History called the source %d times", offline.calls)
	}
}
