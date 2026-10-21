//go:build windows && amd64 && cgo

package fltkengine

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"animeportable/apps/desktop/fltkengine/internal/mpvwin"
	"animeportable/internal/runtimepin"
)

func TestSnapshotRejectsNilContext(t *testing.T) {
	engine := &Engine{}
	if _, err := engine.Snapshot(nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Snapshot(nil) error = %v, want context.Canceled", err)
	}
}

func TestSecondsToDurationRejectsInvalidValues(t *testing.T) {
	for _, test := range []struct {
		value   float64
		want    time.Duration
		invalid bool
	}{
		{42.75, 42750 * time.Millisecond, false},
		{0, 0, false},
		{-1, 0, true},
		{math.NaN(), 0, true},
		{math.Inf(1), 0, true},
		{1e30, 0, true},
	} {
		got, err := secondsToDuration(test.value)
		if (err != nil) != test.invalid || got != test.want {
			t.Fatalf("value %v: duration=%v, error=%v", test.value, got, err)
		}
	}
}

func propertyEvent(change mpvwin.PropertyChange) *mpvwin.Event {
	return &mpvwin.Event{EventID: mpvwin.EventPropertyChange, Property: &change}
}

func TestApplyPropertyCachesObservedValues(t *testing.T) {
	engine := &Engine{}
	for _, change := range []mpvwin.PropertyChange{
		{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 3.5},
		{Name: "duration", Format: mpvwin.FormatDouble, Double: 1400},
		{Name: "pause", Format: mpvwin.FormatFlag, Flag: true},
		{Name: "video-params/h", Format: mpvwin.FormatString, String: "1080"},
		{Name: "frame-drop-count", Format: mpvwin.FormatString, String: "7"},
		{Name: "hwdec-current", Format: mpvwin.FormatString, String: "d3d11va"},
		{Name: "estimated-vf-fps", Format: mpvwin.FormatString, String: "23.976"},
	} {
		engine.applyProperty(propertyEvent(change))
	}
	observed := engine.observed
	if !observed.positionKnown || observed.positionSeconds != 3.5 || !observed.durationKnown || observed.durationSeconds != 1400 {
		t.Fatalf("position/duration not cached: %+v", observed)
	}
	if !engine.paused || observed.videoHeight != 1080 {
		t.Fatalf("pause=%t height=%d", engine.paused, observed.videoHeight)
	}
	if got := observed.diagnostics; got.FrameDropCount == nil || *got.FrameDropCount != 7 || got.HWDecCurrent != "d3d11va" || got.VideoFPS == nil || *got.VideoFPS != 23.976 {
		t.Fatalf("diagnostics not cached: %+v", got)
	}
}

func TestApplyPropertyHandlesUnavailableValues(t *testing.T) {
	engine := &Engine{paused: true}
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 9}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatNone}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "video-params/h", Format: mpvwin.FormatString, String: "720"}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "video-params/h", Format: mpvwin.FormatNone}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "pause", Format: mpvwin.FormatNone}))
	if engine.observed.positionKnown || engine.observed.videoHeight != 0 {
		t.Fatalf("unavailable properties must clear the cache: %+v", engine.observed)
	}
	if !engine.paused {
		t.Fatal("an unavailable pause property must keep the last known state")
	}
}

func TestApplyPropertyIgnoresOtherEvents(t *testing.T) {
	engine := &Engine{}
	engine.applyProperty(&mpvwin.Event{EventID: mpvwin.EventSeek})
	engine.applyProperty(&mpvwin.Event{EventID: mpvwin.EventPropertyChange})
	if engine.observed != (observedProperties{}) || engine.paused {
		t.Fatal("non-property events changed the cache")
	}
}

func TestLibraryLocation(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "animeportable.exe")
	getenv := func(values map[string]string) func(string) string {
		return func(name string) string { return values[name] }
	}
	path, hash, err := libraryLocation(executable, getenv(nil))
	if err != nil || path != filepath.Join(filepath.Dir(executable), "libmpv-2.dll") || hash != runtimepin.LibMPVSHA256 {
		t.Fatalf("default runtime: path=%q hash=%q err=%v", path, hash, err)
	}
	override := filepath.Join(t.TempDir(), "custom-libmpv.dll")
	customHash := strings.Repeat("a", 64)
	path, hash, err = libraryLocation(executable, getenv(map[string]string{
		"ANIMEPORTABLE_LIBMPV_OVERRIDE":        override,
		"ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash,
	}))
	if err != nil || path != override || hash != customHash {
		t.Fatalf("custom runtime: path=%q hash=%q err=%v", path, hash, err)
	}
	for name, values := range map[string]map[string]string{
		"missing hash":  {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override},
		"missing path":  {"ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash},
		"relative path": {"ANIMEPORTABLE_LIBMPV_OVERRIDE": "libmpv-2.dll", "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": customHash},
		"short hash":    {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override, "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": "abc"},
		"nonhex hash":   {"ANIMEPORTABLE_LIBMPV_OVERRIDE": override, "ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256": strings.Repeat("g", 64)},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := libraryLocation(executable, getenv(values))
			if !errors.Is(err, mpvwin.ErrInvalidLibrary) {
				t.Fatalf("expected invalid runtime configuration, got %v", err)
			}
		})
	}
}

