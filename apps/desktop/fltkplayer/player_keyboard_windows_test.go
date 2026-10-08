//go:build windows && cgo

package fltkplayer

import (
	"reflect"
	"testing"
	"time"

	fltk "github.com/pwiecz/go-fltk"
)

func keyboardTestView(callbacks Callbacks) *View {
	return &View{
		callbacks:  callbacks,
		state:      State{Volume: 100, Focus: -1},
		lastVolume: 100,
	}
}

func keyDown(view *View, key, modifiers int) bool {
	return view.handleKeyEvent(fltk.KEYDOWN, key, modifiers)
}

func keyUp(view *View, key int) bool {
	return view.handleKeyEvent(fltk.KEYUP, key, 0)
}

func TestPlayerShortcutsSeekRegardlessOfControlFocus(t *testing.T) {
	var seeks []int
	plays, selections, navigations := 0, 0, 0
	view := keyboardTestView(Callbacks{
		Seek:          func(seconds int) { seeks = append(seeks, seconds) },
		PlayPause:     func() { plays++ },
		SelectEpisode: func(int) { selections++ },
		Navigate:      func(int) { navigations++ },
	})
	focuses := []int{-1, focusMenu, focusEpisodes, focusProgress, focusPlayPause, focusSeekBack, focusSeekForward, focusStop, focusVolume, focusFullscreen}
	want := make([]int, 0, len(focuses)*4)
	for _, focus := range focuses {
		view.SetState(State{Playing: true, Focus: focus, Volume: 100})
		view.SetState(State{Playing: true, Loading: true, Focus: focus, Volume: 100})
		view.SetState(State{Playing: true, Loading: false, Focus: focus, Volume: 100})
		want = append(want, -5, 5, -10, 10)
		for _, key := range []int{0xff51, 0xff53, 'j', 'L'} {
			if !keyDown(view, key, 0) {
				t.Fatalf("focus %d did not handle key %d", focus, key)
			}
			keyUp(view, key)
		}
	}
	if !reflect.DeepEqual(seeks, want) {
		t.Fatalf("seek callbacks = %v, want %v", seeks, want)
	}
	if plays != 0 || selections != 0 || navigations != 0 {
		t.Fatalf("directional seeks invoked other actions: play=%d select=%d navigate=%d", plays, selections, navigations)
	}
}

func TestPlayerHeldSeekAndVolumeKeysRepeat(t *testing.T) {
	var seeks, volumes []int
	view := keyboardTestView(Callbacks{
		Seek:   func(seconds int) { seeks = append(seeks, seconds) },
		Volume: func(percent int) { volumes = append(volumes, percent) },
	})
	keyDown(view, 0xff53, 0)
	keyDown(view, 0xff53, 0)
	keyUp(view, 0xff53)
	if !reflect.DeepEqual(seeks, []int{5, 5}) {
		t.Fatalf("held Right seeks = %v, want [5 5]", seeks)
	}
	keyDown(view, 0xff54, 0)
	keyDown(view, 0xff54, 0)
	keyUp(view, 0xff54)
	if !reflect.DeepEqual(volumes, []int{95, 90}) {
		t.Fatalf("held Down volumes = %v, want [95 90]", volumes)
	}
}

func TestPlayerVolumeBoundsAndMuteRestoresLastNonzero(t *testing.T) {
	var volumes []int
	view := keyboardTestView(Callbacks{Volume: func(percent int) { volumes = append(volumes, percent) }})
	view.state.Volume = 98
	wantVolumes := []int{100, 100}
	keyDown(view, 0xff52, 0)
	keyUp(view, 0xff52)
	if view.state.Volume != 100 {
		t.Fatalf("volume after up = %d, want 100", view.state.Volume)
	}
	keyDown(view, 0xff52, 0)
	keyUp(view, 0xff52)
	if view.state.Volume != 100 {
		t.Fatalf("volume exceeded maximum: %d", view.state.Volume)
	}
	for step := 0; step < 20; step++ {
		keyDown(view, 0xff54, 0)
		keyUp(view, 0xff54)
		wantVolumes = append(wantVolumes, max(0, 100-(step+1)*5))
	}
	if view.state.Volume != 0 || view.lastVolume != 5 {
		t.Fatalf("volume lower bound/state = %d/%d, want 0/5", view.state.Volume, view.lastVolume)
	}
	keyDown(view, 'M', 0)
	if view.state.Volume != 5 {
		t.Fatalf("restored volume = %d, want 5", view.state.Volume)
	}
	keyUp(view, 'M')
	keyDown(view, 'm', 0)
	if view.state.Volume != 0 {
		t.Fatalf("muted volume = %d, want 0", view.state.Volume)
	}
	keyUp(view, 'm')
	keyDown(view, 'm', 0)
	keyUp(view, 'm')
	if view.state.Volume != 5 {
		t.Fatalf("second mute cycle restored volume = %d, want 5", view.state.Volume)
	}
	wantVolumes = append(wantVolumes, 5, 0, 5)
	if !reflect.DeepEqual(volumes, wantVolumes) {
		t.Fatalf("volume callbacks = %v", volumes)
	}
}

