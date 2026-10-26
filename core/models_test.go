// SPDX-License-Identifier: MPL-2.0

package core

import "testing"

func TestPlaybackSpeedRange(t *testing.T) {
	for _, speed := range []PlaybackSpeed{0, 75, 100, 125, 150, 200} {
		if !speed.Valid() {
			t.Errorf("PlaybackSpeed(%d).Valid() = false", speed)
		}
	}
	for _, speed := range []PlaybackSpeed{-100, 50, 110, 175, 250} {
		if speed.Valid() {
			t.Errorf("PlaybackSpeed(%d).Valid() = true", speed)
		}
	}
}
