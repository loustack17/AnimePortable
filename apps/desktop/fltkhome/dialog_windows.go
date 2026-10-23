//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
	"strings"

	"animeportable/apps/desktop/appicon"
	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

// missingFolderLabels are the buttons of the startup question; the first is the safe answer.
func missingFolderLabels() []string { return []string{uitext.T().Quit, uitext.T().UseDefaultFolder} }

func missingFolderMessage(path string) string { return fmt.Sprintf(uitext.T().MissingFolder, path) }

// AskMissingDataFolder asks, before the main window exists, whether to use the default data
// folder because the chosen one is missing. It returns true for 使用預設位置 and false for 結束,
// which Esc and closing the window also give, so leaving the question never creates or moves
// data. The stored theme is not readable yet, so the dialog follows the Windows app theme.
func AskMissingDataFolder(path string) bool {
	useAppFonts()
	return showDialog(nil, uitext.T().MissingFolderTitle, missingFolderMessage(path), systemUsesDarkTheme(), missingFolderLabels()...) == 1
}

// shownDark is the theme applyTheme last applied. FLTK's own colours are process-wide, so the
// theme on screen is too; ShowNotice reads it for messages raised outside the Home window.
// Read and written on the UI thread only.
var shownDark bool

// ShowNotice shows a message with one 確定 button in the app's style and the theme on screen,
// centred over parent, a live top-level window. It must run on the UI thread.
func ShowNotice(parent *fltk.Window, title, message string) {
	showDialog(parent, title, message, shownDark, uitext.T().OK)
}

// useAppFonts selects the scheme and the CJK-capable fonts every window of the app uses.
func useAppFonts() {
	fltk.SetScheme("oxy")
	fltk.SetFont(fltk.HELVETICA, "Microsoft JhengHei UI")
	fltk.SetFont(fltk.HELVETICA_BOLD, "Microsoft JhengHei UI Bold")
}

// dialogFocus is the keyboard state of a dialog: which button has focus, and what a key does.
// Button 0 is the safe answer: it has focus first and Esc answers it. The last button is the
// primary one, drawn filled.
type dialogFocus struct{ count, focus int }

// key applies a key command. Enter and Space answer with the focused button, Esc with button 0;
// Tab and the arrows move focus around the buttons (Shift+Tab backwards).
func (state *dialogFocus) key(command keyCommand) (answer int, done bool) {
	switch command {
	case keyBack:
		return 0, true
	case keyActivate:
		return state.focus, true
	case keyNext, keyRight, keyDown:
		state.focus = (state.focus + 1) % state.count
	case keyPrevious, keyLeft, keyUp:
		state.focus = (state.focus + state.count - 1) % state.count
	}
	return 0, false
}

// movesFocus reports whether command moves focus between the buttons.
func movesFocus(command keyCommand) bool {
	switch command {
	case keyNext, keyPrevious, keyLeft, keyRight, keyUp, keyDown:
		return true
	}
	return false
}

const (
	dialogMinWidth = 440
	dialogMaxWidth = 640
	dialogMinLines = 3
	// dialogScreenRoom is left free around a dialog on the screen's work area.
	dialogScreenRoom = 40
	dialogCardMargin = 16
	dialogTextMargin = 24
	dialogTitleH     = 26
	dialogLineH      = 22
	dialogButtonH    = 36
	dialogButtonMinW = 112
	dialogButtonGap  = 12
	// dialogRingRoom is the room around a button for its focus ring, which a widget cannot
	// draw outside its own bounds.
	dialogRingRoom = 4
)

// dialog is the app's modal question or notice: a rounded card with a title, a message and
// one button per label, in the palette of the current theme.
type dialog struct {
	window  *fltk.Window
	card    *fltk.Box
	buttons []*fltk.Button
	labels  []string
	lines   []string
	title   string
	dark    bool
	state   dialogFocus
	answer  int
	done    bool

	width, height int
	// visible is the number of message lines shown and offset the first of them. A message taller
	// than the screen scrolls, so all of it stays reachable.
	visible, offset int
}

// showDialog shows a modal dialog and returns the index of the label chosen. Esc and closing the
// window answer 0, so the first label must be the one that changes nothing. It starts a nested
// event loop and returns when the dialog is answered. The dialog centres over parent, or, when
// parent is nil, on the screen the mouse was last on.
func showDialog(parent *fltk.Window, title, message string, dark bool, labels ...string) int {
	return newDialog(parent, title, message, dark, labels...).run()
}

