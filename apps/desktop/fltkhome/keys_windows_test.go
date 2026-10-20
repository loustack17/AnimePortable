//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

func TestClassifyKey(t *testing.T) {
	for _, test := range []struct {
		name       string
		key, state int
		want       keyCommand
	}{
		{"enter", fltk.ENTER_KEY, 0, keyActivate},
		{"enter 13", 13, 0, keyActivate},
		{"keypad enter", 0xff8d, 0, keyActivate},
		{"space", ' ', 0, keyActivate},
		{"escape", fltk.ESCAPE, 0, keyBack},
		{"escape 27", 27, 0, keyBack},
		{"up", fltk.UP, 0, keyUp},
		{"down", fltk.DOWN, 0, keyDown},
		{"left", fltk.LEFT, 0, keyLeft},
		{"right", fltk.RIGHT, 0, keyRight},
		{"tab", fltk.TAB, 0, keyNext},
		{"shift tab", fltk.TAB, fltk.SHIFT, keyPrevious},
		{"slash", '/', 0, keySearch},
		{"ctrl k", 'k', fltk.CTRL, keySearch},
		{"ctrl shift k", 'K', fltk.CTRL | fltk.SHIFT, keySearch},
		{"ctrl enter", fltk.ENTER_KEY, fltk.CTRL, keyNone},
		{"alt escape", fltk.ESCAPE, fltk.ALT, keyNone},
		{"ctrl alt k", 'k', fltk.CTRL | fltk.ALT, keyNone},
		{"win k", 'k', fltk.META, keyNone},
		{"plain k", 'k', 0, keyNone},
	} {
		if got := classifyKey(test.key, test.state); got != test.want {
			t.Errorf("%s: classifyKey(%#x, %#x) = %d, want %d", test.name, test.key, test.state, got, test.want)
		}
	}
}

var sendMessage = syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW")

// Windows virtual-key codes for the keys the tests press.
const (
	vkReturn = 0x0D
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkSlash  = 0xBF
)

// pressKey sends a key press and release to the window as Windows delivers it, so FLTK runs
// its whole keyboard path: the focused widget, then the window shortcut, then FLTK's default
// Escape handling.
func pressKey(t *testing.T, window *fltk.Window, virtualKey uintptr) {
	t.Helper()
	handle := window.RawHandle()
	if handle == 0 {
		t.Fatal("window has no native handle")
	}
	var extended uintptr
	if virtualKey >= vkLeft && virtualKey <= vkDown {
		extended = 1 << 24 // the arrow keys, not the keypad
	}
	const wmKeyDown, wmKeyUp = 0x0100, 0x0101
	sendMessage.Call(handle, wmKeyDown, virtualKey, 1|extended)
	sendMessage.Call(handle, wmKeyUp, virtualKey, 1|extended|3<<30)
}

// pumpFor runs FLTK events for a while, for checks that nothing happens.
func pumpFor(duration time.Duration) {
	for deadline := time.Now().Add(duration); time.Now().Before(deadline); {
		fltk.Wait(0.01)
	}
}

func keyboardTestLibrary(count int) []backend.Anime {
	library := make([]backend.Anime, count)
	for index := range library {
		library[index] = backend.Anime{ID: fmt.Sprintf("anime-%02d", index), Title: fmt.Sprintf("作品 %02d", index)}
	}
	return library
}

func keyboardTestHistory(count int) []backend.History {
	history := make([]backend.History, count)
	for index := range history {
		history[index] = backend.History{
			AnimeID: fmt.Sprintf("anime-%02d", index), EpisodeID: fmt.Sprintf("episode-%02d", index), Position: 1000,
			LastPlayed: time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC).Add(-time.Duration(index) * time.Hour).Format(time.RFC3339),
		}
	}
	return history
}

