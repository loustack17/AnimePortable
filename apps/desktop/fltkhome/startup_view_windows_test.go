//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

func TestNativeStartupOpensHomeWithoutRecordChoiceAndKeepsLegacyUntouched(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	root := t.TempDir()
	executable := filepath.Join(root, "AnimePortable.exe")
	if err := os.WriteFile(executable, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	legacy := filepath.Join(configDir, "AnimePortable", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy data stays unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := backend.PlanPortable(executable, configDir)
	if err != nil || !plan.OfferImport {
		t.Fatalf("portable plan=%#v, err=%v", plan, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	window := fltk.NewWindow(1000, 618, "Startup test")
	ui := &view{window: window, ctx: ctx, cancelContext: cancel, service: &searchViewTestService{appearance: "dark"}, plan: plan}
	ui.build()
	t.Cleanup(func() {
		ui.searchModel.close()
		cancel()
		window.Hide()
		window.Destroy()
	})
	ui.start()
	pumpSearchEvents(t, func() bool { return ui.mainGroup.Visible() })
	if ui.startupGroup.Visible() || len(ui.startupGroup.Children()) != 0 || !ui.dark || !ui.themeToggle.IsActive() {
		t.Fatal("startup blocked on choices or failed to apply saved theme")
	}
	if ui.navigation[0].Y() >= ui.navigation[4].Y() || ui.navigation[4].Y() >= ui.navigation[1].Y() || ui.browseButton.Label() != "搜尋作品" {
		t.Fatal("Search is not second or missing Home entry")
	}
	if _, err := os.Stat(plan.DatabasePath); err != nil {
		t.Fatalf("portable state not created: %v", err)
	}
	data, err := os.ReadFile(legacy)
	if err != nil || string(data) != "legacy data stays unchanged" {
		t.Fatal("normal startup modified legacy data")
	}
}

func TestNativePreviewOnlyPlaysAfterEpisodeAction(t *testing.T) {
	service := &searchViewTestService{episodes: func(context.Context, string) ([]backend.Episode, error) {
		return []backend.Episode{{ID: "episode-1"}, {ID: "episode-2"}}, nil
	}}
	ui := newSearchViewTestFixture(t, service)
	ui.selected = 4
	ui.searchModel.loaded = true
	playedEpisodes := make([]string, 0, 3)
	ui.onPlay = func(request backend.PlayRequest) {
		playedEpisodes = append(playedEpisodes, request.EpisodeID)
		if request.AnimeID != "local" {
			t.Fatalf("playback=%#v", request)
		}
		ui.window.Hide()
		ui.window.Show()
		ui.renderPreview()
	}
	ui.openSearchResult(backend.Anime{ID: "local", Title: "作品"})
	pumpSearchEvents(t, func() bool {
		return ui.searchModel.preview != nil && ui.detailEpisodesDone
	})
	if len(playedEpisodes) != 0 || ui.previewContinue.Visible() {
		t.Fatal("preview autoplayed or Continue appeared for an unwatched series")
	}
	ui.handleDetailEpisodeKey(1, classifyKey(13, 0))
	pumpSearchEvents(t, func() bool { return len(playedEpisodes) == 1 })
	if playedEpisodes[0] != "episode-2" {
		t.Fatalf("Enter played episode %q, want exact row episode-2", playedEpisodes[0])
	}
	ui.handleDetailEpisodeKey(0, classifyKey(32, 0))
	pumpSearchEvents(t, func() bool { return len(playedEpisodes) == 2 })
	if playedEpisodes[1] != "episode-1" {
		t.Fatalf("Space played episode %q, want exact row episode-1", playedEpisodes[1])
	}
	if ui.searchModel.preview == nil || ui.previewTitle.Label() != "作品" {
		t.Fatal("return after failed playback lost preview")
	}
	ui.playDetailEpisodeAt(0)
	pumpSearchEvents(t, func() bool { return len(playedEpisodes) == 3 })
	if playedEpisodes[2] != "episode-1" {
		t.Fatalf("episode actions played %v", playedEpisodes)
	}
}

func TestNativeLateStartupCompletionDoesNotReopenWhileClosing(t *testing.T) {
	ui := newSearchViewTestFixture(t, &searchViewTestService{})
	ui.mainGroup.Hide()
	ui.closing = true
	ui.finishStartup(nil, true, nil)
	if ui.mainGroup.Visible() || ui.dark {
		t.Fatal("late startup completion reopened the view during shutdown")
	}
}

func TestNativePendingFocusDoesNotRunDuringAppearanceDrain(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	called := false
	ui.focusAfterKey(func() { called = true })
	ui.closing = true
	processed := false
	fltk.AddTimeout(0.03, func() { processed = true })
	pumpSearchEvents(t, func() bool { return processed })
	if called {
		t.Fatal("queued focus action ran during close drain")
	}
}

func TestNativeBackRejectsLateEpisodeCompletion(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	returned := make(chan struct{})
	service := &searchViewTestService{episodes: func(context.Context, string) ([]backend.Episode, error) {
		close(started)
		<-release
		close(returned)
		return []backend.Episode{{ID: "late"}}, nil
	}}
	ui := newSearchViewTestFixture(t, service)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	ui.selected = 4
	ui.searchModel.loaded = true
	plays := 0
	ui.onPlay = func(backend.PlayRequest) { plays++ }
	ui.openSearchResult(backend.Anime{ID: "local", Title: "作品"})
	pumpSearchEvents(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	})
	ui.backToSearchResults()
	unblock()
	pumpSearchEvents(t, func() bool {
		select {
		case <-returned:
			return !ui.searchModel.requestActive
		default:
			return false
		}
	})
	processed := false
	fltk.AddTimeout(0.05, func() { processed = true })
	pumpSearchEvents(t, func() bool { return processed })
	if plays != 0 || ui.searchPreview || ui.detailModel.animeID != "" {
		t.Fatal("late source completion played after Back")
	}
}
