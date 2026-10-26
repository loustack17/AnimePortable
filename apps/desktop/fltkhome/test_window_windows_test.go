//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"syscall"
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

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

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	setWindowLongPtr         = user32.NewProc("SetWindowLongPtrW")
	callWindowProc           = user32.NewProc("CallWindowProcW")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	keepFocusWindowProc      = syscall.NewCallback(keepFocusFromOtherThreadsProc)
	// windowProcs holds each subclassed window's FLTK window procedure. Only the test's locked
	// FLTK thread reads and writes it.
	windowProcs = map[uintptr]uintptr{}
)

// keepFocusFromOtherThreads keeps the focused widget of a test window when a window of another
// thread or process takes the foreground.
//
// FLTK clears its focused widget when its window loses keyboard focus (WM_KILLFOCUS). The full
// native suite runs the fltkengine and fltkplayer test windows in parallel processes, and they or
// any other app can take the foreground in the middle of a test: the focused row then loses focus
// and the key a test sends next reaches no widget, so the test times out or sees focus elsewhere.
// Focus moving to another window of this thread, such as a dialog, still reaches FLTK. Every
// test that sends keys or checks focus uses newHomeRefreshFixture, which installs this.
func keepFocusFromOtherThreads(t *testing.T, window *fltk.Window) {
	t.Helper()
	handle := window.RawHandle()
	if handle == 0 {
		t.Fatal("window has no native handle")
	}
	const gwlpWndProc = ^uintptr(3) // -4
	previous, _, err := setWindowLongPtr.Call(handle, gwlpWndProc, keepFocusWindowProc)
	if previous == 0 {
		t.Fatalf("replacing the window procedure failed: %v", err)
	}
	windowProcs[handle] = previous
	t.Cleanup(func() {
		setWindowLongPtr.Call(handle, gwlpWndProc, windowProcs[handle])
		delete(windowProcs, handle)
	})
}

func keepFocusFromOtherThreadsProc(handle, message, wParam, lParam uintptr) uintptr {
	const wmKillFocus = 0x0008
	if message == wmKillFocus && (wParam == 0 || windowThread(wParam) != windowThread(handle)) {
		return 0
	}
	result, _, _ := callWindowProc.Call(windowProcs[handle], handle, message, wParam, lParam)
	return result
}

func windowThread(handle uintptr) uintptr {
	thread, _, _ := getWindowThreadProcessID.Call(handle, 0)
	return thread
}
