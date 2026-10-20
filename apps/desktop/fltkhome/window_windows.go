//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	RemoveHistory(context.Context, string) error
	Following(context.Context) ([]backend.Following, error)
	Follow(context.Context, string) error
	Unfollow(context.Context, string) error
	Schedule(context.Context) ([]backend.ScheduleItem, error)
	CachedSchedule(context.Context) ([]backend.ScheduleItem, error)
}

const homeEpisodeLookupTimeout = 3 * time.Second

// Home layout: the first section's heading and first row, and the gap between sections.
const (
	homeFirstSectionY = 274
	homeFirstRowY     = 318
	homeSectionGap    = 20
)

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
	homeCaptionLabels  []*fltk.Box
	homeLookupCancel   context.CancelFunc
	loadFailed         bool
	homeLoading        bool
	homeRefreshPending bool
	homeLoadGeneration uint64
	searchModel        searchModel
	detailModel        detailModel
	detailCancel       context.CancelFunc
	detailGeneration   uint64
	detailEpisodesDone bool
	detailEpisodesErr  bool
	detailHistoryErr   bool
	detailCoverErr     bool
	// metadataLookedUp holds anime looked up on AniList/Bangumi this session, so an anime
	// with no match is not queried again on every open. Touched only on the UI thread.
	metadataLookedUp   map[string]bool
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
	previewTitle       *fltk.Box
	previewDescription *fltk.Box
	previewSynopsis    *scrollingText
	previewMeta        *fltk.Box
	previewCover       *fltk.Box
	previewEpisodes    *fltk.Scroll
	previewEpisodeRows []*fltk.Button
	previewEpisodeIDs  []string
	previewEpisodeText *fltk.Box
	previewPrevious    *fltk.Button
	previewNext        *fltk.Button
	previewContinue    *fltk.Button
	previewRetry       *fltk.Button
	previewStatus      *fltk.Box
	previewCoverImage  *fltk.RgbImage
	searchPreview      bool
	pageGeneration     uint64
	searchInputVersion uint64
	closeOnce          sync.Once
	closeButton        *fltk.Button
	themeToggle        *fltk.Button
	dark               bool
	appearance         *appearancePersistence
	closing            bool

	previewFollow *fltk.Button
	// followPending blocks a second follow or unfollow until the first one finishes.
	followPending bool
	// detailOrigin is where Back and Escape on the detail screen return to.
	detailOrigin             detailOrigin
	homeUpdateButtons        []*fltk.Button
	homeUpdateOffsets        []int
	homeCardLabels           []*fltk.Box
	homeCardOffset           int
	continueHeading          *fltk.Box
	homeUpdateKey            string
	followingPage            *fltk.Group
	followingStatus          *fltk.Box
	followingRefresh         *fltk.Button
	followingScroll          *fltk.Scroll
	followingButtons         []*fltk.Button
	followingRowIDs          []string
	followingItems           []followingRow
	followingGeneration      uint64
	followingLoaded          bool
	followingLoadFailed      bool
	followingRefreshing      bool
	followingRefreshFailures int
	followingRefreshTotal    int
	// followingRefreshAttemptedAt throttles the automatic refresh, success or not, so a
	// failing source is not asked again on every visit to Home or 追蹤.
	followingRefreshAttemptedAt time.Time

	historyPage          *fltk.Group
	historyStatus        *fltk.Box
	historyRetry         *fltk.Button
	historyScroll        *fltk.Scroll
	historyButtons       []*fltk.Button
	historyRowIDs        []string
	historyItems         []historyRow
	historyGeneration    uint64
	historyLoading       bool
	historyLoaded        bool
	historyLoadFailed    bool
	historyRemovePending bool

	schedulePage          *fltk.Group
	scheduleStatus        *fltk.Box
	scheduleRefresh       *fltk.Button
	scheduleScroll        *fltk.Scroll
	scheduleHeaders       []*fltk.Box
	scheduleCards         [][]*fltk.Button
	scheduleColumns       []scheduleDay
	scheduleGeneration    uint64
	scheduleLoaded        bool
	scheduleLoadFailed    bool
	scheduleRefreshing    bool
	scheduleRefreshFailed bool
	// scheduleOpened is the day and index of the show last opened, for focus on return.
	scheduleOpened [2]int
	// scheduleRefreshAttemptedAt throttles the automatic fetch, success or not.
	scheduleRefreshAttemptedAt time.Time
}

