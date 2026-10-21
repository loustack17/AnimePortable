//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

const followingSection = 2

func (ui *view) buildFollowingView() {
	ui.followingPage, ui.followingStatus, ui.followingRefresh, ui.followingScroll = ui.buildRowListPage("更新集數", ui.refreshFollowing, func() { ui.followingList().focus(0, 0) })
}

func (ui *view) followingVisible() bool {
	return !ui.closing && ui.ctx.Err() == nil && ui.selected == followingSection
}

// showFollowing shows the stored list at once and then refreshes episode lists from
// the source in the background.
func (ui *view) showFollowing() {
	ui.followingPage.Show()
	ui.renderFollowing()
	ui.loadFollowing()
	ui.refreshFollowingIfDue()
}

// refreshFollowingIfDue refreshes when the last automatic refresh, successful or not, was
// long enough ago, so a failing source is not asked again on every visit to Home or 追蹤.
func (ui *view) refreshFollowingIfDue() {
	ui.startFollowingRefresh(false)
}

// refreshFollowing is 更新集數: it refreshes now unless a refresh is already running.
func (ui *view) refreshFollowing() {
	ui.startFollowingRefresh(true)
}

func (ui *view) loadFollowing() {
	if ui.service == nil || ui.closing || ui.ctx.Err() != nil {
		return
	}
	ui.followingGeneration++
	generation := ui.followingGeneration
	go func() {
		library, libraryErr := ui.service.Library(ui.ctx)
		following, followingErr := ui.service.Following(ui.ctx)
		history, historyErr := ui.service.History(ui.ctx)
		fltk.Awake(func() {
			if generation != ui.followingGeneration || !ui.followingVisible() {
				return
			}
			ui.followingLoaded = true
			ui.followingLoadFailed = firstError(libraryErr, followingErr, historyErr) != nil
			if !ui.followingLoadFailed {
				ui.followingItems = followingRows(following, library, history)
			}
			ui.renderFollowing()
		})
	}()
}

func (ui *view) startFollowingRefresh(force bool) {
	if ui.service == nil || ui.followingRefreshing || ui.closing || ui.ctx.Err() != nil || !ui.refresh.Attempt("following", force) {
		return
	}
	ui.followingRefreshing = true
	ui.followingRefreshFailures = 0
	ui.renderFollowingStatus()
	go func() {
		// Loading an episode list stores its episode numbers, which Following reads.
		following, err := ui.service.Following(ui.ctx)
		var unreachable atomic.Int32
		jobs := make([]func(context.Context) error, 0, len(following))
		for _, item := range following {
			jobs = append(jobs, func(ctx context.Context) error {
				_, err := ui.service.Episodes(ctx, item.AnimeID)
				if errors.Is(err, backend.ErrOffline) || errors.Is(err, context.DeadlineExceeded) {
					unreachable.Add(1)
				}
				return err
			})
		}
		// Even 更新集數 runs in the background share: nobody waits on any one of these
		// lists, so a detail screen opened meanwhile must not wait behind them.
		result := ui.refresh.RunInBackground(ui.ctx, core.EpisodeRefreshTimeout, jobs...)
		failures, total := result.Failed, result.Total
		// Offline only when every request failed to reach Anime1; a removed show or a
		// storage error is not shown as offline.
		offline := err == nil && total > 0 && int(unreachable.Load()) == total
		if err != nil {
			failures, total = 1, 1
		}
		fltk.Awake(func() {
			ui.followingRefreshing = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			ui.followingRefreshFailures, ui.followingRefreshTotal = failures, total
			ui.homeOffline = offline
			ui.syncHomeNotice()
			if ui.followingVisible() {
				ui.loadFollowing()
			} else if ui.selected == 0 && failures < total {
				// Some episode lists were stored, so 近期更新 may have changed.
				ui.reloadHomeIfUpdatesChanged()
			}
		})
	}()
}