func TestNativeEscapeGoesBackAndNeverClosesTheApp(t *testing.T) {
	service := &homeRefreshTestService{}
	ui := newHomeRefreshFixture(t, service)
	ui.showSection(0)
	ui.navigation[0].TakeFocus()

	pressKey(t, ui.window, vkEscape)
	pumpFor(100 * time.Millisecond)
	if ui.closing || ui.selected != 0 || !ui.window.IsShown() {
		t.Fatalf("Escape on Home: closing %v, section %d, shown %v", ui.closing, ui.selected, ui.window.IsShown())
	}

	for _, section := range []int{scheduleSection, followingSection, historySection, SearchSection, 5} {
		ui.showSection(section)
		ui.navigation[section].TakeFocus()
		pressKey(t, ui.window, vkEscape)
		pumpSearchEvents(t, func() bool { return ui.selected == 0 && ui.navigation[0].HasFocus() })
	}

	// The detail screen returns to the page it was opened from.
	ui.openDetailFrom(detailFromFollowing, backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview })
	ui.previewBack.TakeFocus()
	pressKey(t, ui.window, vkEscape)
	// A detail queued behind 搜尋's library load reopens when the library lands, which starts a
	// new page generation; the Escape already pressed must still go back (CI 39498639041).
	ui.pageGeneration++
	pumpSearchEvents(t, func() bool { return ui.selected == followingSection && !ui.searchPreview })

	// Two presses before the first one runs go back one page, not two.
	ui.openDetailFrom(detailFromFollowing, backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview })
	ui.previewBack.TakeFocus()
	pressKey(t, ui.window, vkEscape)
	pressKey(t, ui.window, vkEscape)
	pumpSearchEvents(t, func() bool { return ui.selected == followingSection })
	pumpFor(100 * time.Millisecond)
	if ui.selected != followingSection {
		t.Fatalf("two quick Escapes left 追蹤 for section %d", ui.selected)
	}

	// Before the main screen shows, no page handles Escape; FLTK's default must not close the app.
	ui.mainGroup.Hide()
	pressKey(t, ui.window, vkEscape)
	pumpFor(100 * time.Millisecond)
	if ui.closing {
		t.Fatal("Escape closed the app")
	}
}

func TestNativeSlashOpensSearchFromEveryScreen(t *testing.T) {
	service := &homeRefreshTestService{}
	ui := newHomeRefreshFixture(t, service)
	for _, section := range []int{0, scheduleSection, followingSection, historySection, 5, SearchSection} {
		ui.showSection(section)
		ui.navigation[section].TakeFocus()
		ui.searchInput.SetValue("舊查詢")
		pressKey(t, ui.window, vkSlash)
		pumpSearchEvents(t, func() bool { return ui.selected == SearchSection && ui.searchInput.HasFocus() })
		if value := ui.searchInput.Value(); value != "舊查詢" || ui.searchInput.InsertPosition() != len(value) || ui.searchInput.Mark() != 0 {
			t.Fatalf("from section %d: input %q selection %d..%d, want the whole query selected", section, value, ui.searchInput.Mark(), ui.searchInput.InsertPosition())
		}
	}

	ui.openDetailFrom(detailFromHome, backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview })
	ui.previewBack.TakeFocus()
	pressKey(t, ui.window, vkSlash)
	pumpSearchEvents(t, func() bool { return !ui.searchPreview && ui.searchInput.HasFocus() })
	// A result opened next returns to the results, not to Home where the first detail came from.
	ui.openSearchResult(backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview })
	if label := ui.previewBack.Label(); label != "返回結果" {
		t.Fatalf("back button after / = %q, want 返回結果", label)
	}
	ui.previewBack.TakeFocus()
	pressKey(t, ui.window, vkEscape)
	pumpSearchEvents(t, func() bool { return ui.selected == SearchSection && !ui.searchPreview })

	// Ctrl+K reaches the window only when no widget takes it; the input does, so it selects
	// the query there instead of deleting the rest of the line. `/` in the input is typed.
	ui.searchInput.SetInsertPosition(2, 2)
	if !ui.searchInputKey(keySearch, true) || ui.searchInput.Value() != "舊查詢" || ui.searchInput.Mark() != 0 || ui.searchInput.InsertPosition() != len("舊查詢") {
		t.Fatal("Ctrl+K in the input did not select the query")
	}
	if ui.searchInputKey(keySearch, false) {
		t.Fatal("`/` in the input was taken instead of typed")
	}
	ui.showSection(followingSection)
	if !ui.windowKey(keySearch, 'k') {
		t.Fatal("Ctrl+K was not handled by the window")
	}
	pumpSearchEvents(t, func() bool { return ui.selected == SearchSection && ui.searchInput.HasFocus() })
}