func NewWindow(plan backend.PortablePlan, planErr error, service Service, ctx context.Context, cancel context.CancelFunc, onPlay func(backend.PlayRequest), onClose func()) (*fltk.Window, func(int)) {
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
	window.SetCallback(ui.cancel)
	ui.build()
	if planErr != nil {
		ui.showStartupError("無法使用這個資料夾，請將程式解壓到可寫入的位置後重新開啟。")
		return window, ui.showSection
	}
	ui.start()
	return window, ui.navigate
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
		if list := ui.homeFocusables(); len(list) > 0 {
			return list[0]
		}
		return ui.themeToggle
	})
	ui.continueHeading = ui.contentText(253, homeFirstSectionY, 600, 32, "繼續觀看", 20, false, true)
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
	ui.buildFollowingView()
	ui.buildHistoryView()
	ui.buildScheduleView()
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
		ui.resizeFollowing()
		ui.resizeHistory()
		ui.resizeSchedule()
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
	enteringHome := index == 0 && ui.selected != 0
	if ui.selected != index {
		ui.pageGeneration++
	}
	if index == 4 && ui.searchPreview {
		ui.pageGeneration++
		ui.stopDetailLoad()
		ui.searchModel.cancel()
		ui.searchPreview = false
	}
	if ui.selected == 4 && index != 4 {
		ui.stopDetailLoad()
		ui.searchModel.cancel()
		ui.searchPreview = false
	}
	ui.selected = index
	if index != 4 {
		ui.detailOrigin = detailFromSearch
	}
	if ui.followingPage != nil && index != followingSection {
		ui.followingPage.Hide()
	}
	if ui.historyPage != nil && index != historySection {
		ui.historyPage.Hide()
	}
	if ui.schedulePage != nil && index != scheduleSection {
		ui.schedulePage.Hide()
	}
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
		if enteringHome {
			// Following and history can change on other pages; Home must not show them stale.
			ui.refreshHome()
		}
	} else if index == 4 {
		ui.contentHint.SetLabel("搜尋作品名稱，先查看已儲存的作品，再按 Enter 更新搜尋。")
		ui.scroll.Hide()
		ui.searchPage.Show()
		ui.previewGroup.Hide()
		ui.loadSearchLibrary()
		ui.renderSearch()
	} else if index == followingSection && ui.followingPage != nil {
		ui.contentHint.SetLabel("追蹤作品的最新集數與觀看進度；按作品開啟詳情。")
		ui.scroll.Hide()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
		ui.showFollowing()
	} else if index == historySection && ui.historyPage != nil {
		ui.contentHint.SetLabel("最近觀看的作品與進度；按作品繼續播放。")
		ui.scroll.Hide()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
		ui.showHistory()
	} else if index == scheduleSection && ui.schedulePage != nil {
		ui.contentHint.SetLabel("接下來 7 天的新番時間表；按作品開啟詳情或搜尋片源。")
		ui.scroll.Hide()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
		ui.showSchedule()
	} else {
		ui.contentHint.SetLabel("此頁面尚未建立")
		ui.scroll.Hide()
		ui.searchPage.Hide()
		ui.previewGroup.Hide()
	}
	ui.window.Redraw()
}

func (ui *view) navigate(index int) {
	alreadyHome := index == 0 && ui.selected == 0
	ui.showSection(index)
	if alreadyHome {
		// showSection reloads Home only when entering it; after playback Home may already be shown.
		ui.refreshHome()
	}
}

