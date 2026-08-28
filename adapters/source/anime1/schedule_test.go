package anime1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func scheduleFixture() string {
	return `<html><body><table><thead><tr><th colspan="7">2026年冬季(1-3月)新番</th></tr><tr><th>日</th><th>一</th><th>二</th><th>三</th><th>四</th><th>五</th><th>六</th></tr></thead><tbody><tr><td><a href="/?cat=42">Alpha Show</a><br>Unlinked</td><td></td><td><a href="/?cat=7"><b>Beta</b> Show</a></td><td><a href="https://external.example/show">External</a></td><td></td><td></td><td></td></tr><tr><td colspan="7"><a href="https://anime1.me"><span>Anime1.me</span></a></td></tr></tbody></table></body></html>`
}

func scheduleResponse(body string) *securehttp.Response {
	return &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: []byte(body)}
}

func taipeiDate(year int, month time.Month, day int) time.Time {
	location, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		panic(err)
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location)
}

func scheduleExpected() []core.SourceScheduleItem {
	alpha := core.SourceRef{Provider: providerID, ID: "42"}
	beta := core.SourceRef{Provider: providerID, ID: "7"}
	return []core.SourceScheduleItem{
		{Anime: core.SourceAnime{Ref: alpha, Title: "Alpha Show"}, Episode: core.SourceEpisode{Ref: core.EpisodeRef{Anime: alpha}}, AirsAt: taipeiDate(2026, time.January, 4), Precision: core.SchedulePrecisionDay},
		{Anime: core.SourceAnime{Ref: beta, Title: "Beta Show"}, Episode: core.SourceEpisode{Ref: core.EpisodeRef{Anime: beta}}, AirsAt: taipeiDate(2026, time.January, 6), Precision: core.SchedulePrecisionDay},
	}
}

func TestScheduleNormalizesDayPrecisionAndUnknownEpisode(t *testing.T) {
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(scheduleFixture())}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(items, scheduleExpected()) {
		t.Fatalf("items = %#v", items)
	}
	if len(client.requests) != 1 || client.requests[0].Method != http.MethodGet || client.requests[0].URL.Path != "/2026年冬季新番" || client.requests[0].URL.RawQuery != "" {
		t.Fatalf("requests = %#v", client.requests)
	}
}

func TestSchedulePreservesSourceOrderWithinDay(t *testing.T) {
	body := strings.Replace(scheduleFixture(), `<a href="/?cat=42">Alpha Show</a>`, `<a href="/?cat=43">Zeta Show</a><a href="/?cat=42">Alpha Show</a>`, 1)
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 5)})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Anime.Title != "Zeta Show" || items[1].Anime.Title != "Alpha Show" {
		t.Fatalf("items = %#v", items)
	}
}

func TestScheduleFetchesAtMostTwoSeasonPages(t *testing.T) {
	spring := strings.Replace(scheduleFixture(), "2026年冬季(1-3月)新番", "2026年春季(4-6月)新番", 1)
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(scheduleFixture()), scheduleResponse(spring)}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.March, 29), To: taipeiDate(2026, time.April, 5)})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 || client.requests[0].URL.Path != "/2026年冬季新番" || client.requests[1].URL.Path != "/2026年春季新番" || len(items) != 2 {
		t.Fatalf("requests=%d items=%#v", len(client.requests), items)
	}
}

func TestScheduleValidatesQueryBeforeRequest(t *testing.T) {
	validStart := taipeiDate(2026, time.January, 1)
	queries := []core.ScheduleQuery{
		{},
		{From: validStart, To: validStart},
		{From: validStart, To: validStart.Add(-time.Hour)},
		{From: validStart, To: validStart.Add(maxScheduleRange + time.Nanosecond)},
		{From: taipeiDate(minScheduleYear-1, time.January, 1), To: taipeiDate(minScheduleYear-1, time.January, 2)},
		{From: taipeiDate(maxScheduleYear, time.December, 31), To: taipeiDate(maxScheduleYear+1, time.January, 1)},
	}
	for _, query := range queries {
		client := &fakeClient{}
		if _, err := newWithDo(client).Schedule(context.Background(), query); !errors.Is(err, errScheduleInvalidQuery) || len(client.requests) != 0 {
			t.Fatalf("query=%#v requests=%d error=%v", query, len(client.requests), err)
		}
	}
	if _, err := newWithDo(&fakeClient{}).Schedule(nil, core.ScheduleQuery{}); !errors.Is(err, errScheduleUnavailable) {
		t.Fatalf("nil context error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newWithDo(&fakeClient{}).Schedule(ctx, core.ScheduleQuery{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(scheduleFixture())}}
	if _, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: validStart, To: validStart.Add(maxScheduleRange)}); err != nil {
		t.Fatalf("exact range rejected: %v", err)
	}
}

