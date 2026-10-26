//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"slices"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

const settingsSection = 5

const (
	settingsRowHeight     = 56
	settingsRowGap        = 8
	settingsHeadingHeight = 44
	settingsSwitchW       = 44
	settingsSwitchH       = 22
	settingsSegmentH      = 32
	settingsMarginX       = 15
	settingsMarginTop     = 6
	settingsMarginBottom  = 16
	// settingsScrollbar is the room the scroll's bar takes at its right edge.
	settingsScrollbar = 16
)

// themeSegments are the choices of 主題; value is the stored appearance.
var themeSegments = []segmentOption{
	{func(text *uitext.Strings) string { return text.ThemeSystem }, "system"},
	{func(text *uitext.Strings) string { return text.ThemeDark }, "dark"},
	{func(text *uitext.Strings) string { return text.ThemeLight }, "light"},
}

var proxySegments = []segmentOption{
	{func(text *uitext.Strings) string { return text.ProxyNone }, "none"},
	{func(text *uitext.Strings) string { return text.ProxySystem }, "system"},
	{func(text *uitext.Strings) string { return text.ProxyManual }, "manual"},
}

// languageSegments are the choices of 語言, each named in its own language.
func languageSegments() []segmentOption {
	options := make([]segmentOption, len(uitext.LanguageCodes))
	for index, code := range uitext.LanguageCodes {
		options[index] = segmentOption{fixedText(uitext.LanguageNames[index]), code}
	}
	return options
}

// focusTarget is a widget keyboard focus can land on.
type focusTarget interface {
	X() int
	Y() int
	W() int
	H() int
	IsActive() bool
	TakeFocus() int
}

// settingsRow is one line of the 設定 scroll. The page lays rows out top to bottom; a row
// that is not visible keeps its widgets hidden but placed, because the scroll sizes itself from
// every child.
type settingsRow struct {
	height  func(width int) int // the space the row takes, including the gap below it
	visible func() bool         // nil means always
	place   func(x, y, width int)
	show    func(visible bool)
	target  func() focusTarget // nil for rows that never take focus
	sync    func()             // enables the row's widgets and redraws it after a change
}

func (row *settingsRow) isShown() bool { return row.visible == nil || row.visible() }

func fixedHeight(height int) func(int) int { return func(int) int { return height } }

func setShown(widget interface {
	Show()
	Hide()
}, visible bool) {
	if visible {
		widget.Show()
	} else {
		widget.Hide()
	}
}

// settingsToggle is one switch row. A row that enabled reports false for is disabled and
// shows disabledHint, which says what to change first; its switch still shows its value.
type settingsToggle struct {
	button       *fltk.Button
	title        localText
	hint         localText
	disabledHint localText
	on           func() bool
	enabled      func() bool
	change       func()
}

