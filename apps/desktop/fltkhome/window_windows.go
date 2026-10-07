//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"fmt"
	"sync"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

type Service interface {
	appearanceService
	Start(context.Context) error
	Library(context.Context) ([]backend.Anime, error)
	Search(context.Context, string) ([]backend.Anime, error)
	Detail(context.Context, string) (backend.Detail, error)
	Episodes(context.Context, string) ([]backend.Episode, error)
	History(context.Context) ([]backend.History, error)
	Following(context.Context) ([]backend.Following, error)
}

type view struct {
	window        *fltk.Window
	service       Service
	plan          backend.PortablePlan
	ctx           context.Context
	cancelContext context.CancelFunc
	onPlay        func(backend.PlayRequest)
	onClose       func()

	message            *fltk.Box
	header             *fltk.Box
	tagline            *fltk.Box
	sectionTitle       *fltk.Box
	contentHint        *fltk.Box
	scroll             *fltk.Scroll
	scrollOriginX      int
	scrollOriginY      int
	loadingText        *fltk.Box
	retryButton        *fltk.Button
	browseButton       *fltk.Button
	contentLabels      []themedLabel
	staticLabels       int
	cards              []*fltk.Box
	cardButtons        []*fltk.Button
	startupGroup       *fltk.Group
	mainGroup          *fltk.Group
	navigation         []*fltk.Button
	selected           int
	rows               []homeRow
	loadFailed         bool
	searchModel        searchModel
	searchPage         *fltk.Group
	searchInput        *fltk.Input
	searchButton       *fltk.Button
	searchBack         *fltk.Button
	searchScroll       *fltk.Scroll
	searchStatus       *fltk.Box
	searchPageText     *fltk.Box
	searchPrevious     *fltk.Button
	searchNext         *fltk.Button
	searchRows         []*fltk.Button
	previewGroup       *fltk.Group
	previewBack        *fltk.Button
	previewPlay        *fltk.Button
	previewTitle       *fltk.Box
	previewNative      *fltk.Box
	previewDescription *fltk.Box
	searchPreview      bool
	pageGeneration     uint64
	searchInputVersion uint64
	closeOnce          sync.Once
	closeButton        *fltk.Button
	themeToggle        *fltk.Button
	dark               bool
	appearance         *appearancePersistence
	closing            bool
}

func NewWindow(plan backend.PortablePlan, planErr error, service Service, ctx context.Context, cancel context.CancelFunc, onPlay func(backend.PlayRequest), onClose func()) (*fltk.Window, func(int), func(string)) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cancel == nil {
		cancel = func() {}
	}
	window := fltk.NewWindow(1000, 618, "AnimePortable")
	window.SetSizeRange(760, 480, 0, 0, 0, 0, false)
	ui := &view{window: window, service: service, plan: plan, ctx: ctx, cancelContext: cancel, onPlay: onPlay, onClose: onClose}
	ui.appearance = newAppearancePersistence(service, func(error) {
		fltk.Awake(func() {
			if ui.ctx.Err() == nil && !ui.closing {
				ui.message.SetLabel("無法儲存主題設定，請重試。")
				ui.window.Redraw()
			}
		})
	})
	showMessage := func(message string) { ui.message.SetLabel(message); ui.window.Redraw() }
	window.SetCallback(ui.cancel)
	ui.build()
	if planErr != nil {
		ui.showStartupError("無法使用這個資料夾，請將程式解壓到可寫入的位置後重新開啟。")
		return window, ui.showSection, showMessage
	}
	ui.start()
	return window, ui.showSection, showMessage
}

