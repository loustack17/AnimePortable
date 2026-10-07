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
	t.Cleanup(func() { ui.searchModel.close(); window.Hide(); window.Destroy() })
	for _, count := range []int{1, rowLimit, 0, 1} {
		ui.rows = make([]homeRow, count)
		for index := range ui.rows {
			ui.rows[index].Title = "作品"
		}
		ui.populateHome(nil, nil)
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
		last := ui.cards[count-1]
		for _, label := range ui.contentLabels[ui.staticLabels:] {
			if label.widget.Label() == "追蹤中" && label.widget.Y()-(last.Y()+last.H()) != 20 {
				t.Fatal("following heading overlaps rows or leaves excessive gap")
			}
		}
	}
}
