//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import fltk "github.com/pwiecz/go-fltk"

// SearchSection is the index of 搜尋 in sections; the player opens it for `/` and Ctrl+K.
const SearchSection = 4

// keyCommand is what a key means anywhere in the main window. Screens decide what a command
// does on them; only classifyKey looks at key codes.
type keyCommand uint8

const (
	keyNone keyCommand = iota
	keyActivate
	keyBack
	keyUp
	keyDown
	keyLeft
	keyRight
	keyNext
	keyPrevious
	keySearch
)

// classifyKey maps a key and the modifier state to a command. Ctrl, Alt or Win with any key
// but Ctrl+K means nothing, so system and input shortcuts keep working.
func classifyKey(key, state int) keyCommand {
	modifiers := state & (fltk.CTRL | fltk.ALT | fltk.META)
	if modifiers == fltk.CTRL && (key == 'k' || key == 'K') {
		return keySearch
	}
	if modifiers != 0 {
		return keyNone
	}
	if isEnterKey(key) {
		return keyActivate
	}
	switch key {
	case ' ':
		return keyActivate
	case fltk.ESCAPE, 27:
		return keyBack
	case fltk.UP:
		return keyUp
	case fltk.DOWN:
		return keyDown
	case fltk.LEFT:
		return keyLeft
	case fltk.RIGHT:
		return keyRight
	case fltk.TAB, 9:
		if state&fltk.SHIFT != 0 {
			return keyPrevious
		}
		return keyNext
	// The key that types /: on layouts where it needs Shift it does not match, and Ctrl+K
	// still works.
	case '/':
		return keySearch
	}
	return keyNone
}

// isEnterKey reports Enter: 13 and 0xff8d are the main and keypad Enter as some keyboards
// report them.
func isEnterKey(key int) bool {
	return key == fltk.ENTER_KEY || key == 13 || key == 0xff8d
}

// keyRoutes says where focus moves from a widget for each arrow key. A nil route leaves the
// key to FLTK; an empty func keeps focus where it is.
type keyRoutes struct {
	up, down, left, right func()
}

func (routes keyRoutes) route(command keyCommand) func() {
	switch command {
	case keyUp:
		return routes.up
	case keyDown:
		return routes.down
	case keyLeft:
		return routes.left
	case keyRight:
		return routes.right
	}
	return nil
}

func stayFocused() {}

// bindKeys makes button run action on a click, Enter or Space, and move focus along routes.
func (ui *view) bindKeys(button *fltk.Button, action func(), routes keyRoutes) {
	button.SetCallback(action)
	activate := ui.deferredKeyAction(action)
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		command := classifyKey(fltk.EventKey(), fltk.EventState())
		if command == keyActivate {
			activate()
			return true
		}
		if route := routes.route(command); route != nil {
			ui.focusAfterKey(route)
			return true
		}
		return false
	})
}

func (ui *view) bindButton(button *fltk.Button, action func()) {
	ui.bindKeys(button, action, keyRoutes{})
}

// deferredKeyAction runs action just after the key event, so the action may destroy the
// focused widget; repeated keys before it runs are dropped.
func (ui *view) deferredKeyAction(action func()) func() {
	queued := false
	return func() {
		if queued {
			return
		}
		queued = true
		fltk.AddTimeout(0.01, func() {
			queued = false
			if ui.ctx.Err() == nil && !ui.closing {
				action()
			}
		})
	}
}

func (ui *view) focusAfterKey(action func()) {
	fltk.AddTimeout(0.01, func() {
		if ui.ctx.Err() == nil && !ui.closing {
			action()
		}
	})
}

// handleWindowShortcut handles the keys no focused widget took: Esc goes back and `/` and
// Ctrl+K open 搜尋 on every screen.
func (ui *view) handleWindowShortcut(event fltk.Event) bool {
	if event != fltk.SHORTCUT {
		return false
	}
	key := fltk.EventKey()
	return ui.windowKey(classifyKey(key, fltk.EventState()), key)
}

