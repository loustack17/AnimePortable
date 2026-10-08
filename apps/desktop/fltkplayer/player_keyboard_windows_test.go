//go:build windows && cgo

package fltkplayer

import (
	"reflect"
	"runtime"
	"testing"
	"time"

	fltk "github.com/pwiecz/go-fltk"
)

func keyboardTestView(callbacks Callbacks) *View {
	return &View{
		callbacks:  callbacks,
		state:      State{Volume: 100, Focus: focusProgress},
		lastVolume: 100,
	}
}

func keyDown(view *View, key, modifiers int) bool {
	return view.handleKeyEvent(fltk.KEYDOWN, key, modifiers)
}

func keyUp(view *View, key int) bool {
	return view.handleKeyEvent(fltk.KEYUP, key, 0)
}

func TestPlayerContextualArrowsAfterTab(t *testing.T) {
	var seeks, volumes []int
	plays, stops, selections, navigations := 0, 0, 0, 0
	view := keyboardTestView(Callbacks{
		Seek:          func(seconds int) { seeks = append(seeks, seconds) },
		Volume:        func(percent int) { volumes = append(volumes, percent) },
		PlayPause:     func() { plays++ },
		Stop:          func() { stops++ },
		SelectEpisode: func(int) { selections++ },
		Navigate:      func(int) { navigations++ },
	})
	if view.State().Focus != FocusProgress {
		t.Fatalf("default focus = %d, want progress %d", view.State().Focus, FocusProgress)
	}
	keyDown(view, 0xff51, 0)
	keyUp(view, 0xff51)
	keyDown(view, 0xff53, 0)
	keyUp(view, 0xff53)
	if !reflect.DeepEqual(seeks, []int{-5, 5}) {
		t.Fatalf("progress arrows seeks = %v, want [-5 5]", seeks)
	}
	if view.State().Focus != focusProgress {
		t.Fatalf("progress arrows changed focus to %d", view.State().Focus)
	}
	pressTab := func(modifiers int) {
		t.Helper()
		keyDown(view, 9, modifiers)
		keyUp(view, 9)
	}
	pressTab(0)
	if view.State().Focus != focusPlayPause {
		t.Fatalf("first Tab focus = %d, want play/pause", view.State().Focus)
	}
	for _, key := range []int{0xff51, 0xff53, 0xff52, 0xff54} {
		if !keyDown(view, key, 0) {
			t.Fatalf("button focus did not consume arrow %d", key)
		}
		keyUp(view, key)
	}
	if view.State().Focus != focusPlayPause || !reflect.DeepEqual(seeks, []int{-5, 5}) || plays != 0 {
		t.Fatalf("play focus arrows changed target/actions: focus=%d seeks=%v plays=%d", view.State().Focus, seeks, plays)
	}
	pressTab(0)
	pressTab(0)
	pressTab(0)
	pressTab(0)
	if view.State().Focus != focusVolume {
		t.Fatalf("Tab traversal focus = %d, want volume", view.State().Focus)
	}
	for _, key := range []int{0xff51, 0xff53, 0xff52, 0xff54} {
		keyDown(view, key, 0)
		keyUp(view, key)
	}
	if !reflect.DeepEqual(volumes, []int{95, 100, 100, 95}) || !reflect.DeepEqual(seeks, []int{-5, 5}) || view.State().Focus != focusVolume {
		t.Fatalf("volume focus arrows: volumes=%v seeks=%v focus=%d", volumes, seeks, view.State().Focus)
	}
	pressTab(fltk.SHIFT)
	if view.State().Focus != focusStop {
		t.Fatalf("Shift+Tab from volume focus = %d, want stop", view.State().Focus)
	}
	for _, key := range []int{0xff51, 0xff53, 0xff52, 0xff54} {
		keyDown(view, key, 0)
		keyUp(view, key)
	}
	if view.State().Focus != focusStop || stops != 0 || selections != 0 || navigations != 0 {
		t.Fatalf("stop focus arrows changed target/actions: focus=%d stop=%d select=%d navigate=%d", view.State().Focus, stops, selections, navigations)
	}
	keyDown(view, 'j', 0)
	keyUp(view, 'j')
	keyDown(view, 'L', 0)
	keyUp(view, 'L')
	if !reflect.DeepEqual(seeks, []int{-5, 5, -10, 10}) || view.State().Focus != focusStop {
		t.Fatalf("J/L independent seek = %v, focus=%d", seeks, view.State().Focus)
	}
}

