//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"fmt"
	"time"

	fltk "github.com/pwiecz/go-fltk"
)

const scheduleSection = 1

// scheduleRefreshTimeout bounds one refresh: the schedule lookup and the catalog lookup the
// backend bounds at 20 seconds each, with room to store the result.
const scheduleRefreshTimeout = 45 * time.Second

// scheduleRefreshInterval is the least time between automatic fetches, so returning to
// 時間表 does not refetch every time; 重新整理 forces one.
const scheduleRefreshInterval = 10 * time.Minute

// Week grid layout: a day header above each column, then the day's show cards. Columns never
// get narrower than scheduleMinColumnWidth, so a card keeps two readable lines; a narrow
// window scrolls the grid sideways instead. The right margin leaves room for the scrollbar.
const (
	scheduleHeaderHeight   = 46
	scheduleCardHeight     = 66
	scheduleGap            = 8
	scheduleMarginLeft     = 15
	scheduleMarginRight    = 20
	scheduleMinColumnWidth = 90
)

func (ui *view) buildScheduleView() {
	ui.schedulePage, ui.scheduleStatus, ui.scheduleRefresh, ui.scheduleScroll = ui.buildRowListPage("重新整理", ui.refreshSchedule)
	ui.scheduleScroll.SetType(fltk.SCROLL_BOTH)
}

func (ui *view) scheduleVisible() bool {
	return !ui.closing && ui.ctx.Err() == nil && ui.selected == scheduleSection
}

// showSchedule shows the stored schedule at once and then fetches it in the background.
func (ui *view) showSchedule() {
	ui.schedulePage.Show()
	ui.renderSchedule()
	ui.loadSchedule()
	if ui.scheduleRefreshAttemptedAt.IsZero() || time.Since(ui.scheduleRefreshAttemptedAt) >= scheduleRefreshInterval {
		ui.refreshSchedule()
	}
}

// loadSchedule reads the stored schedule; it never waits on the network.
func (ui *view) loadSchedule() {
	if ui.service == nil || ui.closing || ui.ctx.Err() != nil {
		return
	}
	ui.scheduleGeneration++
	generation := ui.scheduleGeneration
	go func() {
		now := time.Now()
		items, err := ui.service.CachedSchedule(ui.ctx)
		fltk.Awake(func() {
			if generation != ui.scheduleGeneration || !ui.scheduleVisible() {
				return
			}
			ui.scheduleLoaded = true
			ui.scheduleLoadFailed = err != nil
			if !ui.scheduleLoadFailed {
				ui.scheduleColumns = scheduleWeek(items, now)
			}
			ui.renderSchedule()
		})
	}()
}

// refreshSchedule fetches the schedule, which replaces the stored one, then reloads the
// page from storage.
func (ui *view) refreshSchedule() {
	if ui.service == nil || ui.scheduleRefreshing || ui.closing || ui.ctx.Err() != nil {
		return
	}
	ui.scheduleRefreshing = true
	ui.scheduleRefreshFailed = false
	ui.scheduleRefreshAttemptedAt = time.Now()
	ui.renderScheduleStatus()
	go func() {
		ctx, cancel := context.WithTimeout(ui.ctx, scheduleRefreshTimeout)
		_, err := ui.service.Schedule(ctx)
		cancel()
		fltk.Awake(func() {
			ui.scheduleRefreshing = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			ui.scheduleRefreshFailed = err != nil
			if ui.scheduleVisible() {
				// The status must not say 正在更新 while the reload is still running.
				ui.renderScheduleStatus()
				ui.loadSchedule()
			}
		})
	}()
}

func (ui *view) scheduleShowCount() int {
	count := 0
	for _, day := range ui.scheduleColumns {
		count += len(day.Shows)
	}
	return count
}

func (ui *view) renderScheduleStatus() {
	shows := ui.scheduleShowCount()
	status := ""
	switch {
	case !ui.scheduleLoaded:
		status = "正在載入時間表…"
	case ui.scheduleLoadFailed:
		status = "無法讀取已儲存的時間表，請按「重新整理」重試。"
	case shows > 0:
		status = fmt.Sprintf("接下來 %d 天共 %d 部播出。", scheduleDays, shows)
	case ui.scheduleRefreshing:
		status = "正在取得時間表…"
	case ui.scheduleRefreshFailed:
		status = "無法取得時間表，請按「重新整理」重試。"
	default:
		status = fmt.Sprintf("接下來 %d 天沒有播出資料。", scheduleDays)
	}
	if shows > 0 || ui.scheduleLoadFailed {
		if ui.scheduleRefreshing {
			status += "\n正在更新時間表，先顯示上次記錄。"
		} else if ui.scheduleRefreshFailed {
			status += "\n無法更新時間表，顯示上次記錄。"
		}
	}
	ui.scheduleStatus.SetLabel(status)
	ui.scheduleStatus.Redraw()
}

