//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

// usageService is the backend as the Home window sees it during one evening of use: a small
// stored library, one show half watched, one followed, and a network that can drop out.
type usageService struct {
	noSettingsActions
	mu        sync.Mutex
	library   []backend.Anime
	episodes  map[string][]backend.Episode
	history   []backend.History
	following []backend.Following
	schedule  []backend.ScheduleItem
	settings  backend.Settings
	saved     []backend.Settings
	offline   bool
}

func (service *usageService) Start(context.Context) error { return nil }

func (service *usageService) Settings(context.Context) (backend.Settings, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.settings, nil
}

func (service *usageService) SaveSettings(_ context.Context, settings backend.Settings) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.settings = settings
	service.saved = append(service.saved, settings)
	return nil
}

func (service *usageService) lastSaved() (backend.Settings, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if len(service.saved) == 0 {
		return backend.Settings{}, false
	}
	return service.saved[len(service.saved)-1], true
}

func (service *usageService) Library(context.Context) ([]backend.Anime, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.library), nil
}

func (service *usageService) Search(_ context.Context, query string) ([]backend.Anime, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.offline {
		return nil, errors.New("network unreachable")
	}
	var found []backend.Anime
	for _, anime := range service.library {
		if anime.Title == query {
			found = append(found, anime)
		}
	}
	return found, nil
}

func (service *usageService) Detail(_ context.Context, animeID string) (backend.Detail, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	for _, anime := range service.library {
		if anime.ID == animeID {
			following := slices.ContainsFunc(service.following, func(item backend.Following) bool { return item.AnimeID == animeID })
			return backend.Detail{Anime: anime, Following: following}, nil
		}
	}
	return backend.Detail{}, backend.ErrUnavailable
}

func (service *usageService) Episodes(_ context.Context, animeID string) ([]backend.Episode, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.offline {
		return nil, errors.New("network unreachable")
	}
	return slices.Clone(service.episodes[animeID]), nil
}

func (service *usageService) History(context.Context) ([]backend.History, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.history), nil
}

func (service *usageService) RemoveHistory(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.history = slices.DeleteFunc(service.history, func(item backend.History) bool { return item.AnimeID == animeID })
	return nil
}

func (service *usageService) Following(context.Context) ([]backend.Following, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.following), nil
}

func (service *usageService) Follow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.following = append(service.following, backend.Following{AnimeID: animeID})
	return nil
}

func (service *usageService) Unfollow(_ context.Context, animeID string) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.following = slices.DeleteFunc(service.following, func(item backend.Following) bool { return item.AnimeID == animeID })
	return nil
}

func (service *usageService) Schedule(ctx context.Context) ([]backend.ScheduleItem, error) {
	return service.CachedSchedule(ctx)
}

func (service *usageService) CachedSchedule(context.Context) ([]backend.ScheduleItem, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.Clone(service.schedule), nil
}

func (service *usageService) setHistory(history []backend.History) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.history = history
}

func (service *usageService) setOffline(offline bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.offline = offline
}

func (service *usageService) followed(animeID string) bool {
	service.mu.Lock()
	defer service.mu.Unlock()
	return slices.ContainsFunc(service.following, func(item backend.Following) bool { return item.AnimeID == animeID })
}

