//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
	"slices"
	"strings"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

func (ui *view) buildSearchView() {
	ui.searchPage = fltk.NewGroup(236, 110, 741, 487)
	ui.searchPage.Begin()
	ui.searchInput = fltk.NewInput(252, 118, 477, 42, "")
	ui.searchInput.SetLabelSize(16)
	ui.searchInput.SetCallbackCondition(fltk.WhenChanged | fltk.WhenEnterKeyAlways)
	ui.searchInput.SetCallback(func() {
		query := ui.searchInput.Value()
		enter := isEnterKey(fltk.EventKey())
		ui.searchInputVersion++
		inputVersion := ui.searchInputVersion
		pageGeneration := ui.pageGeneration
		fltk.AddTimeout(0.01, func() {
			if ui.ctx.Err() != nil || ui.closing || ui.pageGeneration != pageGeneration || ui.searchInputVersion != inputVersion || ui.selected != 4 || ui.searchPreview || ui.mainGroup == nil || !ui.mainGroup.Visible() {
				return
			}
			ui.searchModel.setQuery(query)
			ui.renderSearch()
			if enter {
				ui.submitSearch()
			}
		})
	})
	ui.searchInput.SetEventHandler(ui.handleSearchInputKey)
	ui.searchButton = fltk.NewButton(741, 118, 104, 42, "搜尋")
	ui.styleButton(ui.searchButton, nil)
	ui.bindSearchButton(ui.searchButton, ui.submitSearch)
	ui.searchBack = fltk.NewButton(853, 118, 104, 42, "返回首頁")
	ui.styleButton(ui.searchBack, nil)
	ui.bindSearchButton(ui.searchBack, func() { ui.showSection(0) })
	ui.searchStatus = fltk.NewBox(fltk.NO_BOX, 252, 169, 705, 29, "")
	ui.searchStatus.SetLabelSize(12)
	ui.searchStatus.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.searchScroll = fltk.NewScroll(236, 201, 741, 355)
	ui.searchScroll.SetType(fltk.SCROLL_VERTICAL)
	ui.searchScroll.SetBox(fltk.FLAT_BOX)
	ui.searchScroll.End()
	ui.searchPageText = fltk.NewBox(fltk.NO_BOX, 455, 565, 280, 28, "")
	ui.searchPageText.SetLabelSize(12)
	ui.searchPageText.SetAlign(fltk.ALIGN_CENTER | fltk.ALIGN_INSIDE)
	ui.searchPrevious = fltk.NewButton(252, 561, 113, 36, "上一頁")
	ui.styleButton(ui.searchPrevious, nil)
	ui.bindSearchButton(ui.searchPrevious, func() {
		ui.searchModel.movePage(-1)
		ui.renderSearch()
	})
	ui.searchNext = fltk.NewButton(844, 561, 113, 36, "下一頁")
	ui.styleButton(ui.searchNext, nil)
	ui.bindSearchButton(ui.searchNext, func() {
		ui.searchModel.movePage(1)
		ui.renderSearch()
	})
	ui.searchPage.End()
	ui.searchPage.Hide()

	ui.previewGroup = fltk.NewGroup(236, 110, 741, 487)
	ui.previewGroup.Begin()
	ui.previewBack = fltk.NewButton(252, 118, 113, 38, "返回結果")
	ui.styleButton(ui.previewBack, nil)
	ui.bindSearchButton(ui.previewBack, ui.leaveDetail)
	ui.previewContinue = fltk.NewButton(539, 118, 126, 38, "繼續觀看")
	ui.styleButton(ui.previewContinue, nil)
	ui.bindSearchButton(ui.previewContinue, ui.continueDetailEpisode)
	ui.previewRetry = fltk.NewButton(677, 118, 96, 38, "重試載入")
	ui.styleButton(ui.previewRetry, nil)
	ui.bindSearchButton(ui.previewRetry, ui.retryDetailLoad)
	ui.previewRetry.Hide()
	ui.previewFollow = fltk.NewButton(0, 118, followIconSize, 38, "追蹤")
	ui.styleFollowIcon(ui.previewFollow, func() bool { return ui.searchModel.preview != nil && ui.searchModel.preview.Following })
	ui.bindSearchButton(ui.previewFollow, ui.toggleDetailFollow)
	ui.previewFollow.Hide()
	ui.previewTitle = fltk.NewBox(fltk.NO_BOX, 252, 166, 408, 42, "")
	ui.previewTitle.SetLabelFont(fltk.HELVETICA_BOLD)
	ui.previewTitle.SetLabelSize(20)
	ui.previewTitle.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewCover = fltk.NewBox(fltk.NO_BOX, 252, 215, 166, 248, "封面載入中…")
	ui.previewCover.SetAlign(fltk.ALIGN_CENTER | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewCover.SetDrawHandler(func(defaultDraw func()) {
		if ui.previewCoverImage != nil {
			imageWidth, imageHeight := ui.previewCoverImage.DataW(), ui.previewCoverImage.DataH()
			drawWidth, drawHeight := ui.previewCover.W(), ui.previewCover.H()
			if drawWidth*imageHeight > drawHeight*imageWidth {
				drawWidth = drawHeight * imageWidth / imageHeight
			} else {
				drawHeight = drawWidth * imageHeight / imageWidth
			}
			ui.previewCoverImage.Scale(drawWidth, drawHeight, true, false)
			ui.previewCoverImage.Draw(ui.previewCover.X()+(ui.previewCover.W()-drawWidth)/2, ui.previewCover.Y()+(ui.previewCover.H()-drawHeight)/2, drawWidth, drawHeight)
		} else {
			defaultDraw()
		}
	})
	ui.previewMeta = fltk.NewBox(fltk.NO_BOX, 434, 212, 226, 65, "")
	ui.previewMeta.SetLabelSize(13)
	ui.previewMeta.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_TOP | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewDescription = fltk.NewBox(fltk.NO_BOX, 434, 285, 226, 215, "")
	ui.previewDescription.SetLabelSize(14)
	ui.previewSynopsis = newScrollingText(ui.previewDescription, func() fltk.Color { return ui.colors().background })
	ui.previewStatus = fltk.NewBox(fltk.NO_BOX, 684, 166, 273, 28, "")
	ui.previewStatus.SetLabelSize(12)
	ui.previewStatus.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewEpisodes = fltk.NewScroll(684, 197, 273, 317)
	ui.previewEpisodes.SetType(fltk.SCROLL_VERTICAL)
	ui.previewEpisodes.SetBox(fltk.FLAT_BOX)
	ui.previewEpisodes.End()
	ui.placeFollowIcon()
	ui.previewEpisodeText = fltk.NewBox(fltk.NO_BOX, 684, 519, 166, 25, "")
	ui.previewEpisodeText.SetLabelSize(11)
	ui.previewEpisodeText.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE)
	ui.previewPrevious = fltk.NewButton(684, 547, 92, 34, "上一頁")
	ui.styleButton(ui.previewPrevious, nil)
	ui.bindSearchButton(ui.previewPrevious, func() { ui.detailModel.movePage(-1); ui.renderDetailEpisodes() })
	ui.previewNext = fltk.NewButton(865, 547, 92, 34, "下一頁")
	ui.styleButton(ui.previewNext, nil)
	ui.bindSearchButton(ui.previewNext, func() { ui.detailModel.movePage(1); ui.renderDetailEpisodes() })
	ui.previewGroup.End()
	ui.previewGroup.Hide()

	ui.bindWindowKeys()
}

