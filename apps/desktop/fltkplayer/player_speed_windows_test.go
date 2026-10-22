//go:build windows && cgo

package fltkplayer

import (
	"reflect"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	fltk "github.com/pwiecz/go-fltk"
)

func TestSpeedLabelsWriteMultipliers(t *testing.T) {
	want := []string{"0.75×", "1×", "1.25×", "1.5×", "2×"}
	if got := speedLabels(); !reflect.DeepEqual(got, want) {
		t.Fatalf("speed labels = %q, want %q", got, want)
	}
}

func TestSpeedKeyboardOpensListMovesAndChooses(t *testing.T) {
	var chosen []int
	leaves, seeks := 0, 0
	view := keyboardTestView(Callbacks{
		SetSpeed: func(percent int) { chosen = append(chosen, percent) },
		Leave:    func() { leaves++ },
		Seek:     func(int) { seeks++ },
	})
	press := func(key int) {
		t.Helper()
		keyDown(view, key, 0)
		keyUp(view, key)
	}

	// Tab reaches the speed button between the volume control and fullscreen.
	view.state.Focus = focusVolume
	press(9)
	if view.state.Focus != focusSpeed {
		t.Fatalf("focus after Tab from volume = %d, want speed %d", view.state.Focus, focusSpeed)
	}
	press(9)
	if view.state.Focus != focusFullscreen {
		t.Fatalf("focus after Tab from speed = %d, want fullscreen", view.state.Focus)
	}
	keyDown(view, 9, fltk.SHIFT)
	keyUp(view, 9)
	if view.state.Focus != focusSpeed {
		t.Fatalf("focus after Shift+Tab from fullscreen = %d, want speed", view.state.Focus)
	}

	// Enter opens the list on the current speed (1x); Down and Up move the cursor and stop at the ends.
	press(fltk.ENTER_KEY)
	if !view.speedOpen || view.speedCursor != 1 {
		t.Fatalf("after Enter open=%v cursor=%d, want open on 1x", view.speedOpen, view.speedCursor)
	}
	for range 4 {
		press(0xff54)
	}
	if view.speedCursor != 4 {
		t.Fatalf("cursor after four Down = %d, want the last row", view.speedCursor)
	}
	for range 3 {
		press(0xff52)
	}
	if view.speedCursor != 1 {
		t.Fatalf("cursor after three Up = %d, want 1", view.speedCursor)
	}
	press(fltk.ENTER_KEY)
	if view.speedOpen || !reflect.DeepEqual(chosen, []int{100}) || view.speedPercent != 100 {
		t.Fatalf("Enter chose %v, open=%v; want [100] and a closed list", chosen, view.speedOpen)
	}

	// Space also chooses, and a chosen row shows as the current speed.
	press(fltk.ENTER_KEY)
	press(0xff54)
	press(' ')
	if view.speedOpen || !reflect.DeepEqual(chosen, []int{100, 125}) || view.speedPercent != 125 {
		t.Fatalf("Space chose %v, speed %d; want a final 125", chosen, view.speedPercent)
	}
	if got := speedLabel(view.speedPercent); got != "1.25×" {
		t.Fatalf("button label = %q", got)
	}

	// The list opens on the current speed.
	press(fltk.ENTER_KEY)
	if view.speedCursor != 2 {
		t.Fatalf("list opened on row %d, want 2 for 1.25x", view.speedCursor)
	}

	// Esc closes the list without choosing and without leaving the player.
	press(27)
	if view.speedOpen || len(chosen) != 2 || leaves != 0 {
		t.Fatalf("Esc: open=%v chosen=%v leaves=%d; want closed, nothing chosen, no leave", view.speedOpen, chosen, leaves)
	}
	press(27)
	if leaves != 1 {
		t.Fatalf("a second Esc left %d times, want 1", leaves)
	}

	// Arrows seek on the progress bar only; with the list open they belong to the list.
	view.state.Focus = focusProgress
	view.openSpeeds()
	press(0xff53)
	if seeks != 0 {
		t.Fatalf("Right with the list open sought %d times", seeks)
	}

	// The old skip-intro key does nothing now.
	view.speedOpen = false
	press('s')
	if seeks != 0 || len(chosen) != 2 {
		t.Fatalf("S did something: seeks=%d chosen=%v", seeks, chosen)
	}
}

func TestSpeedListClosesWhenTabbingAway(t *testing.T) {
	view := keyboardTestView(Callbacks{})
	view.state.Focus = focusSpeed
	view.openSpeeds()
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.speedOpen {
		t.Fatal("the speed list stayed open after Tab")
	}
}

func TestSpeedAndEpisodeListsAreNeverOpenTogether(t *testing.T) {
	view := keyboardTestView(Callbacks{})
	view.episodes = []string{"第 1 集"}
	view.openSpeeds()
	view.openEpisodes()
	if view.speedOpen || !view.episodesOpen {
		t.Fatalf("after opening episodes: speed=%v episodes=%v", view.speedOpen, view.episodesOpen)
	}
	view.openSpeeds()
	if !view.speedOpen || view.episodesOpen {
		t.Fatalf("after opening speed: speed=%v episodes=%v", view.speedOpen, view.episodesOpen)
	}
}

func TestSetSpeedShowsOnlyOfferedSteps(t *testing.T) {
	view := keyboardTestView(Callbacks{})
	view.SetSpeed(150)
	if view.speedPercent != 150 {
		t.Fatalf("speed = %d, want 150", view.speedPercent)
	}
	view.SetSpeed(110)
	if view.speedPercent != 100 {
		t.Fatalf("an unknown speed shows as %d, want normal speed", view.speedPercent)
	}
}