// dialogScreen returns the work area the dialog sits in and the point it centres on.
func dialogScreen(parent *fltk.Window) (workX, workY, workW, workH, centerX, centerY int) {
	if parent != nil {
		centerX, centerY = parent.XRoot()+parent.W()/2, parent.YRoot()+parent.H()/2
	} else {
		centerX, centerY = fltk.EventXRoot(), fltk.EventYRoot()
	}
	workX, workY, workW, workH = fltk.ScreenWorkArea(fltk.ScreenNum(centerX, centerY))
	if parent == nil {
		centerX, centerY = workX+workW/2, workY+workH/2
	}
	return
}

// leaveOpenGroups makes sure the next window is a top-level window. FLTK adds a new widget to
// the group last opened with Begin, and ending a nested group reopens its parent, so a page that
// rebuilds its rows after construction leaves a group of the Home window open. A dialog created
// then becomes a child of the Home window: it is drawn inside it, outside its bounds, and the
// modal wait never ends. Ending any top-level window reopens no parent, which clears the state
// for the whole process. The parent is passed in rather than remembered, so a destroyed window
// is never ended; a dialog without one only runs before the Home window exists.
func leaveOpenGroups(parent *fltk.Window) {
	if parent != nil {
		parent.End()
	}
}

func newDialog(parent *fltk.Window, title, message string, dark bool, labels ...string) *dialog {
	d := &dialog{labels: labels, title: title, dark: dark, state: dialogFocus{count: len(labels)}}
	measure := func(value string) int {
		measured, _ := fltk.MeasureText(value, false)
		return measured
	}

	fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
	widths := make([]int, len(labels))
	rowWidth := (len(labels) - 1) * dialogButtonGap
	for index, label := range labels {
		widths[index] = max(dialogButtonMinW, measure(label)+40)
		rowWidth += widths[index]
	}
	inset := dialogCardMargin + dialogTextMargin
	d.width = max(dialogMinWidth, rowWidth+2*inset)
	fltk.SetDrawFont(fltk.HELVETICA, 15)
	workX, workY, workW, workH, centerX, centerY := dialogScreen(parent)
	messageTop := dialogCardMargin + 20 + dialogTitleH + 8
	chrome := messageTop + 20 + dialogButtonH + 20 + dialogCardMargin
	maxLines := max(dialogMinLines, (workH-dialogScreenRoom-chrome)/dialogLineH)
	d.lines = wrapLines(message, d.width-2*inset, measure)
	if len(d.lines) > maxLines {
		// Widen before scrolling: a long path reads better on fewer, longer lines.
		d.width = max(d.width, min(dialogMaxWidth, workW-dialogScreenRoom))
		inset = dialogCardMargin + dialogTextMargin
		d.lines = wrapLines(message, d.width-2*inset, measure)
	}
	d.visible = min(len(d.lines), maxLines)
	buttonTop := messageTop + d.visible*dialogLineH + 20
	d.height = buttonTop + dialogButtonH + 20 + dialogCardMargin

	x := max(workX, min(workX+workW-d.width, centerX-d.width/2))
	y := max(workY, min(workY+workH-d.height, centerY-d.height/2))
	leaveOpenGroups(parent)
	d.window = fltk.NewWindowWithPosition(x, y, d.width, d.height, title)
	appicon.Set(d.window)
	d.window.SetColor(d.colors().background)
	d.window.Begin()
	d.card = fltk.NewBox(fltk.NO_BOX, 0, 0, d.width, d.height, "")
	d.card.SetDrawHandler(func(func()) { d.drawCard(messageTop) })
	d.card.SetEventHandler(func(event fltk.Event) bool {
		return event == fltk.MOUSEWHEEL && d.scroll(fltk.EventDY()*3)
	})
	buttonX := d.width - inset - rowWidth
	for index, label := range labels {
		button := fltk.NewButton(buttonX-dialogRingRoom, buttonTop-dialogRingRoom, widths[index]+2*dialogRingRoom, dialogButtonH+2*dialogRingRoom, label)
		button.SetBox(fltk.NO_BOX)
		button.SetDrawHandler(func(func()) { d.drawButton(index) })
		button.SetCallback(func() { d.finish(index) })
		button.SetEventHandler(func(event fltk.Event) bool {
			switch event {
			case fltk.KEYDOWN:
				switch fltk.EventKey() {
				case pageUpKey:
					return d.scroll(-d.visible)
				case pageDownKey:
					return d.scroll(d.visible)
				}
				return d.handleKey(classifyKey(fltk.EventKey(), fltk.EventState()))
			case fltk.FOCUS:
				// A click moves focus too; the ring follows it.
				d.state.focus = index
				d.redrawButtons()
			}
			return false
		})
		d.buttons = append(d.buttons, button)
		buttonX += widths[index] + dialogButtonGap
	}
	d.window.End()
	d.window.SetModal()
	d.window.SetCallback(func() { d.finish(0) })
	return d
}

