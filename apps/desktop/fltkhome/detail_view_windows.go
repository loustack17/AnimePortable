//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

func (ui *view) loadDetailResources(generation uint64, animeID string) {
	if ui.service == nil || ui.detailCancel != nil || ui.closing {
		return
	}
	ctx, cancel := context.WithCancel(ui.ctx)
	ui.detailCancel = cancel
	if coverService, ok := ui.service.(interface {
		GetCover(context.Context, string) (backend.Cover, error)
	}); ok {
		metadataService, canRefresh := ui.service.(interface {
			RefreshMetadata(context.Context, string) (bool, error)
		})
		// Anime1 has no covers or synopses: look the anime up when nothing is stored for it yet.
		hasMetadata := ui.searchModel.preview != nil && ui.searchModel.preview.Metadata != nil
		canRefresh = canRefresh && !hasMetadata && !ui.metadataLookedUp[animeID]
		if canRefresh {
			if ui.metadataLookedUp == nil {
				ui.metadataLookedUp = make(map[string]bool)
			}
			ui.metadataLookedUp[animeID] = true
		}
		go func() {
			cover, err := coverService.GetCover(ctx, animeID)
			var refreshed *backend.Detail
			var refreshErr error
			if errors.Is(err, core.ErrNotFound) && canRefresh {
				var found bool
				refreshErr = context.Canceled // until the lookup gets a refresh slot and runs
				ui.refresh.Run(ctx, core.MetadataRefreshTimeout, func(lookupCtx context.Context) error {
					found, refreshErr = metadataService.RefreshMetadata(lookupCtx, animeID)
					return refreshErr
				})
				if refreshErr == nil && found {
					if detail, detailErr := ui.service.Detail(ctx, animeID); detailErr == nil {
						refreshed = &detail
					}
					cover, err = coverService.GetCover(ctx, animeID)
				}
			}
			noCover := errors.Is(err, core.ErrNotFound)
			var pixels []byte
			if err == nil {
				pixels, err = decodeDetailCover(cover.Bytes, cover.MediaType)
			}
			fltk.Awake(func() {
				if canRefresh && (ctx.Err() != nil || refreshErr != nil) {
					// The user left, or the providers failed; allow another lookup on the next open.
					// Only a completed lookup with no confident match is not repeated this session.
					delete(ui.metadataLookedUp, animeID)
				}
				if !ui.detailRequestCurrent(generation, animeID) || ctx.Err() != nil {
					return
				}
				if refreshed != nil && ui.searchModel.preview != nil && ui.searchModel.preview.Anime.ID == animeID {
					ui.searchModel.preview = refreshed
				}
				if noCover {
					ui.detailCoverErr = false
					ui.previewCover.SetLabel(uitext.T().CoverNone)
				} else if err == nil {
					image, imageErr := newDetailCoverImage(pixels)
					if imageErr == nil {
						ui.releaseDetailCover()
						ui.previewCoverImage = image
						ui.detailCoverErr = false
						ui.previewCover.SetLabel("")
					} else {
						ui.detailCoverErr = true
						ui.previewCover.SetLabel(uitext.T().CoverFailed)
					}
				} else {
					ui.detailCoverErr = true
					ui.previewCover.SetLabel(uitext.T().CoverFailed)
				}
				ui.renderPreview()
				ui.previewCover.Redraw()
			})
		}()
	} else {
		ui.previewCover.SetLabel(uitext.T().CoverNone)
	}
	// The stored list shows at once; Anime1's answer replaces it. A first answer for a list
	// takes Anime1 one to three seconds (Loop 37d).
	if stored, ok := ui.service.(interface {
		StoredEpisodes(context.Context, string) ([]backend.Episode, error)
	}); ok {
		go func() {
			episodes, err := stored.StoredEpisodes(ctx, animeID)
			fltk.Awake(func() {
				if err != nil || !ui.detailRequestCurrent(generation, animeID) || ctx.Err() != nil || !ui.detailModel.applyStored(episodes) {
					return
				}
				ui.renderPreview()
				ui.renderDetailEpisodes()
			})
		}()
	}
	go func() {
		episodes, err := ui.service.Episodes(ctx, animeID)
		fltk.Awake(func() {
			if !ui.detailRequestCurrent(generation, animeID) || ctx.Err() != nil {
				return
			}
			ui.detailEpisodesDone = true
			ui.detailEpisodesErr = ui.detailModel.applyRefresh(episodes, err)
			ui.renderPreview()
			ui.renderDetailEpisodes()
		})
	}()
	go func() {
		history, err := ui.service.History(ctx)
		fltk.Awake(func() {
			if !ui.detailRequestCurrent(generation, animeID) || ctx.Err() != nil {
				return
			}
			ui.detailHistoryErr = err != nil
			if err == nil {
				ui.detailModel.setHistory(history)
			}
			ui.renderPreview()
			ui.renderDetailEpisodes()
		})
	}()
}

