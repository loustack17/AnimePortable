//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine"
	"animeportable/apps/desktop/fltkplayer"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

const (
	// endWait bounds a wait for mpv, which can be slow on a busy PC.
	endWait = 30 * time.Second
	// nearEnd starts the 60 s fixture close to its end.
	nearEnd = 58 * time.Second
)

// endFixture is one player session: the engine on the player window and the episode it plays.
type endFixture struct {
	t      *testing.T
	engine *fltkengine.Engine
	media  string
	ends   []uint64
}

// TestPlayerRealUse opens the one player window of the run and watches like a user: play an
// episode, seek, change the speed, let autoplay load the next one paused, watch to the end, hit
// a broken episode, go back to Home, and play again with hardware decoding turned off. Every
// player window and real playback check belongs in this walk.
func TestPlayerRealUse(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() {
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	preparePinnedRuntime(t)
	t.Setenv("ANIMEPORTABLE_PLAYBACK_DIAGNOSTICS", "0")
	fixture := &endFixture{t: t, media: filepath.Join(t.TempDir(), "black.mp4")}
	writeBlackVideo(t, fixture.media)
	missing := filepath.Join(t.TempDir(), "missing.mp4")

	view := fltkplayer.NewWindow(fltkplayer.Callbacks{})
	view.SetEpisodes([]string{"TEST EPISODE"}, 0)
	view.SetState(fltkplayer.State{Playing: true, Volume: 100, Episode: 0, Focus: fltkplayer.FocusProgress})
	view.Window().Show()
	t.Cleanup(func() {
		view.SetRenderHook(nil)
		view.Close()
		view.Window().Hide()
		view.Window().Destroy()
	})

	step(t, "first session plays with hardware decoding and draws the controls", func() {
		fixture.open(view, fltkengine.Options{HardwareDecoding: true})
		fixture.drawsMediaAndControls(view)
		if got := fixture.property("hwdec"); got != "auto" {
			t.Fatalf("hwdec = %q, want auto", got)
		}
	})

	step(t, "seek keeps progress focus", func() {
		if err := pumpFLTKError(t, 10*time.Second, func() error { return fixture.engine.Control(context.Background(), "pause", 0) }); err != nil {
			t.Fatal(err)
		}
		if err := pumpFLTKError(t, 10*time.Second, func() error { return fixture.engine.Control(context.Background(), "seek_absolute", 2) }); err != nil {
			t.Fatal(err)
		}
		if position := fixture.snapshot().Position; position < 1500*time.Millisecond || position > 2500*time.Millisecond {
			t.Fatalf("position after seeking to 2 s = %s", position)
		}
		if view.State().Focus != fltkplayer.FocusProgress {
			t.Fatalf("seek moved focus to %d", view.State().Focus)
		}
	})

	step(t, "speed changes live and carries to the next episode", func() {
		if err := pumpFLTKError(t, 10*time.Second, func() error { return fixture.engine.Control(context.Background(), "speed", 150) }); err != nil {
			t.Fatal(err)
		}
		if got := fixture.property("speed"); got != "1.500000" {
			t.Fatalf("speed after the control = %q, want 1.500000", got)
		}
	})

	step(t, "autoplay loads the next episode paused at its start", func() {
		started := false
		fixture.engine.SetStartHandler(func(generation uint64, stage fltkengine.StartStage) {
			started = started || generation == 2 && stage == fltkengine.StartPlaying
		})
		const start = 10 * time.Second
		fixture.loadWithPause(fixture.media, start, true, 2)
		if !pumpUntil(t, endWait, func() bool { return started }) {
			t.Fatal("the paused next episode never reported its start")
		}
		snapshot := fixture.snapshot()
		if !snapshot.Paused || snapshot.Position != start {
			t.Fatalf("next episode: paused=%t position=%s, want paused at %s", snapshot.Paused, snapshot.Position, start)
		}
		if got := fixture.property("speed"); got != "1.500000" {
			t.Fatalf("speed on the next episode = %q, want the chosen 1.500000", got)
		}
	})

	step(t, "watching to the end reports the end once", func() {
		fixture.load(fixture.media, nearEnd, 3)
		fixture.waitForEnds(1)
		fltkWaitFor(t, 2*time.Second)
		if !slices.Equal(fixture.ends, []uint64{3}) {
			t.Fatalf("ends = %v, want [3]", fixture.ends)
		}
		if snapshot := fixture.snapshot(); snapshot.Position != snapshot.Duration {
			t.Fatalf("after the end: position=%s duration=%s", snapshot.Position, snapshot.Duration)
		}
	})

	step(t, "a broken episode fails without an end", func() {
		fixture.load(missing, 0, 4)
		if !pumpUntil(t, endWait, func() bool { return fixture.engine.FailedGeneration(4) }) {
			t.Fatal("loading a missing file never failed")
		}
		fltkWaitFor(t, 2*time.Second)
		if !slices.Equal(fixture.ends, []uint64{3}) {
			t.Fatalf("ends = %v after a failed load, want [3]", fixture.ends)
		}
	})

	step(t, "back to Home closes the session and the window comes back", func() {
		fixture.close(view)
		view.Window().Hide()
		fltk.Wait(0.05)
		view.Window().Show()
		if !pumpUntil(t, 5*time.Second, func() bool { return view.Window().IsShown() }) {
			t.Fatal("the player window did not show again")
		}
	})

	step(t, "second session without hardware decoding draws again", func() {
		fixture.open(view, fltkengine.Options{})
		fixture.drawsMediaAndControls(view)
		if got := fixture.property("hwdec-current"); got != "no" {
			t.Fatalf("hwdec-current with hardware decoding off = %q, want no", got)
		}
		if got := fixture.property("speed"); got != "1.000000" {
			t.Fatalf("speed of a new session = %q, want 1.000000", got)
		}
		fixture.close(view)
	})
}

// step names one part of the walk so a failure says where the user was.
func step(t *testing.T, name string, run func()) {
	t.Helper()
	t.Log(name)
	run()
}

// open starts a player session on the window, as main does for each show played.
func (fixture *endFixture) open(view *fltkplayer.View, options fltkengine.Options) {
	t := fixture.t
	t.Helper()
	newContext, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	engine, err := pumpFLTKResult(t, 15*time.Second, func() (*fltkengine.Engine, error) {
		return fltkengine.New(newContext, view.Video(), options)
	})
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	fixture.engine = engine
	t.Cleanup(func() { fixture.close(view) })
	view.SetRenderHook(func(width, height int) { _ = engine.Render(width, height) })
	engine.SetEndHandler(func(generation uint64) { fixture.ends = append(fixture.ends, generation) })
}

func (fixture *endFixture) close(view *fltkplayer.View) {
	if fixture.engine == nil {
		return
	}
	view.SetRenderHook(nil)
	if err := pumpFLTKError(fixture.t, 15*time.Second, fixture.engine.Close); err != nil {
		fixture.t.Errorf("close engine: %v", err)
	}
	fixture.engine = nil
}

// drawsMediaAndControls plays the episode from the start and checks that video frames reach the
// window and the controls are drawn over them.
func (fixture *endFixture) drawsMediaAndControls(view *fltkplayer.View) {
	t := fixture.t
	t.Helper()
	engine := fixture.engine
	ready, readyDraws := false, 0
	engine.SetStateHandler(func(_, duration time.Duration, _ bool, resolution int) {
		ready = duration >= 59900*time.Millisecond && duration <= 60100*time.Millisecond && resolution == 180
	})
	view.SetRenderHook(func(width, height int) {
		_ = engine.Render(width, height)
		if ready {
			readyDraws++
		}
	})
	view.SetFocus(fltkplayer.FocusProgress)
	fixture.load(fixture.media, 0, 1)
	if !pumpUntil(t, 8*time.Second, func() bool { return ready && readyDraws > 0 }) {
		t.Fatal("no video frame was drawn")
	}
	drawn := readyDraws
	view.SetFocus(fltkplayer.FocusProgress)
	if !pumpUntil(t, 5*time.Second, func() bool { return readyDraws > drawn }) {
		t.Fatal("the window did not draw again after a focus change")
	}
	view.Video().MakeCurrent()
	if code := drainGLErrors(); code != 0 {
		t.Fatalf("OpenGL error %#x after drawing", code)
	}
	if err := assertVisibleControlGlyphs(view.Video().W(), view.Video().H()); err != nil {
		t.Fatalf("player controls are not drawn over the video: %v", err)
	}
}

func (fixture *endFixture) load(path string, startAt time.Duration, generation uint64) {
	fixture.t.Helper()
	fixture.loadWithPause(path, startAt, false, generation)
}

func (fixture *endFixture) loadWithPause(path string, startAt time.Duration, paused bool, generation uint64) {
	fixture.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := pumpFLTKError(fixture.t, 10*time.Second, func() error {
		return fixture.engine.Load(ctx, path, startAt, paused, generation)
	}); err != nil {
		fixture.t.Fatalf("load generation %d: %v", generation, err)
	}
}

func (fixture *endFixture) snapshot() core.PlaybackSnapshot {
	fixture.t.Helper()
	snapshot, err := pumpFLTKResult(fixture.t, 10*time.Second, func() (core.PlaybackSnapshot, error) {
		return fixture.engine.Snapshot(context.Background())
	})
	if err != nil {
		fixture.t.Fatalf("snapshot: %v", err)
	}
	return snapshot
}

func (fixture *endFixture) waitForEnds(count int) {
	fixture.t.Helper()
	if !pumpUntil(fixture.t, endWait, func() bool { return len(fixture.ends) >= count }) {
		fixture.t.Fatalf("timed out waiting for end %d; ends=%v", count, fixture.ends)
	}
}

func (fixture *endFixture) property(name string) string {
	fixture.t.Helper()
	value, err := pumpFLTKResult(fixture.t, 10*time.Second, func() (string, error) {
		return fixture.engine.CoreProperty(context.Background(), name)
	})
	if err != nil {
		fixture.t.Fatalf("read %s: %v", name, err)
	}
	return value
}

// fltkWaitFor runs the FLTK loop for duration, so a test can check that something does not happen.
func fltkWaitFor(t *testing.T, duration time.Duration) {
	t.Helper()
	pumpUntil(t, duration, func() bool { return false })
}