func TestPlayerVolumeStartsMutedWithDefaultRestore(t *testing.T) {
	var volumes []int
	view := keyboardTestView(Callbacks{Volume: func(percent int) { volumes = append(volumes, percent) }})
	view.state.Volume = 0
	keyDown(view, 'm', 0)
	if view.state.Volume != 100 || !reflect.DeepEqual(volumes, []int{100}) {
		t.Fatalf("unmute = %d, callbacks %v; want 100", view.state.Volume, volumes)
	}
}

func TestPlayerOpenEpisodeListKeepsArrowAndSpaceSelection(t *testing.T) {
	selected := -1
	plays := 0
	var seeks []int
	view := keyboardTestView(Callbacks{
		Seek:          func(seconds int) { seeks = append(seeks, seconds) },
		SelectEpisode: func(index int) { selected = index },
		PlayPause:     func() { plays++ },
	})
	view.episodes = []string{"one", "two", "three"}
	view.episodesOpen = true
	view.episodeCursor = 1
	keyDown(view, 'k', 0)
	keyUp(view, 'k')
	if plays != 1 || !view.episodesOpen {
		t.Fatalf("K with open episode list: plays=%d list=%v", plays, view.episodesOpen)
	}
	if !keyDown(view, 0xff54, 0) {
		t.Fatal("episode list did not handle Down")
	}
	if view.episodeCursor != 2 {
		t.Fatalf("episode cursor = %d, want 2", view.episodeCursor)
	}
	keyUp(view, 0xff54)
	if !keyDown(view, ' ', 0) {
		t.Fatal("episode list did not handle Space")
	}
	if view.episodesOpen || selected != 2 || len(seeks) != 0 {
		t.Fatalf("list state open=%v selected=%d seeks=%v", view.episodesOpen, selected, seeks)
	}
}

func TestPlayerRejectsModifiedShortcuts(t *testing.T) {
	plays, fullscreen := 0, 0
	var seeks []int
	view := keyboardTestView(Callbacks{
		PlayPause:  func() { plays++ },
		Fullscreen: func() { fullscreen++ },
		Seek:       func(seconds int) { seeks = append(seeks, seconds) },
	})
	for _, modifier := range []int{fltk.CTRL, fltk.ALT, fltk.META} {
		for _, key := range []int{'k', 'f', 'j', 0xff51} {
			if keyDown(view, key, modifier) {
				t.Fatalf("modified key %d with modifier %d was handled", key, modifier)
			}
			keyUp(view, key)
		}
	}
	if plays != 0 || fullscreen != 0 || len(seeks) != 0 {
		t.Fatalf("modified shortcuts invoked callbacks: play=%d fullscreen=%d seeks=%v", plays, fullscreen, seeks)
	}
}

func TestPlayerOneShotShortcutRepeatsOnlyAfterReleaseOrWindowFocusLoss(t *testing.T) {
	plays := 0
	view := keyboardTestView(Callbacks{PlayPause: func() { plays++ }})
	keyDown(view, 'K', 0)
	keyDown(view, 'K', 0)
	if plays != 1 {
		t.Fatalf("held K invoked play/pause %d times, want 1", plays)
	}
	keyUp(view, 'K')
	keyDown(view, 'K', 0)
	if plays != 2 {
		t.Fatalf("released K invoked play/pause %d times, want 2", plays)
	}
	if !view.handleUnfocus('K') {
		t.Fatal("widget UNFOCUS was not accepted")
	}
	keyDown(view, 'K', 0)
	if plays != 2 {
		t.Fatalf("key-bearing focus transfer repeated held K: plays=%d, want 2", plays)
	}
	view.handleUnfocus(0)
	keyDown(view, 'K', 0)
	if plays != 3 {
		t.Fatalf("focus-loss reset did not permit next K press: %d", plays)
	}
	view.handleWindowEvent(fltk.HIDE)
	keyDown(view, 'K', 0)
	if plays != 4 {
		t.Fatalf("hide reset did not permit next K press: %d", plays)
	}
}

func TestPlayerEpisodeListTakesPriorityWhenMenuIsAlsoOpen(t *testing.T) {
	selected, navigated := -1, -1
	view := keyboardTestView(Callbacks{
		SelectEpisode: func(index int) { selected = index },
		Navigate:      func(index int) { navigated = index },
	})
	view.episodes = []string{"one", "two", "three"}
	view.episodeCursor = 1
	view.episodesOpen = true
	view.menuPinned = true
	view.menuCursor = 1
	keyDown(view, 0xff54, 0)
	keyUp(view, 0xff54)
	if view.episodeCursor != 2 || view.menuCursor != 1 {
		t.Fatalf("Down moved episode/menu cursors to %d/%d, want 2/1", view.episodeCursor, view.menuCursor)
	}
	keyDown(view, fltk.ENTER_KEY, 0)
	if selected != 2 || navigated != -1 || view.episodesOpen {
		t.Fatalf("Enter selected=%d navigated=%d list=%v", selected, navigated, view.episodesOpen)
	}
}

