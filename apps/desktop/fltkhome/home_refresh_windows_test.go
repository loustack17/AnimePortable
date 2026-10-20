//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

type homeRefreshTestService struct {
	mu               sync.Mutex
	history          []backend.History
	historyErr       error
	historyCall      int
	episodes         map[string][]backend.Episode
	episodeCall      int
	blockEpisodeCall int
	episodeEntered   chan struct{}
	episodeRelease   chan struct{}
	blockCall        int
	entered          chan struct{}
	release          chan struct{}
	following        []backend.Following
}

func (service *homeRefreshTestService) Start(context.Context) error { return nil }
func (service *homeRefreshTestService) Settings(context.Context) (backend.Settings, error) {
	return backend.Settings{Appearance: "light"}, nil
}
func (service *homeRefreshTestService) SaveSettings(context.Context, backend.Settings) error {
	return nil
}
func (service *homeRefreshTestService) Library(context.Context) ([]backend.Anime, error) {
	return []backend.Anime{{ID: "anime", Title: "作品"}}, nil
}
func (service *homeRefreshTestService) Search(context.Context, string) ([]backend.Anime, error) {
	return nil, nil
}
func (service *homeRefreshTestService) Detail(context.Context, string) (backend.Detail, error) {
	return backend.Detail{}, nil
}
func (service *homeRefreshTestService) Episodes(_ context.Context, animeID string) ([]backend.Episode, error) {
	service.mu.Lock()
	service.episodeCall++
	call := service.episodeCall
	episodes := append([]backend.Episode(nil), service.episodes[animeID]...)
	block := call == service.blockEpisodeCall
	entered, release := service.episodeEntered, service.episodeRelease
	service.mu.Unlock()
	if block {
		entered <- struct{}{}
		<-release
	}
	return episodes, nil
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

func (service *homeRefreshTestService) blockEpisodeLookup(call int) (<-chan struct{}, chan<- struct{}) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.blockEpisodeCall = call
	service.episodeEntered = make(chan struct{}, 1)
	service.episodeRelease = make(chan struct{})
	return service.episodeEntered, service.episodeRelease
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
	ui.appearance = newAppearancePersistence(service, nil)
	ui.build()
	ui.mainGroup.Show()
	window.Show()
	t.Cleanup(func() {
		ui.searchModel.close()
		_ = ui.appearance.close()
		cancel()
		window.Hide()
		window.Destroy()
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

func TestNativeHomeEpisodeLookupIsImmediate(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "opaque-4", Position: 125000, LastPlayed: "2026-10-06T10:05:00Z"}}}
	service.setEpisodes("anime", []backend.Episode{{ID: "opaque-4", Number: "04"}})
	entered, release := service.blockEpisodeLookup(1)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	lookupEntered := false
	pumpSearchEvents(t, func() bool {
		if len(ui.rows) == 0 || len(ui.homeCaptionLabels) == 0 {
			return false
		}
		if !lookupEntered {
			select {
			case <-entered:
				lookupEntered = true
			default:
			}
		}
		return lookupEntered
	})
	if got := ui.homeCaptionLabels[0].Label(); got != "集數：未知 · 上次播放位置 2:05" {
		t.Fatalf("immediate Continue caption = %q", got)
	}
	unblock()
	pumpSearchEvents(t, func() bool { return ui.homeCaptionLabels[0].Label() == "集數：04 · 上次播放位置 2:05" })
}

func TestNativeHomeEpisodeLookupRejectsStaleResult(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "opaque-4", Position: 125000, LastPlayed: "2026-10-06T10:05:00Z"}}}
	service.setEpisodes("anime", []backend.Episode{{ID: "opaque-4", Number: "04"}})
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	pumpSearchEvents(t, func() bool {
		return len(ui.rows) == 1 && ui.homeCaptionLabels[0].Label() == "集數：04 · 上次播放位置 2:05"
	})
	staleGeneration := ui.homeLoadGeneration
	staleRow := ui.rows[0]

	service.setHistory([]backend.History{{AnimeID: "anime", EpisodeID: "opaque-4", Position: 42750, LastPlayed: "2026-10-06T10:06:00Z"}}, nil)
	service.setEpisodes("anime", []backend.Episode{{ID: "opaque-4", Number: "05"}})
	ui.navigate(0)
	pumpSearchEvents(t, func() bool {
		return !ui.homeLoading && len(ui.rows) == 1 && ui.homeCaptionLabels[0].Label() == "集數：05 · 上次播放位置 0:42"
	})
	ui.applyHomeEpisodeResult(staleGeneration, 0, staleRow, "04")
	if got := ui.homeCaptionLabels[0].Label(); got != "集數：05 · 上次播放位置 0:42" {
		t.Fatalf("stale lookup changed refreshed Continue caption to %q", got)
	}
	request := playRequest(ui.rows[0].History)
	if request.AnimeID != "anime" || request.EpisodeID != "opaque-4" || request.StartAt != 42750 {
		t.Fatalf("refreshed Continue request = %#v, want exact episode and resume position", request)
	}
}

func TestNativeHomeEpisodeLookupUpdatesWhileSearchIsVisible(t *testing.T) {
	service := &homeRefreshTestService{history: []backend.History{{AnimeID: "anime", EpisodeID: "opaque-4", Position: 125000, LastPlayed: "2026-10-06T10:05:00Z"}}}
	service.setEpisodes("anime", []backend.Episode{{ID: "opaque-4", Number: "04"}})
	entered, release := service.blockEpisodeLookup(1)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	ui := newHomeRefreshFixture(t, service)
	ui.loadHome()
	lookupEntered := false
	pumpSearchEvents(t, func() bool {
		if len(ui.homeCaptionLabels) == 0 {
			return false
		}
		if !lookupEntered {
			select {
			case <-entered:
				lookupEntered = true
			default:
			}
		}
		return lookupEntered
	})
	ui.showSection(4)
	unblock()
	pumpSearchEvents(t, func() bool { return ui.homeCaptionLabels[0].Label() == "集數：04 · 上次播放位置 2:05" })
	if ui.selected != 4 {
		t.Fatal("episode lookup test left Search before the result was applied")
	}
	// Returning to Home reloads it; the refreshed caption must show the episode again.
	ui.showSection(0)
	pumpSearchEvents(t, func() bool {
		return !ui.homeLoading && len(ui.homeCaptionLabels) == 1 && ui.homeCaptionLabels[0].Label() == "集數：04 · 上次播放位置 2:05"
	})
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
func (service *homeRefreshTestService) Schedule(context.Context) ([]backend.ScheduleItem, error) {
	return nil, nil
}
func (service *homeRefreshTestService) CachedSchedule(context.Context) ([]backend.ScheduleItem, error) {
	return nil, nil
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