func (ui *view) build() {
	fltk.SetScheme("oxy")
	fltk.SetFont(fltk.HELVETICA, "Microsoft JhengHei UI")
	fltk.SetFont(fltk.HELVETICA_BOLD, "Microsoft JhengHei UI Bold")
	ui.window.Begin()
	header := fltk.NewBox(fltk.NO_BOX, 22, 16, 250, 42, "AnimePortable")
	header.SetLabelFont(fltk.HELVETICA_BOLD)
	header.SetLabelSize(24)
	header.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	header.SetLabelColor(fltk.ColorFromRgb(34, 42, 58))
	ui.header = header
	ui.message = fltk.NewBox(fltk.NO_BOX, 280, 20, 690, 38, "")
	ui.message.SetLabelSize(14)
	ui.message.SetLabelColor(fltk.ColorFromRgb(85, 96, 116))
	ui.themeToggle = fltk.NewButton(928, 18, 42, 40, "")
	ui.themeToggle.SetBox(fltk.NO_BOX)
	ui.themeToggle.SetTooltip("切換暗色")
	ui.themeToggle.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := ui.themeToggle.X(), ui.themeToggle.Y(), ui.themeToggle.W(), ui.themeToggle.H()
		if ui.themeToggle.HasFocus() {
			roundedRect(x, y, width, height, 13, colors.accent)
		} else {
			roundedRect(x, y, width, height, 13, colors.soft)
		}
		roundedRect(x+3, y+3, width-6, height-6, 11, colors.soft)
		fltk.SetDrawColor(colors.accent)
		centerX, centerY := x+width/2, y+height/2
		if ui.dark {
			fltk.DrawPie(centerX-6, centerY-6, 12, 12, 0, 360)
			for _, ray := range [][4]int{{0, -13, 0, -9}, {0, 9, 0, 13}, {-13, 0, -9, 0}, {9, 0, 13, 0}, {-9, -9, -7, -7}, {7, 7, 9, 9}, {-9, 9, -7, 7}, {7, -7, 9, -9}} {
				fltk.DrawLine(centerX+ray[0], centerY+ray[1], centerX+ray[2], centerY+ray[3])
			}
		} else {
			fltk.DrawPie(centerX-9, centerY-9, 18, 18, 0, 360)
			fltk.SetDrawColor(colors.soft)
			fltk.DrawPie(centerX-4, centerY-13, 18, 18, 0, 360)
		}
	})
	toggleTheme := func() {
		if ui.closing {
			return
		}
		ui.dark = !ui.dark
		if ui.appearance != nil {
			ui.appearance.setDark(ui.dark)
		}
		ui.applyTheme()
	}
	ui.bindButton(ui.themeToggle, toggleTheme)
	ui.bindThemeKeys(toggleTheme)
	ui.themeToggle.Deactivate()
	ui.startupGroup = fltk.NewGroup(18, 78, 964, 520)
	ui.startupGroup.End()
	ui.startupGroup.Hide()
	ui.mainGroup = fltk.NewGroup(0, 0, 1000, 618)
	ui.mainGroup.Begin()
	ui.sectionTitle = fltk.NewBox(fltk.NO_BOX, 252, 35, 580, 38, "今天想看什麼？")
	ui.sectionTitle.SetLabelFont(fltk.HELVETICA_BOLD)
	ui.sectionTitle.SetLabelSize(25)
	ui.sectionTitle.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.contentHint = fltk.NewBox(fltk.NO_BOX, 252, 74, 580, 25, "輕鬆找到下一個喜歡的故事")
	ui.contentHint.SetLabelSize(12)
	ui.contentHint.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.contentHint.SetLabelColor(fltk.ColorFromRgb(85, 96, 116))
	ui.scroll = fltk.NewScroll(236, 110, 741, 487)
	ui.scroll.SetType(fltk.SCROLL_BOTH)
	ui.scroll.SetBox(fltk.FLAT_BOX)
	hero := fltk.NewBox(fltk.NO_BOX, 250, 115, 711, 139)
	ui.scrollOriginX = ui.scroll.X() - hero.X()
	ui.scrollOriginY = ui.scroll.Y() - hero.Y()
	hero.SetDrawHandler(func(func()) { roundedRect(hero.X(), hero.Y(), hero.W(), hero.H(), 20, fltk.ColorFromRgb(48, 55, 125)) })
	heroTitle := fltk.NewBox(fltk.NO_BOX, 278, 134, 600, 35, "繼續探索喜歡的故事")
	heroTitle.SetLabelFont(fltk.HELVETICA_BOLD)
	heroTitle.SetLabelSize(22)
	heroTitle.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	heroTitle.SetLabelColor(fltk.WHITE)
	heroHint := fltk.NewBox(fltk.NO_BOX, 278, 171, 600, 22, "最近觀看、追蹤與播出資訊，都在這裡。")
	heroHint.SetLabelSize(13)
	heroHint.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	heroHint.SetLabelColor(fltk.WHITE)
	browse := fltk.NewButton(278, 205, 132, 37, "搜尋作品")
	ui.browseButton = browse
	browse.SetBox(fltk.NO_BOX)
	browse.SetDrawHandler(func(func()) {
		x, y, width, height := browse.X(), browse.Y(), browse.W(), browse.H()
		if browse.HasFocus() {
			roundedRect(x, y, width, height, 13, ui.colors().accent)
		} else {
			roundedRect(x, y, width, height, 13, fltk.WHITE)
		}
		roundedRect(x+3, y+3, width-6, height-6, 11, fltk.WHITE)
		fltk.SetDrawColor(fltk.ColorFromRgb(48, 55, 125))
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 13)
		fltk.Draw(browse.Label(), x, y, width, height, fltk.ALIGN_CENTER)
	})
	browseAction := func() { ui.showSection(4); ui.focusSearchInput() }
	ui.bindButton(browse, browseAction)
	ui.bindContentKeys(browse, browseAction, nil, func() *fltk.Button {
		if ui.retryButton != nil && ui.retryButton.Visible() {
			return ui.retryButton
		}
		if len(ui.cardButtons) > 0 {
			return ui.cardButtons[0]
		}
		return ui.themeToggle
	})
	ui.contentText(253, 274, 600, 32, "繼續觀看", 20, false, true)
	ui.loadingText = fltk.NewBox(fltk.NO_BOX, 253, 326, 680, 36, "正在載入繼續觀看…")
	ui.loadingText.SetLabelSize(13)
	ui.loadingText.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.retryButton = fltk.NewButton(253, 365, 126, 36, "重新載入")
	ui.styleButton(ui.retryButton, func() bool { return true })
	ui.bindButton(ui.retryButton, ui.loadHome)
	ui.bindContentKeys(ui.retryButton, ui.loadHome, func() *fltk.Button { return ui.browseButton }, func() *fltk.Button { return ui.themeToggle })
	ui.retryButton.Hide()
	ui.scroll.End()
	ui.buildSearchView()
	ui.mainGroup.End()
	ui.mainGroup.Hide()
	navPanel := fltk.NewGroup(14, 18, 205, 582)
	navPanel.Begin()
	navBackground := fltk.NewBox(fltk.NO_BOX, 14, 18, 205, 582)
	navBackground.SetDrawHandler(func(func()) {
		roundedRect(navBackground.X(), navBackground.Y(), navBackground.W(), navBackground.H(), 23, ui.colors().surface)
	})
	header.Resize(34, 32, 170, 38)
	header.SetLabelSize(20)
	navPanel.Add(header)
	tagline := fltk.NewBox(fltk.NO_BOX, 35, 63, 160, 22, "YOUR ANIME SPACE")
	tagline.SetLabelSize(10)
	tagline.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.tagline = tagline
	ui.navigation = make([]*fltk.Button, len(sections))
	for position, index := range navigationOrder {
		button := fltk.NewButton(27, 111+position*55, 177, 43, sections[index])
		ui.bindNavigationKeys(button, index)
		button.SetLabelSize(15)
		ui.styleButton(button, func() bool { return ui.selected == index })
		ui.navigation[index] = button
	}
	navPanel.End()
	ui.window.End()
	ui.applyTheme()
	ui.staticLabels = len(ui.contentLabels)
	ui.window.SetResizeHandler(func() {
		ui.scroll.Resize(236, 110, max(400, ui.window.W()-259), max(250, ui.window.H()-131))
		ui.resizeSearch()
	})
}