func (ui *view) bindSearchButton(button *fltk.Button, action func()) {
	previewAction := button == ui.previewBack || button == ui.previewContinue || button == ui.previewRetry || button == ui.previewFollow || button == ui.previewPrevious || button == ui.previewNext
	activate := ui.deferredSearchAction(action, previewAction)
	button.SetCallback(activate)
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch classifyKey(fltk.EventKey(), fltk.EventState()) {
		case keyActivate:
			activate()
		case keyUp:
			if button == ui.searchNext || button == ui.searchPrevious {
				ui.deferredSearchAction(func() { ui.focusSearchResult(max(0, len(ui.searchRows)-1)) }, false)()
			} else {
				return false
			}
		case keyDown:
			switch {
			case button == ui.searchPrevious || button == ui.searchButton || button == ui.searchBack:
				ui.deferredSearchAction(func() { ui.focusSearchResult(0) }, false)()
			case previewAction && button != ui.previewPrevious && button != ui.previewNext && len(ui.previewEpisodeRows) > 0:
				// The buttons above the episode list lead into it.
				ui.deferredSearchAction(ui.focusDetailEpisodes, true)()
			default:
				return false
			}
		default:
			return false
		}
		return true
	})
}

func (ui *view) loadSearchLibrary() {
	if ui.service == nil {
		ui.searchModel.phase = searchPhaseFailed
		ui.searchModel.libraryFailed = true
		ui.renderSearch()
		return
	}
	request, ctx, ok := ui.searchModel.beginLibrary()
	if !ok {
		return
	}
	if ui.selected == 4 {
		ui.renderSearch()
	}
	go func() {
		items, err := ui.service.Library(ctx)
		fltk.Awake(func() {
			ui.searchModel.finishLibrary(request, items, err)
			if ui.ctx.Err() != nil || ui.closing {
				return
			}
			if ui.selected == 4 {
				ui.refreshSearchView()
			}
		})
	}()
}