// reloadHomeIfUpdatesChanged reloads Home only when its 近期更新 rows would change, because
// a reload scrolls Home to the top and rebuilds its buttons, losing keyboard focus.
func (ui *view) reloadHomeIfUpdatesChanged() {
	if ui.service == nil || ui.closing || ui.ctx.Err() != nil {
		return
	}
	go func() {
		following, err := ui.service.Following(ui.ctx)
		fltk.Awake(func() {
			if err != nil || ui.closing || ui.ctx.Err() != nil || ui.selected != 0 || ui.homeLoading {
				return
			}
			if recentUpdateKey(recentUpdateRows(following, nil, rowLimit, time.Now())) != ui.homeUpdateKey {
				ui.refreshHome()
			}
		})
	}()
}

func (ui *view) unfollow(animeID string) {
	if ui.service == nil || ui.followPending || ui.closing {
		return
	}
	ui.followPending = true
	go func() {
		err := ui.service.Unfollow(ui.ctx, animeID)
		fltk.Awake(func() {
			ui.followPending = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			ui.message.SetLabel(followErrorMessage(err))
			if ui.followingVisible() {
				ui.loadFollowing()
			}
		})
	}()
}

func followErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return "無法更新追蹤，請重試。"
}

// detailOrigin is the page the detail screen was opened from; the zero value is Search.
type detailOrigin uint8

const (
	detailFromSearch detailOrigin = iota
	detailFromHome
	detailFromFollowing
	detailFromSchedule
)

func (origin detailOrigin) backLabel() string {
	switch origin {
	case detailFromHome:
		return "返回首頁"
	case detailFromFollowing:
		return "返回追蹤"
	case detailFromSchedule:
		return "返回時間表"
	}
	return "返回結果"
}

// openDetailFrom opens an anime's detail screen (which lives in the Search page) so that
// Back and Escape return to origin.
func (ui *view) openDetailFrom(origin detailOrigin, item backend.Anime) {
	if ui.closing {
		return
	}
	ui.showSection(4)
	ui.detailOrigin = origin
	ui.openSearchResult(item)
}

// leaveDetail goes back to where the detail screen was opened from.
func (ui *view) leaveDetail() {
	switch ui.detailOrigin {
	case detailFromHome:
		ui.showSection(0)
		ui.focusContent()
	case detailFromFollowing:
		ui.showSection(followingSection)
		ui.focusFollowing()
	case detailFromSchedule:
		ui.showSection(scheduleSection)
		ui.focusSchedule()
	default:
		ui.backToSearchResults()
	}
}

func (ui *view) focusFollowing() {
	if len(ui.followingButtons) > 0 {
		ui.followingList().focus(0, 0)
		return
	}
	ui.followingRefresh.TakeFocus()
}

func (ui *view) renderFollowingStatus() {
	status := ""
	switch {
	case !ui.followingLoaded:
		status = "正在載入追蹤清單…"
	case ui.followingLoadFailed:
		status = "無法載入追蹤清單，請按「更新集數」重試。"
	case len(ui.followingItems) == 0:
		status = "尚未追蹤任何作品。在作品頁按星號即可追蹤。"
	default:
		status = fmt.Sprintf("追蹤 %d 部。", len(ui.followingItems))
	}
	if ui.followingRefreshing {
		status += "\n正在更新最新集數，先顯示上次記錄。"
	} else if ui.followingRefreshFailures > 0 && ui.followingRefreshFailures >= ui.followingRefreshTotal {
		status += "\n無法更新最新集數，顯示上次記錄的狀態。"
	} else if ui.followingRefreshFailures > 0 {
		status += fmt.Sprintf("\n%d 部作品無法更新集數，顯示上次記錄的狀態。", ui.followingRefreshFailures)
	}
	ui.followingStatus.SetLabel(status)
	ui.followingStatus.Redraw()
}

func (ui *view) followingList() rowList {
	return rowList{
		page: ui.followingPage, status: ui.followingStatus, action: ui.followingRefresh, scroll: ui.followingScroll,
		buttons: &ui.followingButtons, rowIDs: &ui.followingRowIDs,
	}
}

