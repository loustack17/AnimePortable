// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"slices"

	"animeportable/apps/desktop/backend"
)

// autoplaySettings is the part of the backend autoplay reads.
type autoplaySettings interface {
	Settings(context.Context) (backend.Settings, error)
}

// playerAutoplay decides what happens when an episode ends normally. ended, and the updates
// passed to awake, run on the UI thread; the settings read runs through start. A decision is
// made once per view and only while that generation is still playing in that view. Load
// generations restart in every playback session, so the view token is what tells two
// sessions that reuse a generation apart.
type playerAutoplay struct {
	ctx      context.Context
	settings autoplaySettings
	start    func(work func())   // runs work off the UI thread
	awake    func(update func()) // runs an update on the UI thread
	still    func(generation uint64) bool
	view     func() uint64            // changes on every user action and every autoplay switch
	episodes func() []backend.Episode // the ordered list the player shows
	playing  func() string            // the playing episode ID
	// advance switches to episodes()[index]; it calls failed if the switch fails.
	advance func(index int, failed func())
	finish  func() // ends playback and leaves the player showing the stopped episode
	fail    func() // reports a failed switch
	handled uint64 // the view token of the last end handled
}

// ended handles the normal end of the file loaded as generation.
func (autoplay *playerAutoplay) ended(generation uint64) {
	view := autoplay.view()
	if view == autoplay.handled || !autoplay.still(generation) {
		return
	}
	autoplay.handled = view
	playingID := autoplay.playing()
	episodes := autoplay.episodes()
	current := slices.IndexFunc(episodes, func(episode backend.Episode) bool { return episode.ID == playingID })
	next := current + 1
	if current < 0 || next >= len(episodes) {
		autoplay.finish()
		return
	}
	autoplay.start(func() {
		settings, err := autoplay.settings.Settings(autoplay.ctx)
		autoplay.awake(func() {
			if autoplay.view() != view || !autoplay.still(generation) || autoplay.playing() != playingID {
				return
			}
			if !autoplayEnabled(settings, err) {
				autoplay.finish()
				return
			}
			// advance's contract does not promise a single failure call; report once.
			reported := false
			autoplay.advance(next, func() {
				if reported {
					return
				}
				reported = true
				// Finish first: the failure message is modal, and the player must already be
				// stopped while it shows.
				autoplay.finish()
				autoplay.fail()
			})
		})
	})
}

// autoplayEnabled is the single on/off decision. It is the stored AutoplayNext setting read
// when an episode ends, so the control that saves it needs no other change; an unreadable
// setting stops playback rather than guessing.
func autoplayEnabled(settings backend.Settings, err error) bool {
	return err == nil && settings.AutoplayNext == "enabled"
}
