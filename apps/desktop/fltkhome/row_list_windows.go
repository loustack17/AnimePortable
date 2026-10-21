//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"slices"

	fltk "github.com/pwiecz/go-fltk"
)

// rowIconSize is the width of each icon button at the end of a list row.
const rowIconSize = 40

// rowIconGap is the space before each icon button.
const rowIconGap = 8

// rowList is the scrolling list shared by 追蹤 and 歷史紀錄: each row is a title/status
// button followed by the same number (icons) of icon buttons. buttons and rowIDs point at
// the page's own view fields, which hold the buttons in row order (title, its icons, next
// title, …) and the anime ID of each row.
type rowList struct {
	page    *fltk.Group
	status  *fltk.Box
	action  *fltk.Button // the page's top-right button, whose right edge the icons line up with
	scroll  *fltk.Scroll
	icons   int
	buttons *[]*fltk.Button
	rowIDs  *[]string
}

// buildRowListPage builds a hidden page with a status line, a top-right action button and
// an empty row list below them. Down on the action button runs focusContent.
func (ui *view) buildRowListPage(actionLabel string, onAction, focusContent func()) (*fltk.Group, *fltk.Box, *fltk.Button, *fltk.Scroll) {
	page := fltk.NewGroup(236, 110, 741, 487)
	page.Begin()
	status := fltk.NewBox(fltk.NO_BOX, 252, 118, 580, 40, "")
	status.SetLabelSize(12)
	status.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	action := fltk.NewButton(845, 118, 112, 40, actionLabel)
	ui.styleButton(action, nil)
	ui.bindKeys(action, onAction, keyRoutes{up: ui.focusNavigation, left: ui.focusNavigation, down: focusContent})
	scroll := fltk.NewScroll(236, 166, 741, 431)
	scroll.SetType(fltk.SCROLL_VERTICAL)
	scroll.SetBox(fltk.FLAT_BOX)
	scroll.End()
	page.End()
	page.Hide()
	return page, status, action, scroll
}

// listRow is one row; it has as many icons as its list's icons count.
type listRow struct {
	id, title, status, tooltip string
	open                       func()
	icons                      []rowIcon
}

// rowIcon is an icon button; its label is not drawn but names the action for the tooltip.
type rowIcon struct {
	label    string
	style    func(*fltk.Button)
	activate func()
}

// renderRowList rebuilds the rows, keeping the scroll position and the focused row. When the
// focused row is gone its neighbour takes focus; when no rows are left, focusEmpty does.
// renderStatus runs before focus is restored, because focusEmpty may depend on it.
func (ui *view) renderRowList(list rowList, rows []listRow, renderStatus, focusEmpty func()) {
	if list.page == nil {
		return
	}
	focusedID, focusedRow, focusedColumn := "", -1, 0
	columns := list.columns()
	for index, button := range *list.buttons {
		if button.HasFocus() && index/columns < len(*list.rowIDs) {
			focusedID, focusedRow, focusedColumn = (*list.rowIDs)[index/columns], index/columns, index%columns
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
		ui.bindKeys(open, row.open, list.routes(ui, index, 0))
		*list.buttons = append(*list.buttons, open)
		for column, item := range row.icons {
			icon := fltk.NewButton(0, 0, 1, 1, item.label)
			icon.SetTooltip(item.label)
			item.style(icon)
			ui.bindKeys(icon, item.activate, list.routes(ui, index, column+1))
			*list.buttons = append(*list.buttons, icon)
		}
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
	list.focus(target, focusedColumn)
}

// routes moves focus from the row's title (column 0) or one of its icons (columns 1 and up):
// Up and Down to the same column of the neighbouring row, Left and Right along the row. Up
// from the first row reaches the page button when it shows, and Left from a title the
// navigation.
func (list rowList) routes(ui *view, index, column int) keyRoutes {
	routes := keyRoutes{
		up: func() {
			if index > 0 {
				list.focus(index-1, column)
			} else if list.action.Visible() && list.action.IsActive() {
				list.action.TakeFocus()
			} else {
				ui.focusNavigation()
			}
		},
		down: func() { list.focus(index+1, column) },
	}
	if column == 0 {
		routes.left = ui.focusNavigation
	} else {
		routes.left = func() { list.focus(index, column-1) }
	}
	if column < list.icons {
		routes.right = func() { list.focus(index, column+1) }
	} else {
		routes.right = stayFocused
	}
	return routes
}

func (list rowList) columns() int { return 1 + list.icons }

// button is a row's title (column 0) or icon.
func (list rowList) button(index, column int) *fltk.Button {
	return (*list.buttons)[index*list.columns()+column]
}

// focus focuses a row's title or icon and scrolls it into view; a row that does not exist
// leaves focus where it is.
func (list rowList) focus(index, column int) {
	if index < 0 || index*list.columns()+column >= len(*list.buttons) {
		return
	}
	button := list.button(index, column)
	scrollIntoView(list.scroll, button)
	button.TakeFocus()
}

func (list rowList) placeRow(index int) {
	scroll := list.scroll
	openWidth := max(220, scroll.W()-35-list.icons*(rowIconGap+rowIconSize))
	y := scroll.Y() + 9 + index*64
	list.button(index, 0).Resize(scroll.X()+15, y, openWidth, 56)
	for column := 1; column <= list.icons; column++ {
		x := scroll.X() + 15 + openWidth + column*rowIconGap + (column-1)*rowIconSize
		list.button(index, column).Resize(x, y+8, rowIconSize, 40)
	}
}

func (ui *view) resizeRowList(list rowList) {
	if list.page == nil {
		return
	}
	ui.resizeListPage(list.page, list.status, list.action, list.scroll)
	for index := range *list.rowIDs {
		list.placeRow(index)
	}
}

// resizeListPage fits a page built by buildRowListPage to the window and scrolls it to the
// top; the caller then lays out its content.
func (ui *view) resizeListPage(page *fltk.Group, status *fltk.Box, action *fltk.Button, scroll *fltk.Scroll) {
	width := max(400, ui.window.W()-259)
	height := max(250, ui.window.H()-131)
	page.Resize(236, 110, width, height)
	action.Resize(236+width-20-112, 118, 112, 40)
	status.Resize(252, 118, max(150, width-32-112-12), 40)
	scroll.Resize(236, 166, width, max(120, height-56))
	scroll.ScrollTo(0, 0)
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
