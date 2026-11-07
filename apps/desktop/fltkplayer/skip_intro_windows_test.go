//go:build windows && cgo

package fltkplayer

import (
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

// inOpening is an episode playing 30 s into an opening from 20 s to 110 s.
func inOpening() State {
	return State{Volume: 100, Focus: focusProgress, Playing: true, Position: 30, Duration: 1420, OpeningStart: 20, OpeningEnd: 110}
}

func TestSkipIntroShowsOnlyInsideTheOpening(t *testing.T) {
	for _, test := range []struct {
		name  string
		edit  func(*State)
		shown bool
	}{
		{"inside", func(*State) {}, true},
		{"at the start", func(state *State) { state.Position = 20 }, true},
		{"before", func(state *State) { state.Position = 19.5 }, false},
		{"last second", func(state *State) { state.Position = 109.2 }, false},
		{"after", func(state *State) { state.Position = 110 }, false},
		{"no opening", func(state *State) { state.OpeningStart, state.OpeningEnd = 0, 0 }, false},
		{"loading", func(state *State) { state.Loading = true }, false},
		{"stopped", func(state *State) { state.Playing = false }, false},
		{"paused", func(state *State) { state.Paused = true }, true},
	} {
		state := inOpening()
		test.edit(&state)
		if got := state.skipShown(); got != test.shown {
			t.Errorf("%s: shown %t, want %t", test.name, got, test.shown)
		}
	}
}

func TestSkipIntroKeyJumpsToTheEndOfTheOpening(t *testing.T) {
	var seeks []int
	view := keyboardTestView(Callbacks{SeekTo: func(seconds int) { seeks = append(seeks, seconds) }})
	view.SetState(State{Volume: 100, Focus: focusProgress, Playing: true, Position: 5, Duration: 1420, OpeningStart: 20, OpeningEnd: 109.4})
	if keyDown(view, 's', 0) || len(seeks) != 0 {
		t.Fatalf("S before the opening was used: %v", seeks)
	}
	keyUp(view, 's')
	state := view.State()
	state.Position = 30
	view.SetState(state)
	if !keyDown(view, 'S', 0) || len(seeks) != 1 || seeks[0] != 110 {
		t.Fatalf("S in the opening sought %v, want [110]", seeks)
	}
	keyUp(view, 'S')
	if view.state.skipShown() {
		t.Fatal("the button still shows after skipping")
	}
	// Seeking back into the opening shows it again.
	state = view.State()
	state.Position = 25
	view.SetState(state)
	if !keyDown(view, 's', 0) || len(seeks) != 2 {
		t.Fatalf("S after seeking back sought %v", seeks)
	}
}

func TestSkipIntroJoinsTheTabOrderOnlyWhileShown(t *testing.T) {
	var seeks []int
	view := keyboardTestView(Callbacks{SeekTo: func(seconds int) { seeks = append(seeks, seconds) }})
	view.SetState(inOpening())
	press := func(key, modifiers int) {
		t.Helper()
		keyDown(view, key, modifiers)
		keyUp(view, key)
	}
	press(9, 0)
	if view.state.Focus != focusSkip {
		t.Fatalf("Tab from progress = %d, want skip", view.state.Focus)
	}
	press(9, fltk.SHIFT)
	press(9, 0)
	press(fltk.ENTER_KEY, 0)
	if len(seeks) != 1 || seeks[0] != 110 || view.state.Focus != focusProgress {
		t.Fatalf("Enter on skip: seeks %v, focus %d", seeks, view.state.Focus)
	}

	// Focus on the button moves to the progress bar when the opening ends by itself.
	view.SetState(inOpening())
	view.SetFocus(focusSkip)
	state := view.State()
	state.Position = 110
	view.SetState(state)
	if view.state.Focus != focusProgress {
		t.Fatalf("focus after the opening = %d, want progress", view.state.Focus)
	}
	press(9, 0)
	if view.state.Focus != focusPlayPause {
		t.Fatalf("Tab with no button = %d, want play/pause", view.state.Focus)
	}
	press(9, fltk.SHIFT)
	if view.state.Focus != focusProgress {
		t.Fatalf("Shift+Tab with no button = %d, want progress", view.state.Focus)
	}
}
