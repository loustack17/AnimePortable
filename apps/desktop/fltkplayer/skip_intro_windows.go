//go:build windows && cgo

package fltkplayer

import (
	"math"

	fltk "github.com/pwiecz/go-fltk"
)

// The 跳過片頭 button is drawn by overlay_windows.cc with the same numbers; keep both in step.
// It sits above the control bar at the right, as insets from the right and bottom edges.
const (
	skipButtonRightInset  = 24
	skipButtonBottomInset = 140
	skipButtonHeight      = 40
	skipButtonPadding     = 20
	skipLabelSize         = 16
	// skipHideBefore hides the button this many seconds before the opening ends, so it never
	// flashes up just as the song finishes.
	skipHideBefore = 1
)

// skipShown reports whether the episode is inside its opening, the only time 跳過片頭 shows.
func (state State) skipShown() bool {
	return state.OpeningEnd > state.OpeningStart && state.Playing && !state.Loading &&
		state.Position >= state.OpeningStart && state.Position < state.OpeningEnd-skipHideBefore
}

// skipButtonWidth fits the label in the overlay's bold font.
func skipButtonWidth(label string) int {
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, skipLabelSize)
	width, _ := fltk.MeasureText(label, false)
	return width + 2*skipButtonPadding
}

func (view *View) skipButtonAt(x, y int) bool {
	width, height := view.video.W(), view.video.H()
	left, top := width-skipButtonRightInset-view.skipWidth, height-skipButtonBottomInset
	return view.state.skipShown() && x >= left && x <= width-skipButtonRightInset && y >= top && y <= top+skipButtonHeight
}

// skipOpening jumps to the end of the opening. The position moves at once, so the button goes
// before the engine reports the seek.
func (view *View) skipOpening() {
	if !view.state.skipShown() {
		return
	}
	end := view.state.OpeningEnd
	if view.callbacks.SeekTo != nil {
		view.callbacks.SeekTo(int(math.Ceil(end)))
	}
	view.state.Position = end
	if view.state.Focus == focusSkip {
		view.state.Focus = focusProgress
	}
	view.Redraw()
}
