//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"slices"

	fltk "github.com/pwiecz/go-fltk"
)

// rowIconSize is the width of the icon button at the end of each list row.
const rowIconSize = 40

// rowList is the scrolling list shared by 追蹤 and 歷史紀錄: each row is a title/status
// button followed by an icon button. buttons and rowIDs point at the page's own view
// fields, which hold the buttons in row order (title, icon, title, icon, …) and the
// anime ID of each row.
type rowList struct {
	page    *fltk.Group
	status  *fltk.Box
	action  *fltk.Button // the page's top-right button, whose right edge the icons line up with
	scroll  *fltk.Scroll
	buttons *[]*fltk.Button
	rowIDs  *[]string
}

// buildRowListPage builds a hidden page with a status line, a top-right action button and
// an empty row list below them.
func (ui *view) buildRowListPage(actionLabel string, onAction func()) (*fltk.Group, *fltk.Box, *fltk.Button, *fltk.Scroll) {
	page := fltk.NewGroup(236, 110, 741, 487)
	page.Begin()
	status := fltk.NewBox(fltk.NO_BOX, 252, 118, 580, 40, "")
	status.SetLabelSize(12)
	status.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	action := fltk.NewButton(845, 118, 112, 40, actionLabel)
	ui.styleButton(action, nil)
	ui.bindButton(action, onAction)
	scroll := fltk.NewScroll(236, 166, 741, 431)
	scroll.SetType(fltk.SCROLL_VERTICAL)
	scroll.SetBox(fltk.FLAT_BOX)
	scroll.End()
	page.End()
	page.Hide()
	return page, status, action, scroll
}

type listRow struct {
	id, title, status, tooltip string
	open                       func()
	iconLabel                  string
	styleIcon                  func(*fltk.Button)
	activateIcon               func()
}

// renderRowList rebuilds the rows, keeping the scroll position and the focused row. When the
// focused row is gone its neighbour takes focus; when no rows are left, focusEmpty does.
// renderStatus runs before focus is restored, because focusEmpty may depend on it.
func (ui *view) renderRowList(list rowList, rows []listRow, renderStatus, focusEmpty func()) {
	if list.page == nil {
		return
	}
	focusedID, focusedRow, focusedColumn := "", -1, 0
	for index, button := range *list.buttons {
		if button.HasFocus() && index/2 < len(*list.rowIDs) {
			focusedID, focusedRow, focusedColumn = (*list.rowIDs)[index/2], index/2, index%2
		}
		list.scroll.Remove(button)
		button.Destroy()
	}
	*list.buttons = nil
	*list.rowIDs = nil
	// Keep the scroll position: a reload must not jump the list to the top.
	scrollY := list.scroll.YPosition()
	list.scroll.ScrollTo(0, 0)
	list.scroll.Begin()
	for index, row := range rows {
		open := fltk.NewButton(0, 0, 1, 1, row.title)
		open.SetTooltip(row.tooltip)
		ui.styleTitleStatusRow(open, row.title, row.status)
		ui.bindButton(open, row.open)
		icon := fltk.NewButton(0, 0, 1, 1, row.iconLabel)
		icon.SetTooltip(row.iconLabel)
		row.styleIcon(icon)
		ui.bindButton(icon, row.activateIcon)
		*list.buttons = append(*list.buttons, open, icon)
		*list.rowIDs = append(*list.rowIDs, row.id)
		list.placeRow(index)
	}
	list.scroll.End()
	if len(*list.rowIDs) > 0 {
		lastRowBottom := 9 + len(*list.rowIDs)*64
		list.scroll.ScrollTo(0, max(0, min(scrollY, lastRowBottom-list.scroll.H())))
	}
	renderStatus()
	ui.applyTheme()
	if focusedRow < 0 {
		return
	}
	target := slices.Index(*list.rowIDs, focusedID)
	if target < 0 {
		target = min(focusedRow, len(*list.rowIDs)-1)
	}
	if target < 0 {
		focusEmpty()
		return
	}
	(*list.buttons)[target*2+focusedColumn].TakeFocus()
}

func (list rowList) placeRow(index int) {
	scroll := list.scroll
	openWidth := max(220, scroll.W()-35-rowIconSize-8)
	y := scroll.Y() + 9 + index*64
	(*list.buttons)[index*2].Resize(scroll.X()+15, y, openWidth, 56)
	(*list.buttons)[index*2+1].Resize(scroll.X()+15+openWidth+8, y+8, rowIconSize, 40)
}

func (ui *view) resizeRowList(list rowList) {
	if list.page == nil {
		return
	}
	width := max(400, ui.window.W()-259)
	height := max(250, ui.window.H()-131)
	list.page.Resize(236, 110, width, height)
	list.action.Resize(236+width-20-112, 118, 112, 40)
	list.status.Resize(252, 118, max(150, width-32-112-12), 40)
	list.scroll.Resize(236, 166, width, max(120, height-56))
	list.scroll.ScrollTo(0, 0)
	for index := range *list.rowIDs {
		list.placeRow(index)
	}
}

// styleTitleStatusRow draws the title above a status line.
func (ui *view) styleTitleStatusRow(button *fltk.Button, title, status string) {
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
		fltk.PushClip(x+3, y+3, width-6, height-6)
		fltk.SetDrawColor(colors.ink)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
		fltk.Draw(title, x+18, y+6, width-36, 24, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
		fltk.SetDrawColor(colors.muted)
		fltk.SetDrawFont(fltk.HELVETICA, 12)
		fltk.Draw(status, x+18, y+30, width-36, 20, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
		fltk.PopClip()
	})
}
