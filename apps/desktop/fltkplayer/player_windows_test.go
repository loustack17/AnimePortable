//go:build windows && cgo

package fltkplayer

import "testing"

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
