// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"reflect"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
)

func TestHistoryRowsListNewestEpisodePerAnimeIncludingFinished(t *testing.T) {
	now := time.Date(2026, 10, 19, 20, 0, 0, 0, time.Local)
	at := func(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
	history := []backend.History{
		{AnimeID: "a", EpisodeID: "a-1", EpisodeNumber: "1", Position: 60000, Duration: 1440000, LastPlayed: at(now.Add(-3 * time.Hour))},
		{AnimeID: "a", EpisodeID: "a-2", EpisodeNumber: "2", Position: 90000, Duration: 1440000, LastPlayed: at(now.Add(-time.Hour))},
		{AnimeID: "done", EpisodeID: "d-12", EpisodeNumber: "12", Position: 1440000, Duration: 1440000, Completed: true, LastPlayed: at(now.AddDate(0, 0, -1))},
		{AnimeID: "unknown", EpisodeID: "u", Position: 30000, LastPlayed: at(now.AddDate(0, 0, -5))},
		{AnimeID: "opened", EpisodeID: "o", EpisodeNumber: "SP", LastPlayed: "not a time"},
	}
	rows := historyRows(history, []backend.Anime{{ID: "a", Title: " 甲 "}, {ID: "done", Title: "乙"}}, now)
	want := []historyRow{
		{History: history[1], Title: "甲", Status: "第 2 集 · 1:30 / 24:00 · 上次觀看：今天"},
		{History: history[2], Title: "乙", Status: "第 12 集 · 已看完 · 上次觀看：昨天"},
		{History: history[3], Title: "未知作品", Status: "集數未知 · 0:30 · 上次觀看：5 天前"},
		{History: history[4], Title: "未知作品", Status: "第 SP 集 · 尚未開始"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows = %#v\nwant %#v", rows, want)
	}
}

func TestHistoryPlayRequestResumesOnlyResumableEntries(t *testing.T) {
	tests := []struct {
		name string
		item backend.History
		want int64
	}{
		{"partly watched resumes", backend.History{Position: 90000, Duration: 1440000}, 90000},
		{"unknown duration resumes", backend.History{Position: 90000}, 90000},
		{"finished restarts", backend.History{Position: 1440000, Duration: 1440000, Completed: true}, 0},
		{"at the end restarts", backend.History{Position: 1440000, Duration: 1440000}, 0},
		{"unstarted starts at zero", backend.History{}, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.item.AnimeID, test.item.EpisodeID = "anime", "episode"
			want := backend.PlayRequest{AnimeID: "anime", EpisodeID: "episode", StartAt: test.want}
			if got := historyPlayRequest(test.item); got != want {
				t.Fatalf("request = %#v, want %#v", got, want)
			}
		})
	}
}
