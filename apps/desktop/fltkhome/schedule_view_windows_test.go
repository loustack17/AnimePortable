//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

// scheduleViewTestService serves a stored schedule; Schedule waits for release, then
// replaces the stored schedule with fetched or fails with fetchErr.
type scheduleViewTestService struct {
	*searchViewTestService
	mu         sync.Mutex
	cached     []backend.ScheduleItem
	cachedErr  error
	fetched    []backend.ScheduleItem
	fetchErr   error
	fetchCalls int
	release    chan struct{}
}

func (service *scheduleViewTestService) Detail(_ context.Context, animeID string) (backend.Detail, error) {
	return backend.Detail{Anime: backend.Anime{ID: animeID, Title: animeID + " 作品"}}, nil
}

func (service *scheduleViewTestService) CachedSchedule(context.Context) ([]backend.ScheduleItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.cached), service.cachedErr
}

func (service *scheduleViewTestService) Schedule(ctx context.Context) ([]backend.ScheduleItem, error) {
	service.mu.Lock()
	service.fetchCalls++
	release := service.release
	service.mu.Unlock()
	select {
	case <-release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.fetchErr != nil {
		return nil, service.fetchErr
	}
	service.cached = slices.Clone(service.fetched)
	return slices.Clone(service.fetched), nil
}

func (service *scheduleViewTestService) setCached(items []backend.ScheduleItem) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.cached = items
}

func (service *scheduleViewTestService) calls() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.fetchCalls
}

func newScheduleViewTestFixture(t *testing.T, service *scheduleViewTestService) *view {
	t.Helper()
	ui := newSearchViewTestFixture(t, service)
	ui.mainGroup.Begin()
	ui.buildScheduleView()
	// Left from the first day returns to the navigation, which the search fixture lacks.
	ui.navigation = make([]*fltk.Button, len(sections))
	for index := range ui.navigation {
		ui.navigation[index] = fltk.NewButton(0, 0, 1, 1, sections[index])
	}
	ui.mainGroup.End()
	return ui
}

// scheduleTestItem is a known show airing weekly on the weekday days after today; an empty
// animeID makes it a show the app does not know.
func scheduleTestItem(animeID string, days int) backend.ScheduleItem {
	title := animeID + " 作品"
	if animeID == "" {
		title = "未知新番 第二季"
	}
	return backend.ScheduleItem{AnimeID: animeID, Title: title, Query: strings.Fields(title)[0], Weekday: int(time.Now().AddDate(0, 0, days).Weekday())}
}

func scheduleCardCounts(ui *view) []int {
	counts := make([]int, 0, len(ui.scheduleCards))
	for _, cards := range ui.scheduleCards {
		counts = append(counts, len(cards))
	}
	return counts
}

