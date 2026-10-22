//go:build windows && cgo

package fltkplayer

import (
	"slices"
	"strconv"

	"animeportable/core"
)

// The speed button and its list are drawn by overlay_windows.cc with the same numbers; keep
// both in step. The button sits left of the fullscreen button, as insets from the right edge.
const (
	speedButtonLeftInset  = 127
	speedButtonRightInset = 63
	speedListWidth        = 84
	speedRowHeight        = 34
	speedListRowsOffset   = 4
	// The list's bottom edge is this far above the window's bottom, clear of the control bar.
	speedListBottomInset = 92
	// playerMinWidth keeps the button clear of the time label and the volume panel: the label
	// ends about 520 px from the left when the volume panel is open.
	playerMinWidth  = 760
	playerMinHeight = 480
)

// speedLabels are the list rows, slowest first, in core.PlaybackSpeedSteps order.
func speedLabels() []string {
	labels := make([]string, len(core.PlaybackSpeedSteps))
	for index, step := range core.PlaybackSpeedSteps {
		labels[index] = speedLabel(int(step))
	}
	return labels
}

// speedLabel writes a percent as a multiplier: 100 is 1×, 125 is 1.25×.
func speedLabel(percent int) string {
	return strconv.FormatFloat(float64(percent)/100, 'f', -1, 64) + "×"
}

func speedIndex(percent int) int {
	return slices.Index(core.PlaybackSpeedSteps[:], core.PlaybackSpeed(percent))
}

func speedListHeight() int {
	return len(core.PlaybackSpeedSteps)*speedRowHeight + 2*speedListRowsOffset
}

func speedListLeft(width int) int { return width - speedButtonRightInset - speedListWidth }

func speedListTop(height int) int { return height - speedListBottomInset - speedListHeight() }

func speedButtonAt(x, width int) bool {
	return x >= width-speedButtonLeftInset && x <= width-speedButtonRightInset
}

// speedRowAt maps a click to a row of the open list; ok is false outside the rows.
func speedRowAt(x, y, width, height int) (index int, ok bool) {
	left, top := speedListLeft(width), speedListTop(height)+speedListRowsOffset
	rows := len(core.PlaybackSpeedSteps)
	if x < left || x >= left+speedListWidth || y < top || y >= top+speedRowHeight*rows {
		return 0, false
	}
	return (y - top) / speedRowHeight, true
}

// SetSpeed shows the speed the player runs at, in percent. An unknown value shows as normal speed.
func (view *View) SetSpeed(percent int) {
	if speedIndex(percent) < 0 {
		percent = int(core.PlaybackSpeedDefault)
	}
	view.speedPercent = percent
	view.Redraw()
}

func (view *View) speedPercentOrDefault() int {
	if view.speedPercent == 0 {
		return int(core.PlaybackSpeedDefault)
	}
	return view.speedPercent
}

func (view *View) openSpeeds() {
	view.episodesOpen = false
	view.speedOpen = true
	view.speedCursor = max(0, speedIndex(view.speedPercentOrDefault()))
}

// chooseSpeed applies a row of the list and closes it.
func (view *View) chooseSpeed(index int) {
	view.speedOpen = false
	if index < 0 || index >= len(core.PlaybackSpeedSteps) {
		return
	}
	view.speedPercent = int(core.PlaybackSpeedSteps[index])
	if view.callbacks.SetSpeed != nil {
		view.callbacks.SetSpeed(view.speedPercent)
	}
}

func (view *View) moveSpeedCursor(delta int) {
	view.speedCursor = max(0, min(len(core.PlaybackSpeedSteps)-1, view.speedCursor+delta))
}
