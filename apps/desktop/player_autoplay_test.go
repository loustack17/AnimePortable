// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"testing"

	"animeportable/apps/desktop/backend"
)

type autoplayTestSettings struct {
	value backend.Settings
	err   error
	reads int
}

func (settings *autoplayTestSettings) Settings(context.Context) (backend.Settings, error) {
	settings.reads++
	return settings.value, settings.err
}

type autoplayTestPlayer struct {
	settings       *autoplayTestSettings
	generation     uint64 // the generation still playing; 0 when nothing is
	view           uint64 // bumped by every user action and autoplay switch
	playingID      string
	advanced       []int
	advancedPaused []bool
	finished       int
	failures       int
	failedCallback func()
	works, updates []func()
}

// newAutoplayTest plays episode "b" of a, b, c under generation 7. Background work and UI
// updates are queued; run drains both, and a test can run them one by one to replay a race.
func newAutoplayTest() (*playerAutoplay, *autoplayTestPlayer, func()) {
	player := &autoplayTestPlayer{
		settings:   &autoplayTestSettings{value: backend.Settings{AutoplayNext: "enabled", AutoPlay: "enabled"}},
		generation: 7,
		view:       1,
		playingID:  "b",
	}
	autoplay := &playerAutoplay{
		ctx:      context.Background(),
		settings: player.settings,
		start:    func(work func()) { player.works = append(player.works, work) },
		awake:    func(update func()) { player.updates = append(player.updates, update) },
		still:    func(generation uint64) bool { return generation == player.generation },
		view:     func() uint64 { return player.view },
		episodes: func() []backend.Episode {
			return []backend.Episode{{ID: "a"}, {ID: "b"}, {ID: "c"}}
		},
		playing: func() string { return player.playingID },
		advance: func(index int, paused bool, failed func()) {
			player.advanced = append(player.advanced, index)
			player.advancedPaused = append(player.advancedPaused, paused)
			player.failedCallback = failed
		},
		finish: func() { player.finished++ },
		fail:   func() { player.failures++ },
	}
	run := func() {
		for len(player.works)+len(player.updates) > 0 {
			works, updates := player.works, player.updates
			player.works, player.updates = nil, nil
			for _, work := range works {
				work()
			}
			for _, update := range updates {
				update()
			}
		}
	}
	return autoplay, player, run
}

func requireAutoplayCounts(t *testing.T, player *autoplayTestPlayer, advanced, finished, failures int) {
	t.Helper()
	if len(player.advanced) != advanced || player.finished != finished || player.failures != failures {
		t.Fatalf("advanced %v, finished %d, failures %d; want %d, %d, %d", player.advanced, player.finished, player.failures, advanced, finished, failures)
	}
}

func TestPlayerAutoplayAdvancesToTheNextOrderedEpisode(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	autoplay.ended(7)
	run()
	if len(player.advanced) != 1 || player.advanced[0] != 2 {
		t.Fatalf("advanced = %v, want [2]", player.advanced)
	}
	requireAutoplayCounts(t, player, 1, 0, 0)
	if player.settings.reads != 1 {
		t.Fatalf("settings reads = %d, want 1", player.settings.reads)
	}
}

func TestPlayerAutoplayLoadsThePlayingOrPausedByAutoPlay(t *testing.T) {
	for name, test := range map[string]struct {
		autoPlay   string
		wantPaused bool
	}{
		"auto play on":  {autoPlay: "enabled"},
		"auto play off": {autoPlay: "disabled", wantPaused: true},
	} {
		t.Run(name, func(t *testing.T) {
			autoplay, player, run := newAutoplayTest()
			player.settings.value = backend.Settings{AutoplayNext: "enabled", AutoPlay: test.autoPlay}
			autoplay.ended(7)
			run()
			requireAutoplayCounts(t, player, 1, 0, 0)
			if len(player.advancedPaused) != 1 || player.advancedPaused[0] != test.wantPaused {
				t.Fatalf("advanced paused = %v, want [%t]", player.advancedPaused, test.wantPaused)
			}
			if player.settings.reads != 1 {
				t.Fatalf("settings reads = %d, want 1", player.settings.reads)
			}
		})
	}
}

func TestPlayerAutoplayFinishesAtTheLastEpisode(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	player.playingID = "c"
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 0, 1, 0)
	if player.settings.reads != 0 {
		t.Fatalf("settings reads = %d, want none without a next episode", player.settings.reads)
	}
}

func TestPlayerAutoplayFinishesWhenDisabledOrSettingsUnreadable(t *testing.T) {
	for name, settings := range map[string]autoplayTestSettings{
		"disabled":                    {value: backend.Settings{AutoplayNext: "disabled", AutoPlay: "enabled"}},
		"disabled with auto play off": {value: backend.Settings{AutoplayNext: "disabled", AutoPlay: "disabled"}},
		"unreadable":                  {value: backend.Settings{AutoplayNext: "enabled", AutoPlay: "enabled"}, err: errors.New("database locked")},
	} {
		t.Run(name, func(t *testing.T) {
			autoplay, player, run := newAutoplayTest()
			*player.settings = settings
			autoplay.ended(7)
			run()
			requireAutoplayCounts(t, player, 0, 1, 0)
		})
	}
}

func TestPlayerAutoplayIgnoresASecondEndOfTheSameGeneration(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	autoplay.ended(7)
	autoplay.ended(7)
	run()
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 1, 0, 0)
	if player.settings.reads != 1 {
		t.Fatalf("settings reads = %d, want 1", player.settings.reads)
	}
}

func TestPlayerAutoplayHandlesTheSameGenerationOfANewSession(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 1, 0, 0)

	// A new playback session restarts its generations at 1, so it can reuse 7 later.
	player.view++
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 2, 0, 0)
}

func TestPlayerAutoplayIgnoresAReplayOfTheSameEpisodeDuringTheSettingsRead(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	autoplay.ended(7)
	// The user leaves and replays the episode: generation and episode ID match again.
	player.view++
	run()
	requireAutoplayCounts(t, player, 0, 0, 0)
}

func TestPlayerAutoplayDoesNothingWhenTheGenerationIsNoLongerPlaying(t *testing.T) {
	// Before the end is handled.
	autoplay, player, run := newAutoplayTest()
	player.generation = 0
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 0, 0, 0)
	if player.settings.reads != 0 {
		t.Fatalf("settings reads = %d, want none", player.settings.reads)
	}

	// While the settings are read: the user left, switched to a new load, or chose another episode.
	for name, change := range map[string]func(*autoplayTestPlayer){
		"left":     func(player *autoplayTestPlayer) { player.generation = 0 },
		"new load": func(player *autoplayTestPlayer) { player.generation = 8 },
		"episode":  func(player *autoplayTestPlayer) { player.playingID = "c" },
	} {
		t.Run(name, func(t *testing.T) {
			autoplay, player, run := newAutoplayTest()
			autoplay.ended(7)
			change(player)
			run()
			requireAutoplayCounts(t, player, 0, 0, 0)
		})
	}
}

func TestPlayerAutoplayReportsAFailedSwitchOnceAndStops(t *testing.T) {
	autoplay, player, run := newAutoplayTest()
	autoplay.ended(7)
	run()
	if player.failedCallback == nil {
		t.Fatal("advance got no failure callback")
	}
	player.failedCallback()
	player.failedCallback()
	requireAutoplayCounts(t, player, 1, 1, 1)

	// The stopped player is not advanced again by a repeated end.
	player.generation = 0
	autoplay.ended(7)
	run()
	requireAutoplayCounts(t, player, 1, 1, 1)
}
