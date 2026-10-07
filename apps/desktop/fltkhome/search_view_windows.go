//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"fmt"
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
		enter := fltk.EventKey() == fltk.ENTER_KEY || fltk.EventKey() == 13 || fltk.EventKey() == 0xff8d
		ui.searchInputVersion++
		inputVersion := ui.searchInputVersion
		pageGeneration := ui.pageGeneration
		fltk.AddTimeout(0.01, func() {
			if ui.ctx.Err() != nil || ui.pageGeneration != pageGeneration || ui.searchInputVersion != inputVersion || ui.selected != 4 || ui.searchPreview || ui.mainGroup == nil || !ui.mainGroup.Visible() {
				return
			}
			ui.searchModel.setQuery(query)
			ui.renderSearch()
			if enter {
				ui.submitSearch()
			}
		})
	})
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
	ui.searchPrevious = fltk.NewButton(252, 562, 113, 36, "上一頁")
	ui.styleButton(ui.searchPrevious, nil)
	ui.bindSearchButton(ui.searchPrevious, func() {
		ui.searchModel.movePage(-1)
		ui.renderSearch()
	})
	ui.searchNext = fltk.NewButton(844, 562, 113, 36, "下一頁")
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
	ui.bindSearchButton(ui.previewBack, ui.backToSearchResults)
	ui.previewTitle = fltk.NewBox(fltk.NO_BOX, 252, 174, 705, 46, "")
	ui.previewTitle.SetLabelFont(fltk.HELVETICA_BOLD)
	ui.previewTitle.SetLabelSize(22)
	ui.previewTitle.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewNative = fltk.NewBox(fltk.NO_BOX, 252, 226, 705, 38, "")
	ui.previewNative.SetLabelSize(15)
	ui.previewNative.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewDescription = fltk.NewBox(fltk.NO_BOX, 252, 278, 705, 300, "")
	ui.previewDescription.SetLabelSize(14)
	ui.previewDescription.SetAlign(fltk.ALIGN_TOP | fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.previewGroup.End()
	ui.previewGroup.Hide()

	ui.window.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.SHORTCUT || ui.mainGroup == nil || !ui.mainGroup.Visible() {
			return false
		}
		key := fltk.EventKey()
		state := fltk.EventState()
		if ui.selected == 4 && (key == 27 || key == 0xff1b) {
			ui.deferredSearchAction(ui.searchEscape, ui.searchPreview)()
			return true
		}
		if ui.selected == 4 && ui.searchInput.HasFocus() {
			return false
		}
		if key == 'k' || key == 'K' {
			if state&fltk.CTRL != 0 {
				ui.deferredMainAction(func() {
					ui.showSection(4)
					if ui.searchPreview {
						ui.backToSearchResults()
					}
					ui.focusSearchInput()
				})()
				return true
			}
		}
		if key == '/' {
			ui.deferredMainAction(func() {
				ui.showSection(4)
				if ui.searchPreview {
					ui.backToSearchResults()
				}
				ui.focusSearchInput()
			})()
			return true
		}
		return false
	})
}