func (ui *view) detailRequestCurrent(generation uint64, animeID string) bool {
	return !ui.closing && ui.ctx.Err() == nil && ui.selected == 4 && ui.searchPreview && ui.detailGeneration == generation && ui.detailModel.animeID == animeID
}

func (ui *view) stopDetailLoad() {
	ui.detailGeneration++
	if ui.detailCancel != nil {
		ui.detailCancel()
		ui.detailCancel = nil
	}
	ui.releaseDetailCover()
	ui.detailModel.close()
}

func (ui *view) pauseDetailResources() {
	ui.detailGeneration++
	if ui.detailCancel != nil {
		ui.detailCancel()
		ui.detailCancel = nil
		if ui.previewCover != nil && ui.previewCover.Label() == uitext.T().CoverLoading {
			ui.detailCoverErr = true
			ui.previewCover.SetLabel(uitext.T().CoverFailed)
			ui.previewRetry.Show()
		}
	}
}

func (ui *view) releaseDetailCover() {
	if ui.previewCoverImage != nil {
		ui.previewCoverImage.Destroy()
		ui.previewCoverImage = nil
	}
}

func (ui *view) retryDetailLoad() {
	item := ui.searchModel.previewItem
	if item.ID == "" || ui.service == nil {
		return
	}
	ui.stopDetailLoad()
	ui.detailModel.open(item.ID)
	ui.detailEpisodesDone = false
	ui.detailEpisodesErr = false
	ui.detailHistoryErr = false
	ui.detailCoverErr = false
	ui.previewCover.SetLabel(uitext.T().CoverLoading)
	request, ctx, ok := ui.searchModel.beginDetail(item)
	if !ok {
		ui.searchModel.queueDetail(item)
		ui.renderPreview()
		return
	}
	ui.renderPreview()
	generation := ui.detailGeneration
	go func() {
		detail, err := ui.service.Detail(ctx, item.ID)
		fltk.Awake(func() {
			applied := ui.searchModel.finishDetail(request, detail, err)
			if applied && err == nil && ui.detailGeneration == generation && ui.searchPreview {
				ui.loadDetailResources(generation, item.ID)
			}
			if ui.selected == 4 {
				ui.refreshSearchView()
			}
		})
	}()
}

func (ui *view) playSelectedDetailEpisode() {
	request, ok := ui.detailModel.playRequest()
	if !ok || ui.onPlay == nil || ui.closing {
		return
	}
	ui.pageGeneration++
	ui.pauseDetailResources()
	ui.onPlay(request)
}

func (ui *view) continueDetailEpisode() {
	request, ok := ui.detailModel.continueRequest()
	if !ok || ui.onPlay == nil || ui.closing {
		return
	}
	ui.pageGeneration++
	ui.pauseDetailResources()
	ui.onPlay(request)
}

