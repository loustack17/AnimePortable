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
		{Weekday: time.Tuesday, Date: "10/20", Today: true, Shows: []scheduleShow{
			{Anime: backend.Anime{Title: "週二新作 第二季"}, Query: "週二新作"},
			{Anime: backend.Anime{ID: "tue", Title: "週二作品"}, Query: "週二作品"},
		}},
		{Weekday: time.Wednesday, Date: "10/21", Shows: []scheduleShow{{Anime: backend.Anime{ID: "wed", Title: "未知作品"}}}},
		{Weekday: time.Thursday, Date: "10/22"},
		{Weekday: time.Friday, Date: "10/23"},
		{Weekday: time.Saturday, Date: "10/24"},
		{Weekday: time.Sunday, Date: "10/25"},
		{Weekday: time.Monday, Date: "10/26", Shows: []scheduleShow{{Anime: backend.Anime{ID: "mon", Title: "週一作品"}, Query: "週一作品"}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("week =\n%#v\nwant\n%#v", got, want)
	}
	if heading := got[0].heading(); heading != "週二\n今天 10/20" {
		t.Fatalf("today's heading = %q", heading)
	}
	if heading := got[1].heading(); heading != "週三\n10/21" {
		t.Fatalf("tomorrow's heading = %q", heading)
	}
}

func TestScheduleWeekWithoutItemsHasEmptyDays(t *testing.T) {
	for _, day := range scheduleWeek(nil, time.Now()) {
		if len(day.Shows) != 0 {
			t.Fatalf("day = %#v", day)
		}
	}
}