func TestSeekCompletesOnRestartAfterSeekStarts(t *testing.T) {
	engine := &Engine{seekPending: true}
	engine.advanceSeek(&mpvwin.Event{EventID: mpvwin.EventPlaybackRestart})
	if !engine.seekPending {
		t.Fatal("a restart from before the seek completed it")
	}
	engine.advanceSeek(&mpvwin.Event{EventID: mpvwin.EventSeek})
	engine.advanceSeek(&mpvwin.Event{EventID: mpvwin.EventPlaybackRestart})
	if engine.seekPending || engine.seekStarted {
		t.Fatal("seek still pending after restart; a seek to the current position gets no time-pos change")
	}
}

func TestSeekCachesTargetAndIgnoresStalePositions(t *testing.T) {
	engine := &Engine{}
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 100}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "duration", Format: mpvwin.FormatDouble, Double: 120}))
	engine.seekPending = true
	engine.cacheSeekTarget(10, "relative")
	if engine.observed.positionSeconds != 110 {
		t.Fatalf("relative target = %v, want 110", engine.observed.positionSeconds)
	}
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 100.5}))
	if engine.observed.positionSeconds != 110 {
		t.Fatalf("stale position overwrote the seek target: %v", engine.observed.positionSeconds)
	}
	engine.cacheSeekTarget(500, "absolute")
	if engine.observed.positionSeconds != 120 {
		t.Fatalf("absolute target not clamped to duration: %v", engine.observed.positionSeconds)
	}
	engine.cacheSeekTarget(-30, "relative")
	if engine.observed.positionSeconds != 90 {
		t.Fatalf("relative target = %v, want 90", engine.observed.positionSeconds)
	}
	engine.cacheSeekTarget(-200, "relative")
	if engine.observed.positionSeconds != 0 || !engine.observed.positionKnown {
		t.Fatalf("target not clamped to zero: %+v", engine.observed)
	}
	engine.seekPending = false
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 0.4}))
	if engine.observed.positionSeconds != 0.4 {
		t.Fatalf("position after the seek was not applied: %v", engine.observed.positionSeconds)
	}
}
func TestSeekEndFileClearsPending(t *testing.T) {
	engine := &Engine{seekPending: true, seekStarted: true}
	engine.advanceSeek(&mpvwin.Event{EventID: mpvwin.EventEnd})
	if engine.seekPending || engine.seekStarted {
		t.Fatal("end-file must clear a pending seek")
	}
}

func TestForgetObservedPlaybackDropsPreviousFileValues(t *testing.T) {
	engine := &Engine{}
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 300}))
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "duration", Format: mpvwin.FormatDouble, Double: 1400}))
	engine.forgetObservedPlayback()
	observed := engine.observed
	if observed.positionKnown || observed.durationKnown || observed.positionSeconds != 0 || observed.durationSeconds != 0 {
		t.Fatalf("observed values survived: %+v", observed)
	}
}

func TestSeekAcceptsPositionAfterSeekStartsAndResumeFallback(t *testing.T) {
	engine := &Engine{position: 300 * time.Second}
	engine.seekPending = true
	engine.cacheSeekTarget(10, "relative")
	if engine.observed.positionSeconds != 310 {
		t.Fatalf("relative seek before first time-pos ignored the resume start: %v", engine.observed.positionSeconds)
	}
	engine.advanceSeek(&mpvwin.Event{EventID: mpvwin.EventSeek})
	engine.applyProperty(propertyEvent(mpvwin.PropertyChange{Name: "time-pos", Format: mpvwin.FormatDouble, Double: 306}))
	if engine.observed.positionSeconds != 306 {
		t.Fatalf("landed keyframe position after SEEK was dropped: %v", engine.observed.positionSeconds)
	}
}

func TestEndsNormallyOnlyForEOFOfTheCurrentFile(t *testing.T) {
	const full = 1440 * time.Second
	cases := []struct {
		name       string
		reason     mpvwin.EndFileReason
		generation uint64
		pending    bool
		position   time.Duration
		duration   time.Duration
		normal     bool
		early      bool
	}{
		{"eof", mpvwin.EndFileEOF, 1, false, full, full, true, false},
		{"eof near the end", mpvwin.EndFileEOF, 1, false, 1420 * time.Second, full, true, false},
		{"eof exactly the tolerance before the end", mpvwin.EndFileEOF, 1, false, full - endTolerance, full, true, false},
		{"eof just beyond the tolerance", mpvwin.EndFileEOF, 1, false, full - endTolerance - time.Nanosecond, full, false, true},
		{"eof of a stream cut short", mpvwin.EndFileEOF, 1, false, 576 * time.Second, full, false, true},
		{"eof without a duration", mpvwin.EndFileEOF, 1, false, 0, 0, false, true},
		{"stop", mpvwin.EndFileStop, 1, false, full, full, false, false},
		{"quit", mpvwin.EndFileQuit, 1, false, full, full, false, false},
		{"redirect", mpvwin.EndFileRedirect, 1, false, full, full, false, false},
		{"error", mpvwin.EndFileError, 1, false, full, full, false, false},
		{"eof of a replaced file while a load is pending", mpvwin.EndFileEOF, 1, true, 576 * time.Second, full, false, false},
		{"eof without a loaded generation", mpvwin.EndFileEOF, 0, false, 576 * time.Second, full, false, false},
	}
	for _, test := range cases {
		if got := endsNormally(test.reason, test.generation, test.pending, test.position, test.duration); got != test.normal {
			t.Errorf("%s: endsNormally = %t, want %t", test.name, got, test.normal)
		}
		if got := endsEarly(test.reason, test.generation, test.pending, test.position, test.duration); got != test.early {
			t.Errorf("%s: endsEarly = %t, want %t", test.name, got, test.early)
		}
	}
}