func (ui *view) playDetailEpisodeAt(index int) {
	ui.detailModel.selectEpisode(index)
	ui.renderDetailEpisodes()
	ui.focusSelectedDetailEpisode()
	ui.playSelectedDetailEpisode()
}

func (ui *view) focusSelectedDetailEpisode() {
	index := ui.detailModel.selected - ui.detailModel.page*detailEpisodePageSize
	ui.focusDetailEpisodeRow(index)
}

func (ui *view) focusDetailEpisodeRow(index int) {
	if index < 0 || index >= len(ui.previewEpisodeRows) || ui.previewEpisodes == nil {
		return
	}
	row := ui.previewEpisodeRows[index]
	scrollIntoView(ui.previewEpisodes, row)
	row.TakeFocus()
}

// focusDetailEpisodes focuses the selected episode, or the first one on the page.
func (ui *view) focusDetailEpisodes() {
	index := ui.detailModel.selected - ui.detailModel.page*detailEpisodePageSize
	if index < 0 || index >= len(ui.previewEpisodeRows) {
		index = 0
	}
	ui.focusDetailEpisodeRow(index)
}

func (ui *view) resizeDetailEpisodeRows() {
	if ui.previewEpisodes == nil {
		return
	}
	focusedIndex := -1
	for index, row := range ui.previewEpisodeRows {
		if row.HasFocus() {
			focusedIndex = index
			break
		}
	}
	ui.previewEpisodes.ScrollTo(0, 0)
	for index, row := range ui.previewEpisodeRows {
		x, width := ui.detailEpisodeRowSpan()
		row.Resize(x, ui.previewEpisodes.Y()+7+index*37, width, 33)
	}
	if focusedIndex >= 0 {
		ui.focusDetailEpisodeRow(focusedIndex)
	}
}

// detailEpisodeRowSpan is the x and width of an episode row: inset from the scroll area
// and clear of its scrollbar. The follow icon lines up with the same right edge.
func (ui *view) detailEpisodeRowSpan() (int, int) {
	return ui.previewEpisodes.X() + 8, max(120, ui.previewEpisodes.W()-27)
}