func TestScheduleRejectsMalformedTablesAndLinks(t *testing.T) {
	valid := scheduleFixture()
	cases := map[string]string{
		"missing table":        strings.Replace(valid, "<table>", "<div>", 1),
		"duplicate table":      valid + "<table></table>",
		"wrong header":         strings.Replace(valid, "<th>日</th>", "<th>一</th>", 1),
		"short row":            strings.Replace(valid, "<td></td><td></td><td></td></tr>", "<td></td><td></td></tr>", 1),
		"malformed category":   strings.Replace(valid, "/?cat=42", "/?cat=0", 1),
		"same origin absolute": strings.Replace(valid, "/?cat=42", "https://anime1.me/?cat=42", 1),
		"duplicate mapping":    strings.Replace(valid, "<br>Unlinked", `<a href="/?cat=42">Again</a>`, 1),
		"duplicate href":       strings.Replace(valid, `href="/?cat=42"`, `href="/?cat=42" href="/?cat=42"`, 1),
		"data colspan":         strings.Replace(valid, `<td><a href="/?cat=42">`, `<td colspan="2"><a href="/?cat=42">`, 1),
		"data rowspan":         strings.Replace(valid, `<td><a href="/?cat=42">`, `<td rowspan="2"><a href="/?cat=42">`, 1),
		"direct table row":     strings.Replace(valid, "<tbody>", "", 1),
		"relative same origin": strings.Replace(valid, "/?cat=42", "./?cat=42", 1),
		"spaced trusted link":  strings.Replace(valid, "/?cat=42", " /?cat=42 ", 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}}
			items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)})
			if !errors.Is(err, errScheduleMalformed) || items != nil {
				t.Fatalf("items=%#v error=%v", items, err)
			}
		})
	}
}

func TestScheduleSkipsExternalNetworkPathAndRawText(t *testing.T) {
	body := strings.Replace(scheduleFixture(), "https://external.example/show", "//external.example/show", 1)
	body = strings.Replace(body, "Unlinked", `<script><a href="/?cat=999">Hidden</a></script>`, 1)
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)})
	if err != nil || !reflect.DeepEqual(items, scheduleExpected()) {
		t.Fatalf("items=%#v error=%v", items, err)
	}
}

func TestScheduleHalfOpenRangeUsesTaipeiCalendar(t *testing.T) {
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(scheduleFixture())}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{
		From: time.Date(2026, time.January, 4, 4, 0, 0, 0, time.UTC),
		To:   time.Date(2026, time.January, 7, 4, 0, 0, 0, time.UTC),
	})
	if err != nil || len(items) != 1 || items[0].Anime.Ref.ID != "7" || !items[0].AirsAt.Equal(taipeiDate(2026, time.January, 6)) {
		t.Fatalf("items=%#v error=%v", items, err)
	}
}

func TestScheduleRejectsCrossSeasonTitleConflict(t *testing.T) {
	spring := strings.Replace(scheduleFixture(), "2026年冬季(1-3月)新番", "2026年春季(4-6月)新番", 1)
	spring = strings.Replace(spring, "Alpha Show", "Changed Title", 1)
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(scheduleFixture()), scheduleResponse(spring)}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.March, 29), To: taipeiDate(2026, time.April, 5)})
	if !errors.Is(err, errScheduleMalformed) || items != nil {
		t.Fatalf("items=%#v error=%v", items, err)
	}
}

func TestScheduleSeasonURLsCoverYearBoundary(t *testing.T) {
	want := []string{
		"https://anime1.me/2026%E5%B9%B4%E5%86%AC%E5%AD%A3%E6%96%B0%E7%95%AA",
		"https://anime1.me/2026%E5%B9%B4%E6%98%A5%E5%AD%A3%E6%96%B0%E7%95%AA",
		"https://anime1.me/2026%E5%B9%B4%E5%A4%8F%E5%AD%A3%E6%96%B0%E7%95%AA",
		"https://anime1.me/2026%E5%B9%B4%E7%A7%8B%E5%AD%A3%E6%96%B0%E7%95%AA",
	}
	for quarter, expected := range want {
		if got := scheduleURL(scheduleSeason{year: 2026, quarter: quarter}).String(); got != expected {
			t.Fatalf("quarter=%d URL=%s", quarter, got)
		}
	}
	_, _, seasons, err := validateScheduleQuery(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.December, 20), To: taipeiDate(2027, time.January, 10)}, taipeiDate(2026, time.January, 1).Location())
	if err != nil || !reflect.DeepEqual(seasons, []scheduleSeason{{year: 2026, quarter: 3}, {year: 2027, quarter: 0}}) {
		t.Fatalf("seasons=%#v error=%v", seasons, err)
	}
}

func TestScheduleBoundsAndFailureSanitization(t *testing.T) {
	tooLarge := scheduleResponse(strings.Repeat(" ", maxScheduleBodyBytes+1))
	wrongMIME := scheduleResponse(scheduleFixture())
	wrongMIME.Header.Set("Content-Type", "application/json")
	cases := []struct {
		name   string
		client *fakeClient
		want   error
	}{
		{name: "nil response", client: &fakeClient{}, want: errScheduleUnavailable},
		{name: "network", client: &fakeClient{err: errors.New("raw-network-secret")}, want: errScheduleUnavailable},
		{name: "body", client: &fakeClient{scheduleResponses: []*securehttp.Response{tooLarge}}, want: errScheduleMalformed},
		{name: "MIME", client: &fakeClient{scheduleResponses: []*securehttp.Response{wrongMIME}}, want: errScheduleMalformed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			items, err := newWithDo(test.client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)})
			if items != nil || !errors.Is(err, test.want) || strings.Contains(fmt.Sprint(err), "raw-network-secret") {
				t.Fatalf("items=%#v error=%v", items, err)
			}
		})
	}
}

