// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"animeportable/adapters/playback/proxy"
)

const (
	startTimelineName = "playback-timeline.log"
	// StartTimelineVariable turns the timeline on when set to 1 at launch. It is a measuring
	// aid for developers (Loop 37d), not a user setting.
	StartTimelineVariable = "ANIMEPORTABLE_START_TIMELINE"
	// A start that never plays must not fill the disk with mpv's retries.
	startTimelineMaxEvents = 200
)

type timelineEvent string

const (
	eventClick        timelineEvent = "click"
	eventResolve      timelineEvent = "resolve"
	eventPlayReturned timelineEvent = "play_returned"
	eventStreamBusy   timelineEvent = "stream_busy"
	eventStreamOpen   timelineEvent = "stream_open"
	eventStreamFailed timelineEvent = "stream_failed"
	eventStreamEnd    timelineEvent = "stream_end"
	eventFileLoaded   timelineEvent = "file_loaded"
	eventPlaying      timelineEvent = "playing"
	eventStartTimeout timelineEvent = "start_timeout"
)

// StartTimeline records, for each episode start, how long each step took from the click to
// the first frame. Lines hold only fixed event names, numbers and failure categories, never
// URLs, tokens, cookies, titles or ids (docs/03_SECURITY.md, Logging). Recording of an attempt
// stops once it plays or times out. A nil *StartTimeline is a valid no-op.
type StartTimeline struct {
	path string
	now  func() time.Time

	mu       sync.Mutex
	attempt  int
	began    time.Time
	events   int
	open     bool
	pending  []string
	flushing bool
	// truncate is set until the first write of the app run, so each run starts a fresh file.
	truncate bool
}

// NewStartTimeline returns a timeline writing next to the database, or nil (off) unless
// enabled is true.
func NewStartTimeline(directory string, enabled bool) *StartTimeline {
	if !enabled {
		return nil
	}
	return &StartTimeline{path: filepath.Join(directory, startTimelineName), now: time.Now, truncate: true}
}

// Begin starts a new attempt at the user's click.
func (timeline *StartTimeline) Begin() {
	if timeline == nil {
		return
	}
	timeline.mu.Lock()
	defer timeline.mu.Unlock()
	timeline.attempt++
	timeline.began = timeline.now()
	timeline.events = 0
	timeline.open = true
	timeline.appendLine(timeline.began, eventClick, "")
}

func (timeline *StartTimeline) resolved(elapsed time.Duration, err error) {
	timeline.add(eventResolve, " took="+milliseconds(elapsed)+" result="+timelineResult(err))
}

// PlayReturned marks the end of the backend's Play call, which resolves and hands mpv the URL.
func (timeline *StartTimeline) PlayReturned(err error) {
	timeline.add(eventPlayReturned, " result="+timelineResult(err))
}

// Stream records one playback proxy request.
func (timeline *StartTimeline) Stream(event proxy.StreamEvent) {
	var name timelineEvent
	switch event.Outcome {
	case proxy.StreamBusy:
		name = eventStreamBusy
	case proxy.StreamOpened:
		name = eventStreamOpen
	case proxy.StreamFailed:
		name = eventStreamFailed
	case proxy.StreamFinished:
		name = eventStreamEnd
	default:
		return
	}
	fields := " start=" + strconv.FormatInt(event.RangeStart, 10) + " status=" + strconv.Itoa(event.Status) + " took=" + milliseconds(event.Elapsed)
	if event.Outcome == proxy.StreamFinished {
		fields += " bytes=" + strconv.FormatInt(event.Bytes, 10)
	}
	timeline.add(name, fields)
}

// FileLoaded marks mpv having opened the file.
func (timeline *StartTimeline) FileLoaded() { timeline.add(eventFileLoaded, "") }

// Playing marks the first frame and ends the attempt.
func (timeline *StartTimeline) Playing() { timeline.finish(eventPlaying) }

// TimedOut marks a start the watchdog gave up on and ends the attempt.
func (timeline *StartTimeline) TimedOut() { timeline.finish(eventStartTimeout) }

func (timeline *StartTimeline) finish(name timelineEvent) {
	timeline.add(name, "")
	if timeline == nil {
		return
	}
	timeline.mu.Lock()
	timeline.open = false
	timeline.mu.Unlock()
}

func (timeline *StartTimeline) add(name timelineEvent, fields string) {
	if timeline == nil {
		return
	}
	timeline.mu.Lock()
	defer timeline.mu.Unlock()
	if !timeline.open || timeline.events >= startTimelineMaxEvents {
		return
	}
	timeline.appendLine(timeline.now(), name, fields)
}

// appendLine queues one line and starts the writer if none runs; the caller holds mu.
func (timeline *StartTimeline) appendLine(at time.Time, name timelineEvent, fields string) {
	timeline.events++
	line := at.UTC().Format(time.RFC3339) + " #" + strconv.Itoa(timeline.attempt) + " +" + milliseconds(at.Sub(timeline.began)) + " " + string(name) + fields
	timeline.pending = append(timeline.pending, line)
	if !timeline.flushing {
		timeline.flushing = true
		// Callers include the UI thread and the proxy's request goroutines; the disk is not.
		go timeline.flush()
	}
}

func (timeline *StartTimeline) flush() {
	for {
		timeline.mu.Lock()
		lines := timeline.pending
		timeline.pending = nil
		truncate := timeline.truncate
		timeline.truncate = false
		if len(lines) == 0 {
			timeline.flushing = false
			timeline.mu.Unlock()
			return
		}
		timeline.mu.Unlock()
		flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
		if truncate {
			flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
		}
		// Diagnostics must never break playback, so a failed write is dropped.
		if file, err := os.OpenFile(timeline.path, flags, 0o600); err == nil {
			_, _ = file.WriteString(strings.Join(lines, "\n") + "\n")
			_ = file.Close()
		}
	}
}

func milliseconds(elapsed time.Duration) string {
	return strconv.FormatInt(elapsed.Milliseconds(), 10) + "ms"
}

// timelineResult is "ok" or the failure log's fixed category; error text never reaches the file.
func timelineResult(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, context.Canceled):
		return "canceled"
	}
	return string(categoryFor(err))
}