func TestNativeSearchKeysReachResultsAndKeepFocus(t *testing.T) {
	service := &homeRefreshTestService{library: keyboardTestLibrary(3)}
	ui := newHomeRefreshFixture(t, service)
	ui.openQuickSearch()
	pumpSearchEvents(t, func() bool { return ui.searchModel.loaded })
	ui.searchInput.SetValue("作品")
	ui.searchModel.setQuery("作品")
	ui.renderSearch()
	if len(ui.searchRows) != 3 {
		t.Fatalf("results = %d, want 3", len(ui.searchRows))
	}
	ui.searchInput.TakeFocus()
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.searchRows[0].HasFocus() })
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.searchRows[1].HasFocus() })

	// Results arriving rebuild the rows; the focused row stays focused.
	ui.renderSearch()
	if !ui.searchRows[1].HasFocus() {
		t.Fatal("focus left the second result after the results were rebuilt")
	}
}

func TestNativeRowListArrowsScrollAndLeaveToNavigation(t *testing.T) {
	const rows = 10
	service := &homeRefreshTestService{library: keyboardTestLibrary(rows), history: keyboardTestHistory(rows)}
	ui := newHomeRefreshFixture(t, service)
	var played []backend.PlayRequest
	ui.onPlay = func(request backend.PlayRequest) { played = append(played, request) }
	ui.showSection(historySection)
	pumpSearchEvents(t, func() bool { return !ui.historyLoading && len(ui.historyRowIDs) == rows })

	ui.historyList().focus(0, 0)
	for row := 1; row < rows; row++ {
		pressKey(t, ui.window, vkDown)
		pumpSearchEvents(t, func() bool { return ui.historyButtons[row*2].HasFocus() })
		button, scroll := ui.historyButtons[row*2], ui.historyScroll
		if button.Y() < scroll.Y() || button.Y()+button.H() > scroll.Y()+scroll.H() {
			t.Fatalf("row %d at y=%d..%d is outside the list %d..%d", row, button.Y(), button.Y()+button.H(), scroll.Y(), scroll.Y()+scroll.H())
		}
	}
	pressKey(t, ui.window, vkDown)
	pumpFor(50 * time.Millisecond)
	if !ui.historyButtons[(rows-1)*2].HasFocus() {
		t.Fatal("Down on the last row moved focus")
	}
	pressKey(t, ui.window, vkRight)
	pumpSearchEvents(t, func() bool { return ui.historyButtons[(rows-1)*2+1].HasFocus() })
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return ui.historyButtons[(rows-1)*2].HasFocus() })
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return ui.navigation[historySection].HasFocus() })

	// Up from the first row: 重新載入 is hidden after a good load, so the navigation.
	ui.historyList().focus(0, 0)
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.navigation[historySection].HasFocus() })

	// Space activates the row like Enter: it continues playback.
	ui.historyList().focus(0, 0)
	pressKey(t, ui.window, vkSpace)
	pumpSearchEvents(t, func() bool { return len(played) == 1 })
	if played[0].AnimeID != "anime-00" {
		t.Fatalf("Space played %#v, want the first row", played[0])
	}
}