func TestNewWindowDefaultsToProgressFocus(t *testing.T) {
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	t.Cleanup(func() { fltk.Unlock(); runtime.UnlockOSThread() })
	view := NewWindow(Callbacks{})
	t.Cleanup(func() { view.Close(); view.window.Hide(); view.window.Destroy() })
	if view.State().Focus != FocusProgress {
		t.Fatalf("NewWindow focus = %d, want progress %d", view.State().Focus, FocusProgress)
	}
}

func TestMissingFocusFallsBackToProgressForArrows(t *testing.T) {
	var seeks []int
	view := keyboardTestView(Callbacks{Seek: func(seconds int) { seeks = append(seeks, seconds) }})
	view.SetState(State{Volume: 80, Focus: -1})
	if view.State().Focus != focusProgress {
		t.Fatalf("missing focus = %d, want progress", view.State().Focus)
	}
	keyDown(view, 0xff51, 0)
	if !reflect.DeepEqual(seeks, []int{-5}) || view.State().Focus != focusProgress {
		t.Fatalf("fallback arrow seeks=%v focus=%d", seeks, view.State().Focus)
	}
}

func TestPlayerFocusSurvivesAutoHidePointerAndStateUpdates(t *testing.T) {
	view := keyboardTestView(Callbacks{})
	view.SetFocus(focusVolume)
	state := view.State()
	state.Playing = true
	state.Volume = 80
	view.SetState(state)
	view.hideControls()
	if view.visible || view.State().Focus != focusVolume {
		t.Fatalf("auto-hide state visible=%v focus=%d", view.visible, view.State().Focus)
	}
	view.showAtSize(500, 200, 1000, 618)
	if !view.visible || view.State().Focus != focusVolume || !view.volumeOpen {
		t.Fatalf("pointer movement state visible=%v focus=%d volumeOpen=%v", view.visible, view.State().Focus, view.volumeOpen)
	}
	state = view.State()
	state.Loading = true
	view.SetState(state)
	state = view.State()
	state.Loading = false
	view.SetState(state)
	if view.State().Focus != focusVolume {
		t.Fatalf("state updates changed selected focus to %d", view.State().Focus)
	}
	view.SetFocus(-1)
	if view.State().Focus != focusProgress {
		t.Fatalf("missing focus fallback = %d, want progress", view.State().Focus)
	}
}

