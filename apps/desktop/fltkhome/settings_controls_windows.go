//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"strings"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

// settingsAction is a row with a button at its end that runs run. While busy reports true the
// button is drawn disabled and ignores presses but keeps focus, so a keyboard user is not
// thrown out when a task starts; enabled reports false for an action that does not apply now.
type settingsAction struct {
	title   localText
	face    localText
	status  func() string
	tone    func() statusTone // nil: neutral
	enabled func() bool       // nil: always
	busy    func() bool       // nil: never
	visible func() bool       // nil: always
	run     func()

	button *fltk.Button
}

// actionFaceW is the narrowest an action's button is; a longer label widens it.
const actionFaceW = 132

func (action *settingsAction) faceWidth() int {
	return max(actionFaceW, boldTextWidth(14, action.face.String())+24)
}

func (action *settingsAction) isBusy() bool { return action.busy != nil && action.busy() }

func (ui *view) addActionRow(action *settingsAction) {
	row := &settingsRow{visible: action.visible}
	action.button = fltk.NewButton(0, 0, 1, 1, "")
	ui.labelled(action.button, action.title)
	ui.styleAction(action)
	activate := func() {
		if !action.isBusy() {
			action.run()
		}
	}
	action.button.SetCallback(activate)
	action.button.SetEventHandler(ui.settingsKeys(row, activate, nil))
	row.height = fixedHeight(settingsRowHeight + settingsRowGap)
	row.place = func(x, y, width int) { action.button.Resize(x, y, width, settingsRowHeight) }
	row.show = func(visible bool) { setShown(action.button, visible) }
	row.target = func() focusTarget { return action.button }
	row.sync = func() {
		ui.setSettingEnabled(action.button, row, action.enabled == nil || action.enabled())
		action.button.Redraw()
	}
	ui.settingsRows = append(ui.settingsRows, row)
}

func (ui *view) styleAction(action *settingsAction) {
	button := action.button
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		active := button.IsActive()
		drawSettingsCard(colors, x, y, width, height, button.HasFocus())
		title := colors.ink
		if !active {
			title = colors.muted
		}
		tone := toneNeutral
		if action.tone != nil {
			tone = action.tone()
		}
		// A result is bold as well as coloured, so it reads without the colour.
		faceW := action.faceWidth()
		drawRowText(x, y, width, height, width-36-faceW-12, action.title.String(), action.status(), title, toneColor(colors, tone), tone != toneNeutral)

		fill, ink := colors.soft, colors.accent
		if !active || action.isBusy() {
			ink = colors.muted
		}
		faceX, faceY := x+width-18-faceW, y+(height-settingsSegmentH)/2
		roundedRect(faceX, faceY, faceW, settingsSegmentH, 10, fill)
		fltk.SetDrawColor(ink)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 14)
		fltk.Draw(action.face.String(), faceX+4, faceY, faceW-8, settingsSegmentH, fltk.ALIGN_CENTER|fltk.ALIGN_CLIP)
	})
}

// settingsInfo is a row that only shows a title and one line under it.
type settingsInfo struct {
	title  localText
	detail func() string

	card *fltk.Box
}

func (ui *view) addInfoRow(info *settingsInfo) {
	info.card = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	info.card.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := info.card.X(), info.card.Y(), info.card.W(), info.card.H()
		drawSettingsCard(colors, x, y, width, height, false)
		drawRowText(x, y, width, height, width-36, info.title.String(), info.detail(), colors.ink, colors.muted, false)
	})
	ui.settingsRows = append(ui.settingsRows, &settingsRow{
		height: fixedHeight(settingsRowHeight + settingsRowGap),
		place:  func(x, y, width int) { info.card.Resize(x, y, width, settingsRowHeight) },
		show:   func(visible bool) { setShown(info.card, visible) },
		sync:   info.card.Redraw,
	})
}

const proxyInputW = 200

// proxyAddressField is the 手動 proxy's host:port row. The text is saved on Enter or when
// focus leaves, and only when core accepts it; otherwise the row says so and the saved value
// stays, so a later save of another setting never carries an address the backend would refuse.
type proxyAddressField struct {
	card    *fltk.Box
	input   *fltk.Input
	invalid bool
}

