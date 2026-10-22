//go:build windows

package fltkhome

import fltk "github.com/pwiecz/go-fltk"

type palette struct {
	background fltk.Color
	surface    fltk.Color
	ink        fltk.Color
	muted      fltk.Color
	accent     fltk.Color
	soft       fltk.Color
	// onAccent is the text colour on an accent-filled button.
	onAccent fltk.Color
	// success and danger colour a result's text on surface; the words stay, so colour is never
	// the only signal. Both reach 4.5:1 against surface.
	success fltk.Color
	danger  fltk.Color
}

// statusTone is how a status line reads: neutral progress or hint, a success, or a failure.
type statusTone uint8

const (
	toneNeutral statusTone = iota
	toneSuccess
	toneFailure
)

// toneColor is the text colour of a status line of tone on a card.
func toneColor(colors palette, tone statusTone) fltk.Color {
	switch tone {
	case toneSuccess:
		return colors.success
	case toneFailure:
		return colors.danger
	}
	return colors.muted
}

type themedLabel struct {
	widget *fltk.Box
	muted  bool
}

func (ui *view) contentText(x, y, width, height int, value string, size int, muted, bold bool) *fltk.Box {
	box := fltk.NewBox(fltk.NO_BOX, x, y, width, height, value)
	box.SetLabelSize(size)
	box.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	if bold {
		box.SetLabelFont(fltk.HELVETICA_BOLD)
	}
	ui.contentLabels = append(ui.contentLabels, themedLabel{widget: box, muted: muted})
	return box
}

var lightPalette = palette{
	background: fltk.ColorFromRgb(247, 249, 253),
	surface:    fltk.ColorFromRgb(255, 255, 255),
	ink:        fltk.ColorFromRgb(28, 37, 57),
	muted:      fltk.ColorFromRgb(105, 116, 136),
	accent:     fltk.ColorFromRgb(80, 91, 224),
	soft:       fltk.ColorFromRgb(235, 238, 255),
	onAccent:   fltk.ColorFromRgb(255, 255, 255),
	success:    fltk.ColorFromRgb(21, 128, 61),
	danger:     fltk.ColorFromRgb(185, 28, 28),
}

var darkPalette = palette{
	background: fltk.ColorFromRgb(17, 22, 35),
	surface:    fltk.ColorFromRgb(29, 36, 54),
	ink:        fltk.ColorFromRgb(237, 241, 252),
	muted:      fltk.ColorFromRgb(166, 176, 199),
	accent:     fltk.ColorFromRgb(160, 170, 255),
	soft:       fltk.ColorFromRgb(48, 57, 87),
	onAccent:   fltk.ColorFromRgb(17, 22, 35),
	success:    fltk.ColorFromRgb(74, 222, 128),
	danger:     fltk.ColorFromRgb(252, 129, 129),
}

func roundedRect(x, y, width, height, radius int, color fltk.Color) {
	fltk.SetDrawColor(color)
	fltk.DrawRectf(x+radius, y, width-2*radius, height)
	fltk.DrawRectf(x, y+radius, radius, height-2*radius)
	fltk.DrawRectf(x+width-radius, y+radius, radius, height-2*radius)
	fltk.DrawPie(x, y, radius*2, radius*2, 90, 180)
	fltk.DrawPie(x+width-radius*2, y, radius*2, radius*2, 0, 90)
	fltk.DrawPie(x, y+height-radius*2, radius*2, radius*2, 180, 270)
	fltk.DrawPie(x+width-radius*2, y+height-radius*2, radius*2, radius*2, 270, 360)
}

func (ui *view) colors() palette {
	if ui.dark {
		return darkPalette
	}
	return lightPalette
}

func (ui *view) styleButton(button *fltk.Button, selected func() bool) {
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		background, foreground := colors.surface, colors.muted
		if selected != nil && selected() {
			background, foreground = colors.soft, colors.accent
		}
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		if button.HasFocus() {
			roundedRect(x, y, width, height, 13, colors.accent)
		} else {
			roundedRect(x, y, width, height, 13, background)
		}
		roundedRect(x+3, y+3, width-6, height-6, 11, background)
		fltk.SetDrawColor(foreground)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
		// Clip so a label longer than the button cannot paint over its neighbours.
		fltk.PushClip(x+3, y+3, width-6, height-6)
		fltk.Draw(button.Label(), x+18, y+3, width-26, height-6, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE)
		fltk.PopClip()
	})
}

