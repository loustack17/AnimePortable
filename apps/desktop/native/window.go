// SPDX-License-Identifier: MPL-2.0

package native

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var destinations = [...]string{"首頁", "時間表", "追蹤", "歷史紀錄", "搜尋", "設定"}

type navigationButton struct {
	widget.Button
	index  int
	canvas fyne.Canvas
	group  *[len(destinations)]*navigationButton
	toMain func()
}

func (button *navigationButton) Tapped(event *fyne.PointEvent) {
	button.Button.Tapped(event)
	button.canvas.Focus(button)
}

func (button *navigationButton) TypedKey(event *fyne.KeyEvent) {
	switch event.Name {
	case fyne.KeyDown, fyne.KeyRight:
		if event.Name == fyne.KeyRight {
			button.toMain()
			return
		}
		button.canvas.Focus(button.group[(button.index+1)%len(button.group)])
	case fyne.KeyUp, fyne.KeyLeft:
		button.canvas.Focus(button.group[(button.index+len(button.group)-1)%len(button.group)])
	case fyne.KeyEnter, fyne.KeyReturn, fyne.KeySpace:
		button.OnTapped()
	default:
		button.Button.TypedKey(event)
	}
}

type scrollButton struct {
	widget.Button
	canvas fyne.Canvas
	scroll *container.Scroll
	toMenu func()
}

func (button *scrollButton) Tapped(event *fyne.PointEvent) {
	button.Button.Tapped(event)
	button.canvas.Focus(button)
}

func (button *scrollButton) TypedKey(event *fyne.KeyEvent) {
	if scrollKey(button.scroll, event.Name) {
		return
	}
	if event.Name == fyne.KeyLeft {
		button.toMenu()
		return
	}
	button.Button.TypedKey(event)
}

func scrollKey(scroll *container.Scroll, key fyne.KeyName) bool {
	switch key {
	case fyne.KeyPageDown:
		scroll.ScrollToOffset(fyne.NewPos(0, scroll.Offset.Y+scroll.Size().Height))
	case fyne.KeyPageUp:
		scroll.ScrollToOffset(fyne.NewPos(0, scroll.Offset.Y-scroll.Size().Height))
	case fyne.KeyHome:
		scroll.ScrollToTop()
	case fyne.KeyEnd:
		scroll.ScrollToBottom()
	default:
		return false
	}
	return true
}

func NewWindow(application fyne.App, service HomeService) fyne.Window {
	window := application.NewWindow("AnimePortable")
	closeHome := ConfigureWindow(window, service)
	window.SetOnClosed(closeHome)
	return window
}

func ConfigureWindow(window fyne.Window, service HomeService) func() {
	window.Resize(fyne.NewSize(1000, 618))
	window.SetPadded(false)
	canvas := window.Canvas()
	content := container.NewVScroll(widget.NewLabel(""))
	var navigation [len(destinations)]*navigationButton
	control := &scrollButton{canvas: canvas, scroll: content, toMenu: func() { canvas.Focus(navigation[0]) }}
	control.Button.Text = "瀏覽內容（PageUp / PageDown）"
	control.Button.OnTapped = func() { scrollKey(content, fyne.KeyPageDown) }
	control.ExtendBaseWidget(control)
	var home *homeView
	show := func(index int) {
		if home != nil {
			home.close()
			home = nil
		}
		for item, button := range navigation {
			button.Importance = widget.MediumImportance
			if item == index {
				button.Importance = widget.HighImportance
			}
			button.Refresh()
		}
		content.ScrollToTop()
		if index == 0 {
			home = newHomeView(service, canvas, content, navigation[0], control)
			content.Content = home.content
		} else {
			content.Content = container.NewPadded(container.NewVBox(
				widget.NewLabelWithStyle(destinations[index], fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewCard(destinations[index], "", widget.NewLabel("此頁面尚未建立")),
			))
		}
		content.Refresh()
	}
	icons := [...]fyne.Resource{theme.HomeIcon(), theme.CalendarIcon(), theme.ConfirmIcon(), theme.HistoryIcon(), theme.SearchIcon(), theme.SettingsIcon()}
	items := make([]fyne.CanvasObject, 0, len(destinations)+2)
	items = append(items, widget.NewLabelWithStyle("AnimePortable", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator())
	for index, label := range destinations {
		current := index
		button := &navigationButton{index: index, canvas: canvas, group: &navigation}
		button.Button.Text = label
		button.Button.Icon = icons[index]
		button.Button.Alignment = widget.ButtonAlignLeading
		button.Button.OnTapped = func() { show(current) }
		button.toMain = func() {
			if home != nil && home.firstPlay != nil {
				canvas.Focus(home.firstPlay)
			} else {
				canvas.Focus(control)
			}
		}
		button.ExtendBaseWidget(button)
		navigation[index] = button
		items = append(items, button)
	}
	window.SetContent(container.NewBorder(nil, nil,
		container.NewPadded(container.NewVBox(items...)), nil,
		container.NewBorder(container.NewPadded(control), nil, nil, nil, content),
	))
	show(0)
	canvas.Focus(navigation[0])
	return func() {
		if home != nil {
			home.close()
		}
	}
}
