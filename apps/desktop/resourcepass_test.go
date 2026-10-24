//go:build resourcepass

// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/internal/resourcepass"
)

type resourceOpenings struct{ calls atomic.Int64 }

// OpeningTimes answers at once for every other call and otherwise waits until the lookup is
// cancelled, as a lookup does when the user leaves the episode first.
func (openings *resourceOpenings) OpeningTimes(ctx context.Context, _, _ string, _ int64) (backend.Opening, error) {
	if openings.calls.Add(1)%2 == 0 {
		<-ctx.Done()
		return backend.Opening{}, ctx.Err()
	}
	return backend.Opening{Found: true, Start: 1000, End: 90000}, nil
}

type resourceSettings struct{}

func (resourceSettings) Settings(context.Context) (backend.Settings, error) {
	return backend.DefaultSettings(), nil
}

// The player's start watch, opening lookup and autoplay run per episode with their own timers
// and goroutines. Each cycle is one episode as main drives them: the start is watched, the
// opening is looked up, the first frame arrives, and the episode ends so autoplay switches to
// the next one; every other lookup is still running when the episode is left. The UI thread
// is one goroutine that runs updates in order, as fltk.Awake does.
func TestResourcePassPlayerHelpers(t *testing.T) {
	updates := make(chan func(), 64)
	uiDone := make(chan struct{})
	onUI := func(update func()) {
		done := make(chan struct{})
		updates <- func() { update(); close(done) }
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the UI thread stopped")
		}
	}
	var view atomic.Uint64
	var playing atomic.Int64
	episodes := []backend.Episode{{ID: "episode-1"}, {ID: "episode-2"}, {ID: "episode-3"}}
	advanced := make(chan struct{}, 1)
	var cancel context.CancelFunc
	var watch *playerStartWatch
	var opening *playerOpening
	var autoplay *playerAutoplay
	resourcepass.Run(t, "player-helpers", 0, func() {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go func() {
			defer close(uiDone)
			for update := range updates {
				update()
			}
		}()
		awake := func(update func()) { updates <- update }
		watch = &playerStartWatch{
			after: func(delay time.Duration, run func()) func() bool { return time.AfterFunc(delay, run).Stop },
			awake: awake, view: view.Load,
			fail: func() { t.Error("a start that showed its first frame was reported as failed") },
		}
		opening = &playerOpening{ctx: ctx, service: &resourceOpenings{}, start: func(work func()) { go work() }, awake: awake, view: view.Load, show: func(backend.Opening) {}}
		autoplay = &playerAutoplay{
			ctx: ctx, settings: resourceSettings{}, start: func(work func()) { go work() }, awake: awake,
			still: func(uint64) bool { return true }, view: view.Load,
			episodes: func() []backend.Episode { return episodes },
			playing:  func() string { return episodes[playing.Load()%int64(len(episodes))].ID },
			advance: func(index int, _ bool, _ func()) {
				playing.Store(int64(index))
				advanced <- struct{}{}
			},
			finish: func() { playing.Store(0); advanced <- struct{}{} },
			fail:   func() { t.Error("an autoplay switch failed") },
		}
	}, func(int) {
		generation := view.Add(1)
		onUI(func() {
			watch.begin(generation)
			opening.playing(generation, backend.PlayRequest{AnimeID: "anime", EpisodeID: autoplay.playing()}, 24*time.Minute)
			watch.playing(generation)
			autoplay.ended(generation)
		})
		select {
		case <-advanced:
		case <-time.After(5 * time.Second):
			t.Fatal("autoplay never moved on")
		}
		onUI(func() { opening.stop(); watch.end() })
	}, func() {
		cancel()
		close(updates)
		<-uiDone
	})
}
