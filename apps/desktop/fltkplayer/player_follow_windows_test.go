//go:build windows && cgo

package fltkplayer

import (
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

func TestPlayerFollowStarJoinsTabRingOnlyWhenStateKnown(t *testing.T) {
	toggles, plays := 0, 0
	view := keyboardTestView(Callbacks{ToggleFollow: func() { toggles++ }, PlayPause: func() { plays++ }})
	view.state.Focus = focusMenu
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusEpisodes {
		t.Fatalf("Tab from menu with unknown follow state = %d, want episodes", view.state.Focus)
	}
	view.activate(focusFollow)
	if toggles != 0 {
		t.Fatal("hidden star toggled following")
	}

	view.state.FollowKnown = true
	view.state.Focus = focusMenu
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusFollow {
		t.Fatalf("Tab from menu = %d, want follow star", view.state.Focus)
	}
	keyDown(view, fltk.ENTER_KEY, 0)
	keyUp(view, fltk.ENTER_KEY)
	keyDown(view, ' ', 0)
	keyUp(view, ' ')
	if toggles != 2 || plays != 0 {
		t.Fatalf("Enter and Space on the star: %d toggles, %d play/pause; want 2 and 0", toggles, plays)
	}
	keyDown(view, 9, 0)
	keyUp(view, 9)
	if view.state.Focus != focusEpisodes {
		t.Fatalf("Tab from star = %d, want episodes", view.state.Focus)
	}
	keyDown(view, 9, fltk.SHIFT)
	keyUp(view, 9)
	if view.state.Focus != focusFollow {
		t.Fatalf("Shift+Tab from episodes = %d, want follow star", view.state.Focus)
	}

	// A state update keeps the star focused; one that hides it moves focus off it.
	view.SetState(State{Volume: 100, Focus: focusFollow, FollowKnown: true})
	if view.state.Focus != focusFollow {
		t.Fatalf("focus after state update = %d", view.state.Focus)
	}
	view.SetState(State{Volume: 100, Focus: focusFollow})
	if view.state.Focus != focusProgress {
		t.Fatalf("focus on hidden star = %d, want progress", view.state.Focus)
	}
}
