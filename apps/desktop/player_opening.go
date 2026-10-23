// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"time"

	"animeportable/apps/desktop/backend"
)

// openingService is the part of the backend the player's 跳過片頭 button uses.
type openingService interface {
	OpeningTimes(ctx context.Context, animeID, episodeID string, lengthMillis int64) (backend.Opening, error)
}

// playerOpening asks once per load for the opening of the episode playing, as soon as the
// player knows the episode's length, and shows it while that load is still on screen. A failed
// or missing lookup shows nothing; playback never waits for it. playing and stop run on the UI
// thread.
type playerOpening struct {
	ctx     context.Context
	service openingService
	start   func(work func())   // runs work off the UI thread
	awake   func(update func()) // runs an update on the UI thread
	view    func() uint64       // the load on screen
	show    func(backend.Opening)
	asked   uint64
	// cancel ends the lookup of the last load, so a lookup nobody waits for does not hold one of
	// the backend's operation slots.
	cancel context.CancelFunc
}

// playing is told about every playback state of load generation; the first one with a length
// starts the lookup for the episode request names.
func (opening *playerOpening) playing(generation uint64, request backend.PlayRequest, length time.Duration) {
	if generation == opening.asked || length <= 0 || request.AnimeID == "" || request.EpisodeID == "" {
		return
	}
	opening.asked = generation
	opening.stop()
	lookupCtx, cancel := context.WithCancel(opening.ctx)
	opening.cancel = cancel
	opening.start(func() {
		defer cancel()
		found, err := opening.service.OpeningTimes(lookupCtx, request.AnimeID, request.EpisodeID, length.Milliseconds())
		opening.awake(func() {
			if err == nil && found.Found && opening.view() == generation {
				opening.show(found)
			}
		})
	})
}

// stop ends the lookup in progress, when the player leaves the episode it was for.
func (opening *playerOpening) stop() {
	if opening.cancel != nil {
		opening.cancel()
		opening.cancel = nil
	}
}
