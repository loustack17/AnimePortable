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

	"animeportable/apps/desktop/backend"
)

type historyViewTestService struct {
	*searchViewTestService
	mu         sync.Mutex
	history    []backend.History
	historyErr error
	removeErr  error
	removed    []string
}

func (service *historyViewTestService) Library(context.Context) ([]backend.Anime, error) {
	return []backend.Anime{{ID: "first", Title: "甲作品"}, {ID: "second", Title: "乙作品"}}, nil
}

func (service *historyViewTestService) History(context.Context) ([]backend.History, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.history), service.historyErr
}

func (service *historyViewTestService) RemoveHistory(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.removeErr != nil {
		return service.removeErr
	}
	service.removed = append(service.removed, animeID)
	service.history = slices.DeleteFunc(service.history, func(item backend.History) bool { return item.AnimeID == animeID })
	return nil
}

func (service *historyViewTestService) setHistory(history []backend.History) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.history = history
}

func newHistoryViewTestFixture(t *testing.T, service *historyViewTestService) (*view, *[]backend.PlayRequest) {
	t.Helper()
	ui := newSearchViewTestFixture(t, service)
	ui.mainGroup.Begin()
	ui.buildHistoryView()
	ui.mainGroup.End()
	played := &[]backend.PlayRequest{}
	ui.onPlay = func(request backend.PlayRequest) { *played = append(*played, request) }
	return ui, played
}

func historyTestEntries() []backend.History {
	return []backend.History{
		{AnimeID: "first", EpisodeID: "first-3", EpisodeNumber: "3", Position: 90000, Duration: 1440000, LastPlayed: "2026-10-19T10:00:00Z"},
		{AnimeID: "second", EpisodeID: "second-12", EpisodeNumber: "12", Position: 1440000, Duration: 1440000, Completed: true, LastPlayed: "2026-10-18T10:00:00Z"},
	}
}

func TestNativeHistoryPageContinuesAndRemoves(t *testing.T) {
	service := &historyViewTestService{searchViewTestService: &searchViewTestService{}, history: historyTestEntries()}
	ui, played := newHistoryViewTestFixture(t, service)

	ui.showSection(historySection)
	pumpSearchEvents(t, func() bool { return ui.historyLoaded && len(ui.historyRowIDs) == 2 })
	if !slices.Equal(ui.historyRowIDs, []string{"first", "second"}) {
		t.Fatalf("rows = %#v, want newest first", ui.historyRowIDs)
	}
	if !ui.historyPage.Visible() || ui.scroll.Visible() || ui.searchPage.Visible() {
		t.Fatal("歷史紀錄 is not the visible page")
	}
	if status := ui.historyStatus.Label(); !strings.Contains(status, "2 部作品") || !strings.Contains(status, "只存在這台電腦") {
		t.Fatalf("status = %q", status)
	}
	for _, size := range [][2]int{{1000, 618}, {760, 480}, {1400, 900}} {
		ui.window.Resize(0, 0, size[0], size[1])
		ui.resizeHistory()
		remove := ui.historyButtons[1]
		if remove.Label() != "移除紀錄" || remove.X()+remove.W() != ui.historyRetry.X()+ui.historyRetry.W() || remove.X()+remove.W() > ui.historyScroll.X()+ui.historyScroll.W() {
			t.Fatalf("window %v: remove icon at x=%d w=%d", size, remove.X(), remove.W())
		}
	}

	// Continue: the partly watched episode resumes; the finished one restarts.
	ui.continueHistory(ui.historyItems[0].History)
	ui.continueHistory(ui.historyItems[1].History)
	want := []backend.PlayRequest{{AnimeID: "first", EpisodeID: "first-3", StartAt: 90000}, {AnimeID: "second", EpisodeID: "second-12"}}
	if !slices.Equal(*played, want) {
		t.Fatalf("play requests = %#v, want %#v", *played, want)
	}

	// While a reload is running the rows show but cannot be played: their positions may be stale.
	ui.loadHistory()
	ui.continueHistory(ui.historyItems[0].History)
	pumpSearchEvents(t, func() bool { return !ui.historyLoading })
	if len(*played) != 2 {
		t.Fatalf("play during reload = %#v", *played)
	}

	// Remove the focused first row: it disappears and focus moves to the row now in its place.
	ui.historyButtons[1].TakeFocus()
	ui.removeHistory("first")
	pumpSearchEvents(t, func() bool { return slices.Equal(ui.historyRowIDs, []string{"second"}) })
	if !ui.historyButtons[1].HasFocus() {
		t.Fatal("focus did not stay on the remaining row's remove icon")
	}
	service.mu.Lock()
	removed := slices.Clone(service.removed)
	service.mu.Unlock()
	if !slices.Equal(removed, []string{"first"}) || ui.message.Label() != "" {
		t.Fatalf("removed = %#v, message = %q", removed, ui.message.Label())
	}

	// Leaving and returning reloads: the removed entry stays removed and new playback shows.
	ui.showSection(0)
	service.setHistory(append(historyTestEntries()[1:], backend.History{AnimeID: "first", EpisodeID: "first-4", EpisodeNumber: "4", Position: 1000, LastPlayed: "2026-10-19T11:00:00Z"}))
	ui.showSection(historySection)
	pumpSearchEvents(t, func() bool { return slices.Equal(ui.historyRowIDs, []string{"first", "second"}) })
}

