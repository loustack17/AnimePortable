// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"reflect"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
)

func TestScheduleWeekStartsTodayAndKeepsSourceOrder(t *testing.T) {
	now := time.Date(2026, 10, 20, 15, 0, 0, 0, time.Local) // a Tuesday
	items := []backend.ScheduleItem{
		{AnimeID: "mon", Title: "週一作品", Query: "週一作品", Weekday: int(time.Monday)},
		{Title: "週二新作 第二季", Query: "週二新作", Weekday: int(time.Tuesday)},
		{AnimeID: "tue", Title: "週二作品", Query: "週二作品", Weekday: int(time.Tuesday)},
		{AnimeID: "wed", Title: " ", Weekday: int(time.Wednesday)},
	}
	got := scheduleWeek(items, now)
	if len(got) != 7 {
		t.Fatalf("days = %d, want 7", len(got))
	}
	want := []scheduleDay{
		{Weekday: "週二", Date: "今天 10/20", Today: true, Shows: []scheduleShow{
			{Anime: backend.Anime{Title: "週二新作 第二季"}, Query: "週二新作"},
			{Anime: backend.Anime{ID: "tue", Title: "週二作品"}, Query: "週二作品"},
		}},
		{Weekday: "週三", Date: "10/21", Shows: []scheduleShow{{Anime: backend.Anime{ID: "wed", Title: "未知作品"}}}},
		{Weekday: "週四", Date: "10/22"},
		{Weekday: "週五", Date: "10/23"},
		{Weekday: "週六", Date: "10/24"},
		{Weekday: "週日", Date: "10/25"},
		{Weekday: "週一", Date: "10/26", Shows: []scheduleShow{{Anime: backend.Anime{ID: "mon", Title: "週一作品"}, Query: "週一作品"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("week =\n%#v\nwant\n%#v", got, want)
	}
}

func TestScheduleWeekWithoutItemsHasEmptyDays(t *testing.T) {
	for _, day := range scheduleWeek(nil, time.Now()) {
		if len(day.Shows) != 0 {
			t.Fatalf("day = %#v", day)
		}
	}
}