func (ui *view) buildSettingsView() {
	page := fltk.NewGroup(236, 110, 741, 487)
	page.Begin()
	ui.settingsScroll = fltk.NewScroll(236, 110, 741, 487)
	ui.settingsScroll.SetType(fltk.SCROLL_VERTICAL)
	ui.settingsScroll.SetBox(fltk.FLAT_BOX)
	ui.useSettingsTaskDefaults()

	// The scroll takes the top left of its children as the origin of its position. This
	// child is always shown and always at the origin, so the margin above the first row never
	// shifts the origin and the layout can start again from the scroll's position.
	ui.settingsOrigin = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")

	ui.settingsNotice = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.settingsNotice.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.settingsRows = append(ui.settingsRows, &settingsRow{
		height:  fixedHeight(28),
		visible: func() bool { return ui.settingsNotice.Label() != "" },
		place:   func(x, y, width int) { ui.settingsNotice.Resize(x+4, y, width-8, 20) },
		show:    func(visible bool) { setShown(ui.settingsNotice, visible) },
	})

	ui.settingsToggles = []settingsToggle{
		{
			title: func(text *uitext.Strings) string { return text.AutoplayNext }, hint: func(text *uitext.Strings) string { return text.AutoplayNextHint },
			on: func() bool { return settingOn(ui.settings.AutoplayNext) }, change: ui.toggleAutoplayNext,
		},
		{
			title: func(text *uitext.Strings) string { return text.AutoPlay }, hint: func(text *uitext.Strings) string { return text.AutoPlayHint }, disabledHint: func(text *uitext.Strings) string { return text.AutoPlayDisabled },
			on:      func() bool { return settingOn(ui.settings.AutoPlay) },
			enabled: func() bool { return settingOn(ui.settings.AutoplayNext) },
			change:  ui.toggleAutoPlay,
		},
		{
			title: func(text *uitext.Strings) string { return text.Resume }, hint: func(text *uitext.Strings) string { return text.ResumeHint },
			on: func() bool { return settingOn(ui.settings.ResumePlayback) }, change: ui.toggleResumePlayback,
		},
		{
			title: func(text *uitext.Strings) string { return text.HardwareDecoding }, hint: func(text *uitext.Strings) string { return text.HardwareDecodingHint },
			on: func() bool { return settingOn(ui.settings.HardwareDecoding) }, change: ui.toggleHardwareDecoding,
		},
	}
	const (
		autoplayNextToggle = iota
		autoPlayToggle
		resumeToggle
		hardwareToggle
	)

	ui.addHeading(func(text *uitext.Strings) string { return text.HeadingAppearance })
	ui.themeControl = &segmentedControl{
		title: func(text *uitext.Strings) string { return text.Theme }, hint: func(text *uitext.Strings) string { return text.ThemeHint }, options: themeSegments, minSegmentW: 92,
		selected: ui.selectedThemeSegment, choose: ui.chooseTheme,
	}
	ui.addSegmentedRow(ui.themeControl)
	ui.languageControl = &segmentedControl{
		title: func(text *uitext.Strings) string { return text.Language }, hint: func(text *uitext.Strings) string { return text.LanguageHint }, options: languageSegments(), minSegmentW: 92,
		selected: func() int { return max(0, indexOfOption(ui.languageControl.options, ui.settings.Language)) },
		choose:   ui.chooseLanguage,
	}
	ui.addSegmentedRow(ui.languageControl)

	ui.addHeading(func(text *uitext.Strings) string { return text.HeadingPlayback })
	ui.addToggleRow(&ui.settingsToggles[autoplayNextToggle])
	ui.addToggleRow(&ui.settingsToggles[autoPlayToggle])
	ui.addToggleRow(&ui.settingsToggles[resumeToggle])
	ui.addToggleRow(&ui.settingsToggles[hardwareToggle])

	ui.addHeading(func(text *uitext.Strings) string { return text.HeadingNetwork })
	ui.proxyControl = &segmentedControl{
		title: func(text *uitext.Strings) string { return text.Proxy }, hint: func(text *uitext.Strings) string { return text.ProxyHint }, options: proxySegments, minSegmentW: 84,
		selected: func() int { return indexOfOption(proxySegments, ui.settings.ProxyMode) },
		choose:   ui.chooseProxyMode,
	}
	ui.addSegmentedRow(ui.proxyControl)
	ui.addProxyAddressRow()
	ui.proxyTest = &settingsAction{
		title: func(text *uitext.Strings) string { return text.TestConnection }, face: func(text *uitext.Strings) string { return text.TestConnection },
		status:  func() string { return ui.tasks.proxyStatus.String() },
		tone:    func() statusTone { return ui.tasks.proxyTone },
		enabled: func() bool { return !ui.settingsLoadFailed },
		busy:    func() bool { return ui.tasks.proxyTesting },
		run:     ui.testProxy,
	}
	ui.addActionRow(ui.proxyTest)

	ui.addHeading(func(text *uitext.Strings) string { return text.HeadingData })
	ui.addInfoRow(&settingsInfo{title: func(text *uitext.Strings) string { return text.DataFolder }, detail: ui.dataFolderText})
	ui.dataChange = &settingsAction{
		title: func(text *uitext.Strings) string { return text.ChangeLocation }, face: func(text *uitext.Strings) string { return text.ChangeLocationButton },
		status: func() string { return ui.tasks.dataStatus.String() },
		busy:   func() bool { return ui.tasks.dataMoving },
		run:    func() { ui.changeDataLocation(false) },
	}
	ui.addActionRow(ui.dataChange)
	ui.dataRestore = &settingsAction{
		title: func(text *uitext.Strings) string { return text.RestoreDefault }, face: func(text *uitext.Strings) string { return text.RestoreDefault },
		status:  func() string { return uitext.T().RestoreDefaultHint },
		enabled: func() bool { return ui.tasks.dataKnown && ui.tasks.data.Custom },
		busy:    func() bool { return ui.tasks.dataMoving },
		run:     func() { ui.changeDataLocation(true) },
	}
	ui.addActionRow(ui.dataRestore)

	ui.addHeading(func(text *uitext.Strings) string { return text.HeadingAbout })
	ui.addInfoRow(&settingsInfo{title: func(text *uitext.Strings) string { return text.CurrentVersion }, detail: ui.versionText})
	ui.updateCheck = &settingsAction{
		title: func(text *uitext.Strings) string { return text.CheckUpdate }, face: func(text *uitext.Strings) string { return text.CheckUpdate },
		status: func() string { return ui.tasks.updateStatus.String() },
		busy:   func() bool { return ui.tasks.updateChecking },
		run:    ui.checkForUpdate,
	}
	ui.addActionRow(ui.updateCheck)
	ui.updateOpen = &settingsAction{
		title: func(text *uitext.Strings) string { return text.DownloadNew }, face: func(text *uitext.Strings) string { return text.OpenDownloadPage },
		status:  func() string { return uitext.T().DownloadPageHint },
		visible: func() bool { return ui.tasks.update != nil && ui.tasks.update.Newer && ui.tasks.update.URL != "" },
		run:     ui.openUpdatePage,
	}
	ui.addActionRow(ui.updateOpen)
	ui.projectOpen = &settingsAction{
		title: func(text *uitext.Strings) string { return text.ProjectPage }, face: func(text *uitext.Strings) string { return text.OpenProjectPage },
		status: func() string { return ui.tasks.projectStatus.String() },
		run:    ui.openProjectPage,
	}
	ui.addActionRow(ui.projectOpen)

	// The scroll ends where its last child does; this child keeps a margin below the last row.
	ui.settingsEnd = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.settingsRows = append(ui.settingsRows, &settingsRow{
		height: fixedHeight(0),
		place:  func(x, y, width int) { ui.settingsEnd.Resize(x, y, 1, settingsMarginBottom) },
		show:   func(visible bool) { setShown(ui.settingsEnd, visible) },
	})
	ui.settingsScroll.End()
	page.End()
	page.Hide()
	ui.settingsPage = page
	ui.layoutSettings()
}