func (ui *view) submitSearch() {
	if ui.service == nil {
		ui.searchModel.phase = searchPhaseFailed
		ui.renderSearch()
		return
	}
	if ui.searchModel.libraryFailed && !ui.searchModel.loaded && !ui.searchModel.requestActive {
		if strings.TrimSpace(ui.searchModel.query) != "" {
			ui.searchModel.pending = searchRequestRemote
		}
		ui.loadSearchLibrary()
		ui.renderSearch()
		return
	}
	ui.startRemoteSearch()
}

func (ui *view) startRemoteSearch() {
	if ui.service == nil {
		ui.searchModel.phase = searchPhaseFailed
		ui.renderSearch()
		return
	}
	request, ctx, ok := ui.searchModel.beginSearch()
	if !ok {
		ui.searchModel.queueSearch()
		ui.renderSearch()
		return
	}
	ui.renderSearch()
	query := ui.searchModel.query
	query = strings.TrimSpace(query)
	go func() {
		items, err := ui.service.Search(ctx, query)
		fltk.Awake(func() {
			ui.searchModel.finishSearch(request, items, err)
			if ui.ctx.Err() != nil || ui.closing {
				return
			}
			if ui.selected == 4 {
				ui.refreshSearchView()
			}
		})
	}()
}

func (ui *view) openSearchResult(item backend.Anime) {
	ui.stopDetailLoad()
	ui.detailModel.open(item.ID)
	ui.detailEpisodesDone = false
	ui.detailEpisodesErr = false
	ui.detailHistoryErr = false
	ui.detailCoverErr = false
	ui.previewCover.SetLabel("封面載入中…")
	ui.pageGeneration++
	detailGeneration := ui.detailGeneration
	request, ctx, ok := ui.searchModel.beginDetail(item)
	if !ok {
		ui.searchModel.queueDetail(item)
		ui.searchPreview = true
		ui.searchPage.Hide()
		ui.previewGroup.Show()
		ui.renderPreview()
		ui.renderDetailEpisodes()
		ui.previewBack.TakeFocus()
		return
	}
	ui.searchPreview = true
	ui.searchPage.Hide()
	ui.previewGroup.Show()
	ui.renderPreview()
	ui.renderDetailEpisodes()
	ui.previewBack.TakeFocus()
	if ui.service == nil {
		ui.searchModel.finishDetail(request, backend.Detail{}, fmt.Errorf("service unavailable"))
		ui.renderPreview()
		return
	}
	go func() {
		detail, err := ui.service.Detail(ctx, item.ID)
		fltk.Awake(func() {
			applied := ui.searchModel.finishDetail(request, detail, err)
			if ui.ctx.Err() != nil || ui.closing {
				return
			}
			if applied && err == nil && ui.detailGeneration == detailGeneration && ui.searchPreview {
				ui.loadDetailResources(detailGeneration, item.ID)
			}
			if ui.selected == 4 {
				ui.refreshSearchView()
			}
		})
	}()
}