func TestNativeHistoryPageEmptyFailureAndRemoveError(t *testing.T) {
	service := &historyViewTestService{searchViewTestService: &searchViewTestService{}}
	ui, _ := newHistoryViewTestFixture(t, service)
	ui.showSection(historySection)
	pumpSearchEvents(t, func() bool { return ui.historyLoaded })
	if status := ui.historyStatus.Label(); !strings.HasPrefix(status, "還沒有觀看紀錄。") || len(ui.historyButtons) != 0 || ui.historyRetry.Visible() {
		t.Fatalf("empty status = %q, buttons = %d, retry = %v", status, len(ui.historyButtons), ui.historyRetry.Visible())
	}

	service.mu.Lock()
	service.historyErr = nil
	service.history = historyTestEntries()
	service.mu.Unlock()
	ui.loadHistory()
	pumpSearchEvents(t, func() bool { return len(ui.historyRowIDs) == 2 })
	service.mu.Lock()
	service.historyErr = errors.New("database locked")
	service.mu.Unlock()
	ui.loadHistory()
	pumpSearchEvents(t, func() bool { return ui.historyLoadFailed })
	// A failed reload drops the old rows, so a stale or removed position cannot be played.
	if len(ui.historyRowIDs) != 0 {
		t.Fatalf("rows after failed reload = %#v", ui.historyRowIDs)
	}
	if status := ui.historyStatus.Label(); !strings.HasPrefix(status, "無法載入觀看紀錄") || !ui.historyRetry.Visible() {
		t.Fatalf("failure status = %q, retry = %v", status, ui.historyRetry.Visible())
	}

	service.mu.Lock()
	service.historyErr = nil
	service.history = historyTestEntries()
	service.removeErr = errors.New("database locked")
	service.mu.Unlock()
	ui.loadHistory()
	pumpSearchEvents(t, func() bool { return !ui.historyLoadFailed && len(ui.historyRowIDs) == 2 })
	if ui.historyRetry.Visible() {
		t.Fatal("retry still visible after a successful reload")
	}
	ui.removeHistory("first")
	pumpSearchEvents(t, func() bool { return !ui.historyRemovePending && ui.message.Label() != "" })
	if ui.message.Label() != "無法移除紀錄，請重試。" || len(ui.historyRowIDs) != 2 {
		t.Fatalf("message = %q, rows = %#v", ui.message.Label(), ui.historyRowIDs)
	}
}
