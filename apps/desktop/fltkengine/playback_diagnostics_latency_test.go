package fltkengine

import (
	"testing"
	"time"
)

func TestLatencyBuckets(t *testing.T) {
	var latency playbackDiagnosticLatency
	for _, delay := range []time.Duration{-time.Second, 9 * time.Millisecond, 10 * time.Millisecond, 24 * time.Millisecond, 49 * time.Millisecond, 99 * time.Millisecond, 199 * time.Millisecond, 200 * time.Millisecond, time.Second} {
		latency.add(delay)
	}
	want := playbackDiagnosticLatencyBuckets{Under10ms: 2, Under25ms: 2, Under50ms: 1, Under100ms: 1, Under200ms: 1, AtLeast200: 2}
	if latency.Buckets != want || latency.Count != 9 || latency.MaxNanos != uint64(time.Second) {
		t.Fatalf("latency = %+v", latency)
	}
}

func TestRenderStartedRecordsUpdateAndAwakeLatency(t *testing.T) {
	diagnostics := newPlaybackDiagnostics(true, "unused")
	diagnostics.recordMPVUpdateCallback()
	diagnostics.recordMPVUpdateCallback()
	diagnostics.recordFLTKAwakeCallback()
	diagnostics.renderStarted(diagnostics.base.Add(time.Since(diagnostics.base) + 30*time.Millisecond))
	diagnostics.renderStarted(time.Now())
	report := diagnostics.report
	if report.UpdateToRender.Count != 1 || report.AwakeToRender.Count != 1 {
		t.Fatalf("counts: update=%d awake=%d", report.UpdateToRender.Count, report.AwakeToRender.Count)
	}
	if report.UpdateToRender.Buckets.Under50ms != 1 || report.UpdateToRender.MaxNanos < uint64(30*time.Millisecond) {
		t.Fatalf("update latency = %+v", report.UpdateToRender)
	}
}

func TestLatencyRecordingIsNilSafe(t *testing.T) {
	var diagnostics *playbackDiagnostics
	diagnostics.recordMPVUpdateCallback()
	diagnostics.recordFLTKAwakeCallback()
	diagnostics.renderStarted(time.Now())
}