func (ui *view) showStartupError(message string) {
	ui.message.SetLabel(message)
	if ui.closeButton == nil {
		ui.startupGroup.Begin()
		ui.closeButton = fltk.NewButton(272, 132, 160, 40, "關閉")
		ui.styleButton(ui.closeButton, nil)
		ui.bindButton(ui.closeButton, ui.cancel)
		ui.startupGroup.End()
	}
	ui.startupGroup.Show()
	ui.window.Redraw()
}

func (ui *view) start() {
	if ui.ctx.Err() != nil || ui.closing {
		return
	}
	ui.message.SetLabel("正在準備資料…")
	ui.startupGroup.Deactivate()
	go func() {
		var err error
		var dark bool
		var appearanceErr error
		if ui.ctx.Err() != nil {
			return
		}
		if !ui.plan.TargetExists {
			err = ui.plan.CreateFresh()
		}
		if err == nil {
			if ui.service == nil {
				err = backend.ErrUnavailable
			} else {
				err = ui.service.Start(ui.ctx)
				if err == nil {
					dark, appearanceErr = loadAppearance(ui.ctx, ui.service)
				}
			}
		}
		fltk.Awake(func() {
			ui.finishStartup(err, dark, appearanceErr)
		})
	}()
}

func (ui *view) finishStartup(err error, dark bool, appearanceErr error) {
	if ui.ctx.Err() != nil || ui.closing {
		return
	}
	if err != nil {
		ui.startupGroup.Activate()
		ui.showStartupError(startupErrorMessage(err, false, false))
		return
	}
	ui.startupGroup.Hide()
	ui.dark = dark
	ui.applyTheme()
	ui.themeToggle.Activate()
	if appearanceErr != nil {
		ui.message.SetLabel("無法載入主題設定，暫時使用亮色。")
	} else {
		ui.message.SetLabel("")
	}
	ui.mainGroup.Show()
	ui.showSection(0)
	ui.navigation[0].TakeFocus()
	ui.loadHome()
	ui.window.Redraw()
}

