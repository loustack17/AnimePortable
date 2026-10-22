//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

type homeRefreshTestService struct {
	noSettingsActions
	mu          sync.Mutex
	history     []backend.History
	historyErr  error
	historyCall int
	episodes    map[string][]backend.Episode
	blockCall   int
	entered     chan struct{}
	release     chan struct{}
	following   []backend.Following
	// fetch, when set, stands in for the network on every Episodes call.
	fetch   func(context.Context, string) error
	fetched []string
	// library replaces the single stored anime when set.
	library []backend.Anime
	// schedule is returned by both the stored and the fetched schedule.
	schedule []backend.ScheduleItem
}

func (service *homeRefreshTestService) Start(context.Context) error { return nil }
func (service *homeRefreshTestService) Settings(context.Context) (backend.Settings, error) {
	return backend.Settings{Appearance: "light"}, nil
}
func (service *homeRefreshTestService) SaveSettings(context.Context, backend.Settings) error {
	return nil
}
func (service *homeRefreshTestService) Library(context.Context) ([]backend.Anime, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.library != nil {
		return append([]backend.Anime(nil), service.library...), nil
	}
	return []backend.Anime{{ID: "anime", Title: "作品"}}, nil
}
func (service *homeRefreshTestService) Search(context.Context, string) ([]backend.Anime, error) {
	return nil, nil
}
func (service *homeRefreshTestService) Detail(context.Context, string) (backend.Detail, error) {
	return backend.Detail{}, nil
}
func (service *homeRefreshTestService) Episodes(ctx context.Context, animeID string) ([]backend.Episode, error) {
	service.mu.Lock()
	service.fetched = append(service.fetched, animeID)
	episodes := append([]backend.Episode(nil), service.episodes[animeID]...)
	fetch := service.fetch
	service.mu.Unlock()
	if fetch != nil {
		if err := fetch(ctx, animeID); err != nil {
			return nil, err
		}
	}
	return episodes, nil
}

func (service *homeRefreshTestService) fetchedIDs() []string {
	service.mu.Lock()
	defer service.mu.Unlock()
	return append([]string(nil), service.fetched...)
}
func (service *homeRefreshTestService) History(context.Context) ([]backend.History, error) {
	service.mu.Lock()
	service.historyCall++
	call := service.historyCall
	history := append([]backend.History(nil), service.history...)
	err := service.historyErr
	block := call == service.blockCall
	entered, release := service.entered, service.release
	service.mu.Unlock()
	if block {
		entered <- struct{}{}
		<-release
	}
	return history, err
}
func (service *homeRefreshTestService) Following(context.Context) ([]backend.Following, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return append([]backend.Following(nil), service.following...), nil
}

func (service *homeRefreshTestService) setHistory(history []backend.History, err error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.history = append([]backend.History(nil), history...)
	service.historyErr = err
}

func (service *homeRefreshTestService) setEpisodes(animeID string, episodes []backend.Episode) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.episodes == nil {
		service.episodes = make(map[string][]backend.Episode)
	}
	service.episodes[animeID] = append([]backend.Episode(nil), episodes...)
}

func (service *homeRefreshTestService) historyCalls() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.historyCall
}

func (service *homeRefreshTestService) blockHistoryCall(call int) (<-chan struct{}, chan<- struct{}) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.blockCall = call
	service.entered = make(chan struct{}, 1)
	service.release = make(chan struct{})
	return service.entered, service.release
}

func newHomeRefreshFixture(t *testing.T, service Service) *view {
	t.Helper()
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	window := fltk.NewWindow(1000, 618, "Home refresh test")
	ctx, cancel := context.WithCancel(context.Background())
	ui := &view{window: window, service: service, ctx: ctx, cancelContext: cancel}
	ui.settingsWriter = newSettingsWriter(service, ui.settingsSaveFailed, ui.settingsSaved)
	ui.build()
	ui.mainGroup.Show()
	window.Show()
	t.Cleanup(func() {
		ui.searchModel.close()
		_ = ui.settingsWriter.close()
		cancel()
		releaseTestWindow(window)
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	return ui
}

func TestNativeReturningHomeRefreshesContinuePosition(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}}}
	ui := newHomeRefreshFixture(t, service)
	var continued backend.PlayRequest
	ui.onPlay = func(request backend.PlayRequest) { continued = request }
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.rows) == 1 })
	if got := playRequest(ui.rows[0].History).StartAt; got != 10000 {
		t.Fatalf("initial Continue start time = %d, want 10000", got)
	}

	service.setHistory([]backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 42750, LastPlayed: "2026-10-06T10:05:00Z"}}, nil)
	ui.navigate(0)
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.rows) == 1 && ui.rows[0].History.Position == 42750 })
	if got := ui.homeCaptionLabels[0].Label(); got != "集數：未知 · 上次播放位置 0:42" {
		t.Fatalf("refreshed Continue label = %q", got)
	}
	ui.playHomeRow(ui.rows[0].History, ui.homeLoadGeneration)
	if continued.AnimeID != "anime" || continued.EpisodeID != "episode" || continued.StartAt != 42750 {
		t.Fatalf("refreshed Continue request = %#v, want current position 42750", continued)
	}
}