func (ui *view) bindSearchButton(button *fltk.Button, action func()) {
	previewAction := button == ui.previewBack
	activate := ui.deferredSearchAction(action, previewAction)
	button.SetCallback(activate)
	button.SetEventHandler(func(event fltk.Event) bool {
		if event != fltk.KEYDOWN {
			return false
		}
		switch fltk.EventKey() {
		case fltk.ENTER_KEY, 13, 0xff8d:
			activate()
		case 27, 0xff1b:
			ui.deferredSearchAction(ui.searchEscape, previewAction)()
		case 0xff52:
			if button == ui.searchNext {
				ui.deferredSearchAction(func() { ui.focusSearchResult(max(0, len(ui.searchRows)-1)) }, false)()
			} else if button == ui.searchPrevious {
				ui.deferredSearchAction(func() { ui.focusSearchResult(max(0, len(ui.searchRows)-1)) }, false)()
			} else {
				return false
			}
		case 0xff54:
			if button == ui.searchPrevious || button == ui.searchButton {
				ui.deferredSearchAction(func() { ui.focusSearchResult(0) }, false)()
			} else {
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
			if ui.ctx.Err() != nil {
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
			if ui.ctx.Err() != nil {
				return
			}
			if ui.selected == 4 {
				ui.refreshSearchView()
			}
		})
	}()
}

func (ui *view) openSearchResult(item backend.Anime) {
	ui.pageGeneration++
	request, ctx, ok := ui.searchModel.beginDetail(item)
	if !ok {
		ui.searchModel.queueDetail(item)
		ui.searchPreview = true
		ui.searchPage.Hide()
		ui.previewGroup.Show()
		ui.renderPreview()
		ui.previewBack.TakeFocus()
		return
	}
	ui.searchPreview = true
	ui.searchPage.Hide()
	ui.previewGroup.Show()
	ui.renderPreview()
	ui.previewBack.TakeFocus()
	if ui.service == nil {
		ui.searchModel.finishDetail(request, backend.Detail{}, fmt.Errorf("service unavailable"))
		ui.renderPreview()
		return
	}
	go func() {
		detail, err := ui.service.Detail(ctx, item.ID)
		fltk.Awake(func() {
			ui.searchModel.finishDetail(request, detail, err)
			if ui.ctx.Err() != nil {
				return
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
			switch fltk.EventKey() {
			case fltk.ENTER_KEY, 13, 0xff8d:
				ui.deferredSearchAction(openItem, false)()
			case 0xff52:
				ui.deferredSearchAction(func() {
					ui.searchModel.selected = selectedIndex
					ui.searchModel.moveSelection(-1)
					ui.renderSearch()
					ui.focusSelectedSearchResult()
				}, false)()
			case 0xff54:
				ui.deferredSearchAction(func() {
					ui.searchModel.selected = selectedIndex
					ui.searchModel.moveSelection(1)
					ui.renderSearch()
					ui.focusSelectedSearchResult()
				}, false)()
			case 27, 0xff1b:
				ui.deferredSearchAction(ui.searchEscape, false)()
			default:
				return false
			}
			return true
		})
		ui.searchRows = append(ui.searchRows, button)
	}
	ui.searchScroll.End()
	ui.searchStatus.SetLabel(ui.searchStatusText())
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
}

func (ui *view) renderPreview() {
	if ui.previewGroup == nil {
		return
	}
	item := ui.searchModel.previewItem
	title, native, description := item.Title, item.NativeTitle, item.Description
	status := ""
	if ui.searchModel.phase == searchPhaseLoading {
		status = "正在載入作品資料…"
	} else if ui.searchModel.requestActive {
		status = "正在取消上一個要求，完成後會顯示預覽…\n\n"
	} else if ui.searchModel.phase == searchPhaseFailed {
		status = "無法載入作品資料，請返回搜尋結果後重試。"
	} else if detail := ui.searchModel.preview; detail != nil {
		if strings.TrimSpace(detail.Anime.Title) != "" {
			title = detail.Anime.Title
		}
		if strings.TrimSpace(detail.Anime.NativeTitle) != "" {
			native = detail.Anime.NativeTitle
		}
		if strings.TrimSpace(detail.Anime.Description) != "" {
			description = detail.Anime.Description
		}
		if detail.Metadata != nil {
			if strings.TrimSpace(detail.Metadata.Title) != "" {
				title = detail.Metadata.Title
			}
			if strings.TrimSpace(detail.Metadata.NativeTitle) != "" {
				native = detail.Metadata.NativeTitle
			}
			if strings.TrimSpace(detail.Metadata.Description) != "" {
				description = detail.Metadata.Description
			}
		}
	}
	ui.previewTitle.SetLabel(title)
	ui.previewTitle.SetTooltip(title)
	ui.previewNative.SetLabel(native)
	ui.previewNative.SetTooltip(native)
	if status != "" && description != "" {
		status += "\n\n"
	}
	ui.previewDescription.SetLabel(status + truncateSearchDescription(description))
	ui.applyTheme()
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

func (ui *view) deferredMainAction(action func()) func() {
	return func() {
		pageGeneration := ui.pageGeneration
		ui.deferredKeyAction(func() {
			if ui.pageGeneration == pageGeneration && ui.mainGroup != nil && ui.mainGroup.Visible() {
				action()
			}
		})()
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
		ui.backToSearchResults()
		return
	}
	ui.showSection(0)
	ui.navigation[0].TakeFocus()
}

func (ui *view) backToSearchResults() {
	ui.pageGeneration++
	ui.searchModel.cancel()
	ui.searchPreview = false
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
	footerY := max(423, ui.window.H()-56)
	ui.searchScroll.Resize(236, 201, width, max(120, footerY-211))
	ui.searchScroll.ScrollTo(0, 0)
	ui.searchPrevious.Resize(252, footerY, 113, 36)
	ui.searchPageText.Resize(365, footerY+2, max(220, width-450), 28)
	ui.searchNext.Resize(right-113, footerY, 113, 36)
	ui.previewBack.Resize(252, 118, 113, 38)
	ui.previewTitle.Resize(252, 174, width-32, 46)
	ui.previewNative.Resize(252, 226, width-32, 38)
	ui.previewDescription.Resize(252, 278, width-32, max(140, height-168))
	for index, button := range ui.searchRows {
		button.Resize(ui.searchScroll.X()+15, ui.searchScroll.Y()+9+index*45, max(300, ui.searchScroll.W()-35), 40)
	}
}

func truncateSearchDescription(value string) string {
	runes := []rune(strings.Join(strings.Fields(value), " "))
	if len(runes) <= 320 {
		return string(runes)
	}
	return string(runes[:319]) + "…"
}