// addHeading adds a section heading above the rows it names.
func (ui *view) addHeading(title localText) {
	heading := fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.labelled(heading, title)
	heading.SetLabelFont(fltk.HELVETICA_BOLD)
	heading.SetLabelSize(16)
	heading.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.settingsHeadings = append(ui.settingsHeadings, heading)
	ui.settingsRows = append(ui.settingsRows, &settingsRow{
		height: fixedHeight(settingsHeadingHeight),
		place:  func(x, y, width int) { heading.Resize(x+4, y+10, width-8, 28) },
		show:   func(visible bool) { setShown(heading, visible) },
	})
}

// settingOn reads a toggle of the resolved settings the backend returns.
func settingOn(value string) bool { return value == "enabled" }

func toggleName(on bool) string {
	if on {
		return "enabled"
	}
	return "disabled"
}

// resolveDark is the theme to show at startup for a resolved appearance.
func resolveDark(appearance string) bool {
	switch appearance {
	case "dark":
		return true
	case "system":
		return systemUsesDarkTheme()
	}
	return false
}

// changeSettings applies change to the settings the UI holds, saves all of them and shows
// the new state. Saving the whole value keeps a theme save and another toggle from
// overwriting each other. It does nothing while the startup load failed: the UI then holds
// defaults, and saving them would overwrite the stored choices. It reports whether the change
// was made.
func (ui *view) changeSettings(change func(*backend.Settings)) bool {
	if ui.closing || ui.settingsLoadFailed {
		return false
	}
	if ui.tasks.dataMoving {
		// Writes are refused while the data moves; show the saved values again.
		ui.syncSettings()
		return false
	}
	change(&ui.settings)
	if ui.settingsWriter != nil {
		ui.settingsWriter.save(ui.settings)
	}
	ui.syncSettings()
	return true
}