func TestNativeOfflineStartShowsStoredHomeWithoutNetwork(t *testing.T) {
	seen := time.Now().UTC().Format(time.RFC3339Nano)
	service := &homeRefreshTestService{
		history:   []backend.History{{AnimeID: "watched", EpisodeID: "opaque-5", EpisodeNumber: "05", Position: 125000, LastPlayed: "2026-10-06T10:05:00Z"}},
		following: []backend.Following{{AnimeID: "followed", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: seen}},
		library:   []backend.Anime{{ID: "watched", Title: "看過的作品"}, {ID: "followed", Title: "追蹤的作品"}},
		// The network holds every request until it is cancelled.
		fetch: func(ctx context.Context, _ string) error { <-ctx.Done(); return ctx.Err() },
	}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.cards) == 1 && len(ui.homeUpdateButtons) == 1 })
	if got := ui.homeCaptionLabels[0].Label(); got != "集數：05 · 上次播放位置 2:05" {
		t.Fatalf("繼續觀看 caption = %q, want the stored episode number", got)
	}
	if got := ui.homeUpdateButtons[0].Label(); got != "追蹤的作品" {
		t.Fatalf("近期更新 row = %q", got)
	}
	// Home itself asks nothing of the network; only the background Following refresh does.
	pumpSearchEvents(t, func() bool { return len(service.fetchedIDs()) == 1 })
	pumpFor(50 * time.Millisecond)
	if got := service.fetchedIDs(); len(got) != 1 || got[0] != "followed" {
		t.Fatalf("network requests = %v, want only the followed anime's refresh", got)
	}
	if ui.message.Label() != "" || ui.loadFailed {
		t.Fatalf("held network showed %q (failed %v) while refreshing", ui.message.Label(), ui.loadFailed)
	}
}

func TestNativeFollowingRefreshIsBoundedAndStopsOnClose(t *testing.T) {
	var running, highest atomic.Int32
	following := make([]backend.Following, 10)
	for index := range following {
		following[index] = backend.Following{AnimeID: fmt.Sprintf("anime-%d", index)}
	}
	service := &homeRefreshTestService{following: following, fetch: func(ctx context.Context, _ string) error {
		now := running.Add(1)
		defer running.Add(-1)
		for previous := highest.Load(); now > previous && !highest.CompareAndSwap(previous, now); previous = highest.Load() {
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	ui := newHomeRefreshFixture(t, service)
	baseline := runtime.NumGoroutine()
	ui.loadHome()
	// Background refresh leaves one of the RefreshLimit slots for what the user asks for.
	background := int32(core.RefreshLimit - 1)
	pumpSearchEvents(t, func() bool { return running.Load() == background })
	// Give another request time to start if the limit did not hold.
	pumpFor(50 * time.Millisecond)
	if got := highest.Load(); got != background {
		t.Fatalf("highest parallel refresh requests = %d, want %d", got, background)
	}
	ui.cancel()
	pumpSearchEvents(t, func() bool { return ui.ctx.Err() != nil && running.Load() == 0 && runtime.NumGoroutine() <= baseline })
	if got := len(service.fetchedIDs()); got != int(background) {
		t.Fatalf("%d refresh requests started, want only the %d running when the window closed", got, background)
	}
}

func TestNativeHomeSaysOfflineCalmlyAndClearsAfterRefresh(t *testing.T) {
	var offline atomic.Bool
	offline.Store(true)
	service := &homeRefreshTestService{
		history:   []backend.History{{AnimeID: "anime", EpisodeID: "episode", EpisodeNumber: "3", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}},
		following: []backend.Following{{AnimeID: "anime"}},
		fetch: func(context.Context, string) error {
			if offline.Load() {
				return backend.ErrOffline
			}
			return nil
		},
	}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return ui.message.Label() == homeOfflineNotice })
	if ui.loadFailed || ui.retryButton.Visible() || len(ui.cardButtons) != 1 || !ui.cardButtons[0].IsActive() {
		t.Fatal("offline refresh hid the stored Home content or offered Retry")
	}
	// Other pages do not carry Home's line; it comes back on Home.
	ui.showSection(followingSection)
	if ui.message.Label() != "" {
		t.Fatalf("message on 追蹤 = %q", ui.message.Label())
	}
	ui.showSection(0)
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && ui.message.Label() == homeOfflineNotice })

	offline.Store(false)
	ui.refreshFollowing()
	pumpSearchEvents(t, func() bool { return !ui.followingRefreshing && ui.message.Label() == "" })
}