func (ui *view) showSection(index int) {
	if ui.closing || index < 0 || index >= len(sections) {
		return
	}
	if ui.selected != index {
		ui.pageGeneration++
	}
	if index == 4 && ui.searchPreview {
		ui.pageGeneration++
		ui.searchModel.cancel()
		ui.searchPreview = false
	}
	if ui.selected == 4 && index != 4 {
		ui.searchModel.cancel()
		ui.searchPreview = false
	}
	ui.selected = index
	for _, button := range ui.navigation {
		button.Redraw()
	}
	ui.sectionTitle.SetLabel(sections[index])
	if index == 0 {
		ui.sectionTitle.SetLabel("今天想看什麼？")
		ui.contentHint.SetLabel("輕鬆找到下一個喜歡的故事")
		ui.scroll.Show()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
	} else if index == 4 {
		ui.contentHint.SetLabel("搜尋作品名稱，先查看已儲存的作品，再按 Enter 更新搜尋。")
		ui.scroll.Hide()
		ui.searchPage.Show()
		ui.previewGroup.Hide()
		ui.loadSearchLibrary()
		ui.renderSearch()
	} else {
		ui.contentHint.SetLabel("此頁面尚未建立")
		ui.scroll.Hide()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
	}
	ui.window.Redraw()
}

