//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

// localText picks one text out of the UI text, so whatever shows it reads the current language.
// A nil localText is empty.
type localText func(*uitext.Strings) string

func (pick localText) String() string {
	if pick == nil {
		return ""
	}
	return pick(uitext.T())
}

// fixedText is a text that reads the same in every language, such as a language's own name.
func fixedText(value string) localText { return func(*uitext.Strings) string { return value } }

// labelled gives widget the label pick names now and again after every language change.
func (ui *view) labelled(widget interface{ SetLabel(string) }, pick localText) {
	widget.SetLabel(pick.String())
	ui.relabels = append(ui.relabels, func() { widget.SetLabel(pick.String()) })
}

// setMessage shows message in the shared message line; nil clears it. The text is read again
// after a language change.
func (ui *view) setMessage(message func() string) {
	ui.messageText = message
	if message == nil {
		ui.message.SetLabel("")
		return
	}
	ui.message.SetLabel(message())
}

// useLanguage switches the UI to the backend language code. Fixed labels change at once, the
// page on screen is laid out and drawn again, and the other pages build their rows in the new
// language when they show next; the player's menu changes when it opens next.
func (ui *view) useLanguage(code string) {
	if uitext.For(code) == uitext.T() {
		return
	}
	uitext.Use(code)
	for _, relabel := range ui.relabels {
		relabel()
	}
	ui.setMessage(ui.messageText)
	// Loaded rows keep their data; only their words change, so a page whose reload fails still
	// shows them in the new language.
	for _, localize := range []func(){ui.localizeFollowing, ui.localizeSchedule} {
		if localize != nil {
			localize()
		}
	}
	ui.syncSectionText()
	ui.syncSettings()
	ui.window.Redraw()
}

// boldTextWidth is how wide value is in the bold UI font at size, so a control can grow to fit
// a longer translation.
func boldTextWidth(size int, value string) int {
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, size)
	width, _ := fltk.MeasureText(value, false)
	return width
}
