//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	fltk "github.com/pwiecz/go-fltk"
)

// segmentOption is one choice of a segmented control; value is what the setting stores.
type segmentOption struct{ title, value string }

// segmentedControl is a settings row with a title and hint and a row of exclusive choices.
// The choices sit at the end of the row, or under the text when the row is too narrow for
// both. The selected one is filled with the accent and carries a check mark and bold text, so
// it differs without colour.
type segmentedControl struct {
	title, hint string
	options     []segmentOption
	segmentW    int
	// selected is the index of the stored choice, or -1 when it matches none.
	selected func() int
	choose   func(value string)

	row     *settingsRow
	card    *fltk.Box
	buttons []*fltk.Button
}

// minSegmentedTextWidth is the room the title and hint need beside the choices; below it the
// choices move under the text.
const minSegmentedTextWidth = 100

func indexOfOption(options []segmentOption, value string) int {
	for index, option := range options {
		if option.value == value {
			return index
		}
	}
	return -1
}

func (control *segmentedControl) controlWidth() int { return len(control.options) * control.segmentW }

func (control *segmentedControl) stacked(width int) bool {
	return width-36-control.controlWidth()-12 < minSegmentedTextWidth
}

func (control *segmentedControl) cardHeight(width int) int {
	if control.stacked(width) {
		return settingsRowHeight + settingsSegmentH + 10
	}
	return settingsRowHeight
}

// addSegmentedRow builds the control inside the scroll and adds its row.
func (ui *view) addSegmentedRow(control *segmentedControl) {
	row := &settingsRow{}
	control.row = row
	control.card = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	control.card.SetDrawHandler(func(func()) { ui.drawSegmentedCard(control) })
	for index, option := range control.options {
		button := fltk.NewButton(0, 0, control.segmentW, settingsSegmentH, option.title)
		control.buttons = append(control.buttons, button)
		ui.styleSegment(control, index)
		ui.bindSegmentKeys(control, index)
	}
	row.height = func(width int) int { return control.cardHeight(width) + settingsRowGap }
	row.place = func(x, y, width int) {
		control.card.Resize(x, y, width, control.cardHeight(width))
		left, top := x+width-18-control.controlWidth(), y+(settingsRowHeight-settingsSegmentH)/2
		if control.stacked(width) {
			left, top = x+18, y+settingsRowHeight
		}
		for index, button := range control.buttons {
			button.Resize(left+index*control.segmentW, top, control.segmentW, settingsSegmentH)
		}
	}
	row.show = func(visible bool) {
		setShown(control.card, visible)
		for _, button := range control.buttons {
			setShown(button, visible)
		}
	}
	row.target = func() focusTarget {
		if selected := control.selected(); selected >= 0 {
			return control.buttons[selected]
		}
		return control.buttons[0]
	}
	row.sync = func() {
		for _, button := range control.buttons {
			ui.setSettingEnabled(button, row, !ui.settingsLoadFailed)
			button.Redraw()
		}
		control.card.Redraw()
	}
	ui.settingsRows = append(ui.settingsRows, row)
}

func (ui *view) bindSegmentKeys(control *segmentedControl, index int) {
	button := control.buttons[index]
	choose := func() { control.choose(control.options[index].value) }
	button.SetCallback(choose)
	button.SetEventHandler(ui.settingsKeys(control.row, choose, func(command keyCommand) bool {
		switch {
		case command == keyLeft && index > 0:
			ui.focusAfterKey(func() { control.buttons[index-1].TakeFocus() })
		case command == keyRight && index < len(control.buttons)-1:
			ui.focusAfterKey(func() { control.buttons[index+1].TakeFocus() })
		case command == keyRight:
			// The last segment is the right edge; FLTK must not move focus elsewhere.
		default:
			return false // Left on the first segment goes to the navigation
		}
		return true
	}))
}

func (ui *view) drawSegmentedCard(control *segmentedControl) {
	colors := ui.colors()
	x, y, width, height := control.card.X(), control.card.Y(), control.card.W(), control.card.H()
	drawSettingsCard(colors, x, y, width, height, false)
	title := colors.ink
	if ui.settingsLoadFailed {
		title = colors.muted
	}
	textWidth := width - 36
	if !control.stacked(width) {
		textWidth -= control.controlWidth() + 12
	}
	drawRowText(x, y, width, height, textWidth, control.title, control.hint, title, colors.muted, false)
}

// styleSegment draws one segment. Unselected ones are outlined and plain. Neighbours share a
// divider: every segment draws its top, bottom and left edge and only the last one its right
// edge. Focus is a thicker ring with a gap to the fill, also on the selected segment.
func (ui *view) styleSegment(control *segmentedControl, index int) {
	button := control.buttons[index]
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		active, selected, focused := button.IsActive(), control.selected() == index, button.HasFocus()

		look := segmentLookFor(colors, active, selected, focused)
		fltk.SetDrawColor(look.edge)
		fltk.DrawRectf(x, y, width, 1)
		fltk.DrawRectf(x, y+height-1, width, 1)
		fltk.DrawRectf(x, y, 1, height)
		if index == len(control.buttons)-1 {
			fltk.DrawRectf(x+width-1, y, 1, height)
		}
		inner := 1
		if focused {
			fltk.SetDrawColor(look.ring)
			fltk.DrawRectf(x, y, width, height)
			fltk.SetDrawColor(colors.surface)
			fltk.DrawRectf(x+look.ringWidth, y+look.ringWidth, width-2*look.ringWidth, height-2*look.ringWidth)
			inner = look.ringWidth + look.gap
		}
		fltk.SetDrawColor(look.fill)
		fltk.DrawRectf(x+inner, y+inner, width-2*inner, height-2*inner)

		font := fltk.HELVETICA
		if selected {
			font = fltk.HELVETICA_BOLD
		}
		fltk.SetDrawColor(look.text)
		fltk.SetDrawFont(font, 14)
		textX := x + 6
		if selected {
			drawCheck(x+10, y+height/2)
			textX = x + 18
		}
		fltk.Draw(button.Label(), textX, y+3, x+width-6-textX, height-6, fltk.ALIGN_CENTER|fltk.ALIGN_CLIP)
	})
}

// drawCheck draws a check mark whose bend is at (x+3, y+4), in the current draw colour.
func drawCheck(x, y int) {
	fltk.SetLineStyle(fltk.SOLID, 2)
	fltk.DrawLine(x-2, y, x+1, y+3)
	fltk.DrawLine(x+1, y+3, x+7, y-4)
	fltk.SetLineStyle(fltk.SOLID, 0)
}

// segmentLook is the colours and focus geometry of one segment. On the selected segment the
// ring is ink, with a surface gap between it and the accent fill, so focus does not vanish
// into the fill; elsewhere it is the accent.
type segmentLook struct {
	edge, fill, text, ring fltk.Color
	ringWidth, gap         int
}

func segmentLookFor(colors palette, active, selected, focused bool) segmentLook {
	look := segmentLook{edge: colors.accent, fill: colors.surface, text: colors.ink, ring: colors.accent, ringWidth: 2, gap: 1}
	switch {
	case !active && selected:
		look.edge, look.fill, look.text = colors.muted, colors.soft, colors.muted
	case !active:
		look.edge, look.text = colors.soft, colors.muted
	case selected:
		look.fill, look.text, look.ring, look.gap = colors.accent, colors.surface, colors.ink, 2
	}
	return look
}
