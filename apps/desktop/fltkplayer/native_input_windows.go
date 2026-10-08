//go:build windows && cgo

package fltkplayer

import (
	"sync"
	"syscall"

	"github.com/ebitengine/purego"
)

const (
	wmNCDestroy          = 0x0082
	wmAppCommand         = 0x0319
	appCommandMute       = 8
	appCommandVolumeDown = 9
	appCommandVolumeUp   = 10
	appCommandPlayPause  = 14
	nativeSubclassID     = 0x41504C59
)

var nativeInput = struct {
	sync.Mutex
	views map[uintptr]*View
}{views: make(map[uintptr]*View)}

var nativeSubclassCallback = purego.NewCallback(nativeSubclassProc)
var nativeComctl32 = syscall.NewLazyDLL("comctl32.dll")
var nativeSetWindowSubclass = nativeComctl32.NewProc("SetWindowSubclass")
var nativeRemoveWindowSubclass = nativeComctl32.NewProc("RemoveWindowSubclass")
var nativeDefSubclassProc = nativeComctl32.NewProc("DefSubclassProc")

func (view *View) syncNativeInput() {
	if view == nil || view.closed || view.window == nil {
		return
	}
	if !view.window.IsShown() {
		view.removeNativeInput()
		return
	}
	wanted := make(map[uintptr]struct{}, 2)
	if handle := view.window.RawHandle(); handle != 0 {
		wanted[handle] = struct{}{}
	}
	if view.video != nil {
		if handle := view.video.RawHandle(); handle != 0 {
			wanted[handle] = struct{}{}
		}
	}

	nativeInput.Lock()
	defer nativeInput.Unlock()
	for handle := range view.nativeHandles {
		if _, keep := wanted[handle]; keep {
			continue
		}
		removeWindowSubclass(handle)
		delete(nativeInput.views, handle)
		delete(view.nativeHandles, handle)
	}
	for handle := range wanted {
		if _, attached := view.nativeHandles[handle]; attached {
			continue
		}
		if setWindowSubclass(handle) {
			if view.nativeHandles == nil {
				view.nativeHandles = make(map[uintptr]struct{}, 2)
			}
			view.nativeHandles[handle] = struct{}{}
			nativeInput.views[handle] = view
		}
	}
}

func (view *View) removeNativeInput() {
	if view == nil {
		return
	}
	nativeInput.Lock()
	defer nativeInput.Unlock()
	for handle := range view.nativeHandles {
		removeWindowSubclass(handle)
		delete(nativeInput.views, handle)
		delete(view.nativeHandles, handle)
	}
}

func nativeSubclassProc(hwnd, message, wParam, lParam, subclassID, reference uintptr) (result uintptr) {
	defaultProc := func() uintptr {
		result, _, _ := nativeDefSubclassProc.Call(hwnd, message, wParam, lParam)
		return result
	}
	defer func() {
		if recover() != nil {
			result = defaultProc()
		}
	}()
	nativeInput.Lock()
	view := nativeInput.views[hwnd]
	nativeInput.Unlock()
	if message == wmNCDestroy {
		nativeInput.Lock()
		if view != nil {
			delete(view.nativeHandles, hwnd)
		}
		delete(nativeInput.views, hwnd)
		nativeInput.Unlock()
		return defaultProc()
	}
	if message == wmAppCommand && view != nil && view.handleNativeAppCommand(int((uint32(lParam)>>16)&0x0fff)) {
		return 1
	}
	return defaultProc()
}

func (view *View) handleNativeAppCommand(command int) bool {
	if view.closed {
		return false
	}
	switch command {
	case appCommandPlayPause:
		if view.callbacks.PlayPause == nil {
			return false
		}
		view.activate(focusPlayPause)
	case appCommandMute:
		if view.callbacks.Volume == nil {
			return false
		}
		if view.state.Volume == 0 {
			view.setVolume(view.lastVolume)
		} else {
			view.lastVolume = view.state.Volume
			view.setVolume(0)
		}
	case appCommandVolumeDown:
		if view.callbacks.Volume == nil {
			return false
		}
		view.setVolume(view.state.Volume - 5)
	case appCommandVolumeUp:
		if view.callbacks.Volume == nil {
			return false
		}
		view.setVolume(view.state.Volume + 5)
	default:
		return false
	}
	return true
}

func setWindowSubclass(hwnd uintptr) bool {
	result, _, _ := nativeSetWindowSubclass.Call(hwnd, nativeSubclassCallback, nativeSubclassID, 0)
	return result != 0
}

func removeWindowSubclass(hwnd uintptr) {
	nativeRemoveWindowSubclass.Call(hwnd, nativeSubclassCallback, nativeSubclassID)
}
