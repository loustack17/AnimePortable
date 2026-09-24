// SPDX-License-Identifier: MPL-2.0

package native

import (
	"testing"

	"animeportable/apps/desktop/backend"
)

func TestLatestHistoryFiltersNewestPerAnimeAndLimitsResults(t *testing.T) {
	items := []backend.History{
		{AnimeID: "completed", EpisodeID: "old", Position: 1000, LastPlayed: "2026-09-12T10:00:00Z"},
		{AnimeID: "completed", EpisodeID: "new", Position: 1000, Completed: true, LastPlayed: "2026-09-12T11:00:00Z"},
		{AnimeID: "zero", EpisodeID: "zero", LastPlayed: "2026-09-12T12:00:00Z"},
		{AnimeID: "at-end", EpisodeID: "end", Position: 10000, Duration: 10000, LastPlayed: "2026-09-12T13:00:00Z"},
		{AnimeID: "newest", EpisodeID: "new", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.910Z"},
		{AnimeID: "newest", EpisodeID: "old", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.900Z"},
		{AnimeID: "second", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.900Z"},
		{AnimeID: "third", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.800Z"},
		{AnimeID: "fourth", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.700Z"},
		{AnimeID: "fifth", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.600Z"},
		{AnimeID: "sixth", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.500Z"},
		{AnimeID: "seventh", EpisodeID: "episode", Position: 500, Duration: 1000, LastPlayed: "2026-09-12T14:00:00.400Z"},
	}

	got := latestHistory(items)
	want := []string{"newest:new", "second:episode", "third:episode", "fourth:episode", "fifth:episode", "sixth:episode"}
	if len(got) != len(want) {
		t.Fatalf("latestHistory returned %d rows, want %d: %#v", len(got), len(want), got)
	}
	for index, item := range got {
		if key := item.AnimeID + ":" + item.EpisodeID; key != want[index] {
			t.Errorf("latestHistory[%d] = %q, want %q", index, key, want[index])
		}
	}
}

func TestLatestHistoryAllowsUnknownDurationAndFiltersInvalidProgress(t *testing.T) {
	items := []backend.History{
		{AnimeID: "unknown-duration", EpisodeID: "valid", Position: 1, LastPlayed: "2026-09-12T10:00:00Z"},
		{AnimeID: "negative-duration", EpisodeID: "valid", Position: 1, Duration: -1, LastPlayed: "2026-09-12T09:00:00Z"},
		{AnimeID: "negative-position", EpisodeID: "invalid", Position: -1, LastPlayed: "2026-09-12T08:00:00Z"},
	}

	got := latestHistory(items)
	if len(got) != 2 || got[0].AnimeID != "unknown-duration" || got[1].AnimeID != "negative-duration" {
		t.Fatalf("latestHistory = %#v, want unknown and non-positive duration entries only", got)
	}
}
