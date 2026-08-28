package anime1

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	maxScheduleRange      = 35 * 24 * time.Hour
	maxScheduleBodyBytes  = 1 << 20
	maxScheduleDOMNodes   = 20000
	maxScheduleMappings   = 128
	maxScheduleItems      = 512
	maxScheduleTitleBytes = 4 << 10
	maxScheduleTitleRunes = 1024
	minScheduleYear       = 2000
	maxScheduleYear       = 2100
)

var (
	errScheduleUnavailable  = errors.New("anime1: schedule unavailable")
	errScheduleInvalidQuery = errors.New("anime1: invalid schedule query")
	errScheduleMalformed    = errors.New("anime1: malformed schedule")
)

type scheduleSeason struct {
	year    int
	quarter int
}

type scheduleMapping struct {
	ref     core.SourceRef
	title   string
	weekday time.Weekday
}

func (client *Client) Schedule(ctx context.Context, query core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	location, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		return nil, errScheduleUnavailable
	}
	from, to, seasons, err := validateScheduleQuery(ctx, query, location)
	if err != nil {
		return nil, err
	}
	if client == nil || client.do == nil {
		return nil, errScheduleUnavailable
	}
	mappings := make(map[scheduleSeason][]scheduleMapping, len(seasons))
	titles := make(map[core.SourceRef]string)
	for _, season := range seasons {
		body, err := client.fetchSchedulePage(ctx, scheduleURL(season))
		if err != nil {
			return nil, err
		}
		pageMappings, err := parseSchedulePage(ctx, body, season)
		if err != nil {
			return nil, err
		}
		for _, mapping := range pageMappings {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if title, exists := titles[mapping.ref]; exists && title != mapping.title {
				return nil, errScheduleMalformed
			}
			titles[mapping.ref] = mapping.title
		}
		mappings[season] = pageMappings
	}
	items := make([]core.SourceScheduleItem, 0)
	for day := dayStart(from, location); day.Before(to); day = day.AddDate(0, 0, 1) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if day.Before(from) {
			continue
		}
		season := seasonFor(day)
		for _, mapping := range mappings[season] {
			if mapping.weekday != day.Weekday() {
				continue
			}
			if len(items) >= maxScheduleItems {
				return nil, errScheduleMalformed
			}
			items = append(items, core.SourceScheduleItem{
				Anime:     core.SourceAnime{Ref: mapping.ref, Title: mapping.title},
				Episode:   core.SourceEpisode{Ref: core.EpisodeRef{Anime: mapping.ref}},
				AirsAt:    day,
				Precision: core.SchedulePrecisionDay,
			})
		}
	}
	sort.SliceStable(items, func(left, right int) bool { return items[left].AirsAt.Before(items[right].AirsAt) })
	return items, nil
}

func validateScheduleQuery(ctx context.Context, query core.ScheduleQuery, location *time.Location) (time.Time, time.Time, []scheduleSeason, error) {
	if ctx == nil {
		return time.Time{}, time.Time{}, nil, errScheduleUnavailable
	}
	if err := ctx.Err(); err != nil {
		return time.Time{}, time.Time{}, nil, err
	}
	if location == nil || query.From.IsZero() || query.To.IsZero() || !query.From.Before(query.To) || query.To.Sub(query.From) > maxScheduleRange {
		return time.Time{}, time.Time{}, nil, errScheduleInvalidQuery
	}
	from := query.From.In(location)
	to := query.To.In(location)
	if from.Year() < minScheduleYear || from.Year() > maxScheduleYear || to.Year() < minScheduleYear || to.Year() > maxScheduleYear {
		return time.Time{}, time.Time{}, nil, errScheduleInvalidQuery
	}
	seasons := make([]scheduleSeason, 0, 2)
	seen := make(map[scheduleSeason]struct{}, 2)
	for day := dayStart(from, location); day.Before(to); day = day.AddDate(0, 0, 1) {
		season := seasonFor(day)
		if _, exists := seen[season]; exists {
			continue
		}
		if len(seasons) == 2 {
			return time.Time{}, time.Time{}, nil, errScheduleInvalidQuery
		}
		seen[season] = struct{}{}
		seasons = append(seasons, season)
	}
	if len(seasons) == 0 {
		return time.Time{}, time.Time{}, nil, errScheduleInvalidQuery
	}
	return from, to, seasons, nil
}

func dayStart(value time.Time, location *time.Location) time.Time {
	local := value.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}

func seasonFor(value time.Time) scheduleSeason {
	return scheduleSeason{year: value.Year(), quarter: (int(value.Month()) - 1) / 3}
}

func scheduleURL(season scheduleSeason) *url.URL {
	names := [...]string{"冬", "春", "夏", "秋"}
	return &url.URL{Scheme: "https", Host: "anime1.me", Path: fmt.Sprintf("/%d年%s季新番", season.year, names[season.quarter])}
}

