//go:build windows && cgo

package fltkplayer

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	fltk "github.com/pwiecz/go-fltk"
)

func TestPlayerFollowStarJoinsTabRingOnlyWhenStateKnown(t *testing.T) {
	toggles, plays := 0, 0
	view := keyboardTestView(Callbacks{ToggleFollow: func() { toggles++ }, PlayPause: func() { plays++ }})
	view.state.Focus = focusMenu
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusEpisodes {
		t.Fatalf("Tab from menu with unknown follow state = %d, want episodes", view.state.Focus)
	}
	view.activate(focusFollow)
	if toggles != 0 {
		t.Fatal("hidden star toggled following")
	}

	view.state.FollowKnown = true
	view.state.Focus = focusMenu
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusFollow {
		t.Fatalf("Tab from menu = %d, want follow star", view.state.Focus)
	}
	keyDown(view, fltk.ENTER_KEY, 0)
	keyUp(view, fltk.ENTER_KEY)
	keyDown(view, ' ', 0)
	keyUp(view, ' ')
	if toggles != 2 || plays != 0 {
		t.Fatalf("Enter and Space on the star: %d toggles, %d play/pause; want 2 and 0", toggles, plays)
	}
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusEpisodes {
		t.Fatalf("Tab from star = %d, want episodes", view.state.Focus)
	}
	keyDown(view, 9, fltk.SHIFT)
	keyUp(view, 9)
	if view.state.Focus != focusFollow {
		t.Fatalf("Shift+Tab from episodes = %d, want follow star", view.state.Focus)
	}

	// A state update keeps the star focused; one that hides it moves focus off it.
	view.SetState(State{Volume: 100, Focus: focusFollow, FollowKnown: true})
	if view.state.Focus != focusFollow {
		t.Fatalf("focus after state update = %d", view.state.Focus)
	}
	view.SetState(State{Volume: 100, Focus: focusFollow})
	if view.state.Focus != focusProgress {
		t.Fatalf("focus on hidden star = %d, want progress", view.state.Focus)
	}
}

func TestPlayerFollowStarClickAndDraw(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	toggles := 0
	view := NewWindow(Callbacks{ToggleFollow: func() { toggles++ }})
	t.Cleanup(func() { view.Close(); view.window.Hide(); view.window.Destroy() })
	view.window.Show()
	fltk.Wait(0.01)
	width := view.video.W()
	starX, starY := width-222, 33

	if got := view.actionAt(starX, starY); got != -1 {
		t.Fatalf("hidden star hit = %d, want none", got)
	}
	view.SetState(State{Volume: 100, Focus: focusProgress, FollowKnown: true})
	if got := view.actionAt(starX, starY); got != focusFollow {
		t.Fatalf("star hit = %d, want follow", got)
	}
	if got := view.actionAt(width-190, starY); got != focusEpisodes {
		t.Fatalf("episode selector hit = %d, want episodes", got)
	}
	view.activate(view.actionAt(starX, starY))
	if toggles != 1 {
		t.Fatalf("click toggled %d times, want 1", toggles)
	}

	gl := syscall.NewLazyDLL("opengl32.dll")
	call := func(name string, args ...uintptr) { t.Helper(); _, _, _ = gl.NewProc(name).Call(args...) }
	// count reads the star's box and counts accent-blue and white pixels.
	count := func(state State) (accent, white int) {
		t.Helper()
		view.SetState(state)
		view.video.MakeCurrent()
		call("glDisable", 0x0C11)
		call("glClearColor", 0, 0, 0, 0)
		call("glClear", 0x00004000)
		drawPlayerOverlay(view.video.W(), view.video.H(), view)
		var doubleBuffered byte
		call("glGetBooleanv", 0x0C32, uintptr(unsafe.Pointer(&doubleBuffered)))
		buffer := uintptr(0x0404)
		if doubleBuffered != 0 {
			buffer = 0x0405
		}
		call("glReadBuffer", buffer)
		size := 34
		pixels := make([]byte, size*size*4)
		call("glReadPixels", uintptr(width-239), uintptr(view.video.H()-50), uintptr(size), uintptr(size), 0x1908, 0x1401, uintptr(unsafe.Pointer(&pixels[0])))
		for index := 0; index < len(pixels); index += 4 {
			red, green, blue := pixels[index], pixels[index+1], pixels[index+2]
			switch {
			case red >= 200 && green >= 200 && blue >= 200:
				white++
			case blue >= 220 && red < 190:
				accent++
			}
		}
		return accent, white
	}
	if accent, white := count(State{Volume: 100, Focus: focusProgress}); accent != 0 || white != 0 {
		t.Fatalf("unknown state drew a star: accent %d, white %d", accent, white)
	}
	if accent, white := count(State{Volume: 100, Focus: focusProgress, FollowKnown: true}); accent != 0 || white < 10 {
		t.Fatalf("not followed: accent %d, white %d; want a white outline", accent, white)
	}
	if accent, _ := count(State{Volume: 100, Focus: focusProgress, FollowKnown: true, Following: true}); accent < 30 {
		t.Fatalf("followed: accent %d; want a filled accent star", accent)
	}
	view.window.Hide()
}