func (ui *view) startPendingSearchAction() {
	kind, item, ok := ui.searchModel.takePending()
	if !ok || ui.selected != 4 {
		return
	}
	switch kind {
	case searchRequestRemote:
		ui.startRemoteSearch()
	case searchRequestDetail:
		ui.openSearchResult(item)
	}
}

func (ui *view) refreshSearchView() {
	if ui.searchPreview {
		ui.renderPreview()
	} else {
		ui.renderSearch()
	}
	ui.startPendingSearchAction()
	if ui.selected == 4 && !ui.searchModel.loaded && !ui.searchModel.libraryFailed && !ui.searchModel.requestActive && ui.searchModel.pending == 0 {
		ui.loadSearchLibrary()
	}
}

func (ui *view) renderSearch() {
	if ui.searchPage == nil {
		return
	}
	// Results are rebuilt when stored or online results arrive; the focused row stays focused.
	focusedRow := slices.IndexFunc(ui.searchRows, (*fltk.Button).HasFocus)
	for _, button := range ui.searchRows {
		ui.searchScroll.Remove(button)
		button.Destroy()
	}
	ui.searchRows = nil
	ui.searchScroll.ScrollTo(0, 0)
	ui.searchScroll.Begin()
	for index, item := range ui.searchModel.visibleResults() {
		selectedIndex := ui.searchModel.pageStart() + index
		button := fltk.NewButton(ui.searchScroll.X()+15, ui.searchScroll.Y()+9+index*45, max(300, ui.searchScroll.W()-35), 40, item.Title)
		button.SetTooltip(item.NativeTitle)
		ui.styleButton(button, func() bool { return ui.searchModel.selected == selectedIndex })
		selectedItem := item
		openItem := func() {
			ui.searchModel.selected = selectedIndex
			ui.openSearchResult(selectedItem)
		}
		button.SetCallback(ui.deferredSearchAction(openItem, false))
		button.SetEventHandler(func(event fltk.Event) bool {
			if event != fltk.KEYDOWN {
				return false
			}
			switch classifyKey(fltk.EventKey(), fltk.EventState()) {
			case keyActivate:
				ui.deferredSearchAction(openItem, false)()
			case keyUp:
				ui.deferredSearchAction(func() {
					ui.searchModel.selected = selectedIndex
					ui.searchModel.moveSelection(-1)
					ui.renderSearch()
					ui.focusSelectedSearchResult()
				}, false)()
			case keyDown:
				ui.deferredSearchAction(func() {
					ui.searchModel.selected = selectedIndex
					ui.searchModel.moveSelection(1)
					ui.renderSearch()
					ui.focusSelectedSearchResult()
				}, false)()
			default:
				return false
			}
			return true
		})
		ui.searchRows = append(ui.searchRows, button)
	}
	ui.searchScroll.End()
	ui.searchStatus.SetLabel(ui.searchStatusText())
	// A deactivated button drops focus; remember which one had it.
	controls := []*fltk.Button{ui.searchButton, ui.searchPrevious, ui.searchNext}
	focusedControl := slices.ContainsFunc(controls, (*fltk.Button).HasFocus)
	if ui.searchModel.phase == searchPhaseLoading || ui.searchModel.requestActive {
		ui.searchButton.Deactivate()
	} else {
		ui.searchButton.Activate()
	}
	page := ui.searchModel.pageNumber()
	ui.searchPageText.SetLabel(searchPageLabel(page, ui.searchModel.pageCount(), len(ui.searchModel.results)))
	if page <= 1 {
		ui.searchPrevious.Deactivate()
	} else {
		ui.searchPrevious.Activate()
	}
	if page == 0 || page >= ui.searchModel.pageCount() {
		ui.searchNext.Deactivate()
	} else {
		ui.searchNext.Activate()
	}
	ui.applyTheme()
	if focusedRow >= 0 {
		ui.focusSearchResult(focusedRow)
	} else if focusedControl && !slices.ContainsFunc(controls, (*fltk.Button).HasFocus) {
		ui.focusSearchResult(0)
	}
}