func TestNativeScheduleGridShowsCacheFirstThenFetchedWeek(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("today", 0), scheduleTestItem("later", 2)},
		fetched:               []backend.ScheduleItem{scheduleTestItem("today", 0), scheduleTestItem("new", 1), scheduleTestItem("", 1), scheduleTestItem("later", 2)},
		release:               make(chan struct{}),
	}
	ui := newScheduleViewTestFixture(t, service)

	ui.showSection(scheduleSection)
	// The stored week shows while the fetch is still waiting.
	pumpSearchEvents(t, func() bool { return ui.scheduleLoaded && len(ui.scheduleCards) == 7 })
	if !slices.Equal(scheduleCardCounts(ui), []int{1, 0, 1, 0, 0, 0, 0}) {
		t.Fatalf("cards per day = %v", scheduleCardCounts(ui))
	}
	if !ui.schedulePage.Visible() || ui.scroll.Visible() || ui.searchPage.Visible() {
		t.Fatal("時間表 is not the visible page")
	}
	if !ui.scheduleRefreshing || !strings.Contains(ui.scheduleStatus.Label(), "先顯示上次記錄") {
		t.Fatalf("refreshing = %v, status = %q", ui.scheduleRefreshing, ui.scheduleStatus.Label())
	}
	if label := ui.scheduleHeaders[0].Label(); !strings.Contains(label, "今天") {
		t.Fatalf("first column header = %q, want today", label)
	}

	close(service.release)
	pumpSearchEvents(t, func() bool {
		return !ui.scheduleRefreshing && slices.Equal(scheduleCardCounts(ui), []int{1, 2, 1, 0, 0, 0, 0})
	})
	if status := ui.scheduleStatus.Label(); status != "接下來 7 天共 4 部播出。" {
		t.Fatalf("status = %q", status)
	}
	if show := ui.scheduleColumns[1].Shows[1]; show.Anime.ID != "" || show.Query != "未知新番" {
		t.Fatalf("unknown show = %#v", show)
	}
	// Seven equal columns side by side with cards under their header. A wide window fits them
	// clear of the scrollbar; the minimum window keeps them readable and scrolls sideways.
	for _, size := range [][2]int{{1000, 618}, {760, 480}, {1400, 900}} {
		ui.window.Resize(0, 0, size[0], size[1])
		ui.resizeSchedule()
		scroll := ui.scheduleScroll
		width := ui.scheduleHeaders[0].W()
		last := ui.scheduleHeaders[6]
		fits := last.X()+last.W()+scheduleMarginRight <= scroll.X()+scroll.W()
		if width < scheduleMinColumnWidth || fits != (size[0] >= 1000) {
			t.Fatalf("window %v: column width %d, fits %v", size, width, fits)
		}
		for day, header := range ui.scheduleHeaders {
			if header.W() != width || day > 0 && header.X() <= ui.scheduleHeaders[day-1].X()+ui.scheduleHeaders[day-1].W() {
				t.Fatalf("window %v: header %d at x=%d w=%d", size, day, header.X(), header.W())
			}
			for _, card := range ui.scheduleCards[day] {
				if card.X() != header.X() || card.W() != width || card.Y() < header.Y()+header.H() {
					t.Fatalf("window %v: card in day %d at x=%d y=%d", size, day, card.X(), card.Y())
				}
			}
		}
	}
	ui.window.Resize(0, 0, 1000, 618)
	ui.resizeSchedule()

	// Returning soon after a fetch shows the cache without fetching again.
	ui.showSection(0)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool {
		return ui.scheduleLoaded && slices.Equal(scheduleCardCounts(ui), []int{1, 2, 1, 0, 0, 0, 0})
	})
	if calls := service.calls(); calls != 1 || ui.scheduleRefreshing {
		t.Fatalf("fetches = %d, refreshing = %v", calls, ui.scheduleRefreshing)
	}
}

func TestNativeScheduleKeepsStoredWeekWhenFetchFails(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("today", 0)},
		fetchErr:              errors.New("bangumi unreachable"),
		release:               make(chan struct{}),
	}
	close(service.release)
	ui := newScheduleViewTestFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return !ui.scheduleRefreshing && ui.scheduleRefreshFailed && ui.scheduleLoaded })
	if ui.scheduleShowCount() != 1 || !strings.HasSuffix(ui.scheduleStatus.Label(), "無法更新時間表，顯示上次記錄。") {
		t.Fatalf("shows = %d, status = %q", ui.scheduleShowCount(), ui.scheduleStatus.Label())
	}

	// With nothing stored, the failure asks for a retry, and 重新整理 fetches again.
	service.setCached(nil)
	ui.loadSchedule()
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 0 })
	if status := ui.scheduleStatus.Label(); status != "無法取得時間表，請按「重新整理」重試。" {
		t.Fatalf("empty status = %q", status)
	}
	ui.focusSchedule()
	if !ui.scheduleRefresh.HasFocus() {
		t.Fatal("an empty schedule does not focus 重新整理")
	}
	service.mu.Lock()
	service.fetchErr = nil
	service.fetched = []backend.ScheduleItem{scheduleTestItem("new", 3)}
	service.mu.Unlock()
	ui.refreshSchedule()
	pumpSearchEvents(t, func() bool { return slices.Equal(scheduleCardCounts(ui), []int{0, 0, 0, 1, 0, 0, 0}) })
	if service.calls() != 2 {
		t.Fatalf("fetches = %d, want 2", service.calls())
	}
}

func TestNativeScheduleShowsLoadFailureAndEmptyWeek(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cachedErr:             errors.New("storage failed"),
		release:               make(chan struct{}),
	}
	close(service.release)
	ui := newScheduleViewTestFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return ui.scheduleLoadFailed && !ui.scheduleRefreshing })
	if status := ui.scheduleStatus.Label(); !strings.HasPrefix(status, "無法讀取已儲存的時間表") || len(ui.scheduleCards) != 0 {
		t.Fatalf("failure status = %q, columns = %d", status, len(ui.scheduleCards))
	}
	service.mu.Lock()
	service.cachedErr = nil
	service.mu.Unlock()
	ui.loadSchedule()
	pumpSearchEvents(t, func() bool { return !ui.scheduleLoadFailed })
	if status := ui.scheduleStatus.Label(); status != "接下來 7 天沒有播出資料。" || len(ui.scheduleHeaders) != 7 {
		t.Fatalf("empty status = %q, headers = %d", status, len(ui.scheduleHeaders))
	}
}

