// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefresherRunsAtMostTheLimitAtOnce(t *testing.T) {
	refresher := NewRefresher(context.Background(), 4, RefreshInterval)
	defer refresher.Close()
	release := make(chan struct{})
	var running, highest atomic.Int32
	jobs := make([]func(context.Context) error, 10)
	for index := range jobs {
		jobs[index] = func(context.Context) error {
			now := running.Add(1)
			for {
				previous := highest.Load()
				if now <= previous || highest.CompareAndSwap(previous, now) {
					break
				}
			}
			<-release
			running.Add(-1)
			return nil
		}
	}
	done := make(chan RefreshResult)
	go func() { done <- refresher.Run(context.Background(), time.Minute, jobs...) }()
	waitFor(t, func() bool { return running.Load() == 4 })
	// Give a fifth job time to start if the limit did not hold.
	time.Sleep(20 * time.Millisecond)
	close(release)
	result := <-done
	if got := highest.Load(); got != 4 {
		t.Fatalf("highest running at once = %d, want 4", got)
	}
	if result != (RefreshResult{Total: 10}) {
		t.Fatalf("result = %+v, want 10 jobs and none failed", result)
	}
}

func TestRefresherLimitIsSharedAcrossRuns(t *testing.T) {
	refresher := NewRefresher(context.Background(), 2, RefreshInterval)
	defer refresher.Close()
	release := make(chan struct{})
	var running atomic.Int32
	job := func(context.Context) error { running.Add(1); <-release; running.Add(-1); return nil }
	var runs sync.WaitGroup
	for range 3 {
		runs.Add(1)
		go func() { defer runs.Done(); refresher.Run(context.Background(), time.Minute, job, job) }()
	}
	waitFor(t, func() bool { return running.Load() == 2 })
	time.Sleep(20 * time.Millisecond)
	if got := running.Load(); got != 2 {
		t.Fatalf("running across three runs = %d, want the limit 2", got)
	}
	close(release)
	runs.Wait()
}

func TestRefresherBackgroundLeavesASlotForTheUser(t *testing.T) {
	refresher := NewRefresher(context.Background(), 4, RefreshInterval)
	defer refresher.Close()
	release := make(chan struct{})
	var running atomic.Int32
	held := func(context.Context) error { running.Add(1); <-release; running.Add(-1); return nil }
	background := make(chan RefreshResult)
	go func() {
		background <- refresher.RunInBackground(context.Background(), time.Minute, held, held, held, held, held)
	}()
	waitFor(t, func() bool { return running.Load() == 3 })
	time.Sleep(20 * time.Millisecond)
	if got := running.Load(); got != 3 {
		t.Fatalf("background jobs running = %d, want 3 of the 4 slots", got)
	}
	asked := make(chan RefreshResult)
	go func() {
		asked <- refresher.Run(context.Background(), time.Minute, func(context.Context) error { return nil })
	}()
	select {
	case result := <-asked:
		if result.Failed != 0 {
			t.Fatalf("user job result = %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("a job the user asked for waited behind background refresh")
	}
	close(release)
	if result := <-background; result != (RefreshResult{Total: 5}) {
		t.Fatalf("background result = %+v", result)
	}
}

func TestRefresherStartsNoJobAfterItsContextEnded(t *testing.T) {
	refresher := NewRefresher(context.Background(), RefreshLimit, RefreshInterval)
	defer refresher.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 100 {
		started := false
		result := refresher.Run(ctx, time.Minute, func(context.Context) error { started = true; return nil })
		if started || result.Failed != 1 {
			t.Fatalf("a cancelled Run started its job (result %+v)", result)
		}
	}
}

func TestRefresherGivesEachJobTheTimeout(t *testing.T) {
	refresher := NewRefresher(context.Background(), RefreshLimit, RefreshInterval)
	defer refresher.Close()
	var deadline time.Time
	var hasDeadline bool
	began := time.Now()
	refresher.Run(context.Background(), 15*time.Second, func(ctx context.Context) error {
		deadline, hasDeadline = ctx.Deadline()
		return nil
	})
	if !hasDeadline || deadline.Before(began.Add(15*time.Second)) || deadline.After(time.Now().Add(15*time.Second)) {
		t.Fatalf("job deadline = %v (set %v), want 15 s after it started", deadline, hasDeadline)
	}
}

func TestRefresherCountsFailedJobs(t *testing.T) {
	refresher := NewRefresher(context.Background(), RefreshLimit, RefreshInterval)
	defer refresher.Close()
	fail := func(context.Context) error { return errors.New("offline") }
	succeed := func(context.Context) error { return nil }
	if result := refresher.Run(context.Background(), time.Minute, fail, succeed, fail); result != (RefreshResult{Total: 3, Failed: 2}) {
		t.Fatalf("result = %+v, want 3 jobs with 2 failed", result)
	}
}

func TestRefresherCloseStopsRunningAndWaitingJobs(t *testing.T) {
	baseline := runtime.NumGoroutine()
	refresher := NewRefresher(context.Background(), 2, RefreshInterval)
	var started, returned atomic.Int32
	held := func(ctx context.Context) error {
		started.Add(1)
		<-ctx.Done()
		returned.Add(1)
		return ctx.Err()
	}
	done := make(chan RefreshResult)
	go func() { done <- refresher.Run(context.Background(), time.Hour, held, held, held, held, held) }()
	waitFor(t, func() bool { return started.Load() == 2 })
	refresher.Close()
	if returned.Load() != 2 {
		t.Fatalf("Close returned with %d of 2 running jobs still running", 2-returned.Load())
	}
	result := <-done
	if started.Load() != 2 || result != (RefreshResult{Total: 5, Failed: 5}) {
		t.Fatalf("started %d jobs, result %+v; want 2 started and all 5 failed", started.Load(), result)
	}
	if result := refresher.Run(context.Background(), time.Minute, held); result.Failed != 1 || started.Load() != 2 {
		t.Fatal("a closed refresher started a job")
	}
	waitFor(t, func() bool { return runtime.NumGoroutine() <= baseline })
}

func TestRefresherStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	refresher := NewRefresher(ctx, 2, RefreshInterval)
	done := make(chan RefreshResult)
	go func() {
		done <- refresher.Run(context.Background(), time.Hour, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() })
	}()
	cancel()
	select {
	case result := <-done:
		if result.Failed != 1 {
			t.Fatalf("result = %+v, want the cancelled job failed", result)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelling the refresher's context did not stop its job")
	}
}

func TestRefresherAttemptThrottlesEachKind(t *testing.T) {
	refresher := NewRefresher(context.Background(), RefreshLimit, 10*time.Minute)
	defer refresher.Close()
	now := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	refresher.now = func() time.Time { return now }
	if !refresher.Attempt("following", false) {
		t.Fatal("first refresh refused")
	}
	if refresher.Attempt("following", false) {
		t.Fatal("second refresh within the interval allowed")
	}
	if !refresher.Attempt("schedule", false) {
		t.Fatal("another kind was throttled by the first")
	}
	if !refresher.Attempt("following", true) {
		t.Fatal("forced refresh refused")
	}
	now = now.Add(9 * time.Minute)
	if refresher.Attempt("following", false) {
		t.Fatal("refresh allowed 9 minutes after a forced one")
	}
	now = now.Add(time.Minute)
	if !refresher.Attempt("following", false) {
		t.Fatal("refresh refused after the interval")
	}
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !ready() {
		t.Fatal("condition not reached within 1 s")
	}
}