func TestNativeHomeDoesNotSayOfflineWhenTheSourceAnswers(t *testing.T) {
	service := &homeRefreshTestService{
		following: []backend.Following{{AnimeID: "removed"}},
		// Anime1 answered, but the show is gone; that is not a lost connection.
		fetch: func(context.Context, string) error { return backend.ErrUnavailable },
	}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return len(service.fetchedIDs()) == 1 && !ui.followingRefreshing })
	if ui.homeOffline || ui.message.Label() != "" {
		t.Fatalf("a failed refresh that reached Anime1 showed %q", ui.message.Label())
	}
}

func TestNativeHomeRefreshErrorRemovesStaleContinueAction(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}}}
	ui := newHomeRefreshFixture(t, service)
	ui.window.Show()
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.cardButtons) == 1 })

	service.setHistory(nil, errors.New("storage unavailable"))
	ui.navigate(0)
	if ui.homeLoading && (ui.cardButtons[0].Visible() || ui.cardButtons[0].IsActive()) {
		t.Fatal("stale Continue action remained available during refresh")
	}
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && ui.loadFailed })
	if ui.cardButtons[0].Visible() || ui.cardButtons[0].IsActive() || !ui.retryButton.Visible() {
		t.Fatal("failed refresh exposed a stale Continue action or hid Retry")
	}
	ui.focusTarget(ui.cardButtons[0])
	if !ui.retryButton.HasFocus() {
		t.Fatal("keyboard focus did not skip the unavailable Continue action")
	}
}

func TestNativeHomeReturnCoalescesRefreshAfterOlderReadAndRejectsQueuedContinue(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}}}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.rows) == 1 })
	oldGeneration := ui.homeLoadGeneration
	plays := 0
	ui.onPlay = func(backend.PlayRequest) { plays++ }

	entered, release := service.blockHistoryCall(2)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	ui.loadHome()
	readEntered := false
	pumpSearchEvents(t, func() bool {
		if readEntered {
			return true
		}
		select {
		case <-entered:
			readEntered = true
			return true
		default:
			return false
		}
	})
	service.setHistory([]backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 42750, LastPlayed: "2026-10-06T10:05:00Z"}}, nil)
	ui.navigate(0)
	if !ui.homeRefreshPending || ui.cardButtons[0].Visible() || ui.cardButtons[0].IsActive() {
		t.Fatal("returning Home did not queue a refresh and disable the old Continue action")
	}
	ui.playHomeRow(ui.rows[0].History, oldGeneration)
	if plays != 0 {
		t.Fatal("a queued Continue callback played the stale row during refresh")
	}
	unblock()
	pumpSearchEvents(t, func() bool {
		return !ui.homeLoading && !ui.homeRefreshPending && len(ui.rows) == 1 && ui.rows[0].History.Position == 42750
	})
	if got := service.historyCalls(); got != 3 {
		t.Fatalf("coalesced Home refresh made %d History calls, want 3 including initial and old reads", got)
	}
	if got := playRequest(ui.rows[0].History).StartAt; got != 42750 {
		t.Fatalf("post-checkpoint Continue start time = %d, want 42750", got)
	}
}

func TestNativeHomeRefreshIsBoundedAndOnlyRunsForHomeNavigation(t *testing.T) {
	service := &homeRefreshTestService{}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading })
	if got := service.historyCalls(); got != 1 {
		t.Fatalf("overlapping initial loads made %d History calls, want 1", got)
	}
	ui.navigate(1)
	if got := service.historyCalls(); got != 1 {
		t.Fatalf("non-Home navigation made another History call: %d", got)
	}
}

