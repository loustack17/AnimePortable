// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

func calendarItemFor(id int, nameCN, name string, weekday int) map[string]any {
	return map[string]any{
		"id":          id,
		"url":         "http://bgm.tv/subject/" + strconv.Itoa(id),
		"type":        2,
		"name":        name,
		"name_cn":     nameCN,
		"summary":     "ignored <b>summary</b>",
		"air_date":    "2026-10-05",
		"air_weekday": weekday,
		"rating":      map[string]any{"total": 10, "score": 7.5},
		"images":      map[string]any{"large": testCoverURL},
		"collection":  map[string]any{"doing": 3},
	}
}

func calendarDays(days ...[]any) []any {
	payload := make([]any, 0, len(days))
	for index, items := range days {
		if items == nil {
			items = []any{}
		}
		payload = append(payload, map[string]any{
			"weekday": map[string]any{"en": "Mon", "cn": "星期一", "ja": "月耀日", "id": index + 1},
			"items":   items,
		})
	}
	return payload
}

func scheduleOf(t *testing.T, response any) ([]core.ScheduleEntry, error) {
	t.Helper()
	return newWithDo(&fakeClient{response: jsonResponse(t, response)}).Schedule(context.Background())
}

func TestBangumiScheduleMapsEntriesInSourceOrder(t *testing.T) {
	payload := calendarDays(
		[]any{
			calendarItemFor(30, "葬送的芙莉莲 第二季", "葬送のフリーレン 第2期", 1),
			calendarItemFor(10, "", "日本語タイトル", 1),
		},
		nil,
		[]any{calendarItemFor(20, "中文標題", "日本語", 7)},
	)
	entries, err := scheduleOf(t, payload)
	if err != nil {
		t.Fatal(err)
	}
	want := []core.ScheduleEntry{
		{Ref: core.MetadataRef{Provider: testProviderID, ID: "30"}, Title: "葬送的芙莉蓮 第二季", NativeTitle: "葬送のフリーレン 第2期", Weekday: time.Monday},
		{Ref: core.MetadataRef{Provider: testProviderID, ID: "10"}, Title: "日本語タイトル", NativeTitle: "日本語タイトル", Weekday: time.Monday},
		{Ref: core.MetadataRef{Provider: testProviderID, ID: "20"}, Title: "中文標題", NativeTitle: "日本語", Weekday: time.Sunday},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Fatalf("entries = %#v, want %#v", entries, want)
	}
}