func (client *Client) fetchSchedulePage(ctx context.Context, target *url.URL) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errScheduleUnavailable
	}
	response, err := client.do.Do(request)
	if err != nil {
		return nil, sanitizeScheduleError(err)
	}
	if response == nil {
		return nil, errScheduleUnavailable
	}
	if err := response.RequireSuccess(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(response.Body) > maxScheduleBodyBytes || !isHTMLResponse(response.Header.Get("Content-Type")) {
		return nil, errScheduleMalformed
	}
	return response.Body, nil
}

func sanitizeScheduleError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var secureError *securehttp.Error
	if errors.As(err, &secureError) {
		return secureError
	}
	return errScheduleUnavailable
}

func parseSchedulePage(ctx context.Context, body []byte, season scheduleSeason) ([]scheduleMapping, error) {
	if len(body) == 0 || len(body) > maxScheduleBodyBytes {
		return nil, errScheduleMalformed
	}
	if err := rejectDuplicateRawAttributes(ctx, body, map[string][]string{"a": {"href"}}, errScheduleMalformed); err != nil {
		return nil, err
	}
	if err := validateScheduleRawSections(ctx, body); err != nil {
		return nil, err
	}
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil || document == nil {
		return nil, errScheduleMalformed
	}
	table, err := findScheduleTable(ctx, document)
	if err != nil {
		return nil, err
	}
	rows, err := scheduleRows(ctx, table, season)
	if err != nil || len(rows) == 0 {
		return nil, errScheduleMalformed
	}
	mappings := make([]scheduleMapping, 0)
	seen := make(map[string]struct{})
	for _, row := range rows {
		cells := directElements(row, atom.Td)
		if len(cells) != 7 || directElementCount(row) != 7 {
			return nil, errScheduleMalformed
		}
		for column, cell := range cells {
			if len(attributeValues(cell, "colspan")) != 0 || len(attributeValues(cell, "rowspan")) != 0 {
				return nil, errScheduleMalformed
			}
			anchors, err := scheduleAnchors(ctx, cell)
			if err != nil {
				return nil, err
			}
			for _, anchor := range anchors {
				mapping, trusted, err := scheduleAnchorMapping(ctx, anchor, time.Weekday(column))
				if err != nil {
					return nil, err
				}
				if !trusted {
					continue
				}
				key := mapping.ref.ID + "\x00" + fmt.Sprint(column)
				if _, exists := seen[key]; exists || len(mappings) >= maxScheduleMappings {
					return nil, errScheduleMalformed
				}
				seen[key] = struct{}{}
				mappings = append(mappings, mapping)
			}
		}
	}
	return mappings, nil
}

func validateScheduleRawSections(ctx context.Context, body []byte) error {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	tables := 0
	theads := 0
	tbodies := 0
	tableOpen := false
	section := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch tokenizer.Next() {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) && tables == 1 && theads == 1 && tbodies == 1 && !tableOpen && section == "" {
				return nil
			}
			return errScheduleMalformed
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			switch strings.ToLower(string(name)) {
			case "table":
				if tableOpen {
					return errScheduleMalformed
				}
				tables++
				tableOpen = true
			case "thead":
				if !tableOpen || section != "" {
					return errScheduleMalformed
				}
				theads++
				section = "thead"
			case "tbody":
				if !tableOpen || section != "" {
					return errScheduleMalformed
				}
				tbodies++
				section = "tbody"
			case "tr":
				if !tableOpen || section == "" {
					return errScheduleMalformed
				}
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			switch strings.ToLower(string(name)) {
			case "thead", "tbody":
				if section != strings.ToLower(string(name)) {
					return errScheduleMalformed
				}
				section = ""
			case "table":
				if !tableOpen || section != "" {
					return errScheduleMalformed
				}
				tableOpen = false
			}
		}
	}
}

