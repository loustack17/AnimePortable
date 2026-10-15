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
		{AnimeID: "same", EpisodeID: "opened-not-watched", Position: 0, Duration: 10000, LastPlayed: "2026-03-03T00:00:00Z"},
	}
	rows := selectHomeRows(history, nil, rowLimit)
	if len(rows) != 1 || rows[0].History.EpisodeID != "new" {
		t.Fatalf("resumable rows = %#v, want latest episode only", rows)
	}
}

func TestFollowingRowsBoundUseKnownNamesAndPutNewEpisodesFirst(t *testing.T) {
	items := []backend.Following{{AnimeID: "known"}, {AnimeID: "missing"}, {AnimeID: "new", NewEpisode: true}}
	library := []backend.Anime{{ID: "known", Title: "  作品名稱  "}, {ID: "new", Title: "新作品"}}
	got := followingRows(items, library, 2)
	if len(got) != 2 || got[0].Anime.ID != "new" || !got[0].New || got[1].Anime.Title != "作品名稱" || got[1].New {
		t.Fatalf("following rows = %#v", got)
	}
	all := followingRows(items, library, rowLimit)
	if len(all) != 3 || all[2].Anime.ID != "missing" || all[2].Anime.Title != "未知作品" {
		t.Fatalf("following rows = %#v", all)
	}
	if got := followingRows(items, library, 0); got != nil {
		t.Fatalf("zero limit returned %#v, want nil", got)
	}
}

func TestFollowingStatusDescribesEachEpisodeState(t *testing.T) {
	for _, test := range []struct {
		item backend.Following
		want string
	}{
		{backend.Following{}, "最新：尚未取得集數 · 尚未觀看"},
		{backend.Following{HasAvailable: true, LatestAvailableNumber: "12"}, "最新：第 12 集 · 尚未觀看"},
		{backend.Following{HasAvailable: true, LatestAvailableNumber: "12", HasWatched: true, LatestWatchedNumber: "10", NewEpisode: true}, "最新：第 12 集 · 看到：第 10 集"},
		{backend.Following{HasWatched: true}, "最新：尚未取得集數 · 看到：集數未知"},
	} {
		if got := followingStatus(test.item); got != test.want {
			t.Errorf("followingStatus(%#v) = %q, want %q", test.item, got, test.want)
		}
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

func TestEpisodeNumberUsesExactIDAndSafeFallback(t *testing.T) {
	episodes := []backend.Episode{
		{ID: "provider:episode-4", Number: "04"},
		{ID: "provider:episode-40", Number: "40"},
	}
	if got := episodeNumber(episodes, "provider:episode-4", nil); got != "04" {
		t.Fatalf("exact episode number = %q, want 04", got)
	}
	for _, test := range []struct {
		name string
		id   string
		err  error
	}{
		{name: "no match", id: "provider:missing"},
		{name: "failed lookup", id: "provider:episode-4", err: errors.New("source unavailable")},
		{name: "empty id"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := episodeNumber(episodes, test.id, test.err); got != "未知" {
				t.Fatalf("episode fallback = %q, want 未知", got)
			}
		})
	}
	if got := homeCaption(homeRow{History: backend.History{Position: 754000}}); got != "集數：未知 · 上次播放位置 12:34" {
		t.Fatalf("fallback caption = %q", got)
	}
}