// The button must clear the position text (which ends about 110 px after x=410 when the
// volume slider is open) and the fullscreen button at every width the window can have, and the
// list must stay inside the window above the control bar at the smallest size.
func TestSpeedButtonAndListFitAtMinimumSize(t *testing.T) {
	const positionTextEnd = 410 + 110
	for _, width := range []int{playerMinWidth, 1000, 1920} {
		left, right := width-speedButtonLeftInset, width-speedButtonRightInset
		if left <= positionTextEnd {
			t.Errorf("width %d: button starts at %d, inside the position text ending near %d", width, left, positionTextEnd)
		}
		if right >= width-51 {
			t.Errorf("width %d: button ends at %d, over the fullscreen button at %d", width, right, width-51)
		}
		if speedListLeft(width) < 0 || speedListLeft(width)+speedListWidth > width {
			t.Errorf("width %d: list spans %d..%d, outside the window", width, speedListLeft(width), speedListLeft(width)+speedListWidth)
		}
	}
	for _, height := range []int{playerMinHeight, 618, 1080} {
		top, bottom := speedListTop(height), speedListTop(height)+speedListHeight()
		if top < 62 {
			t.Errorf("height %d: list top %d is under the title bar area", height, top)
		}
		if bottom > height-84 {
			t.Errorf("height %d: list bottom %d reaches the control bar at %d", height, bottom, height-84)
		}
	}
}

func TestSpeedRowAtMapsRowsAndRejectsOutside(t *testing.T) {
	const width, height = 1000, 618
	left, top := speedListLeft(width), speedListTop(height)+speedListRowsOffset
	for row := 0; row < 5; row++ {
		y := top + row*speedRowHeight + speedRowHeight/2
		if got, ok := speedRowAt(left+10, y, width, height); !ok || got != row {
			t.Errorf("row %d: got %d, %v", row, got, ok)
		}
	}
	for name, point := range map[string][2]int{
		"left of list":  {left - 1, top + 5},
		"right of list": {left + speedListWidth, top + 5},
		"above rows":    {left + 10, top - 1},
		"below rows":    {left + 10, top + 5*speedRowHeight},
	} {
		if _, ok := speedRowAt(point[0], point[1], width, height); ok {
			t.Errorf("%s hit a row", name)
		}
	}
}

func TestPlayerSpeedButtonClickAndDraw(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	var chosen []int
	view := NewWindow(Callbacks{SetSpeed: func(percent int) { chosen = append(chosen, percent) }})
	t.Cleanup(func() { view.Close(); view.window.Hide(); view.window.Destroy() })
	view.window.Show()
	fltk.Wait(0.01)
	width, height := view.video.W(), view.video.H()
	barY := height - 26

	if got := view.actionAt(width-95, barY); got != focusSpeed {
		t.Fatalf("button hit = %d, want speed", got)
	}
	if got := view.actionAt(width-35, barY); got != focusFullscreen {
		t.Fatalf("fullscreen hit = %d, want fullscreen beside the button", got)
	}
	if got := view.actionAt(width-135, barY); got != -1 {
		t.Fatalf("hit just left of the button = %d, want none", got)
	}
	view.activate(view.actionAt(width-95, barY))
	if !view.speedOpen {
		t.Fatal("clicking the button did not open the list")
	}
	row, ok := speedRowAt(speedListLeft(width)+20, speedListTop(height)+speedListRowsOffset+4*speedRowHeight+10, width, height)
	if !ok || row != 4 {
		t.Fatalf("last row hit = %d, %v", row, ok)
	}
	view.chooseSpeed(row)
	if !reflect.DeepEqual(chosen, []int{200}) || view.speedOpen {
		t.Fatalf("choosing the last row gave %v, open=%v; want [200] and a closed list", chosen, view.speedOpen)
	}

	gl := syscall.NewLazyDLL("opengl32.dll")
	call := func(name string, args ...uintptr) { t.Helper(); _, _, _ = gl.NewProc(name).Call(args...) }
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
	// A pixel inside the button, left of its label (glReadPixels counts rows from the bottom).
	var pixel [4]byte
	call("glReadPixels", uintptr(width-122), 14, 1, 1, 0x1908, 0x1401, uintptr(unsafe.Pointer(&pixel[0])))
	if pixel[2] < 50 || pixel[2] <= pixel[0] {
		t.Fatalf("button background pixel = %v, want the blue-grey control fill", pixel)
	}

	// With the list open, a pixel inside the list is the opaque dark panel, not cleared video.
	view.openSpeeds()
	view.video.MakeCurrent()
	call("glClearColor", 0, 0, 0, 0)
	call("glClear", 0x00004000)
	drawPlayerOverlay(width, height, view)
	call("glReadBuffer", buffer)
	listY := height - speedListTop(height) - speedListHeight() + 1
	call("glReadPixels", uintptr(speedListLeft(width)+2), uintptr(listY), 1, 1, 0x1908, 0x1401, uintptr(unsafe.Pointer(&pixel[0])))
	if pixel[3] < 200 {
		t.Fatalf("list panel pixel = %v, want an opaque panel", pixel)
	}
	view.window.Hide()
}

func TestSpeedListBlocksMediaShortcuts(t *testing.T) {
	calls := 0
	view := keyboardTestView(Callbacks{
		Seek:       func(int) { calls++ },
		Volume:     func(int) { calls++ },
		Fullscreen: func() { calls++ },
	})
	view.openSpeeds()
	for _, key := range []int{'m', 'f', 'j', 'l'} {
		keyDown(view, key, 0)
		keyUp(view, key)
	}
	if calls != 0 || !view.speedOpen {
		t.Fatalf("media shortcuts with the speed list open made %d calls, open=%v; want 0 and still open", calls, view.speedOpen)
	}
}