// renderSchedule rebuilds the week grid, keeping the scroll position and the focused card.
func (ui *view) renderSchedule() {
	if ui.schedulePage == nil {
		return
	}
	focusedDay, focusedShow := -1, 0
	for day, cards := range ui.scheduleCards {
		for index, card := range cards {
			if card.HasFocus() {
				focusedDay, focusedShow = day, index
			}
			ui.scheduleScroll.Remove(card)
			card.Destroy()
		}
	}
	for _, header := range ui.scheduleHeaders {
		ui.scheduleScroll.Remove(header)
		header.Destroy()
	}
	ui.scheduleCards, ui.scheduleHeaders = nil, nil
	scrollX, scrollY := ui.scheduleScroll.XPosition(), ui.scheduleScroll.YPosition()
	ui.scheduleScroll.ScrollTo(0, 0)
	ui.scheduleScroll.Begin()
	tallest := 0
	if ui.scheduleLoaded && !ui.scheduleLoadFailed {
		for day, column := range ui.scheduleColumns {
			header := fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, column.Weekday+"\n"+column.Date)
			header.SetLabelFont(fltk.HELVETICA_BOLD)
			header.SetLabelSize(13)
			header.SetAlign(fltk.ALIGN_CENTER | fltk.ALIGN_INSIDE | fltk.ALIGN_CLIP)
			ui.scheduleHeaders = append(ui.scheduleHeaders, header)
			cards := make([]*fltk.Button, 0, len(column.Shows))
			for index, show := range column.Shows {
				card := fltk.NewButton(0, 0, 1, 1, show.Anime.Title)
				if show.Anime.ID == "" {
					card.SetTooltip("搜尋片源：" + show.Query)
				} else {
					card.SetTooltip("開啟作品：" + show.Anime.Title)
				}
				ui.styleScheduleCard(card, show)
				ui.bindScheduleCard(card, day, index)
				cards = append(cards, card)
			}
			ui.scheduleCards = append(ui.scheduleCards, cards)
			tallest = max(tallest, len(cards))
		}
	}
	ui.scheduleScroll.End()
	ui.placeSchedule()
	contentWidth := scheduleMarginLeft + len(ui.scheduleHeaders)*(ui.scheduleColumnWidth()+scheduleGap) + scheduleMarginRight
	contentHeight := scheduleHeaderHeight + scheduleGap + tallest*(scheduleCardHeight+scheduleGap)
	ui.scheduleScroll.ScrollTo(max(0, min(scrollX, contentWidth-ui.scheduleScroll.W())), max(0, min(scrollY, contentHeight-ui.scheduleScroll.H())))
	ui.renderScheduleStatus()
	ui.applyTheme()
	if focusedDay >= 0 {
		ui.focusScheduleNear(focusedDay, focusedShow)
	}
}

// scheduleColumnWidth shares the list width between the days, down to the minimum.
func (ui *view) scheduleColumnWidth() int {
	columns := max(1, len(ui.scheduleHeaders))
	fit := (ui.scheduleScroll.W() - scheduleMarginLeft - scheduleMarginRight - (columns-1)*scheduleGap) / columns
	return max(scheduleMinColumnWidth, fit)
}

// placeSchedule lays out the grid; the scroll must be at the top left.
func (ui *view) placeSchedule() {
	scroll := ui.scheduleScroll
	width := ui.scheduleColumnWidth()
	for day, header := range ui.scheduleHeaders {
		x := scroll.X() + scheduleMarginLeft + day*(width+scheduleGap)
		header.Resize(x, scroll.Y()+4, width, scheduleHeaderHeight)
		for index, card := range ui.scheduleCards[day] {
			card.Resize(x, scroll.Y()+scheduleHeaderHeight+scheduleGap+index*(scheduleCardHeight+scheduleGap), width, scheduleCardHeight)
		}
	}
}

func (ui *view) resizeSchedule() {
	if ui.schedulePage == nil {
		return
	}
	ui.resizeListPage(ui.schedulePage, ui.scheduleStatus, ui.scheduleRefresh, ui.scheduleScroll)
	ui.placeSchedule()
}

// styleScheduleCard draws a show as a card: the title on up to two lines, then what Enter
// does, in the accent color when the show opens directly.
func (ui *view) styleScheduleCard(button *fltk.Button, show scheduleShow) {
	action := "開啟作品"
	if show.Anime.ID == "" {
		action = "搜尋片源"
	}
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		if button.HasFocus() {
			roundedRect(x, y, width, height, 11, colors.accent)
		} else {
			roundedRect(x, y, width, height, 11, colors.surface)
		}
		roundedRect(x+3, y+3, width-6, height-6, 9, colors.surface)
		fltk.PushClip(x+3, y+3, width-6, height-6)
		fltk.SetDrawColor(colors.ink)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 13)
		fltk.Draw(show.Anime.Title, x+9, y+6, width-18, 36, fltk.ALIGN_LEFT|fltk.ALIGN_TOP|fltk.ALIGN_INSIDE|fltk.ALIGN_WRAP|fltk.ALIGN_CLIP)
		if show.Anime.ID == "" {
			fltk.SetDrawColor(colors.muted)
		} else {
			fltk.SetDrawColor(colors.accent)
		}
		fltk.SetDrawFont(fltk.HELVETICA, 11)
		fltk.Draw(action, x+9, y+height-22, width-18, 16, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
		fltk.PopClip()
	})
}

