// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"sync"
	"time"
)

// Background refresh policy. Screens show stored data first and refresh it from the network
// behind it. The limit keeps refresh from flooding the sources or taking every backend
// operation slot that the screens' own reads need.
const (
	// RefreshLimit is how many refresh requests may run at once across the app. Background
	// refresh leaves one of them free, so what the user asked for never waits behind it.
	RefreshLimit = 4
	// RefreshInterval is the least time between automatic refreshes of one kind of data.
	RefreshInterval = 10 * time.Minute
	// EpisodeRefreshTimeout bounds one anime's episode list, so one slow series cannot
	// stall the others.
	EpisodeRefreshTimeout = 15 * time.Second
	// ScheduleStepTimeout bounds each of a schedule refresh's two lookups: the weekly
	// calendar and the catalog it is matched against.
	ScheduleStepTimeout = 20 * time.Second
	// ScheduleRefreshTimeout bounds a whole schedule refresh: both lookups, with room to
	// store the result.
	ScheduleRefreshTimeout = 2*ScheduleStepTimeout + 5*time.Second
	// MetadataRefreshTimeout bounds one anime's AniList and Bangumi lookup.
	MetadataRefreshTimeout = 20 * time.Second
)

// RefreshResult counts the jobs of one Run and how many of them failed or never started.
type RefreshResult struct {
	Total, Failed int
}

// Refresher runs refresh jobs: at most a fixed number at once across every Run, each under
// a timeout, all stopped by Close or by the context it was made with.
type Refresher struct {
	slots chan struct{}
	// background holds one fewer token than slots; background jobs take one first.
	background chan struct{}
	interval   time.Duration
	now        func() time.Time
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	closed     bool
	running    sync.WaitGroup
	attempts   map[string]time.Time
}

// NewRefresher makes a Refresher that runs at most limit jobs at once (at least 2, so
// background work can leave one free) and lets each kind of automatic refresh start once per
// interval. Cancelling ctx stops all its work.
func NewRefresher(ctx context.Context, limit int, interval time.Duration) *Refresher {
	limit = max(limit, 2)
	refreshCtx, cancel := context.WithCancel(ctx)
	return &Refresher{
		slots:      make(chan struct{}, limit),
		background: make(chan struct{}, limit-1),
		interval:   interval,
		now:        time.Now,
		ctx:        refreshCtx,
		cancel:     cancel,
		attempts:   make(map[string]time.Time),
	}
}

// Attempt reports whether a refresh of kind should start now and, if so, records it, so an
// automatic refresh runs at most once per interval whether it succeeds or not. A forced
// refresh, asked for by the user, always starts.
func (refresher *Refresher) Attempt(kind string, force bool) bool {
	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	now := refresher.now()
	if last, ok := refresher.attempts[kind]; ok && !force && now.Sub(last) < refresher.interval {
		return false
	}
	refresher.attempts[kind] = now
	return true
}

// Run runs jobs the user asked for, such as a button press or an opened page. They may use
// every slot, including the one background refresh leaves free.
func (refresher *Refresher) Run(ctx context.Context, timeout time.Duration, jobs ...func(context.Context) error) RefreshResult {
	return refresher.run(ctx, timeout, false, jobs)
}

// RunInBackground runs jobs nobody is waiting for, such as an automatic refresh. They leave
// one slot free for what the user asks for meanwhile.
func (refresher *Refresher) RunInBackground(ctx context.Context, timeout time.Duration, jobs ...func(context.Context) error) RefreshResult {
	return refresher.run(ctx, timeout, true, jobs)
}

// run runs every job, each under timeout once a slot is free, and returns when all started
// jobs have returned. Jobs that cannot start before ctx ends or the refresher closes count
// as failed. A job must not call Close: Close waits for the job's own Run.
func (refresher *Refresher) run(ctx context.Context, timeout time.Duration, background bool, jobs []func(context.Context) error) RefreshResult {
	result := RefreshResult{Total: len(jobs)}
	refresher.mu.Lock()
	if refresher.closed {
		refresher.mu.Unlock()
		result.Failed = len(jobs)
		return result
	}
	refresher.running.Add(1)
	refresher.mu.Unlock()
	defer refresher.running.Done()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(refresher.ctx, cancel)
	defer stop()
	var mu sync.Mutex
	var started sync.WaitGroup
	for index, job := range jobs {
		release, ok := refresher.acquire(runCtx, background)
		if !ok {
			started.Wait()
			result.Failed += len(jobs) - index
			return result
		}
		started.Add(1)
		go func() {
			defer started.Done()
			defer release()
			jobCtx, cancelJob := context.WithTimeout(runCtx, timeout)
			defer cancelJob()
			if err := job(jobCtx); err != nil {
				mu.Lock()
				result.Failed++
				mu.Unlock()
			}
		}()
	}
	started.Wait()
	return result
}

// acquire takes a slot, and first a background token for background work. It fails once
// ctx ends, even when a slot is free at the same moment, so no job starts cancelled.
func (refresher *Refresher) acquire(ctx context.Context, background bool) (func(), bool) {
	if background {
		select {
		case refresher.background <- struct{}{}:
		case <-ctx.Done():
			return nil, false
		}
	}
	releaseBackground := func() {
		if background {
			<-refresher.background
		}
	}
	select {
	case refresher.slots <- struct{}{}:
	case <-ctx.Done():
		releaseBackground()
		return nil, false
	}
	release := func() { <-refresher.slots; releaseBackground() }
	// The refresher's own context is checked too: Close cancels ctx only asynchronously.
	if ctx.Err() != nil || refresher.ctx.Err() != nil {
		release()
		return nil, false
	}
	return release, true
}

// Close stops running refresh work, refuses new work and returns once every Run has
// returned.
func (refresher *Refresher) Close() {
	refresher.mu.Lock()
	refresher.closed = true
	refresher.mu.Unlock()
	refresher.cancel()
	refresher.running.Wait()
}