func (ui *view) renderPreview() {
	if ui.previewGroup == nil {
		return
	}
	item := ui.searchModel.previewItem
	// Only the source's Traditional Chinese title is shown; native (Japanese) titles are not.
	title, description := item.Title, item.Description
	metadata := ""
	status := ""
	if ui.searchModel.phase == searchPhaseLoading {
		if ui.searchModel.active.kind == searchRequestPlayback {
			status = "正在取得集數，準備播放…"
		} else {
			status = "正在載入作品資料…"
		}
	} else if ui.searchModel.requestActive {
		status = "正在取消上一個要求，完成後會顯示預覽…\n\n"
	} else if ui.searchModel.phase == searchPhaseFailed {
		if ui.searchModel.playbackFailed {
			status = "目前無法取得可播放的集數，請檢查網路後按播放重試。"
		} else {
			status = "無法載入作品資料，請返回搜尋結果後重試。"
		}
	} else if detail := ui.searchModel.preview; detail != nil {
		if strings.TrimSpace(detail.Anime.Title) != "" {
			title = detail.Anime.Title
		}
		if strings.TrimSpace(detail.Anime.Description) != "" {
			description = detail.Anime.Description
		}
		if detail.Metadata != nil {
			// The heading keeps the title the user searched for; metadata titles can differ in wording.
			if strings.TrimSpace(detail.Metadata.Description) != "" {
				description = detail.Metadata.Description
			}
			metadata = detailMetadataLabel(detail.Metadata)
		}
	}
	ui.previewTitle.SetLabel(title)
	ui.previewTitle.SetTooltip(title)
	ui.previewMeta.SetLabel(metadata)
	if status != "" && description != "" {
		status += "\n\n"
	}
	ui.previewSynopsis.SetLabel(status + truncateSearchDescription(description))
	if ui.previewCoverImage == nil && ui.previewCover.Label() == "" {
		ui.previewCover.SetLabel("目前沒有封面")
	}
	if _, ok := ui.detailModel.continueRequest(); ok && !ui.searchModel.requestActive && ui.onPlay != nil {
		ui.previewContinue.Show()
	} else {
		ui.previewContinue.Hide()
	}
	if ui.searchModel.phase == searchPhaseFailed || ui.detailEpisodesErr || ui.detailHistoryErr || ui.detailCoverErr {
		ui.previewRetry.Show()
	} else {
		ui.previewRetry.Hide()
	}
	ui.previewBack.SetLabel(ui.detailOrigin.backLabel())
	ui.renderDetailFollow()
	ui.applyTheme()
}

func detailMetadataLabel(metadata *backend.Metadata) string {
	if metadata == nil {
		return ""
	}
	values := make([]string, 0, 4)
	if metadata.Season != "" || metadata.Year != 0 {
		season := metadata.Season
		if metadata.Year != 0 {
			if season != "" {
				season += " "
			}
			season += fmt.Sprint(metadata.Year)
		}
		values = append(values, season)
	}
	if metadata.Studio != "" {
		values = append(values, "製作："+metadata.Studio)
	}
	if metadata.EpisodeCount > 0 {
		values = append(values, fmt.Sprintf("共 %d 集", metadata.EpisodeCount))
	}
	return strings.Join(values, "\n")
}