// refreshHome reloads Home, or queues one more load when a load is already running.
func (ui *view) refreshHome() {
	// browseButton is nil when Home was not built (views that only build Search).
	if ui.browseButton == nil || ui.mainGroup == nil || !ui.mainGroup.Visible() {
		return
	}
	if ui.homeLoading {
		ui.homeRefreshPending = true
		ui.hideHomeContinue()
		return
	}
	ui.loadHome()
}

func (ui *view) loadHome() {
	if ui.homeLoading || ui.ctx.Err() != nil || ui.closing {
		return
	}
	ui.cancelHomeEpisodeLookup()
	if ui.service == nil {
		ui.hideHomeContinue()
		ui.message.SetLabel(homeErrorMessage(fmt.Errorf("service unavailable")))
		ui.loadingText.SetLabel("無法載入作品資料。")
		ui.loadingText.Show()
		ui.retryButton.Show()
		return
	}
	ui.homeLoading = true
	ui.homeLoadGeneration++
	generation := ui.homeLoadGeneration
	ui.hideHomeContinue()
	ui.retryButton.Hide()
	ui.loadingText.SetLabel("正在載入繼續觀看…")
	ui.loadingText.Show()
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
			if generation != ui.homeLoadGeneration || ui.ctx.Err() != nil || ui.closing {
				return
			}
			ui.homeLoading = false
			if ui.homeRefreshPending {
				ui.homeRefreshPending = false
				ui.loadHome()
				return
			}
			if libraryErr != nil || historyErr != nil || followingErr != nil {
				ui.loadFailed = true
				ui.message.SetLabel(homeErrorMessage(firstError(libraryErr, historyErr, followingErr)))
				ui.loadingText.SetLabel("無法載入作品資料。")
				ui.loadingText.Show()
				ui.retryButton.Show()
				ui.window.Redraw()
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

func (ui *view) hideHomeContinue() {
	for _, button := range ui.homeFocusables() {
		button.Deactivate()
		button.Hide()
	}
	for _, card := range ui.cards {
		card.Hide()
	}
	for _, label := range ui.homeCardLabels {
		label.Hide()
	}
}

func (ui *view) playHomeRow(item backend.History, generation uint64) {
	if ui.onPlay != nil && ui.ctx.Err() == nil && !ui.closing && !ui.homeLoading && !ui.loadFailed && ui.selected == 0 && generation == ui.homeLoadGeneration {
		ui.onPlay(playRequest(item))
	}
}

func (ui *view) populateHome(library []backend.Anime, following []backend.Following) {
	ui.scroll.ScrollTo(ui.scroll.XPosition()+ui.browseButton.X()-278, ui.scroll.YPosition()+ui.browseButton.Y()-205)
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
	for _, button := range ui.homeUpdateButtons {
		ui.scroll.Remove(button)
		button.Destroy()
	}
	ui.homeUpdateButtons = nil
	ui.homeUpdateOffsets = nil
	ui.cardButtons = nil
	ui.cards = nil
	ui.homeCaptionLabels = nil
	ui.homeCardLabels = nil
	ui.contentLabels = ui.contentLabels[:ui.staticLabels]
	ui.scroll.Begin()

	// 近期更新 comes first (owner decision 2026-10-15), then 繼續觀看, then 今日播出.
	ui.contentText(253, homeFirstSectionY, 600, 32, "近期更新", 20, false, true)
	updates := recentUpdateRows(following, library, rowLimit, time.Now())
	ui.homeUpdateKey = recentUpdateKey(updates)
	updatesBottom := homeFirstRowY + len(updates)*64 - 8
	if len(updates) == 0 {
		ui.contentText(270, homeFirstRowY+4, 620, 22, recentUpdateEmptyText(following), 12, true, false)
		updatesBottom = homeFirstRowY + 26
	}
	for index, row := range updates {
		y := homeFirstRowY + index*64
		button := fltk.NewButton(250, y, 711, 56, row.Anime.Title)
		button.SetTooltip(row.Status)
		ui.styleTitleStatusRow(button, row.Anime.Title, row.Status)
		item := row.Anime
		openRow := func() {
			if ui.selected == 0 && !ui.homeLoading {
				ui.openDetailFrom(detailFromHome, item)
			}
		}
		ui.bindButton(button, openRow)
		// Keyboard order follows the page: these rows, then 繼續播放 cards, then the theme toggle.
		ui.bindContentKeys(button, openRow, func() *fltk.Button {
			if index > 0 {
				return ui.homeUpdateButtons[index-1]
			}
			return ui.browseButton
		}, func() *fltk.Button {
			if index+1 < len(ui.homeUpdateButtons) {
				return ui.homeUpdateButtons[index+1]
			}
			if len(ui.cardButtons) > 0 {
				return ui.cardButtons[0]
			}
			return ui.themeToggle
		})
		ui.homeUpdateButtons = append(ui.homeUpdateButtons, button)
		ui.homeUpdateOffsets = append(ui.homeUpdateOffsets, y-homeFirstRowY)
	}

	continueY := updatesBottom + homeSectionGap
	cardsY := continueY + homeFirstRowY - homeFirstSectionY
	ui.homeCardOffset = cardsY - homeFirstRowY
	ui.continueHeading.Resize(253, continueY, 600, 32)
	ui.loadingText.Resize(253, continueY+52, 680, 36)
	ui.retryButton.Resize(253, continueY+91, 126, 36)
	ui.loadingText.Show()
	if len(ui.rows) == 0 {
		ui.loadingText.SetLabel("還沒有可繼續觀看的內容。")
	} else {
		ui.loadingText.Hide()
	}
	for index, row := range ui.rows {
		y := cardsY + index*104
		rowGeneration := ui.homeLoadGeneration
		card := fltk.NewBox(fltk.NO_BOX, 250, y, 711, 92)
		card.SetDrawHandler(func(func()) { roundedRect(card.X(), card.Y(), card.W(), card.H(), 16, ui.colors().surface) })
		ui.cards = append(ui.cards, card)
		title := ui.contentText(273, y+14, 450, 25, row.Title, 15, false, true)
		caption := ui.contentText(273, y+48, 450, 20, homeCaption(row), 12, true, false)
		ui.homeCaptionLabels = append(ui.homeCaptionLabels, caption)
		ui.homeCardLabels = append(ui.homeCardLabels, title, caption)
		button := fltk.NewButton(813, y+28, 126, 36, "繼續播放")
		ui.styleButton(button, func() bool { return true })
		item := row.History
		playRow := func() { ui.playHomeRow(item, rowGeneration) }
		ui.bindButton(button, playRow)
		ui.bindContentKeys(button, playRow, func() *fltk.Button {
			if index > 0 {
				return ui.cardButtons[index-1]
			}
			if len(ui.homeUpdateButtons) > 0 {
				return ui.homeUpdateButtons[len(ui.homeUpdateButtons)-1]
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
	todayY := continueY + 111
	if len(ui.rows) > 0 {
		todayY = cardsY + len(ui.rows)*104 - 12 + homeSectionGap
	}
	ui.contentText(253, todayY, 600, 31, "今日播出", 20, false, true)
	ui.contentText(270, todayY+37, 620, 22, "目前沒有本機播出資料。", 12, true, false)
	ui.scroll.End()
	ui.applyTheme()
	ui.resolveHomeEpisodes(ui.homeLoadGeneration)
	// Home shows 近期更新 from stored data; this checks Anime1 for new episodes in the background.
	ui.refreshFollowingIfDue()
}

func (ui *view) cancelHomeEpisodeLookup() {
	if ui.homeLookupCancel != nil {
		ui.homeLookupCancel()
		ui.homeLookupCancel = nil
	}
}

func (ui *view) resolveHomeEpisodes(generation uint64) {
	ui.cancelHomeEpisodeLookup()
	if ui.service == nil || len(ui.rows) == 0 {
		return
	}
	lookupContext, cancel := context.WithCancel(ui.ctx)
	ui.homeLookupCancel = cancel
	rows := append([]homeRow(nil), ui.rows[:min(len(ui.rows), rowLimit)]...)
	for index, row := range rows {
		go func(index int, row homeRow) {
			ctx, cancel := context.WithTimeout(lookupContext, homeEpisodeLookupTimeout)
			defer cancel()
			episodes, err := ui.service.Episodes(ctx, row.History.AnimeID)
			number := episodeNumber(episodes, row.History.EpisodeID, err)
			fltk.Awake(func() {
				ui.applyHomeEpisodeResult(generation, index, row, number)
			})
		}(index, row)
	}
}

func (ui *view) applyHomeEpisodeResult(generation uint64, index int, row homeRow, number string) {
	if generation != ui.homeLoadGeneration || ui.ctx.Err() != nil || ui.closing || ui.homeLoading || index < 0 || index >= len(ui.rows) || index >= len(ui.homeCaptionLabels) {
		return
	}
	current := ui.rows[index]
	if current.History.AnimeID != row.History.AnimeID || current.History.EpisodeID != row.History.EpisodeID {
		return
	}
	current.EpisodeNumber = number
	ui.rows[index] = current
	ui.homeCaptionLabels[index].SetLabel(homeCaption(current))
	ui.window.Redraw()
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
	if ui.selected == followingSection && ui.followingPage != nil {
		ui.focusFollowing()
		return
	}
	if ui.selected == historySection && ui.historyPage != nil {
		ui.focusHistory()
		return
	}
	if ui.selected == scheduleSection && ui.schedulePage != nil {
		ui.focusSchedule()
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

// homeFocusables lists Home's focusable rows in page order: 近期更新, then 繼續播放.
func (ui *view) homeFocusables() []*fltk.Button {
	return append(append([]*fltk.Button(nil), ui.homeUpdateButtons...), ui.cardButtons...)
}

func (ui *view) focusTarget(button *fltk.Button) {
	if button == nil || !button.Visible() || !button.IsActive() {
		if ui.retryButton != nil && ui.retryButton.Visible() && ui.retryButton.IsActive() {
			button = ui.retryButton
		} else if ui.browseButton != nil && ui.browseButton.Visible() && ui.browseButton.IsActive() {
			button = ui.browseButton
		} else {
			button = ui.themeToggle
		}
	}
	if button == ui.browseButton || button == ui.retryButton {
		ui.scrollTo(0)
	}
	for index, card := range ui.cardButtons {
		if card == button {
			ui.scrollTo(ui.homeCardOffset + index*104)
			break
		}
	}
	for index, row := range ui.homeUpdateButtons {
		if row == button && index < len(ui.homeUpdateOffsets) {
			ui.scrollTo(ui.homeUpdateOffsets[index])
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
			if button == ui.browseButton && len(ui.homeFocusables()) > 0 {
				ui.focusAfterKey(func() {
					if list := ui.homeFocusables(); len(list) > 0 {
						ui.focusTarget(list[0])
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
			if len(ui.homeFocusables()) > 0 && ui.scroll.Visible() {
				ui.focusAfterKey(func() {
					if list := ui.homeFocusables(); len(list) > 0 {
						ui.focusTarget(list[len(list)-1])
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
			} else if len(ui.homeFocusables()) > 0 && ui.scroll.Visible() {
				ui.focusAfterKey(func() {
					if list := ui.homeFocusables(); len(list) > 0 {
						ui.focusTarget(list[len(list)-1])
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
		ui.cancelHomeEpisodeLookup()
		ui.homeLoadGeneration++
		ui.homeLoading = false
		ui.homeRefreshPending = false
		ui.searchModel.close()
		ui.stopDetailLoad()
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