// settingsSaveFailed and settingsSaved run on the writer's goroutine; the message changes on
// the UI thread.
func (ui *view) settingsSaveFailed(error) {
	fltk.Awake(func() {
		if ui.ctx.Err() == nil && !ui.closing {
			ui.setMessage(func() string { return uitext.T().SettingsSaveFailed })
			ui.window.Redraw()
		}
	})
}

// settingsSaved clears the save-failure message once a later save worked, and no other message.
func (ui *view) settingsSaved() {
	fltk.Awake(func() {
		if ui.ctx.Err() == nil && !ui.closing && ui.message.Label() == uitext.T().SettingsSaveFailed {
			ui.setMessage(nil)
			ui.window.Redraw()
		}
	})
}

// selectedThemeSegment is the segment of the stored appearance; an unknown value shows as the
// light theme, which is what resolveDark makes of it.
func (ui *view) selectedThemeSegment() int {
	if index := indexOfOption(themeSegments, ui.settings.Appearance); index >= 0 {
		return index
	}
	return indexOfOption(themeSegments, "light")
}

// chooseTheme saves and applies one segment of 主題. 跟隨系統 reads the Windows theme now.
func (ui *view) chooseTheme(value string) {
	ui.changeSettings(func(settings *backend.Settings) {
		settings.Appearance = value
		ui.dark = resolveDark(value)
	})
	ui.applyTheme()
}

// chooseLanguage saves the UI language and switches every screen to it at once.
func (ui *view) chooseLanguage(value string) {
	ui.changeSettings(func(settings *backend.Settings) { settings.Language = value })
	ui.useLanguage(ui.settings.Language)
}

func (ui *view) chooseProxyMode(value string) {
	ui.changeSettings(func(settings *backend.Settings) { settings.ProxyMode = value })
}

func (ui *view) toggleAutoplayNext() {
	ui.changeSettings(func(settings *backend.Settings) {
		settings.AutoplayNext = toggleName(!settingOn(settings.AutoplayNext))
	})
}

func (ui *view) toggleAutoPlay() {
	ui.changeSettings(func(settings *backend.Settings) { settings.AutoPlay = toggleName(!settingOn(settings.AutoPlay)) })
}

func (ui *view) toggleResumePlayback() {
	ui.changeSettings(func(settings *backend.Settings) {
		settings.ResumePlayback = toggleName(!settingOn(settings.ResumePlayback))
	})
}

// toggleHardwareDecoding says when the change applies: mpv reads it only as the player opens,
// and without the notice users expected to restart the app.
func (ui *view) toggleHardwareDecoding() {
	if ui.changeSettings(func(settings *backend.Settings) {
		settings.HardwareDecoding = toggleName(!settingOn(settings.HardwareDecoding))
	}) {
		ui.tasks.notice(uitext.T().HardwareDecoding, uitext.T().HardwareDecodingNotice)
	}
}

// syncSettings enables each row by its dependency, lays the rows out again (the proxy address
// row comes and goes) and redraws them. While the startup load failed the rows that save a
// setting are disabled and a line says why; the rows that only act (updates, data folder) stay.
func (ui *view) syncSettings() {
	notice := ""
	if ui.settingsLoadFailed {
		notice = uitext.T().SettingsLoadFailed
	}
	ui.settingsNotice.SetLabel(notice)
	for _, row := range ui.settingsRows {
		if row.sync != nil {
			row.sync()
		}
	}
	ui.layoutSettings()
}

