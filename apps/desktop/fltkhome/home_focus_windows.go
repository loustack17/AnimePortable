//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"slices"

	fltk "github.com/pwiecz/go-fltk"
)

// homeFocus is the Home row that had focus when a reload hid the rows: a 近期更新 row, or a
// 繼續播放 card when continuing is set.
type homeFocus struct {
	pending    bool
	continuing bool
	animeID    string
	index      int
}

// rememberHomeFocus records the focused Home row before the rows are hidden; hiding them
// drops FLTK's focus.
func (ui *view) rememberHomeFocus() {
	for index, button := range ui.homeUpdateButtons {
		if button.HasFocus() && index < len(ui.homeUpdateIDs) {
			ui.homeFocus = homeFocus{pending: true, animeID: ui.homeUpdateIDs[index], index: index}
		}
	}
	for index, button := range ui.cardButtons {
		if button.HasFocus() && index < len(ui.rows) {
			ui.homeFocus = homeFocus{pending: true, continuing: true, animeID: ui.rows[index].History.AnimeID, index: index}
		}
	}
}

// restoreHomeFocus focuses the remembered row again after a reload: the same anime, else
// the row now at its place, else the other list, else 重新載入 or 搜尋作品. It does nothing
// when the user has moved focus elsewhere in the meantime.
func (ui *view) restoreHomeFocus() {
	focus := ui.homeFocus
	ui.homeFocus = homeFocus{}
	if !focus.pending || ui.selected != 0 || ui.focusOutsideHomeRows() {
		return
	}
	ids, buttons := ui.homeUpdateIDs, ui.homeUpdateButtons
	if focus.continuing {
		ids, buttons = nil, ui.cardButtons
		for _, row := range ui.rows {
			ids = append(ids, row.History.AnimeID)
		}
	}
	var target *fltk.Button
	if index := slices.Index(ids, focus.animeID); index >= 0 && index < len(buttons) {
		target = buttons[index]
	} else if len(buttons) > 0 {
		target = buttons[min(focus.index, len(buttons)-1)]
	} else if list := ui.homeFocusables(); len(list) > 0 {
		target = list[0]
	}
	ui.focusTarget(target)
}

// focusOutsideHomeRows reports whether the user moved focus away from Home's content while it
// reloaded. 搜尋作品 does not count: FLTK gives it focus when the focused row is hidden.
func (ui *view) focusOutsideHomeRows() bool {
	for _, button := range ui.navigation {
		if button.HasFocus() {
			return true
		}
	}
	return ui.themeToggle.HasFocus() || ui.retryButton.HasFocus()
}
