//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"animeportable/apps/desktop/backend"
)

type followingViewTestService struct {
	*searchViewTestService
	mu        sync.Mutex
	following []backend.Following
	detail    backend.Detail
	followed  []string
}

func (service *followingViewTestService) Library(context.Context) ([]backend.Anime, error) {
	return []backend.Anime{{ID: "watched", Title: "乙作品"}, {ID: "fresh", Title: "甲作品"}}, nil
}

func (service *followingViewTestService) Following(context.Context) ([]backend.Following, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return append([]backend.Following(nil), service.following...), nil
}

func (service *followingViewTestService) Detail(context.Context, string) (backend.Detail, error) {
	return service.detail, nil
}

func (service *followingViewTestService) Follow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.followed = append(service.followed, animeID)
	return nil
}

func (service *followingViewTestService) Unfollow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.following = slices.DeleteFunc(service.following, func(item backend.Following) bool { return item.AnimeID == animeID })
	return nil
}

func newFollowingViewTestFixture(t *testing.T, service *followingViewTestService) *view {
	t.Helper()
	ui := newSearchViewTestFixture(t, service)
	ui.mainGroup.Begin()
	ui.buildFollowingView()
	ui.mainGroup.End()
	return ui
}

func TestFollowingPageShowsStoredStateWhenRefreshFails(t *testing.T) {
	unreachable := func(context.Context, string) ([]backend.Episode, error) { return nil, errors.New("anime1 unreachable") }
	service := &followingViewTestService{
		searchViewTestService: &searchViewTestService{episodes: unreachable},
		following: []backend.Following{
			// NewEpisode no longer moves a series up (owner decision 2026-10-19): 乙作品 still sorts first.
			{AnimeID: "fresh", HasAvailable: true, LatestAvailableNumber: "3", NewEpisode: true},
			{AnimeID: "watched", HasAvailable: true, LatestAvailableNumber: "3", HasWatched: true, LatestWatchedNumber: "2"},
		},
	}
	ui := newFollowingViewTestFixture(t, service)

	ui.showSection(followingSection)
	pumpSearchEvents(t, func() bool {
		return ui.followingLoaded && !ui.followingRefreshing && strings.Contains(ui.followingStatus.Label(), "無法更新最新集數，顯示上次記錄的狀態。")
	})
	if !slices.Equal(ui.followingRowIDs, []string{"watched", "fresh"}) {
		t.Fatalf("rows = %#v, want title order", ui.followingRowIDs)
	}
	if status := ui.followingStatus.Label(); !strings.Contains(status, "追蹤 2 部。") || strings.Contains(status, "有新集數") {
		t.Fatalf("status = %q", status)
	}
	if !ui.followingPage.Visible() || ui.searchPage.Visible() {
		t.Fatal("Following page is not the visible page")
	}
	for _, size := range [][2]int{{1000, 618}, {760, 480}, {1400, 900}} {
		ui.window.Resize(0, 0, size[0], size[1])
		ui.resizeFollowing()
		star := ui.followingButtons[1]
		if star.Label() != "取消追蹤" || star.W() != followIconSize || star.X()+star.W() != ui.followingRefresh.X()+ui.followingRefresh.W() {
			t.Fatalf("window %v: unfollow star at x=%d w=%d, refresh ends at %d", size, star.X(), star.W(), ui.followingRefresh.X()+ui.followingRefresh.W())
		}
	}

	ui.unfollow("watched")
	pumpSearchEvents(t, func() bool { return slices.Equal(ui.followingRowIDs, []string{"fresh"}) })
	if ui.message.Label() != "" {
		t.Fatalf("message after unfollow = %q", ui.message.Label())
	}
}

func TestFollowingPageEmptyState(t *testing.T) {
	ui := newFollowingViewTestFixture(t, &followingViewTestService{searchViewTestService: &searchViewTestService{}})
	ui.showSection(followingSection)
	pumpSearchEvents(t, func() bool { return ui.followingLoaded && !ui.followingRefreshing })
	// The first line is the list state; a refresh note may follow it.
	if status, _, _ := strings.Cut(ui.followingStatus.Label(), "\n"); status != "尚未追蹤任何作品。在作品頁、歷史紀錄或播放中按星號即可追蹤。" || len(ui.followingButtons) != 0 {
		t.Fatalf("status = %q, buttons = %d", status, len(ui.followingButtons))
	}
}

