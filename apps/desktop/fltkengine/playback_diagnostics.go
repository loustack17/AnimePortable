package fltkengine

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const playbackDiagnosticsEnvironment = "ANIMEPORTABLE_PLAYBACK_DIAGNOSTICS"

type playbackDiagnostics struct {
	mu          sync.Mutex
	path        string
	report      playbackDiagnosticReport
	firstRender time.Time
	lastRender  time.Time
	openGLOnce  sync.Once
	// base anchors the monotonic offsets below, which are atomics because mpv's thread writes them.
	base time.Time
	// firstUpdate and awakeRan hold offsets from base (plus one, so zero means unset) of the oldest
	// mpv update not yet rendered and of the latest FLTK awake callback not yet followed by a render.
	firstUpdate atomic.Int64
	awakeRan    atomic.Int64
}

type playbackDiagnosticReport struct {
	SchemaVersion          int                             `json:"schema_version"`
	MPVUpdateCallbacks     uint64                          `json:"mpv_update_callbacks"`
	FLTKAwakeCallbacks     uint64                          `json:"fltk_awake_callbacks"`
	RenderCalls            uint64                          `json:"render_calls"`
	RenderUpdateFrames     uint64                          `json:"render_update_frames"`
	RenderGLErrors         uint64                          `json:"render_gl_errors"`
	RenderGLTotalNanos     uint64                          `json:"render_gl_total_nanos"`
	RenderGLMaxNanos       uint64                          `json:"render_gl_max_nanos"`
	RenderIntervalMaxNanos uint64                          `json:"render_interval_max_nanos"`
	RenderSpanNanos        uint64                          `json:"render_span_nanos"`
	MPVPropertySamples     uint64                          `json:"mpv_property_samples"`
	GLVendor               string                          `json:"gl_vendor"`
	GLRenderer             string                          `json:"gl_renderer"`
	MPV                    playbackDiagnosticMPVProperties `json:"mpv"`
	// UpdateToRender is the delay from the first unconsumed mpv update to the Render that consumed it.
	UpdateToRender playbackDiagnosticLatency `json:"update_to_render"`
	// AwakeToRender is the delay from the FLTK awake callback to the next Render start.
	AwakeToRender playbackDiagnosticLatency `json:"awake_to_render"`
}

type playbackDiagnosticLatency struct {
	Count    uint64                           `json:"count"`
	MaxNanos uint64                           `json:"max_nanos"`
	Buckets  playbackDiagnosticLatencyBuckets `json:"buckets"`
}

type playbackDiagnosticLatencyBuckets struct {
	Under10ms  uint64 `json:"lt_10ms"`
	Under25ms  uint64 `json:"lt_25ms"`
	Under50ms  uint64 `json:"lt_50ms"`
	Under100ms uint64 `json:"lt_100ms"`
	Under200ms uint64 `json:"lt_200ms"`
	AtLeast200 uint64 `json:"ge_200ms"`
}

func (latency *playbackDiagnosticLatency) add(delay time.Duration) {
	delay = max(delay, 0)
	latency.Count++
	latency.MaxNanos = max(latency.MaxNanos, uint64(delay))
	switch {
	case delay < 10*time.Millisecond:
		latency.Buckets.Under10ms++
	case delay < 25*time.Millisecond:
		latency.Buckets.Under25ms++
	case delay < 50*time.Millisecond:
		latency.Buckets.Under50ms++
	case delay < 100*time.Millisecond:
		latency.Buckets.Under100ms++
	case delay < 200*time.Millisecond:
		latency.Buckets.Under200ms++
	default:
		latency.Buckets.AtLeast200++
	}
}

type playbackDiagnosticMPVProperties struct {
	FrameDropCount        *uint64  `json:"frame_drop_count"`
	DecoderFrameDropCount *uint64  `json:"decoder_frame_drop_count"`
	HWDecCurrent          string   `json:"hwdec_current"`
	DisplayFPS            *float64 `json:"display_fps"`
	EstimatedDisplayFPS   *float64 `json:"estimated_display_fps"`
	VODelayedFrameCount   *uint64  `json:"vo_delayed_frame_count"`
	VideoFPS              *float64 `json:"video_fps"`
}

func newPlaybackDiagnostics(enabled bool, path string) *playbackDiagnostics {
	if !enabled || path == "" {
		return nil
	}
	return &playbackDiagnostics{
		path:   path,
		base:   time.Now(),
		report: playbackDiagnosticReport{SchemaVersion: 1},
	}
}

func playbackDiagnosticsEnabled(getenv func(string) string) bool {
	return getenv != nil && getenv(playbackDiagnosticsEnvironment) == "1"
}

