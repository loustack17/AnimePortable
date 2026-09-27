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
	Start(context.Context) error
	Library(context.Context) ([]backend.Anime, error)
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

	message       *fltk.Box
	header        *fltk.Box
	tagline       *fltk.Box
	sectionTitle  *fltk.Box
	contentHint   *fltk.Box
	scroll        *fltk.Scroll
	loadingText   *fltk.Box
	retryButton   *fltk.Button
	contentLabels []themedLabel
	staticLabels  int
	cards         []*fltk.Box
	cardButtons   []*fltk.Button
	startupGroup  *fltk.Group
	mainGroup     *fltk.Group
	navigation    []*fltk.Button
	selected      int
	rows          []homeRow
	loadFailed    bool
	closeOnce     sync.Once
	closeButton   *fltk.Button
	themeToggle   *fltk.Button
	dark          bool
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
	showMessage := func(message string) { ui.message.SetLabel(message); ui.window.Redraw() }
	window.SetCallback(ui.cancel)
	ui.build()
	if planErr != nil {
		ui.showStartupError("無法使用這個資料夾，請將程式解壓到可寫入的位置後重新開啟。")
		return window, ui.showSection, showMessage
	}
	if plan.OfferImport {
		ui.showChoices("找到舊版資料。你可以複製一份到這個資料夾，或建立新的空白資料；舊資料都會保留。")
		return window, ui.showSection, showMessage
	}
	ui.start(false)
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
			x, y, width, height = x+3, y+3, width-6, height-6
		}
		roundedRect(x, y, width, height, 11, colors.soft)
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
	ui.themeToggle.SetCallback(func() {
		ui.dark = !ui.dark
		if ui.dark {
			ui.themeToggle.SetTooltip("切換明亮")
		} else {
			ui.themeToggle.SetTooltip("切換暗色")
		}
		ui.applyTheme()
	})
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
	browse := fltk.NewButton(278, 205, 132, 37, "瀏覽內容")
	browse.SetBox(fltk.NO_BOX)
	browse.SetDrawHandler(func(func()) {
		x, y, width, height := browse.X(), browse.Y(), browse.W(), browse.H()
		if browse.HasFocus() {
			roundedRect(x, y, width, height, 13, ui.colors().accent)
			x, y, width, height = x+3, y+3, width-6, height-6
		}
		roundedRect(x, y, width, height, 11, fltk.WHITE)
		fltk.SetDrawColor(fltk.ColorFromRgb(48, 55, 125))
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 13)
		fltk.Draw(browse.Label(), x, y, width, height, fltk.ALIGN_CENTER)
	})
	browse.SetCallback(func() { ui.scroll.ScrollTo(0, 176) })
	ui.contentText(253, 274, 600, 32, "繼續觀看", 20, false, true)
	ui.loadingText = fltk.NewBox(fltk.NO_BOX, 253, 326, 680, 36, "正在載入繼續觀看…")
	ui.loadingText.SetLabelSize(13)
	ui.loadingText.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.retryButton = fltk.NewButton(253, 365, 126, 36, "重新載入")
	ui.styleButton(ui.retryButton, func() bool { return true })
	ui.retryButton.SetCallback(ui.loadHome)
	ui.retryButton.Hide()
	ui.scroll.End()
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
	for index, label := range sections {
		button := fltk.NewButton(27, 111+index*55, 177, 43, label)
		button.SetCallback(func(current int) func() {
			return func() { ui.showSection(current) }
		}(index))
		button.SetLabelSize(15)
		ui.styleButton(button, func() bool { return ui.selected == index })
		ui.navigation = append(ui.navigation, button)
	}
	navPanel.End()
	ui.window.End()
	ui.applyTheme()
	ui.staticLabels = len(ui.contentLabels)
	ui.window.SetResizeHandler(func() { ui.scroll.Resize(236, 110, max(400, ui.window.W()-259), max(250, ui.window.H()-131)) })
}