func (ui *view) loadHome() {
	if ui.service == nil {
		ui.message.SetLabel(homeErrorMessage(fmt.Errorf("service unavailable")))
		ui.loadingText.SetLabel("無法載入作品資料。")
		ui.retryButton.Show()
		return
	}
	ui.retryButton.Hide()
	ui.loadingText.SetLabel("正在載入繼續觀看…")
	var library []backend.Anime
	var history []backend.History
	var following []backend.Following
	var libraryErr, historyErr, followingErr error
	var wait sync.WaitGroup
	wait.Add(3)
	go func() { defer wait.Done(); library, libraryErr = ui.service.Library(ui.ctx) }()
	go func() { defer wait.Done(); history, historyErr = ui.service.History(ui.ctx) }()
	go func() { defer wait.Done(); following, followingErr = ui.service.Following(ui.ctx) }()
	go func() {
		wait.Wait()
		fltk.Awake(func() {
			if ui.ctx.Err() != nil || ui.closing {
				return
			}
			if libraryErr != nil || historyErr != nil || followingErr != nil {
				ui.loadFailed = true
				ui.message.SetLabel(homeErrorMessage(firstError(libraryErr, historyErr, followingErr)))
				ui.loadingText.SetLabel("無法載入作品資料。")
				ui.retryButton.Show()
				return
			}
			ui.loadFailed = false
			ui.message.SetLabel("")
			ui.rows = selectHomeRows(history, library, rowLimit)
			ui.populateHome(library, following)
			ui.window.Redraw()
		})
	}()
}

func (ui *view) populateHome(library []backend.Anime, following []backend.Following) {
	for _, button := range ui.cardButtons {
		ui.scroll.Remove(button)
		button.Destroy()
	}
	for _, card := range ui.cards {
		ui.scroll.Remove(card)
		card.Destroy()
	}
	for _, label := range ui.contentLabels[ui.staticLabels:] {
		ui.scroll.Remove(label.widget)
		label.widget.Destroy()
	}
	ui.cardButtons = nil
	ui.cards = nil
	ui.contentLabels = ui.contentLabels[:ui.staticLabels]
	ui.loadingText.Show()
	ui.scroll.Begin()
	if len(ui.rows) == 0 {
		ui.loadingText.SetLabel("還沒有可繼續觀看的內容。")
	} else {
		ui.loadingText.Hide()
	}
	for index, row := range ui.rows {
		y := 318 + index*104
		card := fltk.NewBox(fltk.NO_BOX, 250, y, 711, 92)
		card.SetDrawHandler(func(func()) { roundedRect(card.X(), card.Y(), card.W(), card.H(), 16, ui.colors().surface) })
		ui.cards = append(ui.cards, card)
		ui.contentText(273, y+14, 450, 25, row.Title, 15, false, true)
		caption := fmt.Sprintf("上次播放位置 %s", formatPosition(row.History.Position))
		ui.contentText(273, y+48, 450, 20, caption, 12, true, false)
		button := fltk.NewButton(813, y+28, 126, 36, "繼續播放")
		ui.styleButton(button, func() bool { return true })
		item := row.History
		playRow := func() {
			if ui.onPlay != nil && ui.ctx.Err() == nil && !ui.closing {
				ui.onPlay(playRequest(item))
			}
		}
		ui.bindButton(button, playRow)
		ui.bindContentKeys(button, playRow, func() *fltk.Button {
			if index > 0 {
				return ui.cardButtons[index-1]
			}
			return ui.browseButton
		}, func() *fltk.Button {
			if index+1 < len(ui.cardButtons) {
				return ui.cardButtons[index+1]
			}
			return ui.themeToggle
		})
		ui.cardButtons = append(ui.cardButtons, button)
	}
	followY := 326 + len(ui.rows)*104
	if len(ui.rows) == 0 {
		followY = 385
	}
	ui.contentText(253, followY, 600, 31, "追蹤中", 20, false, true)
	followNames := followingTitles(following, library, rowLimit)
	followCount := len(followNames)
	followHeight := 70 + max(followCount-1, 0)*34
	card := fltk.NewBox(fltk.NO_BOX, 250, followY+39, 711, followHeight)
	card.SetDrawHandler(func(func()) { roundedRect(card.X(), card.Y(), card.W(), card.H(), 16, ui.colors().surface) })
	ui.cards = append(ui.cards, card)
	if followCount == 0 {
		ui.contentText(270, followY+58, 620, 25, "尚未追蹤任何作品。", 15, false, true)
	} else {
		for index, name := range followNames {
			ui.contentText(270, followY+54+index*34, 620, 26, name, 15, false, true)
		}
	}
	updateY := followY + 39 + followHeight + 22
	ui.contentText(253, updateY, 600, 31, "最近更新", 20, false, true)
	ui.contentText(270, updateY+37, 620, 22, "目前沒有本機更新資料。", 12, true, false)
	ui.contentText(253, updateY+80, 600, 31, "今日播出", 20, false, true)
	ui.contentText(270, updateY+117, 620, 22, "目前沒有本機播出資料。", 12, true, false)
	ui.scroll.End()
	ui.applyTheme()
}

