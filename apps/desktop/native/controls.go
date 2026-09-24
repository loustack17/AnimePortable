// SPDX-License-Identifier: MPL-2.0

package native

import (
	"animeportable/apps/desktop/backend"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type playButton struct {
	widget.Button
	home *homeView
	item backend.History
}

func newPlayButton(home *homeView, item backend.History) *playButton {
	button := &playButton{home: home, item: item}
	button.Button.OnTapped = func() { home.play(item) }
	button.ExtendBaseWidget(button)
	if home.playPending {
		button.Disable()
	}
	return button
}

func (button *playButton) Tapped(event *fyne.PointEvent) {
	button.Button.Tapped(event)
	button.home.canvas.Focus(button)
}

func (button *playButton) FocusGained() {
	button.Button.FocusGained()
	scroll := button.home.scroll
	driver := fyne.CurrentApp().Driver()
	viewTop := driver.AbsolutePositionForObject(scroll).Y
	viewBottom := viewTop + scroll.Size().Height
	actionTop := driver.AbsolutePositionForObject(button).Y
	actionBottom := actionTop + button.Size().Height
	offset := scroll.Offset.Y
	if actionTop < viewTop {
		offset += actionTop - viewTop
	} else if actionBottom > viewBottom {
		offset += actionBottom - viewBottom
	}
	scroll.ScrollToOffset(fyne.NewPos(0, offset))
}

func (button *playButton) TypedKey(event *fyne.KeyEvent) {
	index := -1
	for item, candidate := range button.home.playButtons {
		if candidate == button {
			index = item
			break
		}
	}
	switch event.Name {
	case fyne.KeyUp, fyne.KeyDown:
		if index < 0 || len(button.home.playButtons) == 0 {
			return
		}
		direction := 1
		if event.Name == fyne.KeyUp {
			direction = -1
		}
		next := (index + direction + len(button.home.playButtons)) % len(button.home.playButtons)
		button.home.canvas.Focus(button.home.playButtons[next])
	case fyne.KeyLeft:
		button.home.canvas.Focus(button.home.menu)
	case fyne.KeyEnter, fyne.KeyReturn, fyne.KeySpace:
		if !button.Disabled() {
			button.OnTapped()
		}
	default:
		if scrollKey(button.home.scroll, event.Name) {
			button.home.canvas.Focus(button.home.control)
		} else {
			button.Button.TypedKey(event)
		}
	}
}

var _ fyne.Focusable = (*playButton)(nil)
var _ fyne.Focusable = (*navigationButton)(nil)
var _ fyne.Focusable = (*scrollButton)(nil)