func TestScheduleExactLimits(t *testing.T) {
	longTitle := strings.Repeat("😀", maxScheduleTitleRunes)
	body := strings.Replace(scheduleFixture(), "Alpha Show", longTitle, 1)
	client := &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}}
	items, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)})
	if err != nil || len(items) != 2 {
		t.Fatalf("exact title rejected: %v", err)
	}
	body = strings.Replace(scheduleFixture(), "Alpha Show", longTitle+"a", 1)
	client = &fakeClient{scheduleResponses: []*securehttp.Response{scheduleResponse(body)}}
	if _, err := newWithDo(client).Schedule(context.Background(), core.ScheduleQuery{From: taipeiDate(2026, time.January, 4), To: taipeiDate(2026, time.January, 11)}); !errors.Is(err, errScheduleMalformed) {
		t.Fatalf("title limit +1 error = %v", err)
	}
}

func TestScheduleBodyDOMAndMappingLimits(t *testing.T) {
	exactBody := make([]byte, maxScheduleBodyBytes)
	client := newWithDo(&fakeClient{scheduleResponses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: exactBody}}})
	if _, err := client.fetchSchedulePage(context.Background(), scheduleURL(scheduleSeason{year: 2026})); err != nil {
		t.Fatalf("exact body rejected: %v", err)
	}
	client = newWithDo(&fakeClient{scheduleResponses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: append(exactBody, 0)}}})
	if _, err := client.fetchSchedulePage(context.Background(), scheduleURL(scheduleSeason{year: 2026})); !errors.Is(err, errScheduleMalformed) {
		t.Fatalf("body limit +1 error = %v", err)
	}
	exactDOM := scheduleDocumentWithNodeCount(maxScheduleDOMNodes)
	if _, err := findScheduleTable(context.Background(), exactDOM); err != nil {
		t.Fatalf("exact DOM rejected: %v", err)
	}
	if _, err := findScheduleTable(context.Background(), scheduleDocumentWithNodeCount(maxScheduleDOMNodes+1)); !errors.Is(err, errScheduleMalformed) {
		t.Fatalf("DOM limit +1 error = %v", err)
	}
	exactMappings := scheduleFixtureWithMappings(maxScheduleMappings)
	if mappings, err := parseSchedulePage(context.Background(), []byte(exactMappings), scheduleSeason{year: 2026}); err != nil || len(mappings) != maxScheduleMappings {
		t.Fatalf("exact mappings rejected: count=%d error=%v", len(mappings), err)
	}
	if _, err := parseSchedulePage(context.Background(), []byte(scheduleFixtureWithMappings(maxScheduleMappings+1)), scheduleSeason{year: 2026}); !errors.Is(err, errScheduleMalformed) {
		t.Fatalf("mapping limit +1 error = %v", err)
	}
}

func TestScheduleCancellationReturnsNoPartialResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &cancelAfterPageClient{response: scheduleResponse(scheduleFixture()), cancel: cancel}
	items, err := newWithDo(client).Schedule(ctx, core.ScheduleQuery{From: taipeiDate(2026, time.March, 29), To: taipeiDate(2026, time.April, 5)})
	if !errors.Is(err, context.Canceled) || items != nil || client.calls != 2 {
		t.Fatalf("items=%#v calls=%d error=%v", items, client.calls, err)
	}
	parseContext := &cancelAfterChecksContext{cancelAt: 10}
	if _, err := parseSchedulePage(parseContext, []byte(scheduleFixture()), scheduleSeason{year: 2026}); !errors.Is(err, context.Canceled) {
		t.Fatalf("parse cancellation error = %v", err)
	}
}

func scheduleDocumentWithNodeCount(count int) *html.Node {
	root := &html.Node{Type: html.DocumentNode}
	table := &html.Node{Type: html.ElementNode, DataAtom: atom.Table, Data: "table"}
	root.AppendChild(table)
	current := table
	for index := 2; index < count; index++ {
		child := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
		current.AppendChild(child)
		current = child
	}
	return root
}

func scheduleFixtureWithMappings(count int) string {
	var anchors strings.Builder
	for index := 1; index <= count; index++ {
		fmt.Fprintf(&anchors, `<a href="/?cat=%d">Show %d</a>`, index, index)
	}
	return `<table><thead><tr><th colspan="7">2026年冬季(1-3月)新番</th></tr><tr><th>日</th><th>一</th><th>二</th><th>三</th><th>四</th><th>五</th><th>六</th></tr></thead><tbody><tr><td>` + anchors.String() + `</td><td></td><td></td><td></td><td></td><td></td><td></td></tr></tbody></table>`
}
