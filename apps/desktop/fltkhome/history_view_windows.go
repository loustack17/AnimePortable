//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
	"slices"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

const historySection = 3

// historyRemoveSize is the width of the remove icon at the end of each 歷史紀錄 row.
const historyRemoveSize = 40

func (ui *view) buildHistoryView() {
	ui.historyPage = fltk.NewGroup(236, 110, 741, 487)
	ui.historyPage.Begin()
	ui.historyStatus = fltk.NewBox(fltk.NO_BOX, 252, 118, 580, 40, "")
	ui.historyStatus.SetLabelSize(12)
	ui.historyStatus.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.historyRetry = fltk.NewButton(845, 118, 112, 40, "重新載入")
	ui.styleButton(ui.historyRetry, nil)
	ui.bindButton(ui.historyRetry, ui.loadHistory)
	ui.historyRetry.Hide()
	ui.historyScroll = fltk.NewScroll(236, 166, 741, 431)
	ui.historyScroll.SetType(fltk.SCROLL_VERTICAL)
	ui.historyScroll.SetBox(fltk.FLAT_BOX)
	ui.historyScroll.End()
	ui.historyPage.End()
	ui.historyPage.Hide()
}

func (ui *view) historyVisible() bool {
	return !ui.closing && ui.ctx.Err() == nil && ui.selected == historySection
}

// showHistory shows the last loaded list at once and reloads it, because playback and
// removals change history while the page is hidden. Rows cannot be played until the
// reload lands, so a stale saved position is never used.
func (ui *view) showHistory() {
	ui.historyPage.Show()
	ui.renderHistory()
	ui.loadHistory()
}

func (ui *view) loadHistory() {
	if ui.service == nil || ui.closing || ui.ctx.Err() != nil {
		return
	}
	ui.historyGeneration++
	generation := ui.historyGeneration
	ui.historyLoading = true
	go func() {
		library, libraryErr := ui.service.Library(ui.ctx)
		history, historyErr := ui.service.History(ui.ctx)
		fltk.Awake(func() {
			if generation != ui.historyGeneration || !ui.historyVisible() {
				return
			}
			ui.historyLoading = false
			ui.historyLoaded = true
			ui.historyLoadFailed = firstError(libraryErr, historyErr) != nil
			// A failed load drops the old rows: their positions may be stale or removed.
			ui.historyItems = nil
			if !ui.historyLoadFailed {
				ui.historyItems = historyRows(history, library, time.Now())
			}
			ui.renderHistory()
		})
	}()
}

func (ui *view) continueHistory(item backend.History) {
	if ui.onPlay != nil && ui.historyVisible() && !ui.historyLoading && !ui.historyRemovePending {
		ui.onPlay(historyPlayRequest(item))
	}
}

func (ui *view) removeHistory(animeID string) {
	if ui.service == nil || ui.historyRemovePending || !ui.historyVisible() {
		return
	}
	ui.historyRemovePending = true
	go func() {
		err := ui.service.RemoveHistory(ui.ctx, animeID)
		fltk.Awake(func() {
			ui.historyRemovePending = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			if err != nil {
				ui.message.SetLabel("無法移除紀錄，請重試。")
			} else {
				ui.message.SetLabel("")
			}
			if ui.historyVisible() {
				ui.loadHistory()
			}
		})
	}()
}

func (ui *view) focusHistory() {
	if len(ui.historyButtons) > 0 {
		ui.historyButtons[0].TakeFocus()
		return
	}
	if ui.historyRetry.Visible() {
		ui.historyRetry.TakeFocus()
		return
	}
	ui.navigation[historySection].TakeFocus()
}

func (ui *view) renderHistoryStatus() {
	status := ""
	switch {
	case !ui.historyLoaded:
		status = "正在載入觀看紀錄…"
	case ui.historyLoadFailed:
		status = "無法載入觀看紀錄，請按「重新載入」重試。"
	case len(ui.historyItems) == 0:
		status = "還沒有觀看紀錄。播放任何一集後，會出現在這裡。"
	default:
		status = fmt.Sprintf("%d 部作品。按作品從上次位置繼續播放。\n觀看紀錄只存在這台電腦。", len(ui.historyItems))
	}
	if ui.historyLoadFailed {
		ui.historyRetry.Show()
	} else {
		ui.historyRetry.Hide()
	}
	ui.historyStatus.SetLabel(status)
	ui.historyStatus.Redraw()
}

