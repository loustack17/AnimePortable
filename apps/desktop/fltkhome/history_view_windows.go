//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

const historySection = 3

func (ui *view) buildHistoryView() {
	ui.historyPage, ui.historyStatus, ui.historyRetry, ui.historyScroll = ui.buildRowListPage("重新載入", ui.loadHistory)
	// 重新載入 shows only after a failed load.
	ui.historyRetry.Hide()
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

func (ui *view) historyList() rowList {
	return rowList{
		page: ui.historyPage, status: ui.historyStatus, action: ui.historyRetry, scroll: ui.historyScroll,
		buttons: &ui.historyButtons, rowIDs: &ui.historyRowIDs,
	}
}

func (ui *view) renderHistory() {
	rows := make([]listRow, 0, len(ui.historyItems))
	for _, row := range ui.historyItems {
		item := row.History
		rows = append(rows, listRow{
			id: item.AnimeID, title: row.Title, status: row.Status, tooltip: "繼續播放：" + row.Status,
			open:         func() { ui.continueHistory(item) },
			iconLabel:    "移除紀錄",
			styleIcon:    ui.styleRemoveIcon,
			activateIcon: func() { ui.removeHistory(item.AnimeID) },
		})
	}
	ui.renderRowList(ui.historyList(), rows, ui.renderHistoryStatus, ui.focusHistory)
}

func (ui *view) resizeHistory() {
	ui.resizeRowList(ui.historyList())
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