func (d *dialog) colors() palette {
	if d.dark {
		return darkPalette
	}
	return lightPalette
}

// run shows the dialog, waits for an answer and returns it.
func (d *dialog) run() int {
	d.window.Show()
	d.buttons[d.state.focus].TakeFocus()
	for !d.done {
		fltk.Wait()
	}
	d.destroy()
	return d.answer
}

// destroy deletes the dialog. Its widgets are deleted one by one first: go-fltk's deletion handler
// of a child that still has a draw handler unregisters that handler's id from the plain callback
// map, which drops an unrelated live callback and later crashes the event loop. A widget
// destroyed on its own has its handler ids cleared before FLTK deletes it.
func (d *dialog) destroy() {
	for _, button := range d.buttons {
		button.Destroy()
	}
	d.card.Destroy()
	d.window.Destroy()
}

// finish records the answer and closes the window; only the first answer counts.
func (d *dialog) finish(answer int) {
	if d.done {
		return
	}
	d.done, d.answer = true, answer
	d.window.Hide()
}

// handleKey runs a key command and reports whether the dialog used it.
func (d *dialog) handleKey(command keyCommand) bool {
	answer, done := d.state.key(command)
	switch {
	case done:
		d.finish(answer)
	case movesFocus(command):
		d.buttons[d.state.focus].TakeFocus()
		d.redrawButtons()
	default:
		return false
	}
	return true
}

// pageUpKey and pageDownKey scroll a message taller than the dialog; the arrows keep moving
// between the buttons.
const (
	pageUpKey   = 0xff55
	pageDownKey = 0xff56
)

// scroll moves the message by delta lines. It reports false when the message fits, so the key or
// wheel then goes to others.
func (d *dialog) scroll(delta int) bool {
	if len(d.lines) <= d.visible {
		return false
	}
	d.offset = max(0, min(len(d.lines)-d.visible, d.offset+delta))
	d.window.Redraw()
	return true
}

func (d *dialog) redrawButtons() {
	for _, button := range d.buttons {
		button.Redraw()
	}
}

func (d *dialog) drawCard(messageTop int) {
	colors := d.colors()
	fltk.SetDrawColor(colors.background)
	fltk.DrawRectf(0, 0, d.width, d.height)
	roundedRect(dialogCardMargin, dialogCardMargin, d.width-2*dialogCardMargin, d.height-2*dialogCardMargin, 13, colors.surface)

	left, textWidth := dialogCardMargin+dialogTextMargin, d.width-2*(dialogCardMargin+dialogTextMargin)
	fltk.SetDrawColor(colors.ink)
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, 17)
	fltk.Draw(d.title, left, dialogCardMargin+20, textWidth, dialogTitleH, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
	fltk.SetDrawFont(fltk.HELVETICA, 15)
	for index, line := range d.lines[d.offset : d.offset+d.visible] {
		// "@" starts a symbol in an FLTK label; a path may contain one.
		fltk.Draw(strings.ReplaceAll(line, "@", "@@"), left, messageTop+index*dialogLineH, textWidth, dialogLineH, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE)
	}
	if len(d.lines) > d.visible {
		// A thin thumb at the text's right edge shows that more follows and where the view is.
		track := d.visible * dialogLineH
		thumb := max(12, track*d.visible/len(d.lines))
		top := messageTop + (track-thumb)*d.offset/(len(d.lines)-d.visible)
		roundedRect(d.width-dialogCardMargin-dialogTextMargin/2-2, top, 4, thumb, 2, colors.soft)
	}
}

func (d *dialog) drawButton(index int) {
	colors := d.colors()
	button := d.buttons[index]
	fltk.SetDrawColor(colors.surface)
	fltk.DrawRectf(button.X(), button.Y(), button.W(), button.H())
	x, y := button.X()+dialogRingRoom, button.Y()+dialogRingRoom
	width, height := button.W()-2*dialogRingRoom, button.H()-2*dialogRingRoom
	if d.state.focus == index {
		// The ring is the text colour, so it shows on the accent-filled button too.
		roundedRect(x-3, y-3, width+6, height+6, 13, colors.ink)
		roundedRect(x-1, y-1, width+2, height+2, 11, colors.surface)
	}
	fill, ink := colors.soft, colors.ink
	if index == len(d.buttons)-1 {
		fill, ink = colors.accent, colors.onAccent
	}
	roundedRect(x, y, width, height, 10, fill)
	fltk.SetDrawColor(ink)
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
	fltk.Draw(d.labels[index], x+4, y, width-8, height, fltk.ALIGN_CENTER|fltk.ALIGN_CLIP)
}
