//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"slices"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

// resizeTestService opens the stored anime on the detail screen, so its follow star shows.
type resizeTestService struct{ *homeRefreshTestService }

func (resizeTestService) Detail(context.Context, string) (backend.Detail, error) {
	return backend.Detail{Anime: backend.Anime{ID: "anime", Title: "作品"}}, nil
}

// Windows mouse messages and the left-button flag the tests send.
const (
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	mkLButton     = 0x0001
)

// sendMouse sends a mouse message at window coordinates x, y as Windows delivers it, scaled
// to the screen's pixels, so FLTK runs its whole click path from the window down.
func sendMouse(t *testing.T, window *fltk.Window, message, buttons uintptr, x, y int) {
	t.Helper()
	handle := window.RawHandle()
	if handle == 0 {
		t.Fatal("window has no native handle")
	}
	scale := float64(fltk.ScreenScale(fltk.ScreenNum(window.XRoot(), window.YRoot())))
	pixelX, pixelY := uintptr(float64(x)*scale+0.5), uintptr(float64(y)*scale+0.5)
	sendMessage.Call(handle, message, buttons, pixelY<<16|pixelX&0xffff)
}

// checkPressReaches presses the mouse at the centre of button and checks that FLTK delivered
// the press to it: a pressed button takes focus. The mouse is released away from the button,
// so its action does not run.
func checkPressReaches(t *testing.T, ui *view, name string, button *fltk.Button) {
	t.Helper()
	ui.navigation[ui.selected].TakeFocus()
	sendMouse(t, ui.window, wmLButtonDown, mkLButton, button.X()+button.W()/2, button.Y()+button.H()/2)
	reached := button.HasFocus()
	sendMouse(t, ui.window, wmMouseMove, mkLButton, 1, 1)
	sendMouse(t, ui.window, wmLButtonUp, 0, 1, 1)
	if !reached {
		t.Errorf("%s at %d,%d %dx%d does not receive a click in a %dx%d window", name, button.X(), button.Y(), button.W(), button.H(), ui.window.W(), ui.window.H())
	}
}

type placedWidget interface {
	X() int
	Y() int
	W() int
	H() int
	Visible() bool
	Label() string
	Parent() *fltk.Group
}

// checkInsideAncestors checks that a shown widget, and every group holding it, lies inside the
// group above it up to the window: FLTK drops clicks on any part outside. A scroll's content
// may extend past the scroll, which scrolls it into view.
func checkInsideAncestors(t *testing.T, window *fltk.Window, scrolls []*fltk.Scroll, widget placedWidget) {
	t.Helper()
	chain := []placedWidget{widget}
	for parent := widget.Parent(); parent.Type() != window.Type(); parent = parent.Parent() {
		chain = append(chain, parent)
	}
	for _, link := range chain {
		if !link.Visible() {
			return
		}
	}
	for index, link := range chain {
		left, top, right, bottom := 0, 0, window.W(), window.H()
		if index+1 < len(chain) {
			parent := chain[index+1].(*fltk.Group)
			if slices.ContainsFunc(scrolls, func(scroll *fltk.Scroll) bool { return isScroll(parent, scroll) }) {
				continue
			}
			left, top, right, bottom = parent.X(), parent.Y(), parent.X()+parent.W(), parent.Y()+parent.H()
		}
		if link.X() < left || link.Y() < top || link.X()+link.W() > right || link.Y()+link.H() > bottom {
			t.Errorf("window %dx%d: %q at %d,%d %dx%d sticks out of its parent %d,%d..%d,%d", window.W(), window.H(), link.Label(), link.X(), link.Y(), link.W(), link.H(), left, top, right, bottom)
			return
		}
	}
}

// isScroll tells whether group, a wrapper Parent() made, is scroll: same type and geometry.
// The page groups share Home's scroll geometry but are plain groups.
func isScroll(group *fltk.Group, scroll *fltk.Scroll) bool {
	return group.Type() == scroll.Type() && group.X() == scroll.X() && group.Y() == scroll.Y() && group.W() == scroll.W() && group.H() == scroll.H()
}

// checkLayout checks every shown child of the window's groups.
func checkLayout(t *testing.T, ui *view) {
	t.Helper()
	scrolls := []*fltk.Scroll{ui.scroll, ui.searchScroll, ui.previewEpisodes, ui.followingScroll, ui.historyScroll, ui.scheduleScroll}
	groups := []*fltk.Group{&ui.window.Group, ui.mainGroup, ui.startupGroup, ui.navigation[0].Parent(), ui.searchPage, ui.previewGroup,
		ui.followingPage, ui.historyPage, ui.schedulePage}
	for _, scroll := range scrolls {
		groups = append(groups, &scroll.Group)
	}
	for _, group := range groups {
		for _, child := range group.Children() {
			checkInsideAncestors(t, ui.window, scrolls, child)
		}
	}
}

