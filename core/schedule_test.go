// SPDX-License-Identifier: MPL-2.0

package core

import (
	"reflect"
	"testing"
	"time"
)

func TestMatchScheduleUsesUniqueTitlesAndKeepsStoreMatches(t *testing.T) {
	library := []Anime{
		{ID: "spy", Title: "SPY×FAMILY 間諜家家酒"},
		{ID: "frieren", Title: "葬送的芙莉蓮", NativeTitle: "葬送のフリーレン"},
		{ID: "twin-a", Title: "同名作品"},
		{ID: "twin-b", Title: "同名作品"},
	}
	entries := []ScheduleEntry{
		// Spacing and a standalone x do not matter.
		{Ref: MetadataRef{Provider: "bangumi", ID: "1"}, Title: "SPY x FAMILY間諜家家酒", Weekday: time.Saturday},
		// The native title matches when the Chinese one differs.
		{Ref: MetadataRef{Provider: "bangumi", ID: "2"}, Title: "葬送的芙莉莲", NativeTitle: "葬送のフリーレン", Weekday: time.Friday},
		// Two anime share this title: no guess.
		{Ref: MetadataRef{Provider: "bangumi", ID: "3"}, Title: "同名作品", Weekday: time.Monday},
		// Unknown to the app.
		{Ref: MetadataRef{Provider: "bangumi", ID: "4"}, Title: "新作品", Weekday: time.Monday},
		// The store already matched it by metadata; titles must not override that.
		{Ref: MetadataRef{Provider: "bangumi", ID: "5"}, Title: "葬送的芙莉蓮", Weekday: time.Sunday, AnimeID: "stored"},
	}
	got := MatchSchedule(entries, library)
	want := []AnimeID{"spy", "frieren", "", "", "stored"}
	for index, entry := range got {
		if entry.AnimeID != want[index] {
			t.Fatalf("entry %d AnimeID = %q, want %q", index, entry.AnimeID, want[index])
		}
	}
	if entries[0].AnimeID != "" {
		t.Fatal("MatchSchedule changed its input")
	}
	got[0].AnimeID = ""
	if !reflect.DeepEqual(got[0], entries[0]) {
		t.Fatalf("matched entry changed other fields: %#v", got[0])
	}
}

func TestMatchScheduleIgnoresBlankTitles(t *testing.T) {
	got := MatchSchedule([]ScheduleEntry{{Title: "名稱"}}, []Anime{{ID: "blank", Title: " ", NativeTitle: ""}})
	if got[0].AnimeID != "" {
		t.Fatalf("blank library title matched: %#v", got[0])
	}
}

func TestMatchScheduleKeepsSeasonsApart(t *testing.T) {
	library := []Anime{{ID: "s1", Title: "轉生就是劍"}, {ID: "s2", Title: "轉生就是劍 第二季"}}
	got := MatchSchedule([]ScheduleEntry{{Title: "轉生就是劍 第2期"}, {Title: "轉生就是劍 第三季"}}, library)
	if got[0].AnimeID != "s2" || got[1].AnimeID != "" {
		t.Fatalf("matches = %q, %q; want s2 and none", got[0].AnimeID, got[1].AnimeID)
	}
}

func TestScheduleSearchQueryDropsSuffixes(t *testing.T) {
	for title, want := range map[string]string{
		"藥屋少女的呢喃 第三季":             "藥屋少女的呢喃",
		"魔法騎士雷阿斯 (2026)":          "魔法騎士雷阿斯",
		"東京復仇者 三天戰爭篇":             "東京復仇者",
		"被放逐的作弊賦予魔術師享受隨心所欲的第二人生。": "被放逐的作弊賦予魔術師享受隨心所欲的第二人生。",
		"The Marsupilamis":        "The Marsupilamis",
		"SPY×FAMILY 間諜家家酒":        "SPY×FAMILY 間諜家家酒",
		"ゲルぴよ　第2期":                "ゲルぴよ",
		"  ゲルぴよ 第2期 ":             "ゲルぴよ",
	} {
		if got := ScheduleSearchQuery(title); got != want {
			t.Errorf("ScheduleSearchQuery(%q) = %q, want %q", title, got, want)
		}
	}
}
