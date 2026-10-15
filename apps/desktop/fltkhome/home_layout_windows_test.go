//go:build windows

package fltkhome

import (
	"context"
	"runtime"
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

func TestContinueWatchingLoadedRowsStayAdjacentToHeading(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	window := fltk.NewWindow(1000, 618, "Home layout test")
	ui := &view{window: window, ctx: context.Background()}
	ui.build()
	ui.mainGroup.Show()
	window.Show()
	t.Cleanup(func() { ui.searchModel.close(); window.Hide(); window.Destroy() })
	for cycle, count := range []int{1, rowLimit, 1, 0, 1} {
		if cycle > 0 {
			ui.scroll.ScrollTo(ui.scroll.XPosition()+31, ui.scroll.YPosition()+173)
		}
		ui.rows = make([]homeRow, count)
		for index := range ui.rows {
			ui.rows[index].Title = "作品"
		}
		ui.populateHome(nil, nil)
		if ui.browseButton.X() != 278 || ui.browseButton.Y() != 205 {
			t.Fatal("Home refresh did not restore the static content origin")
		}
		// 近期更新 is the first section; 繼續觀看 follows its empty-state line.
		var updatesHeading, updatesEmpty *fltk.Box
		for _, label := range ui.contentLabels[ui.staticLabels:] {
			switch label.widget.Label() {
			case "近期更新":
				updatesHeading = label.widget
			case recentUpdateEmptyText(nil):
				updatesEmpty = label.widget
			}
		}
		if updatesHeading == nil || updatesEmpty == nil || updatesHeading.Y() != homeFirstSectionY {
			t.Fatalf("count%d: 近期更新 is not the first section", count)
		}
		continueTop := ui.continueHeading.Y()
		if continueTop-(updatesEmpty.Y()+updatesEmpty.H()) != homeSectionGap {
			t.Fatalf("count%d: 繼續觀看 overlaps 近期更新 or leaves excessive gap", count)
		}
		headingBottom := continueTop + ui.continueHeading.H()
		if count == 0 {
			if !ui.loadingText.Visible() || ui.loadingText.Label() != "還沒有可繼續觀看的內容。" || ui.loadingText.Y()-headingBottom != 20 {
				t.Fatal("empty-state feedback missing or detached from 繼續觀看")
			}
			continue
		}
		if ui.cards[0].Y()-headingBottom != 12 || ui.loadingText.Visible() {
			t.Fatalf("count%d: populated section reserves loading space", count)
		}
		if ui.cards[0].X() != 250 || ui.cardButtons[0].X()-ui.cards[0].X() != 563 || ui.cardButtons[0].Y()-ui.cards[0].Y() != 28 {
			t.Fatal("Home refresh misaligned the card and Continue action")
		}
	}
}