func TestBangumiScheduleNeverShowsSimplifiedChinese(t *testing.T) {
	payload := calendarDays([]any{
		// A Chinese work without name_cn: its original name is Simplified Chinese.
		calendarItemFor(40, "", "凡人修仙传", 1),
		// A Japanese original name stays as it is.
		calendarItemFor(41, "", "葬送のフリーレン", 1),
		// Text that changes on a second normalization would be rejected by the store, so the
		// item is skipped here instead of blocking the whole schedule.
		calendarItemFor(42, "+-標題", "", 1),
	})
	entries, err := scheduleOf(t, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Title != "凡人修仙傳" || entries[1].Title != "葬送のフリーレン" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestBangumiScheduleMapsEveryWeekday(t *testing.T) {
	items := make([]any, 0, 7)
	for weekday := 1; weekday <= 7; weekday++ {
		items = append(items, calendarItemFor(weekday, "標題", "title", weekday))
	}
	entries, err := scheduleOf(t, calendarDays(items))
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
	if len(entries) != len(want) {
		t.Fatalf("entries = %#v", entries)
	}
	for index, entry := range entries {
		if entry.Weekday != want[index] || entry.AnimeID != "" {
			t.Fatalf("entry %d = %#v, want weekday %v and no anime", index, entry, want[index])
		}
	}
}

func TestBangumiScheduleSkipsInvalidItems(t *testing.T) {
	badType := calendarItemFor(2, "標題", "title", 1)
	badType["type"] = 1
	noType := calendarItemFor(3, "標題", "title", 1)
	delete(noType, "type")
	badID := calendarItemFor(4, "標題", "title", 1)
	badID["id"] = 0
	stringID := calendarItemFor(5, "標題", "title", 1)
	stringID["id"] = "5"
	zeroWeekday := calendarItemFor(6, "標題", "title", 0)
	highWeekday := calendarItemFor(7, "標題", "title", 8)
	noWeekday := calendarItemFor(8, "標題", "title", 1)
	delete(noWeekday, "air_weekday")
	noTitles := calendarItemFor(9, " ", "\t", 1)
	controlTitle := calendarItemFor(11, "標\x00題", "title", 1)
	oversizedTitle := calendarItemFor(12, strings.Repeat("x", testMaxTitleTextBytes+1), "title", 1)
	wrongShape := calendarItemFor(13, "標題", "title", 1)
	wrongShape["name"] = 5
	items := []any{badType, noType, badID, stringID, zeroWeekday, highWeekday, noWeekday, noTitles, controlTitle, oversizedTitle, wrongShape, "text", 7, nil, calendarItemFor(1, "標題", "title", 3)}
	entries, err := scheduleOf(t, calendarDays(items))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Ref.ID != "1" || entries[0].Weekday != time.Wednesday {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestBangumiScheduleSkipsDuplicateIDAndWeekday(t *testing.T) {
	entries, err := scheduleOf(t, calendarDays(
		[]any{calendarItemFor(1, "第一", "one", 1), calendarItemFor(1, "重複", "dup", 1)},
		[]any{calendarItemFor(1, "第一", "one", 2)},
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Title != "第一" || entries[0].Weekday != time.Monday || entries[1].Weekday != time.Tuesday {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestBangumiScheduleLimit(t *testing.T) {
	build := func(count int) []any {
		items := make([]any, 0, count)
		for index := 1; index <= count; index++ {
			items = append(items, calendarItemFor(index, "標題", "t", index%7+1))
		}
		return items
	}
	entries, err := scheduleOf(t, calendarDays(build(core.MaxScheduleEntries)))
	if err != nil || len(entries) != core.MaxScheduleEntries {
		t.Fatalf("at limit: %d entries, error = %v", len(entries), err)
	}
	entries, err = scheduleOf(t, calendarDays(build(core.MaxScheduleEntries+1)))
	if !errors.Is(err, errMetadataMalformed) || entries != nil {
		t.Fatalf("over limit: entries = %d, error = %v", len(entries), err)
	}
	// Skipped and duplicate items do not count toward the limit.
	items := append(build(core.MaxScheduleEntries), calendarItemFor(1, "標題", "t", 2), map[string]any{"id": 0})
	if entries, err = scheduleOf(t, calendarDays(items)); err != nil || len(entries) != core.MaxScheduleEntries {
		t.Fatalf("skipped items counted: %d entries, error = %v", len(entries), err)
	}
}

func TestBangumiScheduleEmptyCalendar(t *testing.T) {
	entries, err := newWithDo(&fakeClient{response: rawJSONResponse([]byte(`[]`))}).Schedule(context.Background())
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
}

func TestBangumiScheduleRejectsMalformedBodies(t *testing.T) {
	deep := `[{"items":[{"future":` + strings.Repeat("[", testMaxJSONNestingDepth+1) + "0" + strings.Repeat("]", testMaxJSONNestingDepth+1) + `}]}]`
	bodies := map[string]string{
		"object":         `{"days":[]}`,
		"null":           `null`,
		"number":         `7`,
		"string":         `"text"`,
		"empty":          ``,
		"truncated":      `[{"items":`,
		"trailing":       `[] []`,
		"day is number":  `[7]`,
		"items is text":  `[{"items":"text"}]`,
		"items is obj":   `[{"items":{}}]`,
		"duplicate keys": `[{"items":[],"items":[]}]`,
		"nesting":        deep,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			entries, err := newWithDo(&fakeClient{response: rawJSONResponse([]byte(body))}).Schedule(context.Background())
			if !errors.Is(err, errMetadataMalformed) || entries != nil {
				t.Fatalf("entries = %#v, error = %v", entries, err)
			}
		})
	}
}

func TestBangumiScheduleRejectsBadResponses(t *testing.T) {
	for name, response := range map[string]*securehttp.Response{
		"bad MIME":  {StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte(`[]`)},
		"oversized": rawJSONResponse([]byte(strings.Repeat("x", testMaxResponseBodyBytes+1))),
	} {
		t.Run(name, func(t *testing.T) {
			entries, err := newWithDo(&fakeClient{response: response}).Schedule(context.Background())
			if !errors.Is(err, errMetadataMalformed) || entries != nil {
				t.Fatalf("entries = %#v, error = %v", entries, err)
			}
		})
	}
}

func TestBangumiScheduleRedactsFailures(t *testing.T) {
	entries, err := newWithDo(&fakeClient{response: statusResponse(http.StatusInternalServerError, []byte(`{"message":"remote-secret"}`))}).Schedule(context.Background())
	if !errors.Is(err, errMetadataUnavailable) || entries != nil || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
	entries, err = newWithDo(&fakeClient{err: errors.New("remote-secret network failure")}).Schedule(context.Background())
	if !errors.Is(err, errMetadataUnavailable) || entries != nil || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
	entries, err = newWithDo(&fakeClient{response: statusResponse(http.StatusNotFound, nil)}).Schedule(context.Background())
	if !errors.Is(err, errMetadataUnavailable) || errors.Is(err, core.ErrNotFound) || entries != nil {
		t.Fatalf("404 entries = %#v, error = %v", entries, err)
	}
}

func TestBangumiScheduleHonoursContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{response: jsonResponse(t, calendarDays())}
	if _, err := newWithDo(client).Schedule(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled error = %v", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("cancelled context made %d requests", len(client.requests))
	}

	ctx, cancel = context.WithCancel(context.Background())
	client = &fakeClient{handler: func(*http.Request, []byte) (*securehttp.Response, error) {
		cancel()
		return jsonResponse(t, calendarDays()), nil
	}}
	if entries, err := newWithDo(client).Schedule(ctx); !errors.Is(err, context.Canceled) || entries != nil {
		t.Fatalf("cancelled during request: entries = %#v, error = %v", entries, err)
	}
}

func TestBangumiScheduleRequestShape(t *testing.T) {
	client := &fakeClient{response: jsonResponse(t, calendarDays())}
	if _, err := newWithDo(client).Schedule(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("requests = %d", len(client.requests))
	}
	request := client.requests[0]
	assertCommonRequest(t, request)
	if request.Method != http.MethodGet || request.URL.String() != testAPIOrigin+"/calendar" {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	if request.Header.Get("Content-Type") != "" || len(client.bodies[0]) != 0 {
		t.Fatalf("calendar request carried a body: %#v %q", request.Header, client.bodies[0])
	}
}

func TestBangumiScheduleIgnoresRemoteMediaAndText(t *testing.T) {
	item := calendarItemFor(1, "標題", "title", 1)
	item["images"] = map[string]any{"large": "https://evil.example/x.jpg"}
	item["url"] = "http://evil.example/"
	entries, err := scheduleOf(t, calendarDays([]any{item}))
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v, error = %v", entries, err)
	}
	entry := entries[0]
	for _, text := range []string{entry.Title, entry.NativeTitle, entry.Ref.ID} {
		if strings.Contains(text, "evil") || strings.Contains(text, "summary") {
			t.Fatalf("remote text leaked: %#v", entry)
		}
	}
}

func TestBangumiScheduleUnavailableClient(t *testing.T) {
	var client *Client
	if _, err := client.Schedule(context.Background()); !errors.Is(err, errMetadataUnavailable) {
		t.Fatalf("nil client error = %v", err)
	}
}