func TestPlayerHeldFullscreenShortcutSurvivesCallbackFocusTransfer(t *testing.T) {
	fullscreen := 0
	var view *View
	view = keyboardTestView(Callbacks{Fullscreen: func() {
		fullscreen++
		view.handleUnfocus('F')
	}})
	keyDown(view, 'F', 0)
	keyDown(view, 'F', 0)
	if fullscreen != 1 {
		t.Fatalf("held F toggled fullscreen %d times, want 1", fullscreen)
	}
	keyUp(view, 'F')
	keyDown(view, 'F', 0)
	if fullscreen != 2 {
		t.Fatalf("released F toggled fullscreen %d times, want 2", fullscreen)
	}
}

func TestPlayerLetterReleaseMatchesShiftedKeyPress(t *testing.T) {
	plays := 0
	view := keyboardTestView(Callbacks{PlayPause: func() { plays++ }})
	keyDown(view, 'K', fltk.SHIFT)
	keyUp(view, 'k')
	keyDown(view, 'K', fltk.SHIFT)
	if plays != 2 {
		t.Fatalf("second shifted K press invoked play/pause %d times, want 2", plays)
	}
	keyUp(view, 'k')
	keyDown(view, 'k', 0)
	if plays != 3 {
		t.Fatalf("lowercase K after shifted release invoked play/pause %d times, want 3", plays)
	}
}

func TestPlayerMediaShortcutsRevealHiddenControls(t *testing.T) {
	for _, test := range []struct {
		name string
		key  int
	}{
		{"play-pause", 'k'},
		{"mute", 'm'},
		{"fullscreen", 'f'},
		{"seek", 'j'},
		{"arrow-seek", 0xff51},
		{"volume", 0xff52},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			view := keyboardTestView(Callbacks{
				PlayPause:  func() { calls++ },
				Seek:       func(int) { calls++ },
				Volume:     func(int) { calls++ },
				Fullscreen: func() { calls++ },
			})
			view.visible = false
			view.lastMove = time.Unix(1, 0)
			if !keyDown(view, test.key, 0) {
				t.Fatalf("key %d was not handled", test.key)
			}
			if !view.visible || !view.lastMove.After(time.Unix(1, 0)) || calls == 0 {
				t.Fatalf("key %d left controls hidden or unchanged: visible=%v lastMove=%v calls=%d", test.key, view.visible, view.lastMove, calls)
			}
		})
	}
}

func TestPlayerEscapeDismissesListAndMenuBeforeFullscreen(t *testing.T) {
	fullscreen := 0
	view := keyboardTestView(Callbacks{Fullscreen: func() { fullscreen++ }})
	view.state.Fullscreen = true
	view.menuPinned = true
	view.menuHover = true
	view.episodesOpen = true
	keyDown(view, 27, 0)
	keyUp(view, 27)
	if !view.menuVisible() || fullscreen != 0 {
		t.Fatalf("first Escape state menu=%v fullscreen callbacks=%d", view.menuVisible(), fullscreen)
	}
	keyDown(view, 27, 0)
	keyUp(view, 27)
	if view.menuVisible() || fullscreen != 0 {
		t.Fatalf("second Escape state menu=%v fullscreen callbacks=%d", view.menuVisible(), fullscreen)
	}
	keyDown(view, 27, 0)
	if fullscreen != 1 {
		t.Fatalf("third Escape fullscreen callbacks=%d, want 1", fullscreen)
	}
}

func TestPlayerTabTraversalAndEnterRemainAvailable(t *testing.T) {
	plays := 0
	view := keyboardTestView(Callbacks{PlayPause: func() { plays++ }})
	if !keyDown(view, 9, 0) || view.state.Focus != focusMenu {
		t.Fatalf("Tab focus = %d, want menu", view.state.Focus)
	}
	keyUp(view, 9)
	if !keyDown(view, 9, fltk.SHIFT) || view.state.Focus != focusFullscreen {
		t.Fatalf("Shift+Tab focus = %d, want fullscreen", view.state.Focus)
	}
	keyUp(view, 9)
	view.state.Focus = focusPlayPause
	keyDown(view, fltk.ENTER_KEY, 0)
	keyDown(view, fltk.ENTER_KEY, 0)
	if plays != 1 {
		t.Fatalf("held Enter invoked activation %d times, want 1", plays)
	}
	keyUp(view, fltk.ENTER_KEY)
	keyDown(view, fltk.ENTER_KEY, 0)
	if plays != 2 {
		t.Fatalf("Enter after release invoked activation %d times, want 2", plays)
	}
	keyUp(view, fltk.ENTER_KEY)
	view.state.Focus = focusVolume
	keyDown(view, ' ', 0)
	if plays != 3 {
		t.Fatalf("Space with volume focused invoked play/pause %d times, want 3", plays)
	}
}
