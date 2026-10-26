//go:build windows && amd64 && cgo

package fltkengine_test

import (
	"context"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"animeportable/apps/desktop/fltkengine"
	"animeportable/apps/desktop/fltkplayer"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

const (
	endWait = 30 * time.Second
	// The fixture lasts 60 seconds; starting this close to the end keeps each test short.
	nearEnd = 58 * time.Second
)

// endFixture is a real libmpv engine with a rendering window and an end handler that records
// the generation of every normal end it is told about.
type endFixture struct {
	t      *testing.T
	engine *fltkengine.Engine
	media  string
	ends   []uint64
}

func newEndFixture(t *testing.T) *endFixture {
	t.Helper()
	return newEndFixtureWith(t, fltkengine.Options{HardwareDecoding: true})
}

func newEndFixtureWith(t *testing.T, options fltkengine.Options) *endFixture {
	t.Helper()
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

	view := fltkplayer.NewWindow(fltkplayer.Callbacks{})
	view.SetEpisodes([]string{"TEST EPISODE"}, 0)
	view.Window().Show()
	t.Cleanup(func() {
		view.SetRenderHook(nil)
		view.Close()
		view.Window().Hide()
		view.Window().Destroy()
	})

	newContext, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	engine, err := pumpFLTKResult(t, 15*time.Second, func() (*fltkengine.Engine, error) {
		return fltkengine.New(newContext, view.Video(), options)
	})
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	fixture.engine = engine
	t.Cleanup(func() {
		if err := pumpFLTKError(t, 15*time.Second, engine.Close); err != nil {
			t.Errorf("close engine: %v", err)
		}
	})
	view.SetRenderHook(func(width, height int) { _ = engine.Render(width, height) })
	engine.SetEndHandler(func(generation uint64) { fixture.ends = append(fixture.ends, generation) })
	return fixture
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

func TestEngineReportsNormalEndOnceAndKeepsItInSnapshot(t *testing.T) {
	fixture := newEndFixture(t)
	fixture.load(fixture.media, nearEnd, 7)
	fixture.waitForEnds(1)
	// Let a few more ticks run: the end must not be reported again.
	fltkWaitFor(t, 2*time.Second)
	if !slices.Equal(fixture.ends, []uint64{7}) {
		t.Fatalf("end handler calls = %v, want [7]", fixture.ends)
	}
	snapshot := fixture.snapshot()
	if snapshot.Duration < 59*time.Second || snapshot.Position != snapshot.Duration {
		t.Fatalf("snapshot after the end: position=%s duration=%s, want position == duration near 60s", snapshot.Position, snapshot.Duration)
	}
}

func TestEngineDoesNotReportEndOfReplacedOrFailedLoads(t *testing.T) {
	fixture := newEndFixture(t)
	ready := false
	fixture.engine.SetStateHandler(func(_, duration time.Duration, _ bool, _ int) { ready = duration > 0 })
	fixture.load(fixture.media, 0, 1)
	if !pumpUntil(t, endWait, func() bool { return ready }) {
		t.Fatal("first load never reported a duration")
	}
	// loadfile replace ends generation 1 with reason stop.
	fixture.load(fixture.media, nearEnd, 2)
	fixture.waitForEnds(1)
	if !slices.Equal(fixture.ends, []uint64{2}) {
		t.Fatalf("end handler calls = %v, want only the replacing load [2]", fixture.ends)
	}

	fixture.load(filepath.Join(t.TempDir(), "missing.mp4"), 0, 3)
	failed := func() bool { return fixture.engine.FailedGeneration(3) }
	if !pumpUntil(t, endWait, failed) {
		t.Fatal("loading a missing file never failed")
	}
	fltkWaitFor(t, 2*time.Second)
	if !slices.Equal(fixture.ends, []uint64{2}) {
		t.Fatalf("end handler calls = %v after a failed load, want [2]", fixture.ends)
	}
}

func TestEngineReportsStartStagesOncePerLoadIncludingPausedLoads(t *testing.T) {
	fixture := newEndFixture(t)
	type start struct {
		generation uint64
		stage      fltkengine.StartStage
	}
	var starts []start
	fixture.engine.SetStartHandler(func(generation uint64, stage fltkengine.StartStage) {
		starts = append(starts, start{generation, stage})
	})
	reached := func(generation uint64) bool {
		return slices.Contains(starts, start{generation, fltkengine.StartPlaying})
	}

	fixture.load(fixture.media, 0, 1)
	if !pumpUntil(t, endWait, func() bool { return reached(1) }) {
		t.Fatalf("playing load never started; starts=%v", starts)
	}
	// A seek restarts playback in mpv; it is not a new start.
	if err := pumpFLTKError(t, 10*time.Second, func() error { return fixture.engine.Control(context.Background(), "seek", 5) }); err != nil {
		t.Fatal(err)
	}
	fltkWaitFor(t, 2*time.Second)

	// Autoplay loads the next episode paused; that start must be reported too.
	fixture.loadWithPause(fixture.media, 10*time.Second, true, 2)
	if !pumpUntil(t, endWait, func() bool { return reached(2) }) {
		t.Fatalf("paused load never reported its start; starts=%v", starts)
	}

	fixture.load(filepath.Join(t.TempDir(), "missing.mp4"), 0, 3)
	if !pumpUntil(t, endWait, func() bool { return fixture.engine.FailedGeneration(3) }) {
		t.Fatal("loading a missing file never failed")
	}
	fltkWaitFor(t, 2*time.Second)
	want := []start{{1, fltkengine.StartFileLoaded}, {1, fltkengine.StartPlaying}, {2, fltkengine.StartFileLoaded}, {2, fltkengine.StartPlaying}}
	if !slices.Equal(starts, want) {
		t.Fatalf("starts = %v, want %v", starts, want)
	}
}

func TestEnginePausedLoadWaitsAtItsStartAndTheNextLoadPlays(t *testing.T) {
	fixture := newEndFixture(t)
	const start = 10 * time.Second
	var position, duration time.Duration
	fixture.engine.SetStateHandler(func(p, d time.Duration, _ bool, _ int) { position, duration = p, d })

	fixture.loadWithPause(fixture.media, start, true, 1)
	if !pumpUntil(t, endWait, func() bool { return duration > 0 }) {
		t.Fatal("paused load never reported a duration")
	}
	fltkWaitFor(t, 2*time.Second)
	snapshot := fixture.snapshot()
	if !snapshot.Paused || position != start || snapshot.Position != start {
		t.Fatalf("paused load: paused=%t state position=%s snapshot position=%s, want paused at %s", snapshot.Paused, position, snapshot.Position, start)
	}

	fixture.load(fixture.media, start, 2)
	if !pumpUntil(t, endWait, func() bool { return position > start+time.Second }) {
		t.Fatalf("a normal load after a paused one stayed at %s", position)
	}
	if snapshot := fixture.snapshot(); snapshot.Paused {
		t.Fatal("a normal load after a paused one reports paused")
	}
}

func TestEngineLoadToEndCyclesDoNotGrowGoroutinesOrMemory(t *testing.T) {
	const cycles, firstCheckpoint = 30, 10
	fixture := newEndFixture(t)
	type usage struct {
		goroutines int
		private    uintptr
	}
	measure := func() usage {
		runtime.GC()
		return usage{goroutines: runtime.NumGoroutine(), private: privateMemory(t)}
	}
	var atTen usage
	for cycle := 1; cycle <= cycles; cycle++ {
		fixture.load(fixture.media, nearEnd, uint64(cycle))
		fixture.waitForEnds(cycle)
		if cycle == firstCheckpoint {
			atTen = measure()
		}
	}
	atThirty := measure()
	want := make([]uint64, cycles)
	for index := range want {
		want[index] = uint64(index + 1)
	}
	if !slices.Equal(fixture.ends, want) {
		t.Fatalf("end handler calls = %v, want one per cycle %v", fixture.ends, want)
	}
	t.Logf("goroutines: cycle10=%d cycle30=%d; private bytes: cycle10=%d cycle30=%d", atTen.goroutines, atThirty.goroutines, atTen.private, atThirty.private)
	if atThirty.goroutines > atTen.goroutines+2 {
		t.Errorf("goroutines grew from %d to %d", atTen.goroutines, atThirty.goroutines)
	}
	const allowedGrowth = 32 << 20
	if atThirty.private >= atTen.private+allowedGrowth {
		t.Errorf("private memory grew from %d to %d bytes (allowed %d)", atTen.private, atThirty.private, allowedGrowth)
	}
}

// fltkWaitFor runs the FLTK loop for duration, so a test can check that something does not happen.
func fltkWaitFor(t *testing.T, duration time.Duration) {
	t.Helper()
	pumpUntil(t, duration, func() bool { return false })
}

// processMemoryCountersEx is PROCESS_MEMORY_COUNTERS_EX.
type processMemoryCountersEx struct {
	Size                       uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var getProcessMemoryInfo = syscall.NewLazyDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

func privateMemory(t *testing.T) uintptr {
	t.Helper()
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		t.Fatalf("current process: %v", err)
	}
	counters := processMemoryCountersEx{Size: uint32(unsafe.Sizeof(processMemoryCountersEx{}))}
	if result, _, callErr := getProcessMemoryInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size)); result == 0 {
		t.Fatalf("K32GetProcessMemoryInfo: %v", callErr)
	}
	return counters.PrivateUsage
}