// bindScheduleCard makes a card open on click or Enter, and moves focus with the arrow keys:
// Left and Right to the neighbouring day, Up and Down within the day.
func (ui *view) bindScheduleCard(card *fltk.Button, day, index int) {
	open := func() { ui.openScheduleShow(day, index) }
	card.SetCallback(open)
	activate := ui.deferredKeyAction(open)
	card.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, 13, 0xff8d:
			activate()
		case 0xff51:
			ui.focusAfterKey(func() { ui.focusScheduleDay(day, index, -1) })
		case 0xff53:
			ui.focusAfterKey(func() { ui.focusScheduleDay(day, index, 1) })
		case 0xff52:
			if index == 0 {
				ui.focusAfterKey(func() { ui.scheduleRefresh.TakeFocus() })
			} else {
				ui.focusAfterKey(func() { ui.focusScheduleCard(day, index-1) })
			}
		case 0xff54:
			if index+1 < len(ui.scheduleCards[day]) {
				ui.focusAfterKey(func() { ui.focusScheduleCard(day, index+1) })
			}
		default:
			return false
		}
		return true
	})
}

// focusScheduleDay moves to the nearest day in direction step that has shows, keeping the
// row; past the first day it returns to the navigation.
func (ui *view) focusScheduleDay(day, index, step int) {
	for next := day + step; next >= 0 && next < len(ui.scheduleCards); next += step {
		if len(ui.scheduleCards[next]) > 0 {
			ui.focusScheduleCard(next, min(index, len(ui.scheduleCards[next])-1))
			return
		}
	}
	if step < 0 {
		ui.navigation[ui.selected].TakeFocus()
	}
}

// focusScheduleCard focuses a card and scrolls the grid so it is fully visible.
func (ui *view) focusScheduleCard(day, index int) {
	if day < 0 || day >= len(ui.scheduleCards) || index < 0 || index >= len(ui.scheduleCards[day]) {
		return
	}
	scroll := ui.scheduleScroll
	x, y := scroll.XPosition(), scroll.YPosition()
	width := ui.scheduleColumnWidth()
	left := scheduleMarginLeft + day*(width+scheduleGap)
	if left < x {
		x = max(0, left-scheduleMarginLeft)
	} else if right := left + width + scheduleMarginRight; right > x+scroll.W() {
		x = right - scroll.W()
	}
	top := scheduleHeaderHeight + scheduleGap + index*(scheduleCardHeight+scheduleGap)
	if top < y {
		y = max(0, top-scheduleHeaderHeight-scheduleGap)
	} else if bottom := top + scheduleCardHeight + scheduleGap; bottom > y+scroll.H() {
		y = bottom - scroll.H()
	}
	scroll.ScrollTo(x, y)
	ui.scheduleCards[day][index].TakeFocus()
	scroll.Redraw()
}

// focusScheduleNear focuses the card at day and index, or the closest one that exists.
func (ui *view) focusScheduleNear(day, index int) {
	day = min(max(day, 0), len(ui.scheduleCards)-1)
	if day >= 0 && len(ui.scheduleCards[day]) > 0 {
		ui.focusScheduleCard(day, min(index, len(ui.scheduleCards[day])-1))
		return
	}
	for _, step := range []int{-1, 1} {
		for next := day + step; next >= 0 && next < len(ui.scheduleCards); next += step {
			if len(ui.scheduleCards[next]) > 0 {
				ui.focusScheduleCard(next, min(index, len(ui.scheduleCards[next])-1))
				return
			}
		}
	}
	ui.scheduleRefresh.TakeFocus()
}

// focusSchedule focuses the card last opened, so Back from the detail screen lands where the
// user left, or the first show.
func (ui *view) focusSchedule() {
	ui.focusScheduleNear(ui.scheduleOpened[0], ui.scheduleOpened[1])
}

// openScheduleShow opens a show the app knows, or searches the app's source for one it does
// not know yet.
func (ui *view) openScheduleShow(day, index int) {
	if !ui.scheduleVisible() || day >= len(ui.scheduleColumns) || index >= len(ui.scheduleColumns[day].Shows) {
		return
	}
	ui.scheduleOpened = [2]int{day, index}
	show := ui.scheduleColumns[day].Shows[index]
	if show.Anime.ID != "" {
		ui.openDetailFrom(detailFromSchedule, show.Anime)
		return
	}
	ui.showSection(4)
	ui.searchInput.SetValue(show.Query)
	ui.searchModel.setQuery(show.Query)
	ui.renderSearch()
	ui.submitSearch()
	ui.searchInput.TakeFocus()
}