func findScheduleTable(ctx context.Context, document *html.Node) (*html.Node, error) {
	stack := []*html.Node{document}
	var table *html.Node
	nodes := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nodes++
		if nodes > maxScheduleDOMNodes {
			return nil, errScheduleMalformed
		}
		if node.Type == html.ElementNode && node.DataAtom == atom.Table {
			if table != nil {
				return nil, errScheduleMalformed
			}
			table = node
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	if table == nil {
		return nil, errScheduleMalformed
	}
	return table, nil
}

func scheduleRows(ctx context.Context, table *html.Node, season scheduleSeason) ([]*html.Node, error) {
	var head, body *html.Node
	for child := table.FirstChild; child != nil; child = child.NextSibling {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if child.Type != html.ElementNode {
			continue
		}
		switch child.DataAtom {
		case atom.Thead:
			if head != nil {
				return nil, errScheduleMalformed
			}
			head = child
		case atom.Tbody:
			if body != nil {
				return nil, errScheduleMalformed
			}
			body = child
		default:
			return nil, errScheduleMalformed
		}
	}
	if head == nil || body == nil {
		return nil, errScheduleMalformed
	}
	headRows, err := directRows(ctx, head)
	if err != nil || len(headRows) != 2 || !scheduleSeasonHeader(headRows[0], season) || !scheduleHeader(headRows[1]) {
		return nil, errScheduleMalformed
	}
	rows, err := directRows(ctx, body)
	if err != nil || len(rows) == 0 {
		return nil, errScheduleMalformed
	}
	if scheduleFooter(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}
	if len(rows) == 0 {
		return nil, errScheduleMalformed
	}
	return rows, nil
}

func directRows(ctx context.Context, section *html.Node) ([]*html.Node, error) {
	rows := make([]*html.Node, 0)
	for child := section.FirstChild; child != nil; child = child.NextSibling {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if child.Type != html.ElementNode {
			continue
		}
		if child.DataAtom != atom.Tr {
			return nil, errScheduleMalformed
		}
		rows = append(rows, child)
	}
	return rows, nil
}

func scheduleSeasonHeader(row *html.Node, season scheduleSeason) bool {
	cells := directElements(row, atom.Th)
	if len(cells) != 1 || directElementCount(row) != 1 || attributeValue(cells[0], "colspan") != "7" {
		return false
	}
	names := [...]string{"冬", "春", "夏", "秋"}
	ranges := [...]string{"1-3", "4-6", "7-9", "10-12"}
	want := fmt.Sprintf("%d年%s季(%s月)新番", season.year, names[season.quarter], ranges[season.quarter])
	return directText(cells[0]) == want
}

func scheduleFooter(row *html.Node) bool {
	cells := directElements(row, atom.Td)
	if len(cells) != 1 || directElementCount(row) != 1 || attributeValue(cells[0], "colspan") != "7" {
		return false
	}
	anchors, err := scheduleAnchors(context.Background(), cells[0])
	return err == nil && len(anchors) == 1 && attributeValue(anchors[0], "href") == episodesOrigin && directTextOrDescendants(anchors[0]) == "Anime1.me"
}

func scheduleHeader(row *html.Node) bool {
	want := [...]string{"日", "一", "二", "三", "四", "五", "六"}
	cells := directElements(row, atom.Th)
	if len(cells) != len(want) || directElementCount(row) != len(want) {
		return false
	}
	for index, cell := range cells {
		if directText(cell) != want[index] {
			return false
		}
	}
	return true
}

func directElements(parent *html.Node, kind atom.Atom) []*html.Node {
	items := make([]*html.Node, 0)
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == kind {
			items = append(items, child)
		}
	}
	return items
}

func directElementCount(parent *html.Node) int {
	count := 0
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			count++
		}
	}
	return count
}

func directText(node *html.Node) string {
	parts := make([]string, 0)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			parts = append(parts, child.Data)
		}
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func directTextOrDescendants(node *html.Node) string {
	parts := make([]string, 0)
	stack := []*html.Node{node}
	for len(stack) > 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if current.Type == html.TextNode {
			parts = append(parts, current.Data)
		}
		for child := current.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}

func scheduleAnchors(ctx context.Context, root *html.Node) ([]*html.Node, error) {
	stack := []*html.Node{root}
	anchors := make([]*html.Node, 0)
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			continue
		}
		if node.Type == html.ElementNode && node.DataAtom == atom.A {
			anchors = append(anchors, node)
			continue
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return anchors, nil
}

func scheduleAnchorMapping(ctx context.Context, anchor *html.Node, weekday time.Weekday) (scheduleMapping, bool, error) {
	values := attributeValues(anchor, "href")
	if len(values) != 1 {
		return scheduleMapping{}, false, errScheduleMalformed
	}
	href := values[0]
	categoryID, trusted, malformed := scheduleCategoryID(href)
	if malformed {
		return scheduleMapping{}, false, errScheduleMalformed
	}
	if !trusted {
		return scheduleMapping{}, false, nil
	}
	title, err := normalizeScheduleTitle(ctx, anchor)
	if err != nil {
		return scheduleMapping{}, false, err
	}
	if title == "" {
		return scheduleMapping{}, false, nil
	}
	ref := core.SourceRef{Provider: providerID, ID: categoryID}
	return scheduleMapping{ref: ref, title: title, weekday: weekday}, true, nil
}

func scheduleCategoryID(href string) (string, bool, bool) {
	if href != strings.TrimSpace(href) {
		return "", false, true
	}
	if strings.HasPrefix(href, "/?cat=") && strings.Count(href, "?") == 1 && strings.Count(href, "=") == 1 {
		id, ok := parseBoundedPositiveDecimal(strings.TrimPrefix(href, "/?cat="), maxCategoryIDDigits)
		return id, ok, !ok
	}
	reference, err := url.Parse(href)
	if err != nil {
		return "", false, true
	}
	if href == "" || reference == nil {
		return "", false, false
	}
	base, _ := url.Parse(episodesOrigin + "/")
	target := base.ResolveReference(reference)
	if strings.EqualFold(target.Hostname(), "anime1.me") {
		return "", false, true
	}
	return "", false, false
}

func normalizeScheduleTitle(ctx context.Context, root *html.Node) (string, error) {
	parts := make([]string, 0)
	stack := []*html.Node{root}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			continue
		}
		if node.Type == html.TextNode {
			parts = append(parts, node.Data)
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	title := strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	if len(title) > maxScheduleTitleBytes || utf8.RuneCountInString(title) > maxScheduleTitleRunes {
		return "", errScheduleMalformed
	}
	return title, nil
}