func (diagnostics *playbackDiagnostics) recordMPVUpdateCallback() {
	if diagnostics == nil {
		return
	}
	// Only the first update since the last render is kept: that is the one that waited longest.
	diagnostics.firstUpdate.CompareAndSwap(0, int64(time.Since(diagnostics.base))+1)
	diagnostics.mu.Lock()
	diagnostics.report.MPVUpdateCallbacks++
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) recordFLTKAwakeCallback() {
	if diagnostics == nil {
		return
	}
	diagnostics.awakeRan.Store(int64(time.Since(diagnostics.base)) + 1)
	diagnostics.mu.Lock()
	diagnostics.report.FLTKAwakeCallbacks++
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) recordRenderUpdate(frameReady bool) {
	if diagnostics == nil || !frameReady {
		return
	}
	diagnostics.mu.Lock()
	diagnostics.report.RenderUpdateFrames++
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) renderStarted(at time.Time) {
	if diagnostics == nil {
		return
	}
	nowOffset := int64(at.Sub(diagnostics.base)) + 1
	updateOffset := diagnostics.firstUpdate.Swap(0)
	awakeOffset := diagnostics.awakeRan.Swap(0)
	diagnostics.mu.Lock()
	if updateOffset != 0 {
		diagnostics.report.UpdateToRender.add(time.Duration(nowOffset - updateOffset))
	}
	if awakeOffset != 0 {
		diagnostics.report.AwakeToRender.add(time.Duration(nowOffset - awakeOffset))
	}
	diagnostics.report.RenderCalls++
	if diagnostics.firstRender.IsZero() {
		diagnostics.firstRender = at
	} else {
		diagnostics.report.RenderIntervalMaxNanos = maxDuration(diagnostics.report.RenderIntervalMaxNanos, at.Sub(diagnostics.lastRender))
	}
	diagnostics.lastRender = at
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) renderFinished(started, finished time.Time, failed bool) {
	if diagnostics == nil {
		return
	}
	duration := finished.Sub(started)
	if duration < 0 {
		duration = 0
	}
	diagnostics.mu.Lock()
	diagnostics.report.RenderGLTotalNanos += uint64(duration)
	if uint64(duration) > diagnostics.report.RenderGLMaxNanos {
		diagnostics.report.RenderGLMaxNanos = uint64(duration)
	}
	if failed {
		diagnostics.report.RenderGLErrors++
	}
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) captureOpenGL(read func() (string, string)) {
	if diagnostics == nil {
		return
	}
	diagnostics.openGLOnce.Do(func() {
		vendor, renderer := read()
		diagnostics.mu.Lock()
		diagnostics.report.GLVendor = boundedDiagnosticText(vendor, 128)
		diagnostics.report.GLRenderer = boundedDiagnosticText(renderer, 128)
		diagnostics.mu.Unlock()
	})
}

func (diagnostics *playbackDiagnostics) recordMPVProperties(properties playbackDiagnosticMPVProperties) {
	if diagnostics == nil {
		return
	}
	properties.HWDecCurrent = safeHWDecCurrent(properties.HWDecCurrent)
	diagnostics.mu.Lock()
	diagnostics.report.MPV = properties
	diagnostics.report.MPVPropertySamples++
	diagnostics.mu.Unlock()
}

func (diagnostics *playbackDiagnostics) persist() error {
	if diagnostics == nil {
		return nil
	}
	diagnostics.mu.Lock()
	report := diagnostics.report
	if !diagnostics.firstRender.IsZero() && !diagnostics.lastRender.IsZero() {
		report.RenderSpanNanos = maxDuration(report.RenderSpanNanos, diagnostics.lastRender.Sub(diagnostics.firstRender))
	}
	diagnostics.mu.Unlock()
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(diagnostics.path), ".playback-diagnostics-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, diagnostics.path)
}

func parseDiagnosticCount(value string) *uint64 {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseDiagnosticFPS(value string) *float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return nil
	}
	return &parsed
}

func boundedDiagnosticText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) > limit {
		value = value[:limit]
	}
	var result strings.Builder
	result.Grow(len(value))
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			result.WriteByte('_')
			continue
		}
		result.WriteRune(character)
	}
	return result.String()
}

func safeHWDecCurrent(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "":
		return ""
	case "auto", "auto-safe", "cuda", "d3d11va", "d3d11va-copy", "d3d12va", "d3d12va-copy", "dxva2", "dxva2-copy", "nvdec", "nvdec-copy", "qsv", "qsv-copy", "vulkan", "vulkan-copy":
		return value
	case "no":
		return "no"
	default:
		return "other"
	}
}

func maxDuration(current uint64, candidate time.Duration) uint64 {
	if candidate <= 0 || uint64(candidate) <= current {
		return current
	}
	return uint64(candidate)
}