func (ui *view) searchStatusText() string {
	switch ui.searchModel.phase {
	case searchPhaseLoading:
		if ui.searchModel.active.kind == searchRequestLibrary {
			return "正在載入已儲存的作品…"
		}
		return "正在搜尋…"
	case searchPhaseFailed:
		if ui.searchModel.libraryFailed {
			return "無法載入已儲存作品；按搜尋可重試載入及更新線上結果。"
		}
		return "無法完成搜尋或載入作品資料，請檢查網路後重試。"
	}
	if ui.searchModel.requestActive {
		return "正在取消上一個要求，完成後會繼續這次操作…"
	}
	if strings.TrimSpace(ui.searchModel.query) == "" {
		return "輸入作品名稱以篩選已儲存資料；輸入框按 Enter 更新線上搜尋。結果上按 Enter 開啟預覽。"
	}
	if len(ui.searchModel.results) == 0 {
		if ui.searchModel.remoteQuery == ui.searchModel.query {
			return "線上搜尋完成，沒有符合的作品。請修改關鍵字。"
		}
		return "沒有符合的已儲存作品；按 Enter 搜尋線上作品。"
	}
	if ui.searchModel.remoteQuery == ui.searchModel.query {
		return fmt.Sprintf("線上搜尋完成，共找到 %d 部作品。輸入框按 Enter 更新；結果上按 Enter 開啟預覽。", len(ui.searchModel.results))
	}
	return fmt.Sprintf("找到 %d 部已儲存作品。輸入框按 Enter 更新線上搜尋；結果上按 Enter 開啟預覽。", len(ui.searchModel.results))
}

func searchPageLabel(page, count, total int) string {
	if total == 0 {
		return "沒有搜尋結果"
	}
	start := (page-1)*searchResultPageSize + 1
	end := min(total, start+searchResultPageSize-1)
	return fmt.Sprintf("第 %d–%d 部，共 %d 部（頁 %d/%d）", start, end, total, page, count)
}

func (ui *view) focusSearchInput() {
	if ui.selected == 4 && !ui.searchPreview {
		ui.searchInput.TakeFocus()
	}
}

func (ui *view) deferredSearchAction(action func(), preview bool) func() {
	return func() {
		pageGeneration := ui.pageGeneration
		ui.deferredKeyAction(func() {
			if ui.pageGeneration == pageGeneration && ui.selected == 4 && ui.mainGroup != nil && ui.mainGroup.Visible() && ui.searchPreview == preview {
				action()
			}
		})()
	}
}

func (ui *view) focusSearchResult(index int) {
	if len(ui.searchRows) == 0 {
		if ui.selected == 4 && !ui.searchPreview {
			ui.searchInput.TakeFocus()
		}
		return
	}
	index = max(0, min(len(ui.searchRows)-1, index))
	ui.searchRows[index].TakeFocus()
}

func (ui *view) focusSelectedSearchResult() {
	ui.focusSearchResult(ui.searchModel.selected - ui.searchModel.pageStart())
}

func (ui *view) searchEscape() {
	if ui.selected != 4 {
		return
	}
	if ui.searchPreview {
		ui.leaveDetail()
		return
	}
	ui.goHome()
}

func (ui *view) backToSearchResults() {
	ui.pageGeneration++
	ui.stopDetailLoad()
	ui.searchModel.cancel()
	ui.searchPreview = false
	ui.detailOrigin = detailFromSearch
	ui.previewGroup.Hide()
	ui.searchPage.Show()
	ui.renderSearch()
	ui.deferredSearchAction(ui.focusSelectedSearchResult, false)()
}

