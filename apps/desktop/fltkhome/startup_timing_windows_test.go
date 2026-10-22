//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

// offlineService is the real backend with every Internet-backed action held until its
// context ends, like a network that drops packets instead of refusing them.
type offlineService struct{ *backend.Service }

func (offlineService) Episodes(ctx context.Context, _ string) ([]backend.Episode, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (offlineService) Search(ctx context.Context, _ string) ([]backend.Anime, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (offlineService) Schedule(ctx context.Context) ([]backend.ScheduleItem, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// TestNativeOfflineStartupTiming measures how long the app takes from building the window
// to showing Home's stored rows, on a copy of a real database, with the network held.
// Run: ANIMEPORTABLE_STARTUP_DB=<path to animeportable.db> go test -run TestNativeOfflineStartupTiming -count=5 -v ./apps/desktop/fltkhome
func TestNativeOfflineStartupTiming(t *testing.T) {
	source := os.Getenv("ANIMEPORTABLE_STARTUP_DB")
	if source == "" {
		t.Skip("set ANIMEPORTABLE_STARTUP_DB to a database to copy")
	}
	databasePath := filepath.Join(t.TempDir(), "animeportable.db")
	copyFile(t, source, databasePath)
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	service := backend.NewAt(databasePath)
	ctx, cancel := context.WithCancel(context.Background())
	began := time.Now()
	window := fltk.NewWindow(1000, 618, "Startup timing")
	ui := &view{window: window, service: offlineService{service}, plan: backend.PortablePlan{DatabasePath: databasePath, TargetExists: true}, ctx: ctx, cancelContext: cancel}
	ui.settingsWriter = newSettingsWriter(ui.service, nil, nil)
	ui.build()
	window.Show()
	t.Cleanup(func() {
		ui.searchModel.close()
		_ = ui.settingsWriter.close()
		cancel()
		_ = service.Close()
		releaseTestWindow(window)
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	ui.start()
	deadline := time.Now().Add(10 * time.Second)
	for !(ui.homeLoadGeneration > 0 && !ui.homeLoading) && time.Now().Before(deadline) {
		fltk.Wait(0.001)
	}
	elapsed := time.Since(began)
	if !ui.mainGroup.Visible() || ui.homeLoading || ui.loadFailed {
		t.Fatalf("Home did not show stored content within 10 s (loading %v, failed %v)", ui.homeLoading, ui.loadFailed)
	}
	t.Logf("offline start to stored Home: %v (%d 近期更新, %d 繼續觀看)", elapsed.Round(time.Millisecond), len(ui.homeUpdateButtons), len(ui.cards))
	for _, caption := range ui.homeCaptionLabels {
		t.Logf("繼續觀看 caption: %s", caption.Label())
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	source, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.Create(to)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		_ = target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
}
