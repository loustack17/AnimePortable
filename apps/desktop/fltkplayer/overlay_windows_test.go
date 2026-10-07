//go:build windows && cgo

package fltkplayer

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	fltk "github.com/pwiecz/go-fltk"
)

func TestOverlayRendersAcrossReentryWithInheritedViewportAndClip(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	view := NewWindow(Callbacks{})
	t.Cleanup(func() { view.Close(); view.window.Hide(); view.window.Destroy() })
	view.SetEpisodes([]string{"第一集", "第二集"}, 0)
	gl := syscall.NewLazyDLL("opengl32.dll")
	call := func(name string, args ...uintptr) { t.Helper(); _, _, _ = gl.NewProc(name).Call(args...) }
	for pass := 0; pass < 2; pass++ {
		view.window.Show()
		fltk.Wait(0.01)
		view.video.MakeCurrent()
		view.SetState(State{Loading: true, Focus: focusPlayPause, Volume: 100})
		view.SetFocus(focusPlayPause)
		call("glDisable", 0x0C11)
		call("glClear", 0x00004000)
		call("glViewport", 0, 0, 1, 1)
		call("glScissor", 0, 0, 1, 1)
		call("glEnable", 0x0C11)
		drawPlayerOverlay(view.video.W(), view.video.H(), view)
		var viewport [4]int32
		call("glGetIntegerv", 0x0BA2, uintptr(unsafe.Pointer(&viewport[0])))
		if viewport != [4]int32{0, 0, 1, 1} {
			t.Fatalf("pass%d: inherited viewport not restored: %v", pass, viewport)
		}
		enabled, _, _ := gl.NewProc("glIsEnabled").Call(0x0C11)
		if enabled == 0 {
			t.Fatalf("pass%d: inherited clipping not restored", pass)
		}
		var doubleBuffered byte
		call("glGetBooleanv", 0x0C32, uintptr(unsafe.Pointer(&doubleBuffered)))
		buffer := uintptr(0x0404)
		if doubleBuffered != 0 {
			buffer = 0x0405
		}
		call("glReadBuffer", buffer)
		var pixel [4]byte
		call("glReadPixels", 200, uintptr(view.video.H()-20), 1, 1, 0x1908, 0x1401, uintptr(unsafe.Pointer(&pixel[0])))
		if pixel[2] < 10 {
			t.Fatalf("pass%d: control bar did not render outside inherited 1px clip: %v", pass, pixel)
		}
		call("glDisable", 0x0C11)
		view.window.Hide()
	}
}