func (ui *view) bindButton(button *fltk.Button, action func()) {
	button.SetCallback(action)
	activate := ui.deferredKeyAction(action)
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		key := fltk.EventKey()
		if key != fltk.ENTER_KEY && key != 13 && key != 0xff8d {
			return false
		}
		activate()
		return true
	})
}

func (ui *view) deferredKeyAction(action func()) func() {
	queued := false
	return func() {
		if queued {
			return
		}
		queued = true
		fltk.AddTimeout(0.01, func() {
			queued = false
			if ui.ctx.Err() == nil && !ui.closing {
				action()
			}
		})
	}
}

func (ui *view) focusAfterKey(action func()) {
	fltk.AddTimeout(0.01, func() {
		if ui.ctx.Err() == nil && !ui.closing {
			action()
		}
	})
}

func (ui *view) bindNavigationKeys(button *fltk.Button, index int) {
	position := navigationPosition(index)
	button.SetCallback(func() { ui.showSection(index) })
	activate := ui.deferredKeyAction(func() { ui.showSection(index) })
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, 13, 0xff8d:
			activate()
		case 0xff52:
			ui.focusAfterKey(func() { ui.navigation[navigationOrder[max(0, position-1)]].TakeFocus() })
		case 0xff54:
			ui.focusAfterKey(func() { ui.navigation[navigationOrder[min(len(navigationOrder)-1, position+1)]].TakeFocus() })
		case 0xff53:
			ui.focusAfterKey(ui.focusContent)
		case 9, 0xff09:
			if fltk.EventState()&fltk.SHIFT != 0 {
				ui.focusAfterKey(func() { ui.navigation[navigationOrder[max(0, position-1)]].TakeFocus() })
			} else if position+1 < len(navigationOrder) {
				ui.focusAfterKey(func() { ui.navigation[navigationOrder[position+1]].TakeFocus() })
			} else {
				ui.focusAfterKey(ui.focusContent)
			}
		default:
			return false
		}
		return true
	})
}

func (ui *view) focusContent() {
	if ui.selected == 4 {
		if ui.searchPreview {
			ui.previewBack.TakeFocus()
		} else {
			ui.searchInput.TakeFocus()
		}
		return
	}
	if ui.scroll.Visible() && ui.browseButton != nil {
		ui.scrollTo(0)
		ui.browseButton.TakeFocus()
		return
	}
	ui.themeToggle.TakeFocus()
}

func (ui *view) scrollTo(y int) {
	targetY := ui.scrollOriginY + y
	if ui.scroll.XPosition() != ui.scrollOriginX || ui.scroll.YPosition() != targetY {
		ui.scroll.ScrollTo(ui.scrollOriginX, targetY)
	}
}

func (ui *view) focusTarget(button *fltk.Button) {
	if button == nil {
		return
	}
	if button == ui.browseButton || button == ui.retryButton {
		ui.scrollTo(0)
	}
	for index, card := range ui.cardButtons {
		if card == button {
			ui.scrollTo(index * 104)
			break
		}
	}
	button.TakeFocus()
}