func (ui *view) windowKey(command keyCommand, key int) bool {
	if ui.mainGroup == nil || !ui.mainGroup.Visible() {
		return false
	}
	switch command {
	case keyBack:
		ui.backKey()
		return true
	case keySearch:
		ui.quickSearchKey()
		return true
	case keyActivate:
		// Space with no focused button plays the selected episode on the detail screen.
		if ui.selected == SearchSection && ui.searchPreview && key == ' ' {
			ui.deferredSearchAction(ui.playSelectedDetailEpisode, true)()
			return true
		}
	}
	return false
}

// bindWindowKeys installs the window-wide keys. Esc and quick search run after the key event
// and are not tied to the page generation: they act on whatever page shows when they run, so a
// page reloading in between (a queued detail reopening) must not drop the key. Presses that
// arrive before the first one runs are dropped, so Esc goes back one page only. Nothing runs
// while the player hides the window.
func (ui *view) bindWindowKeys() {
	whenShown := func(action func()) func() {
		return ui.deferredKeyAction(func() {
			if ui.window.IsShown() && ui.mainGroup.Visible() {
				action()
			}
		})
	}
	ui.backKey = whenShown(ui.goBack)
	ui.quickSearchKey = whenShown(ui.openQuickSearch)
	ui.window.SetEventHandler(ui.handleWindowShortcut)
}

// isEscapeClose reports whether the window callback was triggered by Esc, which FLTK turns
// into a close request when no widget takes it. Esc must never close the app.
func isEscapeClose() bool {
	event := fltk.EventType()
	return (event == fltk.KEYDOWN || event == fltk.SHORTCUT) && classifyKey(fltk.EventKey(), 0) == keyBack
}

// goBack is Esc on a page: the detail screen returns to where it was opened from, 搜尋 and
// the other pages return to Home, and Home stays.
func (ui *view) goBack() {
	switch ui.selected {
	case 0:
		return
	case SearchSection:
		ui.searchEscape()
	default:
		ui.goHome()
	}
}

func (ui *view) goHome() {
	ui.showSection(0)
	ui.navigation[0].TakeFocus()
}

// openQuickSearch shows 搜尋's results page; showSection closes the detail screen if it is open.
func (ui *view) openQuickSearch() {
	ui.showSection(SearchSection)
	ui.focusQuickSearchInput()
}

// focusQuickSearchInput focuses the search input with its text selected, so typing replaces
// the last query.
func (ui *view) focusQuickSearchInput() {
	ui.focusSearchInput()
	ui.searchInput.SetInsertPosition(len(ui.searchInput.Value()), 0)
}

// handleSearchInputKey takes the keys the input would otherwise use: Down moves to the
// results and Ctrl+K selects the query instead of deleting to the end of the line.
func (ui *view) handleSearchInputKey(event fltk.Event) bool {
	if event != fltk.KEYDOWN {
		return false
	}
	state := fltk.EventState()
	return ui.searchInputKey(classifyKey(fltk.EventKey(), state), state&fltk.CTRL != 0)
}

func (ui *view) searchInputKey(command keyCommand, ctrl bool) bool {
	switch command {
	case keyDown:
		if len(ui.searchRows) == 0 {
			return false
		}
		ui.deferredSearchAction(func() { ui.focusSearchResult(0) }, false)()
		return true
	case keySearch:
		if !ctrl {
			return false // `/` is typed
		}
		ui.searchInput.SetInsertPosition(len(ui.searchInput.Value()), 0)
		return true
	}
	return false
}

// scrollIntoView scrolls scroll vertically so button, one of its children, is fully visible.
func scrollIntoView(scroll *fltk.Scroll, button *fltk.Button) {
	top, bottom := scroll.Y(), scroll.Y()+scroll.H()
	if button.Y() < top {
		scroll.ScrollTo(scroll.XPosition(), max(0, scroll.YPosition()+button.Y()-top))
	} else if buttonBottom := button.Y() + button.H(); buttonBottom > bottom {
		scroll.ScrollTo(scroll.XPosition(), scroll.YPosition()+buttonBottom-bottom)
	}
}

func (ui *view) focusNavigation() {
	ui.navigation[ui.selected].TakeFocus()
}
