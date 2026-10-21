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
	following  []string
	followErr  error
	followed   []string
	unfollowed []string
}

func (service *historyViewTestService) Following(context.Context) ([]backend.Following, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	result := make([]backend.Following, 0, len(service.following))
	for _, animeID := range service.following {
		result = append(result, backend.Following{AnimeID: animeID})
	}
	return result, nil
}

func (service *historyViewTestService) Follow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.followErr != nil {
		return service.followErr
	}
	service.followed = append(service.followed, animeID)
	service.following = append(service.following, animeID)
	return nil
}

func (service *historyViewTestService) Unfollow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.followErr != nil {
		return service.followErr
	}
	service.unfollowed = append(service.unfollowed, animeID)
	service.following = slices.DeleteFunc(service.following, func(id string) bool { return id == animeID })
	return nil
}

func (service *historyViewTestService) followCalls() (followed, unfollowed []string) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.followed), slices.Clone(service.unfollowed)
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
		title, star, remove := ui.historyList().button(0, 0), ui.historyList().button(0, 1), ui.historyList().button(0, 2)
		if remove.Label() != "移除紀錄" || remove.X()+remove.W() != ui.historyRetry.X()+ui.historyRetry.W() || remove.X()+remove.W() > ui.historyScroll.X()+ui.historyScroll.W() {
			t.Fatalf("window %v: remove icon at x=%d w=%d", size, remove.X(), remove.W())
		}
		// The star sits between the title and the remove icon, with the same gaps.
		if star.W() != rowIconSize || title.X()+title.W()+8 != star.X() || star.X()+star.W()+8 != remove.X() {
			t.Fatalf("window %v: title ends at %d, star at x=%d w=%d, remove at x=%d", size, title.X()+title.W(), star.X(), star.W(), remove.X())
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
	ui.historyList().button(0, 2).TakeFocus()
	ui.removeHistory("first")
	pumpSearchEvents(t, func() bool { return slices.Equal(ui.historyRowIDs, []string{"second"}) })
	if !ui.historyList().button(0, 2).HasFocus() {
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

func TestNativeHistoryStarFollowsAndUnfollows(t *testing.T) {
	service := &historyViewTestService{searchViewTestService: &searchViewTestService{}, history: historyTestEntries(), following: []string{"second"}}
	ui, played := newHistoryViewTestFixture(t, service)
	ui.showSection(historySection)
	pumpSearchEvents(t, func() bool { return !ui.historyLoading && len(ui.historyRowIDs) == 2 })
	starLabel := func(row int) string { return ui.historyList().button(row, 1).Label() }
	if starLabel(0) != "追蹤" || starLabel(1) != "取消追蹤" {
		t.Fatalf("stars = %q, %q; want 追蹤 for the unfollowed row, 取消追蹤 for the followed one", starLabel(0), starLabel(1))
	}

	// Following keeps focus on the star; a second toggle while one is pending is ignored.
	ui.historyList().button(0, 1).TakeFocus()
	ui.toggleHistoryFollow("first")
	ui.toggleHistoryFollow("second")
	pumpSearchEvents(t, func() bool { return !ui.followPending && starLabel(0) == "取消追蹤" })
	if followed, unfollowed := service.followCalls(); !slices.Equal(followed, []string{"first"}) || len(unfollowed) != 0 {
		t.Fatalf("followed = %#v, unfollowed = %#v", followed, unfollowed)
	}
	if !ui.historyList().button(0, 1).HasFocus() || ui.message.Label() != "" || len(*played) != 0 {
		t.Fatalf("focus on star = %v, message = %q, played = %#v", ui.historyList().button(0, 1).HasFocus(), ui.message.Label(), *played)
	}

	ui.toggleHistoryFollow("second")
	pumpSearchEvents(t, func() bool { return !ui.followPending && starLabel(1) == "追蹤" })
	if _, unfollowed := service.followCalls(); !slices.Equal(unfollowed, []string{"second"}) {
		t.Fatalf("unfollowed = %#v", unfollowed)
	}

	// A failure keeps the old state and says so.
	service.mu.Lock()
	service.followErr = errors.New("database locked")
	service.mu.Unlock()
	ui.toggleHistoryFollow("first")
	pumpSearchEvents(t, func() bool { return !ui.followPending && ui.message.Label() != "" })
	if ui.message.Label() != "無法更新追蹤，請重試。" || starLabel(0) != "取消追蹤" {
		t.Fatalf("message = %q, star = %q", ui.message.Label(), starLabel(0))
	}

	// A reload reads the stored state again.
	ui.loadHistory()
	pumpSearchEvents(t, func() bool { return !ui.historyLoading })
	if starLabel(0) != "取消追蹤" || starLabel(1) != "追蹤" {
		t.Fatalf("stars after reload = %q, %q", starLabel(0), starLabel(1))
	}
}
