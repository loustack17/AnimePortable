//go:build windows && cgo

package fltkplayer

import (
	"runtime"
	"syscall"
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

var nativeTestUser32 = syscall.NewLazyDLL("user32.dll")

func sendNativeMessage(hwnd, message, wParam, lParam uintptr) uintptr {
	result, _, _ := nativeTestUser32.NewProc("SendMessageW").Call(hwnd, message, wParam, lParam)
	return result
}

func nativeAppCommandLParam(command, device int) uintptr {
	return uintptr(uint32(command|device<<12) << 16)
}

func shownNativeInputView(t *testing.T, callbacks Callbacks) *View {
	t.Helper()
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() {
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	view := NewWindow(callbacks)
	view.Show()
	fltk.Wait(0.05)
	view.syncNativeInput()
	t.Cleanup(func() {
		view.Close()
		view.window.Hide()
		view.window.Destroy()
	})
	return view
}

func TestShownPlayerWindowHandlesMediaAppCommandsOnce(t *testing.T) {
	plays := 0
	var volumes []int
	view := shownNativeInputView(t, Callbacks{
		PlayPause: func() { plays++ },
		Volume:    func(percent int) { volumes = append(volumes, percent) },
	})
	windowHandle := view.window.RawHandle()
	videoHandle := view.video.RawHandle()
	if windowHandle == 0 || videoHandle == 0 {
		t.Fatalf("shown player HWNDs = %x/%x, want nonzero player and video handles", windowHandle, videoHandle)
	}
	wanted := map[uintptr]struct{}{windowHandle: {}, videoHandle: {}}
	if len(wanted) < 1 || len(view.nativeHandles) != len(wanted) {
		t.Fatalf("attached HWND count = %d, want each distinct shown handle (%d)", len(view.nativeHandles), len(wanted))
	}
	for handle := range wanted {
		if _, attached := view.nativeHandles[handle]; !attached || nativeInput.views[handle] != view {
			t.Fatalf("shown handle %#x is not attached to its player", handle)
		}
	}
	view.episodes = []string{"one", "two"}
	view.episodesOpen = true
	view.episodeCursor = 1
	view.menuPinned = true
	view.state.Focus = focusPlayPause
	view.visible = false
	for _, test := range []struct {
		name    string
		hwnd    uintptr
		command int
		device  int
	}{
		{"play-pause", windowHandle, appCommandPlayPause, 0xA},
		{"volume-down", videoHandle, appCommandVolumeDown, 0x4},
		{"volume-up", videoHandle, appCommandVolumeUp, 0x2},
		{"mute", windowHandle, appCommandMute, 0xA},
		{"unmute", videoHandle, appCommandMute, 0x4},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sendNativeMessage(test.hwnd, wmAppCommand, test.hwnd, nativeAppCommandLParam(test.command, test.device)); got != 1 {
				t.Fatalf("WM_APPCOMMAND result = %d, want handled TRUE", got)
			}
		})
	}
	if plays != 1 || view.state.Volume != 100 || len(volumes) != 4 || volumes[0] != 95 || volumes[1] != 100 || volumes[2] != 0 || volumes[3] != 100 {
		t.Fatalf("media callback results plays=%d volume=%d callbacks=%v", plays, view.state.Volume, volumes)
	}
	if !view.visible {
		t.Fatal("Play/Pause app command did not reveal hidden controls")
	}
	if !view.episodesOpen || view.episodeCursor != 1 || !view.menuPinned {
		t.Fatalf("media commands changed selection/menu state: episodesOpen=%v cursor=%d menuPinned=%v", view.episodesOpen, view.episodeCursor, view.menuPinned)
	}
	view.window.Hide()
	if len(view.nativeHandles) != 0 {
		t.Fatalf("hide retained native subclasses: %v", view.nativeHandles)
	}
	view.Show()
	fltk.Wait(0.05)
	view.syncNativeInput()
	for handle := range view.nativeHandles {
		if nativeInput.views[handle] != view {
			t.Fatalf("shown handle %#x is missing its player association", handle)
		}
	}
}

func TestShownPlayerWindowRoutesNativeSpaceAndEnterToFocusedPlayPause(t *testing.T) {
	plays := 0
	view := shownNativeInputView(t, Callbacks{PlayPause: func() { plays++ }})
	view.SetFocus(focusPlayPause)
	hwnd := view.window.RawHandle()
	for _, key := range []uintptr{0x20, 0x0D} {
		baseline := plays
		sendNativeMessage(hwnd, 0x0100, key, 0)
		sendNativeMessage(hwnd, 0x0100, key, 0)
		if plays != baseline+1 {
			t.Fatalf("held native key %#x invoked Play/Pause %d times, want %d", key, plays-baseline, 1)
		}
		sendNativeMessage(hwnd, 0x0101, key, 0)
		sendNativeMessage(hwnd, 0x0100, key, 0)
		if plays != baseline+2 {
			t.Fatalf("released native key %#x invoked Play/Pause %d times, want 2", key, plays-baseline)
		}
		sendNativeMessage(hwnd, 0x0101, key, 0)
	}
}

func TestPlayerNativeSubclassReconcilesFullscreenAndClose(t *testing.T) {
	view := shownNativeInputView(t, Callbacks{})
	initial := make(map[uintptr]struct{}, len(view.nativeHandles))
	for handle := range view.nativeHandles {
		initial[handle] = struct{}{}
	}
	view.window.SetFullscreen(true)
	fltk.Wait(0.05)
	view.syncNativeInput()
	for handle := range view.nativeHandles {
		if nativeInput.views[handle] != view {
			t.Fatalf("fullscreen handle %#x is missing its player association", handle)
		}
	}
	view.window.SetFullscreen(false)
	fltk.Wait(0.05)
	view.syncNativeInput()
	for handle := range initial {
		if _, current := view.nativeHandles[handle]; !current && nativeInput.views[handle] != nil {
			t.Fatalf("stale HWND %#x retained its player association", handle)
		}
	}
	view.Close()
	if len(view.nativeHandles) != 0 {
		t.Fatalf("Close retained native handles: %v", view.nativeHandles)
	}
	for handle := range initial {
		if nativeInput.views[handle] != nil {
			t.Fatalf("Close retained subclass association for %#x", handle)
		}
	}
}