func (ui *view) bindContentKeys(button *fltk.Button, action func(), previous, next func() *fltk.Button) {
	activate := ui.deferredKeyAction(action)
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, 13, 0xff8d:
			activate()
		case 0xff51:
			if button == ui.browseButton || button == ui.retryButton {
				ui.focusAfterKey(func() { ui.navigation[ui.selected].TakeFocus() })
			} else {
				ui.focusAfterKey(func() { ui.focusTarget(ui.browseButton) })
			}
		case 0xff53:
			if button == ui.browseButton && len(ui.cardButtons) > 0 {
				ui.focusAfterKey(func() {
					if len(ui.cardButtons) > 0 {
						ui.focusTarget(ui.cardButtons[0])
					}
				})
			} else if button == ui.retryButton {
				ui.focusAfterKey(func() { ui.focusTarget(ui.themeToggle) })
			} else {
				return true
			}
		case 0xff52:
			if button == ui.browseButton {
				ui.focusAfterKey(func() { ui.focusTarget(ui.themeToggle) })
			} else if previous == nil || previous() == nil {
				ui.focusAfterKey(func() { ui.navigation[ui.selected].TakeFocus() })
			} else {
				ui.focusAfterKey(func() { ui.focusTarget(previous()) })
			}
		case 0xff54:
			if next != nil && next() != nil {
				ui.focusAfterKey(func() { ui.focusTarget(next()) })
			} else {
				return false
			}
		case 9, 0xff09:
			if fltk.EventState()&fltk.SHIFT != 0 {
				if previous == nil || previous() == nil {
					ui.focusAfterKey(func() { ui.navigation[ui.selected].TakeFocus() })
				} else {
					ui.focusAfterKey(func() { ui.focusTarget(previous()) })
				}
			} else if next != nil && next() != nil {
				ui.focusAfterKey(func() { ui.focusTarget(next()) })
			} else {
				return false
			}
		default:
			return false
		}
		return true
	})
}

func (ui *view) bindThemeKeys(action func()) {
	activate := ui.deferredKeyAction(action)
	ui.themeToggle.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, 13, 0xff8d:
			activate()
		case 0xff51:
			ui.focusAfterKey(func() { ui.navigation[0].TakeFocus() })
		case 0xff54:
			ui.focusAfterKey(ui.focusContent)
		case 0xff52:
			if len(ui.cardButtons) > 0 && ui.scroll.Visible() {
				ui.focusAfterKey(func() {
					if len(ui.cardButtons) > 0 {
						ui.focusTarget(ui.cardButtons[len(ui.cardButtons)-1])
					}
				})
			} else if ui.scroll.Visible() {
				ui.focusAfterKey(ui.focusContent)
			} else {
				ui.focusAfterKey(func() { ui.navigation[ui.selected].TakeFocus() })
			}
		case 9, 0xff09:
			if fltk.EventState()&fltk.SHIFT == 0 {
				return false
			}
			if ui.scroll.Visible() && ui.retryButton != nil && ui.retryButton.Visible() {
				ui.focusAfterKey(func() { ui.focusTarget(ui.retryButton) })
			} else if len(ui.cardButtons) > 0 && ui.scroll.Visible() {
				ui.focusAfterKey(func() {
					if len(ui.cardButtons) > 0 {
						ui.focusTarget(ui.cardButtons[len(ui.cardButtons)-1])
					}
				})
			} else {
				ui.focusAfterKey(func() { ui.navigation[len(ui.navigation)-1].TakeFocus() })
			}
		default:
			return false
		}
		return true
	})
}

func (ui *view) cancel() {
	ui.closeOnce.Do(func() {
		ui.closing = true
		ui.searchModel.close()
		ui.mainGroup.Deactivate()
		ui.themeToggle.Deactivate()
		go func() {
			var err error
			if ui.appearance != nil {
				err = ui.appearance.close()
			}
			fltk.Awake(func() {
				if err != nil {
					fltk.MessageBox("設定未儲存", "無法儲存主題設定，下次開啟可能恢復先前的主題。")
				}
				ui.cancelContext()
				if ui.onClose != nil {
					ui.onClose()
				}
			})
		}()
	})
}

func firstError(values ...error) error {
	for _, value := range values {
		if value != nil && !isCanceled(value) {
			return value
		}
	}
	return nil
}
