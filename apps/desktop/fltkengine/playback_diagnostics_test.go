package fltkengine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlaybackDiagnosticsOptInIsExactAndDisabledDoesNotWrite(t *testing.T) {
	for value, want := range map[string]bool{"": false, "true": false, "yes": false, "1": true} {
		if got := playbackDiagnosticsEnabled(func(string) string { return value }); got != want {
			t.Fatalf("enabled for %q = %t, want %t", value, got, want)
		}
	}
	path := filepath.Join(t.TempDir(), "playback-diagnostics.json")
	diagnostics := newPlaybackDiagnostics(false, path)
	if diagnostics != nil {
		t.Fatal("diagnostics were created without opt-in")
	}
	if err := diagnostics.persist(); err != nil {
		t.Fatalf("disabled persist: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("disabled diagnostics output stat error = %v, want not-exist", err)
	}
}

func TestPlaybackDiagnosticsPersistsBoundedAggregateAndWhitelistedProperties(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playback-diagnostics.json")
	diagnostics := newPlaybackDiagnostics(true, path)
	start := time.Unix(100, 0)
	diagnostics.recordMPVUpdateCallback()
	diagnostics.recordMPVUpdateCallback()
	diagnostics.recordFLTKAwakeCallback()
	diagnostics.recordRenderUpdate(true)
	diagnostics.recordRenderUpdate(false)
	diagnostics.renderStarted(start)
	diagnostics.renderFinished(start, start.Add(6*time.Millisecond), false)
	diagnostics.renderStarted(start.Add(250 * time.Millisecond))
	diagnostics.renderFinished(start.Add(260*time.Millisecond), start.Add(272*time.Millisecond), true)
	glReads := 0
	diagnostics.captureOpenGL(func() (string, string) {
		glReads++
		return "Example Vendor", "Example Renderer"
	})
	diagnostics.captureOpenGL(func() (string, string) {
		glReads++
		return "Ignored Vendor", "Ignored Renderer"
	})
	if glReads != 1 {
		t.Fatalf("OpenGL string reads = %d, want 1", glReads)
	}
	diagnostics.recordMPVProperties(playbackDiagnosticMPVProperties{
		FrameDropCount:        parseDiagnosticCount("3"),
		DecoderFrameDropCount: parseDiagnosticCount("4"),
		HWDecCurrent:          `C:\private\decoder.dll`,
		DisplayFPS:            parseDiagnosticFPS("60.0"),
		EstimatedDisplayFPS:   parseDiagnosticFPS("59.9"),
		VODelayedFrameCount:   parseDiagnosticCount("5"),
		VideoFPS:              parseDiagnosticFPS("23.976"),
	})
	if err := diagnostics.persist(); err != nil {
		t.Fatalf("persist: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report playbackDiagnosticReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if report.SchemaVersion != 1 || report.MPVUpdateCallbacks != 2 || report.FLTKAwakeCallbacks != 1 || report.RenderCalls != 2 || report.RenderUpdateFrames != 1 || report.RenderGLErrors != 1 || report.MPVPropertySamples != 1 {
		t.Fatalf("aggregate counters = %+v", report)
	}
	if report.RenderGLTotalNanos != uint64(18*time.Millisecond) || report.RenderGLMaxNanos != uint64(12*time.Millisecond) || report.RenderIntervalMaxNanos != uint64(250*time.Millisecond) || report.RenderSpanNanos != uint64(250*time.Millisecond) {
		t.Fatalf("render timing = %+v", report)
	}
	if report.GLVendor != "Example Vendor" || report.GLRenderer != "Example Renderer" {
		t.Fatalf("OpenGL values were not captured once: vendor=%q renderer=%q", report.GLVendor, report.GLRenderer)
	}
	if report.MPV.FrameDropCount == nil || *report.MPV.FrameDropCount != 3 || report.MPV.DecoderFrameDropCount == nil || *report.MPV.DecoderFrameDropCount != 4 || report.MPV.HWDecCurrent != "other" || report.MPV.DisplayFPS == nil || *report.MPV.DisplayFPS != 60 || report.MPV.EstimatedDisplayFPS == nil || *report.MPV.EstimatedDisplayFPS != 59.9 || report.MPV.VODelayedFrameCount == nil || *report.MPV.VODelayedFrameCount != 5 || report.MPV.VideoFPS == nil || *report.MPV.VideoFPS != 23.976 {
		t.Fatalf("mpv property snapshot = %+v", report.MPV)
	}
	for _, privateValue := range []string{"decoder.dll", "https://", "Ignored Vendor", "private"} {
		if strings.Contains(string(data), privateValue) {
			t.Fatalf("report contains unexpected value %q", privateValue)
		}
	}
	diagnostics.recordMPVProperties(playbackDiagnosticMPVProperties{HWDecCurrent: "d3d11va"})
	if err := diagnostics.persist(); err != nil {
		t.Fatalf("overwrite report: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read overwritten report: %v", err)
	}
	if !strings.Contains(string(data), `"hwdec_current": "d3d11va"`) {
		t.Fatalf("latest report was not written: %s", data)
	}
}

func TestPlaybackDiagnosticParsersRejectMissingAndInvalidValues(t *testing.T) {
	for _, value := range []string{"", "-1", "1.5", "NaN", "+Inf", "path"} {
		if got := parseDiagnosticCount(value); got != nil {
			t.Fatalf("count %q = %d, want unavailable", value, *got)
		}
	}
	for _, value := range []string{"", "-1", "NaN", "+Inf", "path"} {
		if got := parseDiagnosticFPS(value); got != nil {
			t.Fatalf("fps %q = %f, want unavailable", value, *got)
		}
	}
	if got := parseDiagnosticCount("7"); got == nil || *got != 7 {
		t.Fatalf("valid count = %v, want 7", got)
	}
	if got := parseDiagnosticFPS("59.94"); got == nil || *got != 59.94 {
		t.Fatalf("valid fps = %v, want 59.94", got)
	}
	if got := safeHWDecCurrent("d3d11va"); got != "d3d11va" {
		t.Fatalf("whitelisted hwdec = %q", got)
	}
	if got := safeHWDecCurrent("C:\\private\\decoder.dll"); got != "other" {
		t.Fatalf("unrecognized hwdec = %q, want other", got)
	}
	if got := boundedDiagnosticText("vendor\nname", 64); got != "vendor_name" {
		t.Fatalf("bounded text = %q", got)
	}
}