func TestNativeScheduleArrowFocusSkipsEmptyDaysAndKeepsFocusOnReload(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("a", 0), scheduleTestItem("b", 0), scheduleTestItem("c", 3)},
		release:               make(chan struct{}),
	}
	ui := newScheduleViewTestFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 3 })

	// Right from today's second show skips the empty days and lands on the nearest row.
	ui.focusScheduleDay(0, 1, 1)
	if !ui.scheduleCards[3][0].HasFocus() {
		t.Fatal("Right did not reach the next day with shows")
	}
	// Left from the first day returns to the navigation.
	ui.focusScheduleDay(0, 0, -1)
	if !ui.navigation[scheduleSection].HasFocus() {
		t.Fatal("Left from the first day did not return to the navigation")
	}

	// A reload keeps focus on the same day and row.
	ui.focusScheduleCard(0, 1)
	service.setCached([]backend.ScheduleItem{scheduleTestItem("a", 0), scheduleTestItem("b", 0), scheduleTestItem("c", 3), scheduleTestItem("d", 4)})
	ui.loadSchedule()
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 4 })
	if !ui.scheduleCards[0][1].HasFocus() {
		t.Fatal("reload moved focus away from the focused card")
	}
	close(service.release)
}

func TestNativeScheduleKnownShowOpensDetailAndBackReturnsToIt(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("first", 0), scheduleTestItem("second", 0)},
		release:               make(chan struct{}),
	}
	ui := newScheduleViewTestFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 2 })

	ui.openScheduleShow(0, 1)
	pumpSearchEvents(t, func() bool { return ui.searchModel.preview != nil })
	if ui.previewBack.Label() != "返回時間表" || ui.searchModel.preview.Anime.ID != "second" {
		t.Fatalf("back = %q, preview = %#v", ui.previewBack.Label(), ui.searchModel.preview.Anime)
	}
	ui.leaveDetail()
	if ui.selected != scheduleSection || !ui.schedulePage.Visible() || ui.searchPreview {
		t.Fatalf("after back: section %d, schedule visible %v, preview %v", ui.selected, ui.schedulePage.Visible(), ui.searchPreview)
	}
	pumpSearchEvents(t, func() bool {
		return len(ui.scheduleCards) == 7 && len(ui.scheduleCards[0]) == 2 && ui.scheduleCards[0][1].HasFocus()
	})
	close(service.release)
}

func TestNativeScheduleUnknownShowSearchesTheSource(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("", 0)},
		release:               make(chan struct{}),
	}
	ui := newScheduleViewTestFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 1 })

	ui.openScheduleShow(0, 0)
	if ui.selected != 4 || ui.searchPreview || ui.searchInput.Value() != "未知新番" || ui.searchModel.query != "未知新番" {
		t.Fatalf("section %d, preview %v, input %q, query %q", ui.selected, ui.searchPreview, ui.searchInput.Value(), ui.searchModel.query)
	}
	pumpSearchEvents(t, func() bool { return service.searchCalls.Load() > 0 })
	close(service.release)
}

func TestNativeScheduleNarrowWindowScrollsFocusedDayIntoView(t *testing.T) {
	service := &scheduleViewTestService{
		searchViewTestService: &searchViewTestService{},
		cached:                []backend.ScheduleItem{scheduleTestItem("first", 0), scheduleTestItem("last", 6)},
		release:               make(chan struct{}),
	}
	ui := newScheduleViewTestFixture(t, service)
	ui.window.Resize(0, 0, 760, 480)
	ui.resizeSchedule()
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return ui.scheduleShowCount() == 2 })
	scroll := ui.scheduleScroll
	card := ui.scheduleCards[6][0]
	if card.X()+card.W() <= scroll.X()+scroll.W() {
		t.Fatalf("the last day fits at the minimum window (x=%d w=%d); the test needs it off screen", card.X(), card.W())
	}
	ui.focusScheduleDay(0, 0, 1)
	if !card.HasFocus() || card.X() < scroll.X() || card.X()+card.W() > scroll.X()+scroll.W() {
		t.Fatalf("focused card at x=%d w=%d, list x=%d w=%d", card.X(), card.W(), scroll.X(), scroll.W())
	}
	close(service.release)
}
