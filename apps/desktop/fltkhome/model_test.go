// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

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

func TestSelectHomeRowsUsesLatestStartedOrFinishedEpisodePerAnime(t *testing.T) {
	history := []backend.History{
		{AnimeID: "same", EpisodeID: "old", Position: 1000, Duration: 10000, LastPlayed: "2026-01-01T00:00:00Z"},
		{AnimeID: "same", EpisodeID: "new", Position: 2000, Duration: 10000, LastPlayed: "2026-02-01T00:00:00Z"},
		{AnimeID: "completed", EpisodeID: "done", Position: 10000, Duration: 10000, LastPlayed: "2026-03-01T00:00:00Z"},
		{AnimeID: "unstarted", EpisodeID: "zero", Position: 0, LastPlayed: "2026-03-02T00:00:00Z"},
		{AnimeID: "same", EpisodeID: "opened-not-watched", Position: 0, Duration: 10000, LastPlayed: "2026-03-03T00:00:00Z"},
		// Finishing the newest episode shows it, not an older unfinished one (owner decision 2026-10-21).
		{AnimeID: "series", EpisodeID: "ep-08", Position: 10000, Duration: 1440000, LastPlayed: "2026-01-02T00:00:00Z"},
		{AnimeID: "series", EpisodeID: "ep-12", Position: 1422000, Duration: 1436000, Completed: true, LastPlayed: "2026-01-03T00:00:00Z"},
	}
	rows := selectHomeRows(history, nil, rowLimit)
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.History.EpisodeID)
	}
	if want := []string{"done", "new", "ep-12"}; !slices.Equal(got, want) {
		t.Fatalf("Home rows = %v, want %v", got, want)
	}
}

func TestHomeShowsFinishedEpisodeAsWatchedAndReplaysItFromTheStart(t *testing.T) {
	item := backend.History{AnimeID: "series", EpisodeID: "ep-12", EpisodeNumber: "12", Position: 1422000, Duration: 1436000, Completed: true}
	if got := homeCaption(homeRow{History: item}); got != "集數：12 · 已看完" {
		t.Fatalf("finished caption = %q", got)
	}
	if got := historyPlayRequest(item); got != (backend.PlayRequest{AnimeID: "series", EpisodeID: "ep-12"}) {
		t.Fatalf("finished play request = %#v, want a start from the beginning", got)
	}
}

func TestFollowingRowsUseKnownNamesAndSortByTitleOnly(t *testing.T) {
	// A series with unwatched episodes is not moved up or marked (owner decision 2026-10-19).
	items := []backend.Following{{AnimeID: "known"}, {AnimeID: "missing"}, {AnimeID: "new", NewEpisode: true}}
	library := []backend.Anime{{ID: "known", Title: "  乙作品  "}, {ID: "new", Title: "戊作品"}}
	got := followingRows(items, library, nil)
	if len(got) != 3 || got[0].Anime.Title != "乙作品" || got[1].Anime.ID != "new" || got[2].Anime.ID != "missing" || got[2].Anime.Title != "未知作品" {
		t.Fatalf("following rows = %#v", got)
	}
}

