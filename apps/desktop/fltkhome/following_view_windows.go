//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

const followingSection = 2

func (ui *view) buildFollowingView() {
	ui.followingPage, ui.followingStatus, ui.followingRefresh, ui.followingScroll = ui.buildRowListPage(func(text *uitext.Strings) string { return text.UpdateEpisodes }, ui.refreshFollowing, func() { ui.followingList().focus(0, 0) })
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
				ui.localizeFollowing = func() { ui.followingItems = followingRows(following, library, history) }
				ui.localizeFollowing()
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
			} else if ui.selected == HomeSection && failures < total {
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
			if err != nil || ui.closing || ui.ctx.Err() != nil || ui.selected != HomeSection || ui.homeLoading {
				return
			}
			if recentUpdateKey(recentUpdateRows(following, nil, rowLimit, time.Now())) != ui.homeUpdateKey {
				ui.refreshHome()
			}
		})
	}()
}

func (ui *view) unfollow(animeID string) {
	ui.setFollowing(animeID, false, func(error) {
		if ui.followingVisible() {
			ui.loadFollowing()
		}
	})
}

// setFollowing follows or unfollows an anime in the background, one change at a time, and
// reports a failure in the message line. done runs on the UI thread unless the window is
// closing. It returns false when the change was not started.
func (ui *view) setFollowing(animeID string, follow bool, done func(error)) bool {
	if ui.service == nil || ui.followPending || ui.closing {
		return false
	}
	ui.followPending = true
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
			ui.setMessage(func() string { return FollowErrorMessage(err) })
			done(err)
		})
	}()
	return true
}

// FollowErrorMessage is the message for a failed follow or unfollow; empty when err is nil.
// The player's follow star shows it too.
func FollowErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return uitext.T().FollowFailed
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
		status = uitext.T().FollowingLoading
	case ui.followingLoadFailed:
		status = uitext.T().FollowingLoadFailed
	case len(ui.followingItems) == 0:
		status = uitext.T().FollowingEmpty
	default:
		status = fmt.Sprintf(uitext.T().FollowingCount, len(ui.followingItems))
	}
	if ui.followingRefreshing {
		status += "\n" + uitext.T().FollowingRefreshing
	} else if ui.followingRefreshFailures > 0 && ui.followingRefreshFailures >= ui.followingRefreshTotal {
		status += "\n" + uitext.T().FollowingRefreshFailed
	} else if ui.followingRefreshFailures > 0 {
		status += "\n" + fmt.Sprintf(uitext.T().FollowingSomeFailed, ui.followingRefreshFailures)
	}
	ui.followingStatus.SetLabel(status)
	ui.followingStatus.Redraw()
}

func (ui *view) followingList() rowList {
	return rowList{
		page: ui.followingPage, status: ui.followingStatus, action: ui.followingRefresh, scroll: ui.followingScroll,
		icons: 1, buttons: &ui.followingButtons, rowIDs: &ui.followingRowIDs,
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
			icons: []rowIcon{{
				label: uitext.T().Unfollow,
				style: func(button *fltk.Button) { ui.styleFollowIcon(button, func() bool { return true }) },
				activate: func() {
					if ui.followingVisible() {
						ui.unfollow(item.ID)
					}
				},
			}},
		})
	}
	ui.renderRowList(ui.followingList(), rows, ui.renderFollowingStatus, ui.focusFollowing)
}

func (ui *view) resizeFollowing() {
	ui.resizeRowList(ui.followingList())
}