func (service *homeRefreshTestService) Follow(context.Context, string) error        { return nil }
func (service *homeRefreshTestService) Unfollow(context.Context, string) error      { return nil }
func (service *homeRefreshTestService) RemoveHistory(context.Context, string) error { return nil }
func (service *homeRefreshTestService) Schedule(ctx context.Context) ([]backend.ScheduleItem, error) {
	return service.CachedSchedule(ctx)
}
func (service *homeRefreshTestService) CachedSchedule(context.Context) ([]backend.ScheduleItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return append([]backend.ScheduleItem(nil), service.schedule...), nil
}

func TestNativeReturningHomeFromMenuShowsRecentUpdate(t *testing.T) {
	service := &homeRefreshTestService{}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool {
		return !ui.homeLoading && homeHasLabel(ui, "尚未追蹤任何作品。在作品頁按星號即可追蹤。")
	})

	// A new episode of a followed anime was stored while another page was shown.
	ui.showSection(followingSection)
	service.mu.Lock()
	service.following = []backend.Following{{AnimeID: "anime", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: time.Now().UTC().Format(time.RFC3339Nano)}}
	service.mu.Unlock()
	// The 首頁 menu button and Search's 返回首頁 call showSection(0).
	ui.showSection(0)
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.homeUpdateButtons) == 1 })
	update := ui.homeUpdateButtons[0]
	if update.Label() != "作品" || !update.Visible() || !update.IsActive() {
		t.Fatalf("近期更新 row = %q, visible %v, active %v", update.Label(), update.Visible(), update.IsActive())
	}
}
func homeHasLabel(ui *view, text string) bool {
	for _, label := range ui.contentLabels[ui.staticLabels:] {
		if label.widget.Label() == text {
			return true
		}
	}
	return false
}

func TestNativeHomeUpdateOpensDetailAndBackReturnsHome(t *testing.T) {
	service := &homeRefreshTestService{following: []backend.Following{{AnimeID: "anime", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: time.Now().UTC().Format(time.RFC3339Nano)}}}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.homeUpdateButtons) == 1 })
	ui.openDetailFrom(detailFromHome, backend.Anime{ID: "anime", Title: "作品"})
	pumpSearchEvents(t, func() bool { return ui.searchPreview && ui.previewBack.Label() == "返回首頁" })
	ui.leaveDetail()
	if ui.selected != 0 || ui.searchPreview || !ui.scroll.Visible() {
		t.Fatalf("after back: section %d, preview %v, Home visible %v", ui.selected, ui.searchPreview, ui.scroll.Visible())
	}
}

func TestNativeHomeUpdateRowsComeBeforeContinueCardsInKeyboardOrder(t *testing.T) {
	seen := time.Now().UTC().Format(time.RFC3339Nano)
	service := &homeRefreshTestService{
		history: []backend.History{{AnimeID: "anime", EpisodeID: "episode", Position: 10000, LastPlayed: "2026-10-06T10:00:00Z"}},
		following: []backend.Following{
			{AnimeID: "anime", HasAvailable: true, LatestAvailableNumber: "12", RecentlyUpdated: true, LatestSeenAt: seen},
			{AnimeID: "other", HasAvailable: true, LatestAvailableNumber: "3", RecentlyUpdated: true, LatestSeenAt: seen},
		},
	}
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool { return !ui.homeLoading && len(ui.homeUpdateButtons) == 2 })
	list := ui.homeFocusables()
	if len(list) != 3 || list[0] != ui.homeUpdateButtons[0] || list[1] != ui.homeUpdateButtons[1] || list[2] != ui.cardButtons[0] {
		t.Fatalf("Home keyboard order = %d buttons, want both 近期更新 rows then 繼續播放", len(list))
	}
	if ui.homeUpdateButtons[1].Y() >= ui.cardButtons[0].Y() {
		t.Fatal("近期更新 rows are not above 繼續觀看")
	}
	last := ui.cardButtons[0]
	ui.focusTarget(last)
	if !last.HasFocus() || last.Y() < ui.scroll.Y() || last.Y()+last.H() > ui.scroll.Y()+ui.scroll.H() {
		t.Fatalf("focused 繼續播放 at y=%d..%d, scroll view %d..%d", last.Y(), last.Y()+last.H(), ui.scroll.Y(), ui.scroll.Y()+ui.scroll.H())
	}
}