func (ui *view) addProxyAddressRow() {
	field := &proxyAddressField{}
	ui.proxyField = field
	row := &settingsRow{visible: func() bool { return ui.settings.ProxyMode == "manual" }}
	field.card = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	field.card.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := field.card.X(), field.card.Y(), field.card.W(), field.card.H()
		drawSettingsCard(colors, x, y, width, height, field.input.HasFocus())
		title, hint, hintColor := colors.ink, uitext.T().ProxyAddressHint, colors.muted
		if ui.settingsLoadFailed {
			title = colors.muted
		}
		if field.invalid {
			hint, hintColor = uitext.T().ProxyAddressInvalid, colors.ink
		}
		drawRowText(x, y, width, height, width-36-proxyInputW-12, uitext.T().ProxyAddress, hint, title, hintColor, field.invalid)
		// The field is flat; its outline is drawn here, one pixel outside it.
		fltk.SetDrawColor(colors.muted)
		fltk.DrawRect(field.input.X()-1, field.input.Y()-1, field.input.W()+2, field.input.H()+2)
	})
	field.input = fltk.NewInput(0, 0, proxyInputW, settingsSegmentH, "")
	field.input.SetBox(fltk.FLAT_BOX)
	field.input.SetLabelSize(14)
	field.input.SetCallbackCondition(fltk.WhenChanged)
	field.input.SetCallback(func() {
		if field.invalid {
			field.invalid = false
			field.card.Redraw()
		}
	})
	field.input.SetDrawHandler(func(draw func()) {
		draw()
		if field.input.Value() == "" && !field.input.HasFocus() {
			fltk.PushClip(field.input.X(), field.input.Y(), field.input.W(), field.input.H())
			fltk.SetDrawColor(ui.colors().muted)
			fltk.SetDrawFont(fltk.HELVETICA, 14)
			fltk.Draw(uitext.T().ProxyPlaceholder, field.input.X()+6, field.input.Y(), field.input.W()-12, field.input.H(), fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
			fltk.PopClip()
		}
	})
	keys := ui.settingsKeys(row, nil, nil)
	field.input.SetEventHandler(func(event fltk.Event) bool {
		switch event {
		case fltk.UNFOCUS:
			ui.commitProxyAddress()
			return false
		case fltk.KEYDOWN:
			switch command := classifyKey(fltk.EventKey(), fltk.EventState()); {
			case isEnterKey(fltk.EventKey()):
				ui.commitProxyAddress()
				return true
			case command == keyUp || command == keyDown || command == keyNext || command == keyPrevious:
				return keys(event)
			}
		}
		return false // typing, and Left and Right inside the text, stay with the input
	})
	row.height = fixedHeight(settingsRowHeight + settingsRowGap)
	row.place = func(x, y, width int) {
		field.card.Resize(x, y, width, settingsRowHeight)
		field.input.Resize(x+width-18-proxyInputW, y+(settingsRowHeight-settingsSegmentH)/2, proxyInputW, settingsSegmentH)
	}
	row.show = func(visible bool) {
		setShown(field.card, visible)
		setShown(field.input, visible)
	}
	row.target = func() focusTarget { return field.input }
	row.sync = func() {
		ui.setSettingEnabled(field.input, row, !ui.settingsLoadFailed)
		field.card.Redraw()
	}
	ui.settingsRows = append(ui.settingsRows, row)
}

// commitProxyAddress saves the typed address when it is valid and different from the saved one.
// It also runs when the page is left and when the window closes, where no focus-out event
// is guaranteed to arrive first.
func (ui *view) commitProxyAddress() {
	field := ui.proxyField
	if field == nil || ui.settingsLoadFailed || ui.settings.ProxyMode != "manual" {
		return
	}
	text := strings.TrimSpace(field.input.Value())
	if !core.ValidProxyAddress(text) {
		field.invalid = true
		field.card.Redraw()
		return
	}
	field.invalid = false
	field.card.Redraw()
	if text != ui.settings.ProxyAddress {
		ui.changeSettings(func(settings *backend.Settings) { settings.ProxyAddress = text })
	}
}
