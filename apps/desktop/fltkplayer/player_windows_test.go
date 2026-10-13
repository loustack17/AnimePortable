//go:build windows && cgo

package fltkplayer

import (
	"testing"
	"time"
)

func TestProgressPositionClampsDrag(t *testing.T) {
	for _, test := range []struct {
		x, width int
		duration float64
		want     float64
	}{
		{-30, 1000, 200, 0},
		{24, 1000, 200, 0},
		{500, 1000, 200, 100},
		{976, 1000, 200, 200},
		{1200, 1000, 200, 200},
		{500, 1000, 0, 0},
	} {
		if got := progressPosition(test.x, test.width, test.duration); got != test.want {
			t.Errorf("progressPosition(%d, %d, %.0f) = %.0f, want %.0f", test.x, test.width, test.duration, got, test.want)
		}
	}
}

func TestTabFocusStopsAtEnds(t *testing.T) {
	if got := nextTabFocus(-1, false); got != focusMenu {
		t.Fatalf("initial Tab = %d, want menu", got)
	}
	if got := nextTabFocus(-1, true); got != focusFullscreen {
		t.Fatalf("initial Shift+Tab = %d, want fullscreen", got)
	}
	if got := nextTabFocus(focusFullscreen, false); got != focusFullscreen {
		t.Fatalf("final Tab wrapped to %d", got)
	}
	if got := nextTabFocus(focusMenu, true); got != focusMenu {
		t.Fatalf("first Shift+Tab wrapped to %d", got)
	}
}

func TestPlayerControlsHideAfterThreeSecondsOfActivePlayback(t *testing.T) {
	now := time.Unix(100, 0)
	view := &View{state: State{Playing: true}, visible: true, lastMove: now.Add(-controlsIdleTimeout)}
	if !view.shouldHideControls(now) {
		t.Fatal("active playback did not hide controls at the idle timeout")
	}
	view.lastMove = now.Add(-controlsIdleTimeout + time.Nanosecond)
	if view.shouldHideControls(now) {
		t.Fatal("controls hid before the idle timeout")
	}
	for _, state := range []State{
		{Paused: true},
		{Loading: true},
		{},
	} {
		view.state = state
		view.lastMove = now.Add(-controlsIdleTimeout)
		if view.shouldHideControls(now) {
			t.Errorf("state %+v hid controls", state)
		}
	}
	view.visible = false
	view.lastMove = now.Add(-controlsIdleTimeout)
	if view.shouldHideControls(now) {
		t.Fatal("already hidden controls reported another hide")
	}
	view.visible = true
	view.state = State{Playing: true}
	view.menuPinned = true
	if view.shouldHideControls(now) {
		t.Fatal("controls hid while the menu was open")
	}
	view.menuPinned = false
	view.episodesOpen = true
	if view.shouldHideControls(now) {
		t.Fatal("controls hid while the episode list was open")
	}
	view.episodesOpen = false
	view.scrubbing = true
	if view.shouldHideControls(now) {
		t.Fatal("controls hid while scrubbing")
	}
	view.scrubbing = false
	view.dragging = true
	if view.shouldHideControls(now) {
		t.Fatal("controls hid while dragging the volume slider")
	}
}

func TestIdlePointerDoesNotPinControls(t *testing.T) {
	view := &View{state: State{Playing: true}}
	view.showAtSize(500, 618-60, 1000, 618)
	if !view.progressHover || !view.shouldHideControls(view.lastMove.Add(controlsIdleTimeout)) {
		t.Fatal("resting pointer over the control bar prevented idle hiding")
	}
	view.showAtSize(10, 100, 1000, 618)
	if !view.menuHover || !view.shouldHideControls(view.lastMove.Add(controlsIdleTimeout)) {
		t.Fatal("resting pointer over the navigation edge prevented idle hiding")
	}
}

func TestPlaybackProgressUpdatesDoNotRestartIdleTimer(t *testing.T) {
	lastMove := time.Unix(100, 0)
	view := &View{state: State{Playing: true}, visible: true, lastMove: lastMove}
	view.SetState(State{Playing: true, Position: 1})
	if !view.lastMove.Equal(lastMove) {
		t.Fatalf("progress update restarted idle timer at %v, want %v", view.lastMove, lastMove)
	}
}