func TestNativeFollowingPageButtonAndRowsConnect(t *testing.T) {
	seen := time.Now().UTC().Format(time.RFC3339Nano)
	service := &homeRefreshTestService{following: []backend.Following{{AnimeID: "anime", HasAvailable: true, LatestAvailableNumber: "3", LatestSeenAt: seen}}}
	ui := newHomeRefreshFixture(t, service)
	ui.showSection(followingSection)
	pumpSearchEvents(t, func() bool { return len(ui.followingRowIDs) == 1 })
	ui.followingList().focus(0, 0)
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.followingRefresh.HasFocus() })
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.followingButtons[0].HasFocus() })
}

func TestNativeScheduleRefreshButtonLeadsToTheCards(t *testing.T) {
	service := &homeRefreshTestService{schedule: []backend.ScheduleItem{scheduleTestItem("anime", 1)}}
	ui := newHomeRefreshFixture(t, service)
	ui.showSection(scheduleSection)
	pumpSearchEvents(t, func() bool { return len(ui.scheduleCards) > 1 && len(ui.scheduleCards[1]) == 1 })
	ui.scheduleRefresh.TakeFocus()
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.scheduleCards[1][0].HasFocus() })
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.scheduleRefresh.HasFocus() })
}

func TestNativeHomeFocusSurvivesReload(t *testing.T) {
	seen := time.Now().UTC().Format(time.RFC3339Nano)
	service := &homeRefreshTestService{
		library: keyboardTestLibrary(2),
		history: keyboardTestHistory(2),
		following: []backend.Following{
			{AnimeID: "anime-00", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: seen},
			{AnimeID: "anime-01", HasAvailable: true, LatestAvailableNumber: "3", RecentlyUpdated: true, LatestSeenAt: seen},
		},
	}
	ui := newHomeRefreshFixture(t, service)
	reload := func() {
		ui.loadHome()
		pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.homeUpdateButtons) == 2 && len(ui.cardButtons) == 2 })
	}
	reload()

	ui.focusTarget(ui.homeUpdateButtons[1])
	reload()
	if !ui.homeUpdateButtons[1].HasFocus() {
		t.Fatal("focus left the second 近期更新 row after a reload")
	}
	ui.focusTarget(ui.cardButtons[1])
	reload()
	if !ui.cardButtons[1].HasFocus() {
		t.Fatal("focus left the second 繼續播放 card after a reload")
	}

	// The anime in focus is gone: the card now at its place takes focus.
	service.setHistory(keyboardTestHistory(1), nil)
	ui.focusTarget(ui.cardButtons[1])
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.cardButtons) == 1 })
	if !ui.cardButtons[0].HasFocus() {
		t.Fatal("focus did not move to the remaining card")
	}

	// Focus moved elsewhere while loading stays there.
	ui.focusTarget(ui.cardButtons[0])
	ui.loadHome()
	ui.navigation[followingSection].TakeFocus()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading })
	if !ui.navigation[followingSection].HasFocus() {
		t.Fatal("the reload took focus back from the navigation")
	}
}

func TestNativeReturnFromPlayerFocusesTheDestination(t *testing.T) {
	ui := newHomeRefreshFixture(t, &homeRefreshTestService{})
	ui.navigate(followingSection)
	pumpSearchEvents(t, func() bool { return ui.navigation[followingSection].HasFocus() })
	ui.navigate(0)
	pumpSearchEvents(t, func() bool { return ui.navigation[0].HasFocus() })
	ui.navigate(SearchSection)
	pumpSearchEvents(t, func() bool { return ui.searchInput.HasFocus() })
}

func TestNativeDetailArrowsLinkButtonsAndEpisodes(t *testing.T) {
	service := &homeRefreshTestService{}
	service.setEpisodes("anime", []backend.Episode{{ID: "episode-1", Number: "1"}, {ID: "episode-2", Number: "2"}})
	ui := newHomeRefreshFixture(t, service)
	ui.openDetailFrom(detailFromHome, backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview && len(ui.previewEpisodeRows) == 2 })
	ui.focusDetailEpisodeRow(0)
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.previewBack.HasFocus() })
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.previewEpisodeRows[0].HasFocus() })
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.previewEpisodeRows[1].HasFocus() })
}