func TestDetailFollowButtonFollowsAndBackReturnsToFollowing(t *testing.T) {
	service := &followingViewTestService{
		searchViewTestService: &searchViewTestService{},
		detail:                backend.Detail{Anime: backend.Anime{ID: "fresh", Title: "甲作品"}},
	}
	ui := newFollowingViewTestFixture(t, service)
	ui.showSection(followingSection)
	ui.openDetailFrom(detailFromFollowing, backend.Anime{ID: "fresh", Title: "甲作品"})
	pumpSearchEvents(t, func() bool { return ui.searchModel.preview != nil && ui.previewFollow.Visible() })
	if ui.previewFollow.Label() != "追蹤" || ui.previewBack.Label() != "返回追蹤" {
		t.Fatalf("follow = %q, back = %q", ui.previewFollow.Label(), ui.previewBack.Label())
	}
	for _, size := range [][2]int{{1000, 618}, {760, 480}, {1400, 900}} {
		ui.window.Resize(0, 0, size[0], size[1])
		ui.resizeSearch()
		rowX, rowWidth := ui.detailEpisodeRowSpan()
		if right := ui.previewFollow.X() + ui.previewFollow.W(); right != rowX+rowWidth {
			t.Fatalf("window %v: follow icon right edge %d, episode rows end at %d", size, right, rowX+rowWidth)
		}
	}

	ui.toggleDetailFollow()
	pumpSearchEvents(t, func() bool { return !ui.followPending && ui.previewFollow.Label() == "取消追蹤" })
	service.mu.Lock()
	followed := slices.Clone(service.followed)
	service.mu.Unlock()
	if !slices.Equal(followed, []string{"fresh"}) {
		t.Fatalf("followed = %#v", followed)
	}

	ui.leaveDetail()
	if ui.selected != followingSection || !ui.followingPage.Visible() || ui.searchPreview {
		t.Fatalf("after back: section %d, following visible %v, preview %v", ui.selected, ui.followingPage.Visible(), ui.searchPreview)
	}
	ui.showSection(4)
	ui.openSearchResult(backend.Anime{ID: "fresh", Title: "甲作品"})
	pumpSearchEvents(t, func() bool { return ui.searchModel.preview != nil })
	if ui.previewBack.Label() != "返回結果" {
		t.Fatalf("back label from Search = %q", ui.previewBack.Label())
	}
}

func TestFollowingRefreshReportsPartialFailureAndIsNotRepeatedSoonAfterSuccess(t *testing.T) {
	var calls atomic.Int32
	var failFresh atomic.Bool
	failFresh.Store(true)
	episodes := func(_ context.Context, animeID string) ([]backend.Episode, error) {
		calls.Add(1)
		if animeID == "fresh" && failFresh.Load() {
			return nil, errors.New("no source ref")
		}
		return nil, nil
	}
	service := &followingViewTestService{
		searchViewTestService: &searchViewTestService{episodes: episodes},
		following:             []backend.Following{{AnimeID: "fresh"}, {AnimeID: "watched"}},
	}
	ui := newFollowingViewTestFixture(t, service)
	ui.showSection(followingSection)
	pumpSearchEvents(t, func() bool {
		return ui.followingLoaded && !ui.followingRefreshing && strings.Contains(ui.followingStatus.Label(), "1 部作品無法更新集數")
	})

	failFresh.Store(false)
	ui.refreshFollowing()
	if !ui.followingRefreshing {
		t.Fatal("更新集數 did not start a refresh soon after the automatic one")
	}
	pumpSearchEvents(t, func() bool { return !ui.followingRefreshing })
	before := calls.Load()
	ui.showSection(0)
	ui.showSection(followingSection)
	pumpSearchEvents(t, func() bool { return ui.followingLoaded })
	if ui.followingRefreshing || calls.Load() != before {
		t.Fatalf("reopening 追蹤 right after a successful refresh fetched again (%d -> %d calls)", before, calls.Load())
	}
	if strings.Contains(ui.followingStatus.Label(), "無法更新") {
		t.Fatalf("status after success = %q", ui.followingStatus.Label())
	}
}

func TestStarPointsStartAtTheTopAndAlternateRadius(t *testing.T) {
	points := starPoints(100, 100, 10)
	if points[0] != [2]int{100, 90} {
		t.Fatalf("first point = %v, want the top outer point", points[0])
	}
	for index, point := range points {
		dx, dy := point[0]-100, point[1]-100
		outer := dx*dx+dy*dy > 50
		if outer != (index%2 == 0) {
			t.Fatalf("point %d = %v: outer = %v", index, point, outer)
		}
	}
}
