// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"strings"
	"testing"

	"animeportable/apps/desktop/backend"
)

func TestSelectHomeRowsSortsBoundsAndUsesSafeTitleFallback(t *testing.T) {
	history := []backend.History{
		{AnimeID: "old", EpisodeID: "episode-old", Position: 1000, LastPlayed: "2026-01-01T00:00:00Z"},
		{AnimeID: "new", EpisodeID: "episode-new", Position: 9000, LastPlayed: "2026-02-01T00:00:00Z"},
		{AnimeID: "unknown", EpisodeID: "episode-unknown", Position: 1000, LastPlayed: "2025-12-01T00:00:00Z"},
	}
	rows := selectHomeRows(history, []backend.Anime{{ID: "new", Title: "New title"}, {ID: "old", Title: "Old title"}}, 2)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want bounded 2", len(rows))
	}
	if rows[0].History.EpisodeID != "episode-new" || rows[0].Title != "New title" {
		t.Fatalf("first row = %#v, want latest known item", rows[0])
	}
	if rows[1].History.EpisodeID != "episode-old" {
		t.Fatalf("second row = %#v, want next latest item", rows[1])
	}
	if got := selectHomeRows(history, nil, 0); got != nil {
		t.Fatalf("zero limit returned %#v, want nil", got)
	}
	fallback := selectHomeRows([]backend.History{{AnimeID: "missing", Position: 1000}}, nil, 1)
	if fallback[0].Title != "未知作品" {
		t.Fatalf("missing title = %q, want safe fallback", fallback[0].Title)
	}
}

func TestSelectHomeRowsUsesOnlyLatestResumableEpisodePerAnime(t *testing.T) {
	history := []backend.History{
		{AnimeID: "same", EpisodeID: "old", Position: 1000, Duration: 10000, LastPlayed: "2026-01-01T00:00:00Z"},
		{AnimeID: "same", EpisodeID: "new", Position: 2000, Duration: 10000, LastPlayed: "2026-02-01T00:00:00Z"},
		{AnimeID: "completed", EpisodeID: "done", Position: 10000, Duration: 10000, LastPlayed: "2026-03-01T00:00:00Z"},
		{AnimeID: "unstarted", EpisodeID: "zero", Position: 0, LastPlayed: "2026-03-02T00:00:00Z"},
	}
	rows := selectHomeRows(history, nil, rowLimit)
	if len(rows) != 1 || rows[0].History.EpisodeID != "new" {
		t.Fatalf("resumable rows = %#v, want latest episode only", rows)
	}
}

func TestFollowingTitlesBoundsAndUsesKnownNames(t *testing.T) {
	items := []backend.Following{{AnimeID: "known"}, {AnimeID: "missing"}, {AnimeID: "ignored"}}
	got := followingTitles(items, []backend.Anime{{ID: "known", Title: "  作品名稱  "}}, 2)
	if len(got) != 2 || got[0] != "作品名稱" || got[1] != "未知作品" {
		t.Fatalf("following titles = %#v", got)
	}
}

func TestStartupAndHomeErrorsAreRedacted(t *testing.T) {
	private := errors.New("secret C:/Users/example/player.exe token=hidden")
	for _, message := range []string{
		startupErrorMessage(private, false, false),
		startupErrorMessage(private, true, false),
		startupErrorMessage(private, false, true),
		homeErrorMessage(private),
	} {
		if strings.Contains(message, "C:/Users") || strings.Contains(message, "hidden") || strings.Contains(message, "secret") {
			t.Fatalf("error message contains private details: %q", message)
		}
	}
	if got := homeErrorMessage(nil); got != "" {
		t.Fatalf("nil Home error = %q, want empty", got)
	}
}

func TestHomeSelectionPreservesOpaqueIDsAndResumePosition(t *testing.T) {
	item := backend.History{AnimeID: "local:anime-7", EpisodeID: "provider:opaque/id", Position: 125500}
	rows := selectHomeRows([]backend.History{item}, []backend.Anime{{ID: item.AnimeID, Title: "標題"}}, 1)
	selected := rows[0].History
	request := playRequest(selected)
	if want := (backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: item.Position}); request != want {
		t.Fatalf("play request = %#v, want typed IDs and resume position %#v", request, want)
	}
	if got := formatPosition(item.Position); got != "2:05" {
		t.Fatalf("formatted position = %q, want 2:05", got)
	}
	if got := playRequest(backend.History{AnimeID: "anime", EpisodeID: "episode", Position: -1}).StartAt; got != 0 {
		t.Fatalf("negative resume position = %d, want clamped to zero", got)
	}
}
