//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import fltk "github.com/pwiecz/go-fltk"

// releaseTestWindow hides a Home window at the end of a test instead of destroying it.
//
// go-fltk's deletion handler unregisters a child's draw handler id from its plain callback map,
// not from the draw handler map, so destroying a window whose children have draw handlers drops
// whichever live callback or timeout happens to share that id; the event loop of a later test
// then calls it and crashes at address 0. The app never destroys the Home window, and the
// dialogs destroy their children one by one first, so only tests are exposed to it.
func releaseTestWindow(window *fltk.Window) {
	window.Hide()
}