// setSettingEnabled activates or deactivates a widget of row. A focused widget that gets
// disabled hands focus to the row before it, so keyboard users keep their place.
func (ui *view) setSettingEnabled(widget interface {
	Activate()
	Deactivate()
	IsActive() bool
	HasFocus() bool
}, row *settingsRow, enabled bool) {
	if enabled {
		widget.Activate()
		return
	}
	if !widget.IsActive() {
		return
	}
	hadFocus := widget.HasFocus()
	widget.Deactivate()
	if hadFocus {
		ui.focusAfterKey(func() { ui.moveSettingsFocus(row, -1) })
	}
}

func (ui *view) resizeSettings() {
	if ui.settingsPage == nil {
		return
	}
	width, height := max(400, ui.window.W()-259), max(250, ui.window.H()-131)
	ui.settingsPage.Resize(236, 110, width, height)
	ui.settingsScroll.Resize(236, 110, width, height)
	ui.layoutSettings()
}

// layoutSettings places the visible rows one under the other from the scroll's current
// position, then pulls the scroll back when the content became shorter than it.
func (ui *view) layoutSettings() {
	scroll := ui.settingsScroll
	if scroll == nil {
		return
	}
	originX, originY := scroll.X()-scroll.XPosition(), scroll.Y()-scroll.YPosition()
	ui.settingsOrigin.Resize(originX, originY, 1, 1)
	width := max(400, scroll.W()-2*settingsMarginX-settingsScrollbar)
	y := originY + settingsMarginTop
	for _, row := range ui.settingsRows {
		shown := row.isShown()
		row.place(originX+settingsMarginX, y, width)
		row.show(shown)
		if shown {
			y += row.height(width)
		}
	}
	if limit := max(0, y-originY-scroll.H()); scroll.YPosition() > limit {
		scroll.ScrollTo(scroll.XPosition(), limit)
	}
	scroll.Redraw()
}

// firstSettingsStop is the index of the first row that can take focus.
func (ui *view) firstSettingsStop() int {
	return slices.IndexFunc(ui.settingsRows, func(row *settingsRow) bool { return row.target != nil && row.isShown() })
}

// focusSettings focuses the first row that is enabled.
func (ui *view) focusSettings() {
	ui.moveFocusFrom(-1, 1)
}

// moveSettingsFocus focuses the nearest enabled row from row in the direction of step.
// Past the last row focus stays; past the first it goes to the navigation.
func (ui *view) moveSettingsFocus(row *settingsRow, step int) {
	ui.moveFocusFrom(slices.Index(ui.settingsRows, row), step)
}

func (ui *view) moveFocusFrom(index, step int) {
	for next := index + step; next >= 0 && next < len(ui.settingsRows); next += step {
		row := ui.settingsRows[next]
		if row.target == nil || !row.isShown() {
			continue
		}
		target := row.target()
		if target == nil || !target.IsActive() {
			continue
		}
		if next == ui.firstSettingsStop() {
			// Nothing but headings above the first row: show them too.
			ui.settingsScroll.ScrollTo(ui.settingsScroll.XPosition(), 0)
		}
		scrollIntoView(ui.settingsScroll, paddedBounds{target})
		target.TakeFocus()
		return
	}
	if step < 0 {
		ui.focusNavigation()
	}
}

// paddedBounds is a focus target with room around it, so the card it sits on shows as well.
type paddedBounds struct{ focusTarget }

const settingsFocusPad = 12

func (bounds paddedBounds) Y() int { return bounds.focusTarget.Y() - settingsFocusPad }
func (bounds paddedBounds) H() int { return bounds.focusTarget.H() + 2*settingsFocusPad }