func TestHoverOnlyMenuDoesNotStealProgressOrVolumeArrows(t *testing.T) {
	var seeks, volumes []int
	navigations := 0
	view := keyboardTestView(Callbacks{
		Seek:     func(seconds int) { seeks = append(seeks, seconds) },
		Volume:   func(percent int) { volumes = append(volumes, percent) },
		Navigate: func(int) { navigations++ },
	})
	view.menuHover = true
	for _, key := range []int{0xff51, 0xff53} {
		keyDown(view, key, 0)
		keyUp(view, key)
	}
	view.state.Focus = focusVolume
	for _, key := range []int{0xff51, 0xff53, 0xff52, 0xff54} {
		keyDown(view, key, 0)
		keyUp(view, key)
	}
	if !reflect.DeepEqual(seeks, []int{-5, 5}) || !reflect.DeepEqual(volumes, []int{95, 100, 100, 95}) || navigations != 0 || view.menuCursor != 0 {
		t.Fatalf("hover menu stole arrows: seeks=%v volumes=%v navigations=%d cursor=%d", seeks, volumes, navigations, view.menuCursor)
	}
	view.menuPinned = true
	keyDown(view, 0xff54, 0)
	if view.menuCursor != 1 {
		t.Fatalf("pinned menu Down cursor=%d, want 1", view.menuCursor)
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
	view.SetFocus(focusVolume)
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
	view.SetFocus(focusVolume)
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

func TestPlayerSpaceUsesFocusedControlUnlessAnOpenListOwnsSelection(t *testing.T) {
	plays, navigations, selections := 0, 0, 0
	view := keyboardTestView(Callbacks{
		PlayPause:     func() { plays++ },
		Navigate:      func(int) { navigations++ },
		SelectEpisode: func(int) { selections++ },
	})
	view.state.Focus = focusPlayPause
	view.menuHover = true
	if !view.handleKeyEvent(fltk.SHORTCUT, ' ', 0) {
		t.Fatal("focused Play/Pause did not handle native Space shortcut")
	}
	if plays != 1 || navigations != 0 {
		t.Fatalf("hover menu stole focused Play/Pause Space: plays=%d navigations=%d", plays, navigations)
	}
	if !keyUp(view, ' ') || !view.handleKeyEvent(fltk.SHORTCUT, ' ', 0) {
		t.Fatal("Space press after release was not handled")
	}
	if plays != 2 || navigations != 0 {
		t.Fatalf("second focused Space: plays=%d navigations=%d", plays, navigations)
	}
	keyUp(view, ' ')
	keyDown(view, fltk.ENTER_KEY, 0)
	if plays != 3 || navigations != 0 {
		t.Fatalf("hover menu stole focused Play/Pause Enter: plays=%d navigations=%d", plays, navigations)
	}
	keyUp(view, fltk.ENTER_KEY)
	view.menuHover = false
	view.menuPinned = true
	view.state.Focus = focusMenu
	keyDown(view, ' ', 0)
	if navigations != 1 || plays != 3 {
		t.Fatalf("focused pinned menu Space: plays=%d navigations=%d", plays, navigations)
	}
	keyUp(view, ' ')
	view.episodes = []string{"one", "two"}
	view.episodeCursor = 1
	view.episodesOpen = true
	keyDown(view, ' ', 0)
	if selections != 1 || view.episodesOpen || plays != 3 {
		t.Fatalf("open episode list Space: selections=%d open=%v plays=%d", selections, view.episodesOpen, plays)
	}
}

func TestPinnedMenuOwnsSpaceAndEnterAfterMouseActivation(t *testing.T) {
	for _, test := range []struct {
		name string
		key  int
	}{{"space", ' '}, {"enter", fltk.ENTER_KEY}} {
		t.Run(test.name, func(t *testing.T) {
			plays, navigations := 0, 0
			view := keyboardTestView(Callbacks{
				PlayPause: func() { plays++ },
				Navigate:  func(int) { navigations++ },
			})
			view.state.Focus = focusPlayPause
			view.activate(focusMenu)
			if !view.menuPinned || view.state.Focus != focusPlayPause {
				t.Fatalf("menu activation state pinned=%v focus=%d", view.menuPinned, view.state.Focus)
			}
			if !keyDown(view, test.key, 0) {
				t.Fatalf("pinned menu did not handle key %d", test.key)
			}
			if navigations != 1 || plays != 0 {
				t.Fatalf("pinned menu key %d invoked navigate=%d play=%d", test.key, navigations, plays)
			}
		})
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
			if test.key == 0xff52 {
				view.SetFocus(focusVolume)
			}
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
	if !keyDown(view, 9, fltk.SHIFT) || view.state.Focus != focusEpisodes {
		t.Fatalf("initial Shift+Tab focus = %d, want episodes", view.state.Focus)
	}
	keyUp(view, 9)
	if !keyDown(view, 9, 0) || view.state.Focus != focusProgress {
		t.Fatalf("Tab from episodes focus = %d, want progress", view.state.Focus)
	}
	keyUp(view, 9)
	if !keyDown(view, 9, 0) || view.state.Focus != focusPlayPause {
		t.Fatalf("Tab from progress focus = %d, want play/pause", view.state.Focus)
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