func TestFollowingStatusDescribesEachEpisodeState(t *testing.T) {
	for _, test := range []struct {
		item backend.Following
		want string
	}{
		{backend.Following{}, "尚未觀看"},
		{backend.Following{HasAvailable: true, LatestAvailableNumber: "12"}, "尚未觀看"},
		{backend.Following{AnimeID: "a", HasAvailable: true, LatestAvailableNumber: "12", HasWatched: true, LatestWatched: "ep-10", LatestWatchedNumber: "10", NewEpisode: true}, "看到：第 10 集 · 7:52 / 24:00"},
		{backend.Following{AnimeID: "a", HasWatched: true, LatestWatched: "ep-1", LatestWatchedNumber: "01"}, "看到：第 01 集 · 已看完"},
		{backend.Following{AnimeID: "a", HasWatched: true, LatestWatched: "no-history"}, "看到：集數未知"},
		{backend.Following{AnimeID: "a", HasWatched: true, LatestWatched: "opened", LatestWatchedNumber: "11"}, "看到：第 11 集"},
		{backend.Following{AnimeID: "a", HasWatched: true, LatestWatched: "no-duration", LatestWatchedNumber: "3"}, "看到：第 3 集 · 7:52"},
	} {
		history := []backend.History{
			{AnimeID: "other", EpisodeID: "ep-10", Position: 1000},
			{AnimeID: "a", EpisodeID: "ep-10", Position: 472000, Duration: 1440000},
			{AnimeID: "a", EpisodeID: "ep-1", Position: 1440000, Duration: 1440000, Completed: true},
			{AnimeID: "a", EpisodeID: "opened", Duration: 1440000},
			{AnimeID: "a", EpisodeID: "no-duration", Position: 472000},
		}
		if got := followingStatus(test.item, history); got != test.want {
			t.Errorf("followingStatus(%#v) = %q, want %q", test.item, got, test.want)
		}
	}
}

func TestStartupAndHomeErrorsAreRedacted(t *testing.T) {
	private := errors.New("secret C:/Users/example/player.exe token=hidden")
	for _, message := range []string{
		startupErrorMessage(private),
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

func TestHomeCaptionUsesStoredEpisodeNumberAndSafeFallback(t *testing.T) {
	if got := homeCaption(homeRow{History: backend.History{EpisodeNumber: " 04 ", Position: 125000}}); got != "集數：04 · 上次播放位置 2:05" {
		t.Fatalf("stored number caption = %q", got)
	}
	for _, number := range []string{"", "  "} {
		if got := homeCaption(homeRow{History: backend.History{EpisodeNumber: number, Position: 754000}}); got != "集數：未知 · 上次播放位置 12:34" {
			t.Fatalf("fallback caption for %q = %q", number, got)
		}
	}
}

func TestRecentUpdateRowsFilterOrderLimitAndCaption(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	seen := func(hours int) string { return now.Add(-time.Duration(hours) * time.Hour).Format(time.RFC3339Nano) }
	following := []backend.Following{
		{AnimeID: "old", LatestAvailableNumber: "3", RecentlyUpdated: true, LatestSeenAt: seen(50)},
		{AnimeID: "today", LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: seen(2)},
		{AnimeID: "not-recent", LatestAvailableNumber: "5", LatestSeenAt: seen(1)},
		{AnimeID: "yesterday", LatestAvailableNumber: "SP", RecentlyUpdated: true, LatestSeenAt: seen(30)},
		{AnimeID: "bad-time", RecentlyUpdated: true, LatestSeenAt: "not a time"},
	}
	library := []backend.Anime{{ID: "old", Title: "舊"}, {ID: "today", Title: "今天"}, {ID: "yesterday", Title: "昨天"}}
	rows := recentUpdateRows(following, library, 6, now)
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		got = append(got, row.Anime.Title+"|"+row.Status)
	}
	want := []string{"今天|第 12 集 · 今天", "昨天|第 SP 集 · 昨天", "舊|第 3 集 · 2 天前"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %#v, want %#v", got, want)
	}
	if limited := recentUpdateRows(following, library, 2, now); len(limited) != 2 || limited[0].Anime.ID != "today" {
		t.Fatalf("limited rows = %#v", limited)
	}
	if recentUpdateEmptyText(nil) == recentUpdateEmptyText(following) {
		t.Fatal("empty text does not tell no follows from no updates")
	}
}

func TestDaysAgoCountsLocalCalendarDays(t *testing.T) {
	now := time.Date(2026, 10, 14, 8, 0, 0, 0, time.Local)
	for _, test := range []struct {
		at   time.Time
		want string
	}{
		{now.Add(-time.Hour), "今天"},
		{time.Date(2026, 10, 13, 23, 0, 0, 0, time.Local), "昨天"},
		{time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local), "2 天前"},
	} {
		if got := daysAgo(now, test.at); got != test.want {
			t.Errorf("daysAgo(%v) = %q, want %q", test.at, got, test.want)
		}
	}
}
