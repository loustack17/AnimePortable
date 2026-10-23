//go:build windows && cgo

package fltkplayer

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

// inOpening is an episode playing 30 s into an opening from 20 s to 110 s.
func inOpening() State {
	return State{Volume: 100, Focus: focusProgress, Playing: true, Position: 30, Duration: 1420, OpeningStart: 20, OpeningEnd: 110}
}

func TestSkipIntroShowsOnlyInsideTheOpening(t *testing.T) {
	for _, test := range []struct {
		name  string
		edit  func(*State)
		shown bool
	}{
		{"inside", func(*State) {}, true},
		{"at the start", func(state *State) { state.Position = 20 }, true},
		{"before", func(state *State) { state.Position = 19.5 }, false},
		{"last second", func(state *State) { state.Position = 109.2 }, false},
		{"after", func(state *State) { state.Position = 110 }, false},
		{"no opening", func(state *State) { state.OpeningStart, state.OpeningEnd = 0, 0 }, false},
		{"loading", func(state *State) { state.Loading = true }, false},
		{"stopped", func(state *State) { state.Playing = false }, false},
		{"paused", func(state *State) { state.Paused = true }, true},
	} {
		state := inOpening()
		test.edit(&state)
		if got := state.skipShown(); got != test.shown {
			t.Errorf("%s: shown %t, want %t", test.name, got, test.shown)
		}
	}
}

func TestSkipIntroKeyJumpsToTheEndOfTheOpening(t *testing.T) {
	var seeks []int
	view := keyboardTestView(Callbacks{SeekTo: func(seconds int) { seeks = append(seeks, seconds) }})
	view.SetState(State{Volume: 100, Focus: focusProgress, Playing: true, Position: 5, Duration: 1420, OpeningStart: 20, OpeningEnd: 109.4})
	if keyDown(view, 's', 0) || len(seeks) != 0 {
		t.Fatalf("S before the opening was used: %v", seeks)
	}
	keyUp(view, 's')
	state := view.State()
	state.Position = 30
	view.SetState(state)
	if !keyDown(view, 'S', 0) || len(seeks) != 1 || seeks[0] != 110 {
		t.Fatalf("S in the opening sought %v, want [110]", seeks)
	}
	keyUp(view, 'S')
	if view.state.skipShown() {
		t.Fatal("the button still shows after skipping")
	}
	// Seeking back into the opening shows it again.
	state = view.State()
	state.Position = 25
	view.SetState(state)
	if !keyDown(view, 's', 0) || len(seeks) != 2 {
		t.Fatalf("S after seeking back sought %v", seeks)
	}
}

func TestSkipIntroJoinsTheTabOrderOnlyWhileShown(t *testing.T) {
	var seeks []int
	view := keyboardTestView(Callbacks{SeekTo: func(seconds int) { seeks = append(seeks, seconds) }})
	view.SetState(inOpening())
	press := func(key, modifiers int) {
		t.Helper()
		keyDown(view, key, modifiers)
		keyUp(view, key)
	}
	press(9, 0)
	if view.state.Focus != focusSkip {
		t.Fatalf("Tab from progress = %d, want skip", view.state.Focus)
	}
	press(9, fltk.SHIFT)
	press(9, 0)
	press(fltk.ENTER_KEY, 0)
	if len(seeks) != 1 || seeks[0] != 110 || view.state.Focus != focusProgress {
		t.Fatalf("Enter on skip: seeks %v, focus %d", seeks, view.state.Focus)
	}

	// Focus on the button moves to the progress bar when the opening ends by itself.
	view.SetState(inOpening())
	view.SetFocus(focusSkip)
	state := view.State()
	state.Position = 110
	view.SetState(state)
	if view.state.Focus != focusProgress {
		t.Fatalf("focus after the opening = %d, want progress", view.state.Focus)
	}
	press(9, 0)
	if view.state.Focus != focusPlayPause {
		t.Fatalf("Tab with no button = %d, want play/pause", view.state.Focus)
	}
	press(9, fltk.SHIFT)
	if view.state.Focus != focusProgress {
		t.Fatalf("Shift+Tab with no button = %d, want progress", view.state.Focus)
	}
}

func TestSkipIntroClickAndDrawWhileControlsAreHidden(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread(); uitext.Use("zh-TW") })
	var seeks []int
	view := NewWindow(Callbacks{SeekTo: func(seconds int) { seeks = append(seeks, seconds) }})
	t.Cleanup(func() { view.Close(); view.window.Hide(); view.window.Destroy() })
	view.window.Show()
	fltk.Wait(0.01)
	width, height := view.video.W(), view.video.H()
	buttonX, buttonY := width-skipButtonRightInset-view.skipWidth/2, height-skipButtonBottomInset+skipButtonHeight/2

	if got := view.actionAt(buttonX, buttonY); got != -1 {
		t.Fatalf("hidden button hit = %d", got)
	}
	view.SetState(inOpening())
	view.hideControls()
	if got := view.actionAt(buttonX, buttonY); got != focusSkip {
		t.Fatalf("button hit = %d, want skip", got)
	}
	view.activate(focusSkip)
	if len(seeks) != 1 || seeks[0] != 110 {
		t.Fatalf("click sought %v", seeks)
	}

	gl := syscall.NewLazyDLL("opengl32.dll")
	call := func(name string, args ...uintptr) { t.Helper(); _, _, _ = gl.NewProc(name).Call(args...) }
	// white counts white pixels in the button's box: its outline and label.
	white := func(state State) int {
		t.Helper()
		view.SetState(state)
		view.hideControls()
		view.video.MakeCurrent()
		call("glDisable", 0x0C11)
		call("glClearColor", 0, 0, 0, 0)
		call("glClear", 0x00004000)
		drawPlayerOverlay(width, height, view)
		var doubleBuffered byte
		call("glGetBooleanv", 0x0C32, uintptr(unsafe.Pointer(&doubleBuffered)))
		buffer := uintptr(0x0404)
		if doubleBuffered != 0 {
			buffer = 0x0405
		}
		call("glReadBuffer", buffer)
		boxWidth := view.skipWidth
		pixels := make([]byte, boxWidth*skipButtonHeight*4)
		call("glReadPixels", uintptr(width-skipButtonRightInset-boxWidth), uintptr(skipButtonBottomInset-skipButtonHeight), uintptr(boxWidth), uintptr(skipButtonHeight), 0x1908, 0x1401, uintptr(unsafe.Pointer(&pixels[0])))
		count := 0
		for index := 0; index < len(pixels); index += 4 {
			if pixels[index] >= 200 && pixels[index+1] >= 200 && pixels[index+2] >= 200 {
				count++
			}
		}
		return count
	}
	outside := inOpening()
	outside.Position = 200
	if got := white(outside); got != 0 {
		t.Fatalf("outside the opening drew %d white pixels", got)
	}
	for _, code := range uitext.LanguageCodes {
		uitext.Use(code)
		view.updateLabelBuffers()
		label := uitext.T().PlayerSkipIntro
		if view.skipWidth-2*skipButtonPadding < skipLabelWidth(label) || view.skipWidth > width/3 {
			t.Errorf("%s: button %d px for %q", code, view.skipWidth, label)
		}
		if got := white(inOpening()); got < 2*view.skipWidth+100 {
			t.Errorf("%s: hidden controls drew %d white pixels, want outline and label", code, got)
		}
	}
	view.window.Hide()
}

func skipLabelWidth(label string) int {
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, skipLabelSize)
	width, _ := fltk.MeasureText(label, false)
	return width
}