func TestNativeWindowLayoutFollowsResizeAndAcceptsClicks(t *testing.T) {
	seen := time.Now().UTC().Format(time.RFC3339Nano)
	ui := newHomeRefreshFixture(t, resizeTestService{&homeRefreshTestService{
		history:   []backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}},
		following: []backend.Following{{AnimeID: "anime", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: seen}},
		schedule:  []backend.ScheduleItem{{AnimeID: "anime", Title: "作品", Query: "作品", Weekday: int(time.Now().Weekday())}},
	}})
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.homeUpdateButtons) == 1 && len(ui.cardButtons) == 1 })

	for _, size := range [][2]int{{1600, 1000}, {1400, 900}, {760, 480}, {1000, 618}} {
		ui.window.Resize(0, 0, size[0], size[1])
		pumpFor(50 * time.Millisecond)
		width, height := ui.window.W(), ui.window.H()
		if width != size[0] || height != size[1] {
			t.Fatalf("window is %dx%d, want %dx%d; the desktop may be too small for this test", width, height, size[0], size[1])
		}

		navPanel := ui.navigation[0].Parent()
		navBackground := navPanel.Child(0)
		if navPanel.Y() != 18 || navPanel.Y()+navPanel.H() != height-18 || navBackground.Y() != navPanel.Y() || navBackground.H() != navPanel.H() {
			t.Errorf("window %dx%d: navigation spans y=%d..%d, background %d..%d, want 18..%d", width, height, navPanel.Y(), navPanel.Y()+navPanel.H(), navBackground.Y(), navBackground.Y()+navBackground.H(), height-18)
		}
		// The pages' content keeps its place; only what sits at the right or bottom edge moves.
		for position, index := range navigationOrder {
			if button := ui.navigation[index]; button.X() != 27 || button.Y() != 111+position*55 || button.W() != 177 || button.H() != 43 {
				t.Errorf("window %dx%d: navigation button %q moved to %d,%d %dx%d", width, height, button.Label(), button.X(), button.Y(), button.W(), button.H())
			}
		}
		if ui.sectionTitle.X() != 252 || ui.sectionTitle.Y() != 35 || ui.sectionTitle.W() != min(580, width-272) || ui.message.X() != 280 || ui.message.X()+ui.message.W() != width-30 {
			t.Errorf("window %dx%d: title at %d,%d w=%d, message %d..%d", width, height, ui.sectionTitle.X(), ui.sectionTitle.Y(), ui.sectionTitle.W(), ui.message.X(), ui.message.X()+ui.message.W())
		}
		if ui.themeToggle.Y() != 18 || ui.themeToggle.X()+ui.themeToggle.W() != width-30 {
			t.Errorf("window %dx%d: theme toggle ends at x=%d, want %d", width, height, ui.themeToggle.X()+ui.themeToggle.W(), width-30)
		}

		ui.showSection(0)
		pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.cardButtons) == 1 && !ui.followingRefreshing })
		// Home's rows fill the scroll width, but never shrink below the 1000-px design.
		want := max(711, ui.scroll.W()-30)
		if row := ui.homeUpdateButtons[0]; row.W() != want {
			t.Errorf("window %dx%d: 近期更新 row is %d wide, want %d", width, height, row.W(), want)
		}
		if card, play := ui.cards[0], ui.cardButtons[0]; card.W() != want || play.X()+play.W() != card.X()+card.W()-22 {
			t.Errorf("window %dx%d: 繼續觀看 card is %d wide with 繼續播放 ending %d before its edge, want %d and 22", width, height, card.W(), card.X()+card.W()-play.X()-play.W(), want)
		}
		checkLayout(t, ui)
		if ui.scroll.W() >= 741 {
			scrollIntoView(ui.scroll, ui.cardButtons[0])
			checkPressReaches(t, ui, "繼續播放", ui.cardButtons[0])
		}

		ui.showSection(SearchSection)
		pumpSearchEvents(t, func() bool { return ui.searchPage.Visible() && ui.searchButton.IsActive() })
		checkLayout(t, ui)
		checkPressReaches(t, ui, "搜尋", ui.searchButton)
		ui.openDetailFrom(detailFromSearch, backend.Anime{ID: "anime", Title: "作品"})
		pumpSearchEvents(t, func() bool { return ui.searchPreview && ui.searchModel.preview != nil && ui.previewFollow.Visible() })
		checkLayout(t, ui)
		checkPressReaches(t, ui, "detail 追蹤", ui.previewFollow)

		ui.showSection(followingSection)
		pumpSearchEvents(t, func() bool { return ui.followingLoaded && !ui.followingRefreshing && len(ui.followingButtons) == 2 })
		checkLayout(t, ui)
		checkPressReaches(t, ui, "更新集數", ui.followingRefresh)
		checkPressReaches(t, ui, "取消追蹤", ui.followingButtons[1])

		ui.showSection(historySection)
		pumpSearchEvents(t, func() bool { return ui.historyLoaded && len(ui.historyButtons) == 3 })
		checkLayout(t, ui)
		checkPressReaches(t, ui, "歷史紀錄 追蹤", ui.historyList().button(0, 1))
		checkPressReaches(t, ui, "移除紀錄", ui.historyList().button(0, 2))

		ui.showSection(scheduleSection)
		pumpSearchEvents(t, func() bool { return ui.scheduleLoaded && !ui.scheduleRefreshing && len(ui.scheduleCards) > 0 })
		checkLayout(t, ui)
		checkPressReaches(t, ui, "重新整理", ui.scheduleRefresh)

		ui.showSection(5)
		checkLayout(t, ui)
	}
}