func (ui *view) renderHistory() {
	if ui.historyPage == nil {
		return
	}
	focusedID, focusedRow, focusedColumn := "", -1, 0
	for index, button := range ui.historyButtons {
		if button.HasFocus() && index/2 < len(ui.historyRowIDs) {
			focusedID, focusedRow, focusedColumn = ui.historyRowIDs[index/2], index/2, index%2
		}
		ui.historyScroll.Remove(button)
		button.Destroy()
	}
	ui.historyButtons = nil
	ui.historyRowIDs = nil
	// Keep the scroll position: a reload after removing a row must not jump to the top.
	scrollY := ui.historyScroll.YPosition()
	ui.historyScroll.ScrollTo(0, 0)
	ui.historyScroll.Begin()
	for index, row := range ui.historyItems {
		open := fltk.NewButton(0, 0, 1, 1, row.Title)
		open.SetTooltip("繼續播放：" + row.Status)
		// The row looks like a 追蹤 row: title above, status below.
		ui.styleFollowingRow(open, followingRow{Anime: backend.Anime{ID: row.History.AnimeID, Title: row.Title}, Status: row.Status})
		item := row.History
		ui.bindButton(open, func() { ui.continueHistory(item) })
		remove := fltk.NewButton(0, 0, 1, 1, "移除紀錄")
		remove.SetTooltip("移除紀錄")
		ui.styleRemoveIcon(remove)
		ui.bindButton(remove, func() { ui.removeHistory(item.AnimeID) })
		ui.historyButtons = append(ui.historyButtons, open, remove)
		ui.historyRowIDs = append(ui.historyRowIDs, item.AnimeID)
		ui.placeHistoryRow(index)
	}
	ui.historyScroll.End()
	if len(ui.historyRowIDs) > 0 {
		lastRowBottom := 9 + len(ui.historyRowIDs)*64
		ui.historyScroll.ScrollTo(0, max(0, min(scrollY, lastRowBottom-ui.historyScroll.H())))
	}
	ui.renderHistoryStatus()
	ui.applyTheme()
	if focusedRow < 0 {
		return
	}
	// The focused row may be gone after 移除紀錄; keep focus on the row now in its place.
	target := slices.Index(ui.historyRowIDs, focusedID)
	if target < 0 {
		target = min(focusedRow, len(ui.historyRowIDs)-1)
	}
	if target < 0 {
		ui.focusHistory()
		return
	}
	ui.historyButtons[target*2+focusedColumn].TakeFocus()
}

func (ui *view) placeHistoryRow(index int) {
	scroll := ui.historyScroll
	openWidth := max(220, scroll.W()-35-historyRemoveSize-8)
	y := scroll.Y() + 9 + index*64
	ui.historyButtons[index*2].Resize(scroll.X()+15, y, openWidth, 56)
	ui.historyButtons[index*2+1].Resize(scroll.X()+15+openWidth+8, y+8, historyRemoveSize, 40)
}

func (ui *view) resizeHistory() {
	if ui.historyPage == nil {
		return
	}
	width := max(400, ui.window.W()-259)
	height := max(250, ui.window.H()-131)
	ui.historyPage.Resize(236, 110, width, height)
	ui.historyRetry.Resize(236+width-20-112, 118, 112, 40)
	ui.historyStatus.Resize(252, 118, max(150, width-32-112-12), 40)
	ui.historyScroll.Resize(236, 166, width, max(120, height-56))
	ui.historyScroll.ScrollTo(0, 0)
	for index := range ui.historyRowIDs {
		ui.placeHistoryRow(index)
	}
}

// styleRemoveIcon draws a remove button as an X. The label is not drawn; it names the
// action for the tooltip.
func (ui *view) styleRemoveIcon(button *fltk.Button) {
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		if button.HasFocus() {
			roundedRect(x, y, width, height, 13, colors.accent)
		} else {
			roundedRect(x, y, width, height, 13, colors.surface)
		}
		roundedRect(x+3, y+3, width-6, height-6, 11, colors.surface)
		centerX, centerY := x+width/2, y+height/2
		fltk.SetDrawColor(colors.muted)
		fltk.SetLineStyle(fltk.SOLID, 2)
		fltk.DrawLine(centerX-6, centerY-6, centerX+6, centerY+6)
		fltk.DrawLine(centerX-6, centerY+6, centerX+6, centerY-6)
		fltk.SetLineStyle(fltk.SOLID, 0)
	})
}