func (ui *view) applyTheme() {
	shownDark = ui.dark
	colors := ui.colors()
	if ui.dark {
		fltk.SetBackgroundColor(17, 22, 35)
		fltk.SetBackground2Color(29, 36, 54)
		fltk.SetForegroundColor(237, 241, 252)
	} else {
		fltk.SetBackgroundColor(247, 249, 253)
		fltk.SetBackground2Color(255, 255, 255)
		fltk.SetForegroundColor(28, 37, 57)
	}
	ui.window.SetColor(colors.background)
	ui.header.SetLabelColor(colors.ink)
	ui.tagline.SetLabelColor(colors.muted)
	ui.message.SetLabelColor(colors.muted)
	ui.sectionTitle.SetLabelColor(colors.ink)
	ui.contentHint.SetLabelColor(colors.muted)
	ui.scroll.SetColor(colors.background)
	ui.loadingText.SetLabelColor(colors.muted)
	for _, label := range ui.contentLabels {
		if label.muted {
			label.widget.SetLabelColor(colors.muted)
		} else {
			label.widget.SetLabelColor(colors.ink)
		}
	}
	for _, card := range ui.cards {
		card.Redraw()
	}
	for _, button := range ui.homeFocusables() {
		button.Redraw()
	}
	for _, button := range ui.navigation {
		button.Redraw()
	}
	ui.retryButton.Redraw()
	if ui.settingsScroll != nil {
		ui.settingsScroll.SetColor(colors.background)
		ui.settingsNotice.SetLabelColor(colors.muted)
		for _, heading := range ui.settingsHeadings {
			heading.SetLabelColor(colors.ink)
		}
		ui.proxyField.input.SetColor(colors.background)
		ui.proxyField.input.SetLabelColor(colors.ink)
		ui.settingsScroll.Redraw()
	}
	if ui.searchInput != nil {
		ui.searchInput.SetColor(colors.surface)
		ui.searchInput.SetLabelColor(colors.ink)
		ui.searchScroll.SetColor(colors.background)
		ui.searchStatus.SetLabelColor(colors.muted)
		ui.searchPageText.SetLabelColor(colors.muted)
		ui.previewTitle.SetLabelColor(colors.ink)
		ui.previewDescription.SetLabelColor(colors.ink)
		ui.previewMeta.SetLabelColor(colors.muted)
		ui.previewStatus.SetLabelColor(colors.muted)
		ui.previewCover.SetLabelColor(colors.muted)
		ui.searchInput.Redraw()
		ui.searchScroll.Redraw()
		ui.searchStatus.Redraw()
		ui.searchPageText.Redraw()
		ui.previewTitle.Redraw()
		ui.previewDescription.Redraw()
		ui.previewMeta.Redraw()
		ui.previewStatus.Redraw()
		ui.previewCover.Redraw()
		ui.previewEpisodes.Redraw()
		ui.previewEpisodeText.SetLabelColor(colors.muted)
		ui.previewEpisodeText.Redraw()
		for _, button := range append(append(append([]*fltk.Button(nil), ui.searchRows...), ui.previewEpisodeRows...), ui.searchButton, ui.searchBack, ui.searchPrevious, ui.searchNext, ui.previewBack, ui.previewContinue, ui.previewRetry, ui.previewFollow, ui.previewPrevious, ui.previewNext) {
			button.Redraw()
		}
	}
	if ui.followingPage != nil {
		ui.followingStatus.SetLabelColor(colors.muted)
		ui.followingScroll.SetColor(colors.background)
		ui.followingStatus.Redraw()
		ui.followingScroll.Redraw()
		ui.followingRefresh.Redraw()
		for _, button := range ui.followingButtons {
			button.Redraw()
		}
	}
	if ui.schedulePage != nil {
		ui.scheduleStatus.SetLabelColor(colors.muted)
		ui.scheduleScroll.SetColor(colors.background)
		for day, header := range ui.scheduleHeaders {
			if day < len(ui.scheduleColumns) && ui.scheduleColumns[day].Today {
				header.SetLabelColor(colors.accent)
			} else {
				header.SetLabelColor(colors.ink)
			}
		}
		for _, cards := range ui.scheduleCards {
			for _, card := range cards {
				card.Redraw()
			}
		}
		ui.scheduleStatus.Redraw()
		ui.scheduleScroll.Redraw()
		ui.scheduleRefresh.Redraw()
	}
	ui.window.Redraw()
}