func (ui *view) showChoices(message string) {
	ui.message.SetLabel(message)
	ui.startupGroup.Begin()
	create := fltk.NewButton(272, 132, 240, 48, "建立新的空白資料")
	ui.styleButton(create, func() bool { return true })
	create.SetCallback(func() { ui.start(false) })
	copy := fltk.NewButton(530, 132, 240, 48, "複製既有資料")
	ui.styleButton(copy, nil)
	copy.SetCallback(func() { ui.start(true) })
	close := fltk.NewButton(272, 194, 160, 40, "關閉")
	ui.styleButton(close, nil)
	close.SetCallback(ui.cancel)
	ui.startupGroup.End()
	ui.startupGroup.Show()
	fltk.AddTimeout(0.01, func() {
		if ui.ctx.Err() == nil {
			create.TakeFocus()
			create.Redraw()
		}
	})
	ui.window.Redraw()
}

func (ui *view) showStartupError(message string) {
	ui.message.SetLabel(message)
	if ui.closeButton == nil {
		ui.startupGroup.Begin()
		ui.closeButton = fltk.NewButton(272, 132, 160, 40, "關閉")
		ui.styleButton(ui.closeButton, nil)
		ui.closeButton.SetCallback(ui.cancel)
		ui.startupGroup.End()
	}
	ui.startupGroup.Show()
	ui.window.Redraw()
}

func (ui *view) start(copyExisting bool) {
	ui.message.SetLabel("正在準備資料…")
	ui.startupGroup.Deactivate()
	go func() {
		var err error
		imported := false
		if copyExisting {
			err = ui.plan.Import(ui.ctx)
			imported = err == nil
		} else if !ui.plan.TargetExists {
			err = ui.plan.CreateFresh()
		}
		if err == nil && ui.service != nil {
			err = ui.service.Start(ui.ctx)
		}
		fltk.Awake(func() {
			if ui.ctx.Err() != nil {
				return
			}
			if err != nil {
				ui.startupGroup.Activate()
				ui.message.SetLabel(startupErrorMessage(err, copyExisting, imported))
				if !copyExisting || backend.IsPortableImportConflict(err) {
					ui.showStartupError(startupErrorMessage(err, copyExisting, imported))
				}
				return
			}
			ui.startupGroup.Hide()
			ui.mainGroup.Show()
			ui.showSection(0)
			ui.navigation[0].TakeFocus()
			ui.loadHome()
			ui.window.Redraw()
		})
	}()
}

func (ui *view) showSection(index int) {
	if index < 0 || index >= len(sections) {
		return
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
	} else {
		ui.contentHint.SetLabel("此頁面尚未建立")
		ui.scroll.Hide()
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
			if ui.ctx.Err() != nil {
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
		y := 370 + index*104
		card := fltk.NewBox(fltk.NO_BOX, 250, y, 711, 92)
		card.SetDrawHandler(func(func()) { roundedRect(card.X(), card.Y(), card.W(), card.H(), 16, ui.colors().surface) })
		ui.cards = append(ui.cards, card)
		ui.contentText(273, y+14, 450, 25, row.Title, 15, false, true)
		caption := fmt.Sprintf("上次播放位置 %s", formatPosition(row.History.Position))
		ui.contentText(273, y+48, 450, 20, caption, 12, true, false)
		button := fltk.NewButton(813, y+28, 126, 36, "繼續播放")
		ui.styleButton(button, func() bool { return true })
		item := row.History
		button.SetCallback(func() {
			if ui.onPlay != nil && ui.ctx.Err() == nil {
				ui.onPlay(playRequest(item))
			}
		})
		ui.cardButtons = append(ui.cardButtons, button)
	}
	followY := 390 + len(ui.rows)*104
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

func (ui *view) cancel() {
	ui.closeOnce.Do(func() {
		ui.cancelContext()
		if ui.onClose != nil {
			ui.onClose()
		}
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