// settingsKeys is the key handler of a focusable control of row. Up, Down and Tab move
// between rows and Enter and Space run activate (nil: they do nothing). Left and Right go to
// horizontal first; when it does not take Left, focus goes to the navigation, and Right stays
// because the rows are the right edge.
func (ui *view) settingsKeys(row *settingsRow, activate func(), horizontal func(keyCommand) bool) func(fltk.Event) bool {
	run := func() {}
	if activate != nil {
		run = ui.deferredKeyAction(activate)
	}
	return func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch command := classifyKey(fltk.EventKey(), fltk.EventState()); command {
		case keyActivate:
			run()
		case keyUp, keyPrevious:
			ui.focusAfterKey(func() { ui.moveSettingsFocus(row, -1) })
		case keyDown, keyNext:
			ui.focusAfterKey(func() { ui.moveSettingsFocus(row, 1) })
		case keyLeft, keyRight:
			if horizontal != nil && horizontal(command) {
				return true
			}
			if command == keyLeft {
				ui.focusAfterKey(ui.focusNavigation)
			}
		default:
			return false
		}
		return true
	}
}

// addToggleRow builds a title and hint with a switch at its end.
func (ui *view) addToggleRow(toggle *settingsToggle) {
	row := &settingsRow{}
	toggle.button = fltk.NewButton(0, 0, 1, 1, "")
	ui.labelled(toggle.button, toggle.title)
	ui.styleSettingsToggle(toggle)
	toggle.button.SetCallback(toggle.change)
	toggle.button.SetEventHandler(ui.settingsKeys(row, toggle.change, nil))
	row.height = fixedHeight(settingsRowHeight + settingsRowGap)
	row.place = func(x, y, width int) { toggle.button.Resize(x, y, width, settingsRowHeight) }
	row.show = func(visible bool) { setShown(toggle.button, visible) }
	row.target = func() focusTarget { return toggle.button }
	row.sync = func() {
		ui.setSettingEnabled(toggle.button, row, !ui.settingsLoadFailed && (toggle.enabled == nil || toggle.enabled()))
		toggle.button.Redraw()
	}
	ui.settingsRows = append(ui.settingsRows, row)
}

func (ui *view) styleSettingsToggle(toggle *settingsToggle) {
	button := toggle.button
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		drawCard(colors, x, y, width, height, 13, button.HasFocus())

		active, on := button.IsActive(), toggle.on()
		title, hint := colors.ink, toggle.hint
		if !active {
			title, hint = colors.muted, toggle.disabledHint
		}
		drawRowText(x, y, width, height, width-36-settingsSwitchW-12, toggle.title.String(), hint.String(), title, colors.muted, false)

		track, knob := colors.muted, colors.surface
		switch {
		case !active:
			track, knob = colors.soft, colors.muted
		case on:
			track = colors.accent
		}
		trackX, trackY := x+width-18-settingsSwitchW, y+(height-settingsSwitchH)/2
		roundedRect(trackX, trackY, settingsSwitchW, settingsSwitchH, settingsSwitchH/2, track)
		knobX := trackX + 2
		if on {
			knobX = trackX + settingsSwitchW - settingsSwitchH + 2
		}
		fltk.SetDrawColor(knob)
		fltk.DrawPie(knobX, trackY+2, settingsSwitchH-4, settingsSwitchH-4, 0, 360)
	})
}

// drawRowText writes a row's title and, under it, its hint, clipped to textWidth.
func drawRowText(x, y, width, height, textWidth int, title, hint string, titleColor, hintColor fltk.Color, hintBold bool) {
	fltk.PushClip(x+3, y+3, width-6, height-6)
	fltk.SetDrawColor(titleColor)
	fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
	fltk.Draw(title, x+18, y+6, textWidth, 24, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
	fltk.SetDrawColor(hintColor)
	font := fltk.HELVETICA
	if hintBold {
		font = fltk.HELVETICA_BOLD
	}
	fltk.SetDrawFont(font, 12)
	fltk.Draw(hint, x+18, y+30, textWidth, 20, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
	fltk.PopClip()
}
