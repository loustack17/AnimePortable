//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

const followingSection = 2

// followingRefreshTimeout bounds each anime's episode fetch, so one slow series cannot
// stall the refresh of the others.
const followingRefreshTimeout = 15 * time.Second

// followingRefreshInterval is the least time between automatic refreshes, so returning to
// Home or 追蹤 does not refetch every followed anime; 更新集數 forces one.
const followingRefreshInterval = 10 * time.Minute

func (ui *view) buildFollowingView() {
	ui.followingPage = fltk.NewGroup(236, 110, 741, 487)
	ui.followingPage.Begin()
	ui.followingStatus = fltk.NewBox(fltk.NO_BOX, 252, 118, 580, 40, "")
	ui.followingStatus.SetLabelSize(12)
	ui.followingStatus.SetAlign(fltk.ALIGN_LEFT | fltk.ALIGN_INSIDE | fltk.ALIGN_WRAP)
	ui.followingRefresh = fltk.NewButton(845, 118, 112, 40, "更新集數")
	ui.styleButton(ui.followingRefresh, nil)
	ui.bindButton(ui.followingRefresh, ui.refreshFollowing)
	ui.followingScroll = fltk.NewScroll(236, 166, 741, 431)
	ui.followingScroll.SetType(fltk.SCROLL_VERTICAL)
	ui.followingScroll.SetBox(fltk.FLAT_BOX)
	ui.followingScroll.End()
	ui.followingPage.End()
	ui.followingPage.Hide()
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

func (ui *view) refreshFollowingIfDue() {
	if ui.followingRefreshAttemptedAt.IsZero() || time.Since(ui.followingRefreshAttemptedAt) >= followingRefreshInterval {
		ui.refreshFollowing()
	}
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

func (ui *view) refreshFollowing() {
	if ui.service == nil || ui.followingRefreshing || ui.closing || ui.ctx.Err() != nil {
		return
	}
	ui.followingRefreshing = true
	ui.followingRefreshAttemptedAt = time.Now()
	ui.followingRefreshFailures = 0
	ui.renderFollowingStatus()
	go func() {
		// Loading an episode list stores its episode numbers, which Following reads.
		following, err := ui.service.Following(ui.ctx)
		failures, total := 0, len(following)
		if err != nil {
			failures, total = 1, 1
		}
		for _, item := range following {
			if ui.ctx.Err() != nil {
				break
			}
			ctx, cancel := context.WithTimeout(ui.ctx, followingRefreshTimeout)
			if _, err := ui.service.Episodes(ctx, item.AnimeID); err != nil {
				failures++
			}
			cancel()
		}
		fltk.Awake(func() {
			ui.followingRefreshing = false
			if ui.closing || ui.ctx.Err() != nil {
				return
			}
			ui.followingRefreshFailures, ui.followingRefreshTotal = failures, total
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
)

func (origin detailOrigin) backLabel() string {
	switch origin {
	case detailFromHome:
		return "返回首頁"
	case detailFromFollowing:
		return "返回追蹤"
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
	default:
		ui.backToSearchResults()
	}
}

func (ui *view) focusFollowing() {
	if len(ui.followingButtons) > 0 {
		ui.followingButtons[0].TakeFocus()
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

func (ui *view) renderFollowing() {
	if ui.followingPage == nil {
		return
	}
	focusedID, focusedRow, focusedColumn := "", -1, 0
	for index, button := range ui.followingButtons {
		if button.HasFocus() && index/2 < len(ui.followingRowIDs) {
			focusedID, focusedRow, focusedColumn = ui.followingRowIDs[index/2], index/2, index%2
		}
		ui.followingScroll.Remove(button)
		button.Destroy()
	}
	ui.followingButtons = nil
	ui.followingRowIDs = nil
	// Keep the scroll position: a background refresh must not jump the list to the top.
	scrollY := ui.followingScroll.YPosition()
	ui.followingScroll.ScrollTo(0, 0)
	ui.followingScroll.Begin()
	for index, row := range ui.followingItems {
		open := fltk.NewButton(0, 0, 1, 1, row.Anime.Title)
		open.SetTooltip(row.Status)
		ui.styleFollowingRow(open, row)
		item := row.Anime
		ui.bindButton(open, func() {
			if ui.followingVisible() {
				ui.openDetailFrom(detailFromFollowing, item)
			}
		})
		remove := fltk.NewButton(0, 0, 1, 1, "取消追蹤")
		remove.SetTooltip("取消追蹤")
		ui.styleFollowIcon(remove, func() bool { return true })
		ui.bindButton(remove, func() {
			if ui.followingVisible() {
				ui.unfollow(item.ID)
			}
		})
		ui.followingButtons = append(ui.followingButtons, open, remove)
		ui.followingRowIDs = append(ui.followingRowIDs, item.ID)
		ui.placeFollowingRow(index)
	}
	ui.followingScroll.End()
	if len(ui.followingRowIDs) > 0 {
		lastRowBottom := 9 + len(ui.followingRowIDs)*64
		ui.followingScroll.ScrollTo(0, max(0, min(scrollY, lastRowBottom-ui.followingScroll.H())))
	}
	ui.renderFollowingStatus()
	ui.applyTheme()
	if focusedRow < 0 {
		return
	}
	// The focused row may be gone after 取消追蹤; keep focus on the row now in its place.
	target := slices.Index(ui.followingRowIDs, focusedID)
	if target < 0 {
		target = min(focusedRow, len(ui.followingRowIDs)-1)
	}
	if target < 0 {
		ui.followingRefresh.TakeFocus()
		return
	}
	ui.followingButtons[target*2+focusedColumn].TakeFocus()
}

func (ui *view) placeFollowingRow(index int) {
	scroll := ui.followingScroll
	removeWidth := followIconSize
	// The star's right edge lines up with the 更新集數 button above it.
	openWidth := max(220, scroll.W()-35-removeWidth-8)
	y := scroll.Y() + 9 + index*64
	ui.followingButtons[index*2].Resize(scroll.X()+15, y, openWidth, 56)
	ui.followingButtons[index*2+1].Resize(scroll.X()+15+openWidth+8, y+8, removeWidth, 40)
}

// styleFollowingRow draws the title above the episode state.
func (ui *view) styleFollowingRow(button *fltk.Button, row followingRow) {
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
		fltk.PushClip(x+3, y+3, width-6, height-6)

		fltk.SetDrawColor(colors.ink)
		fltk.SetDrawFont(fltk.HELVETICA_BOLD, 15)
		fltk.Draw(row.Anime.Title, x+18, y+6, width-36, 24, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
		fltk.SetDrawColor(colors.muted)
		fltk.SetDrawFont(fltk.HELVETICA, 12)
		fltk.Draw(row.Status, x+18, y+30, width-36, 20, fltk.ALIGN_LEFT|fltk.ALIGN_INSIDE|fltk.ALIGN_CLIP)
		fltk.PopClip()
	})
}

func (ui *view) resizeFollowing() {
	if ui.followingPage == nil {
		return
	}
	width := max(400, ui.window.W()-259)
	height := max(250, ui.window.H()-131)
	ui.followingPage.Resize(236, 110, width, height)
	ui.followingRefresh.Resize(236+width-20-112, 118, 112, 40)
	ui.followingStatus.Resize(252, 118, max(150, width-32-112-12), 40)
	ui.followingScroll.Resize(236, 166, width, max(120, height-56))
	ui.followingScroll.ScrollTo(0, 0)
	for index := range ui.followingRowIDs {
		ui.placeFollowingRow(index)
	}
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

// followIconSize is the width of the star follow buttons.
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
