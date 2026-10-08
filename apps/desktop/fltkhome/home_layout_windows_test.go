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
		if count == 0 {
			if !ui.loadingText.Visible() || ui.loadingText.Label() != "還沒有可繼續觀看的內容。" {
				t.Fatal("empty-state feedback missing")
			}
			continue
		}
		var headingBottom int
		for _, label := range ui.contentLabels[:ui.staticLabels] {
			if label.widget.Label() == "繼續觀看" {
				headingBottom = label.widget.Y() + label.widget.H()
			}
		}
		if headingBottom == 0 || ui.cards[0].Y()-headingBottom != 12 || ui.loadingText.Visible() {
			t.Fatalf("count%d: populated section reserves loading space", count)
		}
		if ui.cards[0].X() != 250 || ui.cardButtons[0].X()-ui.cards[0].X() != 563 || ui.cardButtons[0].Y()-ui.cards[0].Y() != 28 {
			t.Fatal("Home refresh misaligned the card and Continue action")
		}
		last := ui.cards[count-1]
		for _, label := range ui.contentLabels[ui.staticLabels:] {
			if label.widget.Label() == "追蹤中" && label.widget.Y()-(last.Y()+last.H()) != 20 {
				t.Fatal("following heading overlaps rows or leaves excessive gap")
			}
		}
	}
}