func (ui *view) renderDetailEpisodes() {
	if ui.previewEpisodes == nil {
		return
	}
	focusedEpisodeID := ""
	for index, button := range ui.previewEpisodeRows {
		if button.HasFocus() && index < len(ui.previewEpisodeIDs) {
			focusedEpisodeID = ui.previewEpisodeIDs[index]
			break
		}
	}
	for _, button := range ui.previewEpisodeRows {
		ui.previewEpisodes.Remove(button)
		button.Destroy()
	}
	ui.previewEpisodeRows = nil
	ui.previewEpisodeIDs = nil
	ui.previewEpisodes.ScrollTo(0, 0)
	ui.previewEpisodes.Begin()
	for offset, episode := range ui.detailModel.visibleEpisodes() {
		index := ui.detailModel.page*detailEpisodePageSize + offset
		label := episode.Number
		if label != "" {
			label = fmt.Sprintf(uitext.T().EpisodeNumber, label)
		} else {
			label = fmt.Sprintf(uitext.T().EpisodeIndex, index+1)
		}
		if episode.Title != "" {
			label += uitext.T().Separator + episode.Title
		}
		if history, ok := ui.detailModel.historyEntry(episode); ok {
			if history.Completed {
				label += uitext.T().Separator + uitext.T().EpisodeWatched
			} else if history.Position > 0 {
				label += uitext.T().Separator + episodeProgressLabel(history)
			}
		}
		x, width := ui.detailEpisodeRowSpan()
		button := fltk.NewButton(x, ui.previewEpisodes.Y()+7+offset*37, width, 33, label)
		selectedIndex := index
		ui.styleButton(button, func() bool { return ui.detailModel.selected == selectedIndex })
		button.SetCallback(ui.deferredSearchAction(func() { ui.playDetailEpisodeAt(selectedIndex) }, true))
		button.SetEventHandler(func(event fltk.Event) bool {
			if event != fltk.KEYDOWN {
				return false
			}
			return ui.handleDetailEpisodeKey(selectedIndex, classifyKey(fltk.EventKey(), fltk.EventState()))
		})
		ui.previewEpisodeRows = append(ui.previewEpisodeRows, button)
		ui.previewEpisodeIDs = append(ui.previewEpisodeIDs, episode.ID)
	}
	ui.previewEpisodes.End()
	if ui.detailEpisodesErr {
		ui.previewStatus.SetLabel(uitext.T().EpisodesLoadFailed)
	} else if !ui.detailEpisodesDone && len(ui.detailModel.episodes) == 0 {
		ui.previewStatus.SetLabel(uitext.T().EpisodesLoading)
	} else if len(ui.detailModel.episodes) == 0 {
		ui.previewStatus.SetLabel(uitext.T().NoEpisodes)
	} else {
		ui.previewStatus.SetLabel("")
	}
	if ui.detailHistoryErr {
		ui.previewStatus.SetLabel(ui.previewStatus.Label() + uitext.T().Separator + uitext.T().ProgressUnavailable)
	}
	pageCount := (len(ui.detailModel.episodes) + detailEpisodePageSize - 1) / detailEpisodePageSize
	if len(ui.detailModel.episodes) == 0 {
		ui.previewEpisodeText.SetLabel(uitext.T().NoEpisodesShort)
	} else {
		start := ui.detailModel.page*detailEpisodePageSize + 1
		end := min(len(ui.detailModel.episodes), start+detailEpisodePageSize-1)
		ui.previewEpisodeText.SetLabel(fmt.Sprintf(uitext.T().EpisodeRange, start, end, len(ui.detailModel.episodes), ui.detailModel.page+1, pageCount))
	}
	if ui.detailModel.page == 0 {
		ui.previewPrevious.Deactivate()
	} else {
		ui.previewPrevious.Activate()
	}
	if pageCount == 0 || ui.detailModel.page >= pageCount-1 {
		ui.previewNext.Deactivate()
	} else {
		ui.previewNext.Activate()
	}
	ui.applyTheme()
	if focusedEpisodeID != "" {
		for index, episodeID := range ui.previewEpisodeIDs {
			if episodeID == focusedEpisodeID {
				ui.focusDetailEpisodeRow(index)
				break
			}
		}
	}
}

func (ui *view) handleDetailEpisodeKey(index int, command keyCommand) bool {
	switch command {
	case keyActivate:
		ui.deferredSearchAction(func() { ui.playDetailEpisodeAt(index) }, true)()
	case keyUp:
		if index == 0 {
			// Above the first episode are 返回 and the other buttons.
			ui.deferredSearchAction(func() { ui.previewBack.TakeFocus() }, true)()
			return true
		}
		ui.deferredSearchAction(func() {
			ui.detailModel.selectEpisode(index)
			ui.detailModel.moveSelection(-1)
			ui.renderDetailEpisodes()
			ui.focusSelectedDetailEpisode()
		}, true)()
	case keyDown:
		ui.deferredSearchAction(func() {
			ui.detailModel.selectEpisode(index)
			ui.detailModel.moveSelection(1)
			ui.renderDetailEpisodes()
			ui.focusSelectedDetailEpisode()
		}, true)()
	default:
		return false
	}
	return true
}

func playbackPositionLabel(milliseconds int64) string {
	seconds := max(0, int(milliseconds/1000))
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

func episodeProgressLabel(history backend.History) string {
	if history.Duration <= 0 {
		return fmt.Sprintf(uitext.T().WatchedPosition, playbackPositionLabel(history.Position))
	}
	return fmt.Sprintf("%s / %s", playbackPositionLabel(history.Position), playbackPositionLabel(history.Duration))
}