// TestHomeRealUse opens the one Home window of the run and walks what a user does in it:
// start, Home, search, detail, play, come back, Following, History, Schedule, Settings, a
// network drop, Escape and closing the window. Every Home window check belongs in this walk.
func TestHomeRealUse(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { uitext.Use("zh-TW") })
	settings := backend.DefaultSettings()
	settings.Appearance = "light"
	service := &usageService{
		library: []backend.Anime{{ID: "watching", Title: "看到一半的作品"}, {ID: "new", Title: "新作品"}},
		episodes: map[string][]backend.Episode{
			"watching": {{ID: "w-1", AnimeID: "watching", Number: "01"}, {ID: "w-2", AnimeID: "watching", Number: "02"}},
			"new":      {{ID: "n-1", AnimeID: "new", Number: "01"}, {ID: "n-2", AnimeID: "new", Number: "02"}},
		},
		history:  []backend.History{{AnimeID: "watching", EpisodeID: "w-1", EpisodeNumber: "01", Position: 10000, LastPlayed: "2026-11-06T10:00:00Z"}},
		schedule: []backend.ScheduleItem{{AnimeID: "new", Title: "新作品", Weekday: 1}},
		settings: settings,
	}
	var played []backend.PlayRequest
	closed := false
	window := fltk.NewWindow(1000, 618, "AnimePortable")
	window.SetSizeRange(760, 480, 0, 0, 0, 0, false)
	ctx, cancel := context.WithCancel(context.Background())
	ui := &view{window: window, service: service, plan: backend.PortablePlan{TargetExists: true}, ctx: ctx, cancelContext: cancel,
		onPlay:  func(request backend.PlayRequest) { played = append(played, request) },
		onClose: func() { closed = true }}
	ui.settingsWriter = newSettingsWriter(service, ui.settingsSaveFailed, ui.settingsSaved)
	ui.build()
	window.Show()
	t.Cleanup(func() {
		cancel()
		releaseTestWindow(window)
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	keepFocusFromOtherThreads(t, window)
	ui.tasks.notice = func(_, message string) { t.Errorf("unexpected notice %q", message) }

	step(t, "start shows Home with Continue watching", func() {
		ui.start()
		pumpUntil(t, func() bool { return ui.mainGroup.Visible() && !ui.homeLoading && len(ui.rows) == 1 })
		if got := ui.rows[0].History.EpisodeID; got != "w-1" {
			t.Fatalf("Continue watching = %q, want w-1", got)
		}
	})

	step(t, "Escape on Home keeps the app open", func() {
		pressKey(t, window, vkEscape)
		pumpFor(50 * time.Millisecond)
		if ui.closing || closed || !window.Visible() {
			t.Fatal("Escape on Home closed the app")
		}
	})

	step(t, "search, open the detail and play episode 2", func() {
		pressKey(t, window, vkSlash)
		pumpUntil(t, func() bool { return ui.selected == SearchSection })
		ui.searchInput.SetValue("新作品")
		ui.searchModel.setQuery("新作品")
		ui.submitSearch()
		pumpUntil(t, func() bool {
			results := ui.searchModel.visibleResults()
			return len(results) == 1 && results[0].ID == "new"
		})
		ui.openSearchResult(ui.searchModel.visibleResults()[0])
		pumpUntil(t, func() bool { return ui.detailEpisodesDone && len(ui.previewEpisodeIDs) == 2 })
		ui.playDetailEpisodeAt(1)
		pumpUntil(t, func() bool { return len(played) == 1 })
		if played[0].AnimeID != "new" || played[0].EpisodeID != "n-2" {
			t.Fatalf("played %#v, want new episode n-2", played[0])
		}
	})

	step(t, "follow from the detail", func() {
		ui.toggleDetailFollow()
		pumpUntil(t, func() bool { return service.followed("new") })
	})

	step(t, "coming back from the player refreshes Continue watching", func() {
		service.setHistory([]backend.History{
			{AnimeID: "new", EpisodeID: "n-2", EpisodeNumber: "02", Position: 42750, LastPlayed: "2026-11-06T11:00:00Z"},
			{AnimeID: "watching", EpisodeID: "w-1", EpisodeNumber: "01", Position: 10000, LastPlayed: "2026-11-06T10:00:00Z"},
		})
		ui.navigate(HomeSection)
		pumpUntil(t, func() bool { return !ui.homeLoading && len(ui.rows) == 2 && ui.rows[0].History.Position == 42750 })
		ui.playHomeRow(ui.rows[0].History, ui.homeLoadGeneration)
		if got := played[len(played)-1]; got.EpisodeID != "n-2" || got.StartAt != 42750 {
			t.Fatalf("Continue played %#v, want n-2 from 42750", got)
		}
	})

	step(t, "Following lists the followed show", func() {
		ui.navigate(followingSection)
		pumpUntil(t, func() bool { return slices.Contains(ui.followingRowIDs, "new") })
	})

	step(t, "History continues and removes", func() {
		ui.navigate(historySection)
		pumpUntil(t, func() bool { return ui.historyLoaded && len(ui.historyItems) == 2 })
		before := len(played)
		ui.continueHistory(service.history[1])
		pumpUntil(t, func() bool { return len(played) == before+1 })
		ui.removeHistory("watching")
		pumpUntil(t, func() bool { return ui.historyLoaded && len(ui.historyItems) == 1 })
	})

	step(t, "Schedule opens a known show's detail and Escape goes back", func() {
		ui.navigate(scheduleSection)
		pumpUntil(t, func() bool { return ui.scheduleLoaded && ui.scheduleShowCount() == 1 })
		day := slices.IndexFunc(ui.scheduleCards, func(cards []*fltk.Button) bool { return len(cards) > 0 })
		ui.openScheduleShow(day, 0)
		pumpUntil(t, func() bool { return ui.searchPreview && ui.detailEpisodesDone })
		pressKey(t, window, vkEscape)
		pumpUntil(t, func() bool { return ui.selected == scheduleSection })
	})

	step(t, "a network drop shows a failure and the next search works", func() {
		service.setOffline(true)
		pressKey(t, window, vkSlash)
		pumpUntil(t, func() bool { return ui.selected == SearchSection })
		ui.searchInput.SetValue("看到一半的作品")
		ui.searchModel.setQuery("看到一半的作品")
		ui.submitSearch()
		pumpUntil(t, func() bool { return ui.searchModel.phase == searchPhaseFailed })
		service.setOffline(false)
		ui.submitSearch()
		pumpUntil(t, func() bool {
			results := ui.searchModel.visibleResults()
			return ui.searchModel.phase != searchPhaseFailed && len(results) == 1 && results[0].ID == "watching"
		})
	})

	step(t, "Settings: dark theme and English are applied and saved", func() {
		ui.navigate(settingsSection)
		ui.chooseTheme("dark")
		ui.chooseLanguage("en")
		pumpUntil(t, func() bool {
			saved, ok := service.lastSaved()
			return ok && saved.Appearance == "dark" && saved.Language == "en"
		})
		if !ui.dark || ui.navigation[HomeSection].Label() != "Home" {
			t.Fatalf("dark = %v, Home label = %q after choosing dark and English", ui.dark, ui.navigation[HomeSection].Label())
		}
	})

	step(t, "a smaller window keeps Home inside it", func() {
		window.Resize(window.X(), window.Y(), 760, 480)
		ui.navigate(HomeSection)
		pumpUntil(t, func() bool { return !ui.homeLoading })
		pumpFor(50 * time.Millisecond)
		// The cards scroll inside Home's scroll area, so the area is what must fit the window.
		if window.W() != 760 || ui.scroll.X()+ui.scroll.W() > window.W() || ui.scroll.Y()+ui.scroll.H() > window.H() {
			t.Fatalf("window %dx%d: Home's scroll area %d,%d %dx%d does not fit", window.W(), window.H(), ui.scroll.X(), ui.scroll.Y(), ui.scroll.W(), ui.scroll.H())
		}
	})

	step(t, "closing the window stops the app", func() {
		ui.cancel()
		pumpUntil(t, func() bool { return closed && ctx.Err() != nil })
	})
}

const (
	vkEscape = 0x1B
	vkLeft   = 0x25
	vkDown   = 0x28
	vkSlash  = 0xBF
)

var sendMessage = user32.NewProc("SendMessageW")

// step names one part of the walk so a failure says where the user was.
func step(t *testing.T, name string, run func()) {
	t.Helper()
	t.Log(name)
	run()
}

// pressKey sends a key press and release to the window as Windows delivers it, so FLTK runs
// its whole keyboard path: the focused widget, then the window shortcut, then FLTK's default
// Escape handling.
func pressKey(t *testing.T, window *fltk.Window, virtualKey uintptr) {
	t.Helper()
	handle := window.RawHandle()
	if handle == 0 {
		t.Fatal("window has no native handle")
	}
	var extended uintptr
	if virtualKey >= vkLeft && virtualKey <= vkDown {
		extended = 1 << 24 // the arrow keys, not the keypad
	}
	const wmKeyDown, wmKeyUp = 0x0100, 0x0101
	sendMessage.Call(handle, wmKeyDown, virtualKey, 1|extended)
	sendMessage.Call(handle, wmKeyUp, virtualKey, 1|extended|3<<30)
}

// pumpUntil runs FLTK events until ready holds; the limit only bounds a failure.
func pumpUntil(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ready() && time.Now().Before(deadline) {
		fltk.Wait(0.01)
	}
	if !ready() {
		t.Fatal("the window did not reach the expected state")
	}
}

// pumpFor runs FLTK events for a while, for checks that nothing happens.
func pumpFor(duration time.Duration) {
	for deadline := time.Now().Add(duration); time.Now().Before(deadline); {
		fltk.Wait(0.01)
	}
}
