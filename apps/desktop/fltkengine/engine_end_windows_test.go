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
		return fltkengine.New(newContext, view.Video())
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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := pumpFLTKError(fixture.t, 10*time.Second, func() error {
		return fixture.engine.Load(ctx, path, startAt, generation)
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
	fltkWaitFor(2 * time.Second)
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
	fltkWaitFor(2 * time.Second)
	if !slices.Equal(fixture.ends, []uint64{2}) {
		t.Fatalf("end handler calls = %v after a failed load, want [2]", fixture.ends)
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

func fltkWaitFor(duration time.Duration) {
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		fltk.Wait(0.01)
	}
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
