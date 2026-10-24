// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/playback/proxy"
)

func TestStartTimelineIsOffUnlessEnabled(t *testing.T) {
	directory := t.TempDir()
	timeline := NewStartTimeline(directory, false)
	if timeline != nil {
		t.Fatal("timeline is on without being enabled")
	}
	timeline.Begin()
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamOpened})
	timeline.Playing()
	if _, err := os.Stat(filepath.Join(directory, startTimelineName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("timeline file exists: %v", err)
	}
}

func TestStartTimelineRecordsOnlyFixedFieldsUntilTheFirstFrame(t *testing.T) {
	directory := t.TempDir()
	timeline := NewStartTimeline(directory, true)
	clock := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	timeline.now = func() time.Time {
		clock = clock.Add(100 * time.Millisecond)
		return clock
	}
	secret := errors.New("GET https://media.example/video.mp4?token=secret-token Cookie: session=secret-cookie")
	timeline.Begin()
	timeline.resolved(750*time.Millisecond, nil)
	timeline.PlayReturned(nil)
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamOpened, RangeStart: 0, Status: 206, Elapsed: 310 * time.Millisecond})
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamFailed, RangeStart: 5000, Status: 502, Elapsed: 40 * time.Millisecond})
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamBusy, RangeStart: -1})
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamFinished, RangeStart: 0, Status: 206, Elapsed: 900 * time.Millisecond, Bytes: 2048})
	timeline.FileLoaded()
	timeline.Playing()
	// After the first frame nothing more is recorded until the next click.
	timeline.Stream(proxy.StreamEvent{Outcome: proxy.StreamOpened, RangeStart: 9999, Status: 206})
	timeline.Begin()
	timeline.resolved(time.Second, &securehttp.Error{Kind: securehttp.KindTimeout})
	timeline.PlayReturned(secret)
	timeline.TimedOut()

	want := []string{
		"2026-10-22T12:00:00Z #1 +0ms click",
		"2026-10-22T12:00:00Z #1 +100ms resolve took=750ms result=ok",
		"2026-10-22T12:00:00Z #1 +200ms play_returned result=ok",
		"2026-10-22T12:00:00Z #1 +300ms stream_open start=0 status=206 took=310ms",
		"2026-10-22T12:00:00Z #1 +400ms stream_failed start=5000 status=502 took=40ms",
		"2026-10-22T12:00:00Z #1 +500ms stream_busy start=-1 status=0 took=0ms",
		"2026-10-22T12:00:00Z #1 +600ms stream_end start=0 status=206 took=900ms bytes=2048",
		"2026-10-22T12:00:00Z #1 +700ms file_loaded",
		"2026-10-22T12:00:00Z #1 +800ms playing",
		"2026-10-22T12:00:01Z #2 +0ms click",
		"2026-10-22T12:00:01Z #2 +100ms resolve took=1000ms result=timeout",
		"2026-10-22T12:00:01Z #2 +200ms play_returned result=unknown",
		"2026-10-22T12:00:01Z #2 +300ms start_timeout",
	}
	got := waitForTimelineLines(t, filepath.Join(directory, startTimelineName), " start_timeout")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("timeline:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, line := range got {
		for _, forbidden := range []string{"https", "media.example", "token", "secret", "Cookie"} {
			if strings.Contains(line, forbidden) {
				t.Fatalf("line %q contains %q", line, forbidden)
			}
		}
	}
}

func TestStartTimelineStartsAFreshFileEachRun(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, startTimelineName)
	if err := os.WriteFile(path, []byte("line from an earlier run\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	timeline := NewStartTimeline(directory, true)
	timeline.Begin()
	if got := waitForTimelineLines(t, path, " click"); len(got) != 1 {
		t.Fatalf("timeline = %q", got)
	}
}

func TestStartTimelineStopsAfterMaxAttempts(t *testing.T) {
	directory := t.TempDir()
	timeline := NewStartTimeline(directory, true)
	for range startTimelineMaxAttempts + 1 {
		timeline.Begin()
		timeline.TimedOut()
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		timeline.mu.Lock()
		idle := !timeline.flushing && len(timeline.pending) == 0
		timeline.mu.Unlock()
		if idle {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timeline writer never finished")
		}
		time.Sleep(5 * time.Millisecond)
	}
	data, err := os.ReadFile(filepath.Join(directory, startTimelineName))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2*startTimelineMaxAttempts || strings.Contains(string(data), "#"+strconv.Itoa(startTimelineMaxAttempts+1)+" ") {
		t.Fatalf("timeline has %d lines, want %d and no attempt past the limit", len(lines), 2*startTimelineMaxAttempts)
	}
}

// waitForTimelineLines waits for the writer to reach a line ending in last.
func waitForTimelineLines(t *testing.T, path, last string) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(path)
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if strings.HasSuffix(lines[len(lines)-1], last) {
			return lines
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeline never reached %q: %q", last, data)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
