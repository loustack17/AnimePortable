// SPDX-License-Identifier: MPL-2.0

package main

import "time"

// startTimeout is how long a start may go without a first frame before it is reported as
// failed: a normal start takes about 2 s, so this only catches starts that hang (Loop 37d).
const startTimeout = 20 * time.Second

// playerStartWatch reports a start that shows no first frame in time instead of leaving the
// player loading forever. begin, playing and the update passed to awake run on the UI thread.
type playerStartWatch struct {
	after func(time.Duration, func()) (stop func() bool) // runs func once after the delay
	awake func(update func())                            // runs an update on the UI thread
	view  func() uint64                                  // changes on every user action
	fail  func()                                         // ends the start and tells the user

	watching uint64 // the view being watched, 0 for none
	stop     func() bool
}

// begin watches the start made in view, replacing any earlier watch.
func (watch *playerStartWatch) begin(view uint64) {
	watch.end()
	watch.watching = view
	watch.stop = watch.after(startTimeout, func() {
		watch.awake(func() {
			if watch.watching != view || watch.view() != view {
				return
			}
			watch.end()
			watch.fail()
		})
	})
}

// playing ends the watch of view: its first frame arrived.
func (watch *playerStartWatch) playing(view uint64) {
	if watch.watching == view {
		watch.end()
	}
}

// end stops watching, for example when the user leaves the player.
func (watch *playerStartWatch) end() {
	if watch.stop != nil {
		watch.stop()
	}
	watch.watching, watch.stop = 0, nil
}
