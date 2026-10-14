//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"strings"

	fltk "github.com/pwiecz/go-fltk"
)

// scrollingText draws a box's label wrapped to the box width and scrolls it with the mouse
// wheel. FLTK wraps labels only at spaces, so Chinese and Japanese text, which has none,
// would otherwise run past the box.
type scrollingText struct {
	box *fltk.Box
	// background repaints the box before drawing, since a scrolled redraw must erase old lines.
	background func() fltk.Color
	offset     int // first visible line
	lines      int // wrapped line count at the last draw
	visible    int // whole lines that fit at the last draw
}

func newScrollingText(box *fltk.Box, background func() fltk.Color) *scrollingText {
	text := &scrollingText{box: box, background: background}
	box.SetDrawHandler(func(func()) { text.draw() })
	box.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.MOUSEWHEEL || text.lines <= text.visible {
			return false
		}
		text.offset = max(0, min(text.lines-text.visible, text.offset+fltk.EventDY()*3))
		box.Redraw()
		return true
	})
	return text
}

// SetLabel shows new text from its first line.
func (text *scrollingText) SetLabel(value string) {
	if value != text.box.Label() {
		text.offset = 0
	}
	text.box.SetLabel(value)
}

func (text *scrollingText) draw() {
	box := text.box
	x, y, width, height := box.X(), box.Y(), box.W(), box.H()
	fltk.SetDrawColor(text.background())
	fltk.DrawRectf(x, y, width, height)
	fltk.SetDrawFont(box.LabelFont(), box.LabelSize())
	fltk.SetDrawColor(box.LabelColor())
	measure := func(value string) int {
		measured, _ := fltk.MeasureText(value, false)
		return measured
	}
	_, lineHeight := fltk.MeasureText("國", false)
	lines := wrapLines(box.Label(), width, measure)
	text.lines = len(lines)
	text.visible = max(1, height/max(1, lineHeight))
	text.offset = max(0, min(text.offset, text.lines-text.visible))
	shown := lines[text.offset:min(text.lines, text.offset+text.visible)]
	if text.offset+len(shown) < text.lines {
		// Mark that more text follows below.
		last := []rune(shown[len(shown)-1])
		for len(last) > 0 && measure(string(last)+"…") > width {
			last = last[:len(last)-1]
		}
		shown[len(shown)-1] = string(last) + "…"
	}
	fltk.PushClip(x, y, width, height)
	for index, line := range shown {
		// "@@" draws a literal "@"; FLTK otherwise reads "@" as the start of a symbol.
		fltk.Draw(strings.ReplaceAll(line, "@", "@@"), x, y+index*lineHeight, width, lineHeight, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE)
	}
	fltk.PopClip()
}

// wrapLines splits text into lines no wider than width, breaking at the last space when the
// line has one and between characters otherwise.
func wrapLines(text string, width int, measure func(string) int) []string {
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		line := []rune{}
		for _, character := range paragraph {
			line = append(line, character)
			if len(line) == 1 || measure(string(line)) <= width {
				continue
			}
			// The character that overflowed starts the next line, unless a space allows a word break.
			breakAt := len(line) - 1
			for index := breakAt - 1; index > 0; index-- {
				if line[index] == ' ' {
					breakAt = index + 1
					break
				}
			}
			lines = append(lines, strings.TrimRight(string(line[:breakAt]), " "))
			line = []rune(strings.TrimLeft(string(line[breakAt:]), " "))
		}
		lines = append(lines, string(line))
	}
	return lines
}