func (ui *view) renderFollowing() {
	rows := make([]listRow, 0, len(ui.followingItems))
	for _, row := range ui.followingItems {
		item := row.Anime
		rows = append(rows, listRow{
			id: item.ID, title: item.Title, status: row.Status, tooltip: row.Status,
			open: func() {
				if ui.followingVisible() {
					ui.openDetailFrom(detailFromFollowing, item)
				}
			},
			iconLabel: "取消追蹤",
			styleIcon: func(button *fltk.Button) { ui.styleFollowIcon(button, func() bool { return true }) },
			activateIcon: func() {
				if ui.followingVisible() {
					ui.unfollow(item.ID)
				}
			},
		})
	}
	ui.renderRowList(ui.followingList(), rows, ui.renderFollowingStatus, ui.focusFollowing)
}

func (ui *view) resizeFollowing() {
	ui.resizeRowList(ui.followingList())
}

func (ui *view) toggleDetailFollow() {
	detail := ui.searchModel.preview
	if detail == nil || ui.service == nil || ui.followPending || ui.closing {
		return
	}
	animeID, follow := detail.Anime.ID, !detail.Following
	ui.followPending = true
	ui.renderPreview()
	go func() {
		var err error
		if follow {
			err = ui.service.Follow(ui.ctx, animeID)
		} else {
			err = ui.service.Unfollow(ui.ctx, animeID)
		}
		fltk.Awake(func() {
			ui.followPending = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			if preview := ui.searchModel.preview; err == nil && preview != nil && preview.Anime.ID == animeID {
				preview.Following = follow
			}
			ui.message.SetLabel(followErrorMessage(err))
			ui.renderPreview()
		})
	}()
}

func (ui *view) renderDetailFollow() {
	detail := ui.searchModel.preview
	if detail == nil || ui.searchModel.requestActive {
		ui.previewFollow.Hide()
		return
	}
	if detail.Following {
		ui.previewFollow.SetLabel("取消追蹤")
	} else {
		ui.previewFollow.SetLabel("追蹤")
	}
	ui.previewFollow.SetTooltip(ui.previewFollow.Label())
	if ui.followPending {
		ui.previewFollow.Deactivate()
	} else {
		ui.previewFollow.Activate()
	}
	ui.previewFollow.Show()
}

// followIconSize is the width of the detail screen's star follow button.
const followIconSize = 40

// placeFollowIcon puts the detail screen's follow icon at the right end of the button
// row, its right edge flush with the episode rows below it.
func (ui *view) placeFollowIcon() {
	x, width := ui.detailEpisodeRowSpan()
	ui.previewFollow.Resize(x+width-followIconSize, 118, followIconSize, 38)
}

// styleFollowIcon draws a follow button as a star: an outline when the anime is not
// followed, filled in the accent color when it is. The label is not drawn; it names the
// action for the tooltip.
func (ui *view) styleFollowIcon(button *fltk.Button, followed func() bool) {
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		if button.HasFocus() {
			roundedRect(x, y, width, height, 13, colors.accent)
		} else {
			roundedRect(x, y, width, height, 13, colors.surface)
		}
		roundedRect(x+3, y+3, width-6, height-6, 11, colors.surface)
		centerX, centerY := x+width/2, y+height/2+1
		points := starPoints(centerX, centerY, 11)
		if followed() {
			fltk.SetDrawColor(colors.accent)
			for index := range points {
				next := points[(index+1)%len(points)]
				fltk.DrawPolygon(centerX, centerY, points[index][0], points[index][1], next[0], next[1])
			}
			return
		}
		fltk.SetDrawColor(colors.muted)
		fltk.SetLineStyle(fltk.SOLID, 2)
		for index := range points {
			next := points[(index+1)%len(points)]
			fltk.DrawLine(points[index][0], points[index][1], next[0], next[1])
		}
		fltk.SetLineStyle(fltk.SOLID, 0)
	})
}

// starPoints returns the ten corners of a five-pointed star, alternating outer and
// inner points, starting at the top.
func starPoints(centerX, centerY, radius int) [10][2]int {
	var points [10][2]int
	for index := range points {
		length := float64(radius)
		if index%2 == 1 {
			length *= 0.45
		}
		angle := -math.Pi/2 + float64(index)*math.Pi/5
		points[index] = [2]int{centerX + int(math.Round(length*math.Cos(angle))), centerY + int(math.Round(length*math.Sin(angle)))}
	}
	return points
}
