// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"slices"

	"animeportable/apps/desktop/backend"
)

// followService is the part of the backend the player's follow star uses.
type followService interface {
	Following(context.Context) ([]backend.Following, error)
	Follow(context.Context, string) error
	Unfollow(context.Context, string) error
}

// playerFollow keeps the player's follow star in step with the stored follow list. load and
// toggle run on the UI thread; the service calls run through start and report back through
// awake. A result is shown only while the same anime is still playing.
type playerFollow struct {
	ctx     context.Context
	service followService
	start   func(work func())   // runs work off the UI thread
	awake   func(update func()) // runs an update on the UI thread
	playing func() string       // the anime being played
	show    func(known, following bool)
	fail    func(error)
	pending bool
	// changes counts toggle starts and ends, so a load that began before a change ended
	// (and so may have read the list before it) is dropped.
	changes int
}

// load reads whether animeID is followed; until it lands the star stays hidden. A failed
// read leaves it hidden.
func (follow *playerFollow) load(animeID string) {
	changes := follow.changes
	follow.start(func() {
		list, err := follow.service.Following(follow.ctx)
		following := slices.ContainsFunc(list, func(item backend.Following) bool { return item.AnimeID == animeID })
		follow.awake(func() {
			if err == nil && changes == follow.changes && follow.playing() == animeID {
				follow.show(true, following)
			}
		})
	})
}

// toggle follows animeID, or unfollows it when following; one change runs at a time.
func (follow *playerFollow) toggle(animeID string, following bool) {
	if follow.pending {
		return
	}
	follow.pending = true
	follow.changes++
	follow.start(func() {
		var err error
		if following {
			err = follow.service.Unfollow(follow.ctx, animeID)
		} else {
			err = follow.service.Follow(follow.ctx, animeID)
		}
		follow.awake(func() {
			follow.pending = false
			follow.changes++
			if err != nil {
				follow.fail(err)
			}
			if playing := follow.playing(); err == nil && playing == animeID {
				follow.show(true, !following)
			} else if playing != "" {
				// A load that overlapped this change was dropped; read the state again.
				follow.load(playing)
			}
		})
	})
}