func (ui *view) resizeSearch() {
	if ui.searchPage == nil {
		return
	}
	width := max(400, ui.window.W()-259)
	height := max(250, ui.window.H()-131)
	right := 236 + width - 20
	ui.searchPage.Resize(236, 110, width, height)
	ui.previewGroup.Resize(236, 110, width, height)
	ui.searchInput.Resize(252, 118, max(180, width-284), 42)
	ui.searchButton.Resize(236+width-235, 118, 104, 42)
	ui.searchBack.Resize(right-104, 118, 104, 42)
	ui.searchStatus.Resize(252, 169, width-32, 29)
	footerY := max(423, ui.window.H()-57)
	ui.searchScroll.Resize(236, 201, width, max(120, footerY-211))
	ui.searchScroll.ScrollTo(0, 0)
	ui.searchPrevious.Resize(252, footerY, 113, 36)
	ui.searchPageText.Resize(365, footerY+2, max(220, width-450), 28)
	ui.searchNext.Resize(right-113, footerY, 113, 36)
	buttonWidths := [3]int{min(113, max(96, width*22/100)), min(142, max(116, width*28/100)), min(96, max(75, width*19/100))}
	buttonSpacing := 8
	buttonTotalWidth := 0
	for _, buttonWidth := range buttonWidths {
		buttonTotalWidth += buttonWidth
	}
	buttonAvailableWidth := width - 32 - buttonSpacing*(len(buttonWidths)-1)
	if buttonTotalWidth > buttonAvailableWidth {
		for index := range buttonWidths[:len(buttonWidths)-1] {
			buttonWidths[index] = buttonWidths[index] * buttonAvailableWidth / buttonTotalWidth
		}
		buttonWidths[len(buttonWidths)-1] = buttonAvailableWidth
		for _, buttonWidth := range buttonWidths[:len(buttonWidths)-1] {
			buttonWidths[len(buttonWidths)-1] -= buttonWidth
		}
	}
	buttonX := 252
	for index, button := range []*fltk.Button{ui.previewBack, ui.previewContinue, ui.previewRetry} {
		button.Resize(buttonX, 118, buttonWidths[index], 38)
		buttonX += buttonWidths[index] + buttonSpacing
	}
	coverWidth := max(120, width*23/100)
	episodeWidth := max(150, width*34/100)
	episodeX := 236 + width - episodeWidth - 20
	metaX := 252 + coverWidth + 16
	metaWidth := max(150, episodeX-metaX-20)
	ui.previewTitle.Resize(252, 166, max(250, episodeX-272), 42)
	ui.previewCover.Resize(252, 215, coverWidth, min(286, max(160, height-239)))
	ui.previewMeta.Resize(metaX, 212, metaWidth, 65)
	ui.previewDescription.Resize(metaX, 285, metaWidth, max(100, 110+height-285-24))
	ui.previewStatus.Resize(episodeX, 166, episodeWidth, 28)
	episodeFooterY := 110 + height - 76
	ui.previewEpisodes.Resize(episodeX, 197, episodeWidth, max(120, episodeFooterY-205))
	ui.previewEpisodeText.Resize(episodeX, episodeFooterY, max(70, episodeWidth-110), 25)
	ui.previewPrevious.Resize(episodeX, episodeFooterY+28, 92, 34)
	ui.previewNext.Resize(episodeX+episodeWidth-92, episodeFooterY+28, 92, 34)
	ui.resizeDetailEpisodeRows()
	ui.placeFollowIcon()
	for index, button := range ui.searchRows {
		button.Resize(ui.searchScroll.X()+15, ui.searchScroll.Y()+9+index*45, max(300, ui.searchScroll.W()-35), 40)
	}
}

// truncateSearchDescription normalizes spacing within each paragraph, keeps paragraph breaks
// for the scrolling synopsis, and bounds its length.
func truncateSearchDescription(value string) string {
	var paragraphs []string
	for _, paragraph := range strings.Split(value, "\n") {
		if words := strings.Fields(paragraph); len(words) > 0 {
			paragraphs = append(paragraphs, strings.Join(words, " "))
		}
	}
	runes := []rune(strings.Join(paragraphs, "\n"))
	if len(runes) <= maxSynopsisRunes {
		return string(runes)
	}
	return string(runes[:maxSynopsisRunes-1]) + "…"
}

// maxSynopsisRunes bounds the synopsis; the box scrolls, so it can hold more than fits.
const maxSynopsisRunes = 1000
