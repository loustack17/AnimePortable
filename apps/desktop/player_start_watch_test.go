// SPDX-License-Identifier: MPL-2.0

package main

import (
	"testing"
	"time"
)

// fakeStartTimers runs nothing by itself: fire runs a pending timer, as if its delay passed.
type fakeStartTimers struct {
	delays  []time.Duration
	pending []func()
	stopped int
}

func (timers *fakeStartTimers) after(delay time.Duration, run func()) func() bool {
	timers.delays = append(timers.delays, delay)
	index := len(timers.pending)
	timers.pending = append(timers.pending, run)
	return func() bool {
		if timers.pending[index] == nil {
			return false
		}
		timers.pending[index] = nil
		timers.stopped++
		return true
	}
}

func (timers *fakeStartTimers) fire(index int) {
	// A real timer can fire after Stop lost the race; the watch must cope with that too.
	if run := timers.pending[index]; run != nil {
		run()
	}
}

func newTestStartWatch(view *uint64, failures *int) (*playerStartWatch, *fakeStartTimers) {
	timers := &fakeStartTimers{}
	return &playerStartWatch{
		after: timers.after,
		awake: func(update func()) { update() },
		view:  func() uint64 { return *view },
		fail:  func() { *failures++ },
	}, timers
}

func TestStartWatchFailsAStartWithoutAFirstFrameAfter20Seconds(t *testing.T) {
	view, failures := uint64(1), 0
	watch, timers := newTestStartWatch(&view, &failures)
	watch.begin(1)
	if len(timers.delays) != 1 || timers.delays[0] != 20*time.Second {
		t.Fatalf("delays = %v", timers.delays)
	}
	timers.fire(0)
	if failures != 1 {
		t.Fatalf("failures = %d", failures)
	}
	timers.fire(0)
	if failures != 1 {
		t.Fatalf("a second firing failed again: %d", failures)
	}
}

func TestStartWatchIgnoresAStartThatPlayed(t *testing.T) {
	view, failures := uint64(1), 0
	watch, timers := newTestStartWatch(&view, &failures)
	watch.begin(1)
	watch.playing(1)
	if timers.stopped != 1 {
		t.Fatalf("timer not stopped")
	}
	// The timer fired just before it was stopped.
	timers.pending[0] = func() { watch.awake(func() {}) }
	timers.fire(0)
	watch.playing(1)
	if failures != 0 {
		t.Fatalf("failures = %d", failures)
	}
}

func TestStartWatchIgnoresAStartTheUserLeftOrReplaced(t *testing.T) {
	view, failures := uint64(1), 0
	watch, timers := newTestStartWatch(&view, &failures)
	watch.begin(1)
	run := timers.pending[0]
	view = 2 // the user stopped or switched episodes before the timer ran
	run()
	if failures != 0 {
		t.Fatalf("left start failed: %d", failures)
	}

	watch.begin(2)
	firstOfSecond := timers.pending[1]
	watch.begin(3) // a new episode replaced the start of view 2
	view = 3
	firstOfSecond()
	if failures != 0 {
		t.Fatalf("replaced start failed: %d", failures)
	}
	timers.fire(2)
	if failures != 1 {
		t.Fatalf("current start: failures = %d", failures)
	}
}

func TestStartWatchEndStopsWatching(t *testing.T) {
	view, failures := uint64(1), 0
	watch, timers := newTestStartWatch(&view, &failures)
	watch.begin(1)
	run := timers.pending[0]
	watch.end()
	run()
	if failures != 0 || timers.stopped != 1 {
		t.Fatalf("failures=%d stopped=%d", failures, timers.stopped)
	}
}
