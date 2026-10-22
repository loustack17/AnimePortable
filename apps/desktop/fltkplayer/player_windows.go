//go:build windows && cgo

package fltkplayer

import (
	"math"
	"strings"
	"sync"
	"time"
	"unsafe"

	fltk "github.com/pwiecz/go-fltk"
)

const (
	focusMenu = iota
	focusPlayPause
	focusSeekBack
	focusSeekForward
	focusStop
	focusVolume
	focusProgress
	focusEpisodes
	focusFullscreen
	focusFollow
	focusSpeed
)

const controlsIdleTimeout = 3 * time.Second

const FocusProgress = focusProgress

type Callbacks struct {
	PlayPause  func()
	Seek       func(seconds int)
	SeekTo     func(seconds int)
	Stop       func()
	Volume     func(percent int)
	Fullscreen func()
	// SetSpeed applies and remembers a playback speed chosen from the speed list, in percent.
	SetSpeed      func(percent int)
	SelectEpisode func(index int)
	Navigate      func(index int)
	// Leave ends playback and returns to the app (Esc in a window).
	Leave func()
	// Search ends playback and opens 搜尋 (/ and Ctrl+K).
	Search func()
	// ToggleFollow follows or unfollows the anime being played (the star left of 選集).
	ToggleFollow func()
}

type State struct {
	Playing    bool
	Paused     bool
	Position   float64
	Duration   float64
	Resolution int
	Volume     int
	Episode    int
	Focus      int
	Fullscreen bool
	Loading    bool
	// FollowKnown shows the follow star; Following fills it.
	FollowKnown bool
	Following   bool
}

type View struct {
	window        *fltk.Window
	video         *fltk.GlWindow
	keyboardFocus *fltk.Button
	callbacks     Callbacks
	state         State
	episodes      []string
	menuItems     []string
	visible       bool
	menuPinned    bool
	menuHover     bool
	episodesOpen  bool
	volumeOpen    bool
	speedOpen     bool
	speedCursor   int
	speedPercent  int
	pressedKeys   map[int]bool
	lastVolume    int
	dragging      bool
	scrubbing     bool
	scrubPosition float64
	progressHover bool
	closed        bool
	nativeHandles map[uintptr]struct{}
	episodeStart  int
	episodeCursor int
	menuCursor    int
	lastMove      time.Time
	mu            sync.RWMutex
	render        func(width, height int)
	episodeText   unsafe.Pointer
	menuText      unsafe.Pointer
	speedText     unsafe.Pointer
}

func NewWindow(callbacks Callbacks) *View {
	view := &View{
		callbacks:  callbacks,
		state:      State{Volume: 100, Focus: focusProgress},
		lastVolume: 100,
		visible:    true,
		lastMove:   time.Now(),
		menuItems:  []string{"首頁", "時間表", "追蹤", "歷史紀錄", "搜尋", "設定"},
	}
	view.updateLabelBuffers()
	fltk.SetFont(fltk.HELVETICA, "Microsoft JhengHei UI")
	fltk.SetFont(fltk.HELVETICA_BOLD, "Microsoft JhengHei UI Bold")
	fltk.SetFont(fltk.COURIER, "Segoe MDL2 Assets")
	view.window = fltk.NewWindow(1000, 618, "AnimePortable")
	// Narrower, the speed button would meet the time label.
	view.window.SetSizeRange(playerMinWidth, playerMinHeight, 0, 0, 0, 0, false)
	view.window.Begin()
	view.keyboardFocus = fltk.NewButton(0, 0, 1, 1, "")
	view.keyboardFocus.SetBox(fltk.NO_BOX)
	view.keyboardFocus.SetEventHandler(view.handle)
	view.video = fltk.NewGlWindow(0, 0, 1000, 618, func() {
		view.mu.RLock()
		render := view.render
		view.mu.RUnlock()
		if render != nil {
			render(view.video.W(), view.video.H())
		}
		drawPlayerOverlay(view.video.W(), view.video.H(), view)
	})
	view.video.SetEventHandler(view.handle)
	view.window.SetEventHandler(view.handleWindowEvent)
	view.window.Resizable(view.video)
	view.window.End()
	view.video.SetResizeHandler(view.video.Redraw)
	fltk.AddTimeout(0.5, view.tick)
	return view
}

func (view *View) handleWindowEvent(event fltk.Event) bool {
	if event == fltk.UNFOCUS {
		return view.handleUnfocus(fltk.EventKey())
	}
	if event == fltk.DEACTIVATE || event == fltk.HIDE {
		view.pressedKeys = nil
		if event == fltk.HIDE {
			view.removeNativeInput()
		}
		return false
	}
	if event == fltk.KEYDOWN || event == fltk.KEYUP || event == fltk.SHORTCUT {
		return view.handle(event)
	}
	return false
}

func (view *View) Window() *fltk.Window { return view.window }

func (view *View) Video() *fltk.GlWindow { return view.video }

func (view *View) Show() {
	if view.closed || view.window == nil {
		return
	}
	view.window.Show()
	view.syncNativeInput()
}

func (view *View) State() State { return view.state }

func (view *View) SetRenderHook(render func(width, height int)) {
	view.mu.Lock()
	view.render = render
	view.mu.Unlock()
	view.Redraw()
}

func (view *View) SetEpisodes(labels []string, selected int) {
	view.episodes = append(view.episodes[:0], labels...)
	view.updateLabelBuffers()
	if selected < 0 || selected >= len(view.episodes) {
		selected = 0
	}
	view.state.Episode = selected
	view.episodeCursor = selected
	if view.state.Episode >= len(view.episodes) {
		view.episodesOpen = false
	}
	view.Redraw()
}

func (view *View) SetMenuItems(labels []string) {
	view.menuItems = append(view.menuItems[:0], labels...)
	view.updateLabelBuffers()
	view.Redraw()
}

func (view *View) Close() {
	if view.closed {
		return
	}
	view.closed = true
	view.removeNativeInput()
	view.freeLabelBuffers()
}

func (view *View) SetState(state State) {
	wasPlaying := view.state.Playing && !view.state.Paused && !view.state.Loading
	wasLoading := view.state.Loading
	wasPaused := view.state.Paused
	fullscreenChanged := state.Fullscreen != view.state.Fullscreen
	if state.Volume < 0 {
		state.Volume = 0
	}
	if state.Volume > 100 {
		state.Volume = 100
	}
	if state.Volume > 0 {
		view.lastVolume = state.Volume
	}
	if !state.focusable(state.Focus) {
		state.Focus = focusProgress
	}
	if state.Episode < 0 || state.Episode >= len(view.episodes) {
		state.Episode = 0
	}
	if !view.episodesOpen && state.Episode != view.state.Episode {
		view.episodeCursor = state.Episode
	}
	view.state = state
	isPlaying := state.Playing && !state.Paused && !state.Loading
	if (isPlaying && !wasPlaying) || (state.Loading && !wasLoading) || (state.Paused && !wasPaused) || (wasPlaying && !state.Playing) {
		view.showControls()
	}
	if fullscreenChanged {
		view.syncNativeInput()
	}
	view.Redraw()
}

func (view *View) SetFocus(index int) {
	if !view.state.focusable(index) {
		index = focusProgress
	}
	view.state.Focus = index
	if index >= 0 {
		view.visible = true
		view.lastMove = time.Now()
		view.volumeOpen = index == focusVolume
		if view.keyboardFocus != nil {
			view.keyboardFocus.TakeFocus()
		}
	}
	view.Redraw()
}

func (view *View) Redraw() {
	if view.video != nil {
		view.video.Redraw()
	}
}

func (view *View) tick() {
	if view.closed {
		return
	}
	view.syncNativeInput()
	if view.shouldHideControls(time.Now()) {
		view.hideControls()
	}
	fltk.RepeatTimeout(0.5, view.tick)
}

func (view *View) showAt(x, y int) {
	view.showAtSize(x, y, view.video.W(), view.video.H())
}

func (view *View) showAtSize(x, y, width, height int) {
	view.lastMove = time.Now()
	view.visible = true
	view.menuHover = x < 18 || view.menuHover && x < 180 && y < height-54
	view.volumeOpen = x >= 232 && x <= 395 && y >= height-60 || view.state.Focus == focusVolume
	view.progressHover = y >= height-82 && y < height-54 && x >= 24 && x <= width-24
	view.Redraw()
}

func (view *View) shouldHideControls(now time.Time) bool {
	return view.visible && view.state.Playing && !view.state.Paused && !view.state.Loading && !view.scrubbing && !view.dragging && !view.menuPinned && !view.episodesOpen && !view.speedOpen && now.Sub(view.lastMove) >= controlsIdleTimeout
}

func (view *View) hideControls() {
	view.visible = false
	view.menuHover = false
	view.volumeOpen = false
	view.Redraw()
}

func (view *View) actionAt(x, y int) int {
	width, height := view.video.W(), view.video.H()
	if x >= 12 && x <= 60 && y >= 10 && y <= 55 {
		return focusMenu
	}
	if x >= width-195 && x <= width-81 && y >= 14 && y <= 53 {
		return focusEpisodes
	}
	if view.state.FollowKnown && x >= width-241 && x <= width-203 && y >= 14 && y <= 53 {
		return focusFollow
	}
	if view.state.Duration > 0 && y >= height-82 && y < height-54 && x >= 24 && x <= width-24 {
		return focusProgress
	}
	if y < height-54 {
		return -1
	}
	if x >= 32 && x <= 68 {
		return focusPlayPause
	}
	if x >= 82 && x <= 118 {
		return focusSeekBack
	}
	if x >= 132 && x <= 168 {
		return focusSeekForward
	}
	if x >= 182 && x <= 218 {
		return focusStop
	}
	if x >= 232 && x <= 395 {
		return focusVolume
	}
	if speedButtonAt(x, width) {
		return focusSpeed
	}
	if x >= width-51 && x <= width-15 {
		return focusFullscreen
	}
	return -1
}

func (view *View) activate(index int) {
	switch index {
	case focusMenu:
		view.menuPinned = !view.menuPinned
		view.menuHover = view.menuPinned
	case focusPlayPause:
		if view.callbacks.PlayPause != nil {
			view.callbacks.PlayPause()
		}
	case focusSeekBack:
		if view.callbacks.Seek != nil {
			view.callbacks.Seek(-10)
		}
	case focusSeekForward:
		if view.callbacks.Seek != nil {
			view.callbacks.Seek(10)
		}
	case focusSpeed:
		if view.speedOpen {
			view.speedOpen = false
		} else {
			view.openSpeeds()
		}
	case focusStop:
		if view.callbacks.Stop != nil {
			view.callbacks.Stop()
		}
	case focusVolume:
		view.volumeOpen = true
	case focusProgress:
		if view.callbacks.SeekTo != nil && view.state.Duration > 0 {
			view.callbacks.SeekTo(int(math.Round(view.state.Position)))
		}
	case focusEpisodes:
		if view.episodesOpen {
			view.episodesOpen = false
		} else if len(view.episodes) > 0 {
			view.openEpisodes()
		}
	case focusFullscreen:
		if view.callbacks.Fullscreen != nil {
			view.callbacks.Fullscreen()
		}
	case focusFollow:
		if view.callbacks.ToggleFollow != nil && view.state.FollowKnown {
			view.callbacks.ToggleFollow()
		}
	}
	view.showControls()
}

func (view *View) setVolumeAt(x int) {
	value := (x - 286) * 100 / 100
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	view.setVolume(value)
}

func progressPosition(x, width int, duration float64) float64 {
	if duration <= 0 || width <= 48 {
		return 0
	}
	return math.Max(0, math.Min(1, float64(x-24)/float64(width-48))) * duration
}

func (view *View) setScrubAt(x int) {
	view.scrubPosition = progressPosition(x, view.video.W(), view.state.Duration)
	view.visible = true
	view.lastMove = time.Now()
	view.Redraw()
}

func (view *View) handle(event fltk.Event) bool {
	switch event {
	case fltk.KEYDOWN, fltk.KEYUP:
		return view.handleKeyEvent(event, fltk.EventKey(), fltk.EventState())
	case fltk.SHORTCUT:
		return view.handleKeyEvent(fltk.KEYDOWN, fltk.EventKey(), fltk.EventState())
	case fltk.FOCUS:
		return true
	case fltk.UNFOCUS:
		return view.handleUnfocus(fltk.EventKey())
	case fltk.DEACTIVATE, fltk.HIDE:
		view.pressedKeys = nil
		if event == fltk.HIDE {
			view.removeNativeInput()
		}
		return false
	}
	x, y := fltk.EventX()-view.video.X(), fltk.EventY()-view.video.Y()
	switch event {
	case fltk.MOVE, fltk.ENTER:
		view.showAt(x, y)
		return true
	case fltk.LEAVE:
		view.progressHover = false
		view.menuHover = false
		view.lastMove = time.Now()
		return true
	case fltk.DRAG:
		if view.scrubbing {
			view.setScrubAt(x)
			return true
		}
		if view.dragging {
			view.setVolumeAt(x)
			return true
		}
		view.showAt(x, y)
		return true
	case fltk.RELEASE:
		if view.scrubbing {
			view.setScrubAt(x)
			view.scrubbing = false
			if view.callbacks.SeekTo != nil {
				view.callbacks.SeekTo(int(math.Round(view.scrubPosition)))
			}
			return true
		}
		view.dragging = false
		return true
	case fltk.MOUSEWHEEL:
		if view.episodesOpen {
			view.episodeStart += fltk.EventDY()
			view.clampEpisodeStart()
			view.Redraw()
			return true
		}
	case fltk.PUSH:
		view.showAt(x, y)
		if view.menuVisible() && x >= 0 && x <= 180 && y >= 62 && y < 62+46*len(view.menuItems) {
			index := (y - 62) / 46
			if view.callbacks.Navigate != nil {
				view.callbacks.Navigate(index)
			}
			view.menuPinned = false
			view.menuHover = false
			view.Redraw()
			return true
		}
		if view.speedOpen {
			if index, ok := speedRowAt(x, y, view.video.W(), view.video.H()); ok {
				view.chooseSpeed(index)
				view.Redraw()
				return true
			}
			if view.actionAt(x, y) != focusSpeed {
				view.speedOpen = false // the button toggles the list itself
			}
		}
		if view.episodesOpen && episodeInBar(x, y, view.video.W(), len(view.episodes)) {
			view.episodeStart = episodeStartAtTrack(y, len(view.episodes))
			view.Redraw()
			return true
		}
		if index, ok := episodeRowAt(x, y, view.video.W(), view.episodeStart, len(view.episodes)); view.episodesOpen && ok {
			view.episodesOpen = false
			view.selectEpisode(index)
			view.Redraw()
			return true
		}
		if view.volumeOpen && y >= view.video.H()-54 && x >= 282 && x <= 395 {
			view.dragging = true
			view.setVolumeAt(x)
			return true
		}
		if view.actionAt(x, y) == focusProgress {
			view.scrubbing = true
			view.state.Focus = focusProgress
			view.setScrubAt(x)
			return true
		}
		if index := view.actionAt(x, y); index >= 0 {
			view.activate(index)
			return true
		}
		if view.episodesOpen {
			view.episodesOpen = false
			view.Redraw()
		}
		return true
	}
	return false
}

func (view *View) handleUnfocus(key int) bool {
	// Key zero marks window-manager focus loss; key-bearing transfers can happen while a shortcut is held.
	if key == 0 {
		view.pressedKeys = nil
	}
	return true
}

func (view *View) handleKeyEvent(event fltk.Event, key, modifiers int) bool {
	if event == fltk.SHORTCUT {
		event = fltk.KEYDOWN
	}
	key = normalizeLetterKey(key)
	if event == fltk.KEYUP {
		if view.pressedKeys == nil || !view.pressedKeys[key] {
			return false
		}
		delete(view.pressedKeys, key)
		return true
	}
	if event != fltk.KEYDOWN {
		return false
	}
	view.showControls()
	// Ctrl+K is the app's quick search; every other modified key is left to the system.
	quickSearch := key == 'k' && modifiers&(fltk.CTRL|fltk.ALT|fltk.META) == fltk.CTRL
	if modifiers&(fltk.CTRL|fltk.ALT|fltk.META) != 0 && !quickSearch {
		return false
	}
	if isOneShotKey(key) {
		if view.pressedKeys == nil {
			view.pressedKeys = make(map[int]bool)
		}
		if view.pressedKeys[key] {
			return true
		}
		view.pressedKeys[key] = true
	}
	if quickSearch || key == '/' {
		if view.callbacks.Search != nil {
			view.callbacks.Search()
		}
		return true
	}
	return view.handleKey(key, modifiers)
}

func isOneShotKey(key int) bool {
	switch key {
	case 9, 0xff09, 27, 0xff1b, fltk.ENTER_KEY, 13, 0xff8d, ' ', 'k', 'm', 'f', '/':
		return true
	default:
		return false
	}
}

func normalizeLetterKey(key int) int {
	if key >= 'A' && key <= 'Z' {
		return key + 'a' - 'A'
	}
	return key
}

func (view *View) handleKey(key, modifiers int) bool {
	key = normalizeLetterKey(key)
	if !view.state.focusable(view.state.Focus) {
		view.state.Focus = focusProgress
	}
	if key == 9 || key == 0xff09 {
		view.moveTabFocus(modifiers&fltk.SHIFT != 0)
		return true
	}
	if key == 27 || key == 0xff1b {
		if view.speedOpen {
			view.speedOpen = false
		} else if view.episodesOpen {
			view.episodesOpen = false
		} else if view.menuVisible() {
			view.menuPinned = false
			view.menuHover = false
		} else if view.state.Fullscreen {
			if view.callbacks.Fullscreen != nil {
				view.callbacks.Fullscreen()
			}
		} else if view.callbacks.Leave != nil {
			view.callbacks.Leave()
		}
		view.Redraw()
		return true
	}
	if key == 'k' {
		view.activate(focusPlayPause)
		return true
	}
	if key == 'm' && !view.menuVisible() && !view.episodesOpen && !view.speedOpen {
		if view.state.Volume == 0 {
			view.setVolume(view.lastVolume)
		} else {
			view.lastVolume = view.state.Volume
			view.setVolume(0)
		}
		return true
	}
	if key == 'f' && !view.menuVisible() && !view.episodesOpen && !view.speedOpen {
		if view.callbacks.Fullscreen != nil {
			view.callbacks.Fullscreen()
		}
		view.showControls()
		return true
	}
	if (key == 'j' || key == 'l') && !view.menuVisible() && !view.episodesOpen && !view.speedOpen {
		step := -10
		if key == 'l' {
			step = 10
		}
		if view.callbacks.Seek != nil {
			view.callbacks.Seek(step)
		}
		view.showControls()
		return true
	}
	if key == int(' ') {
		if view.speedOpen {
			view.chooseSpeed(view.speedCursor)
			view.Redraw()
			return true
		}
		if view.episodesOpen {
			view.selectEpisode(view.episodeCursor)
			view.episodesOpen = false
			view.Redraw()
			return true
		}
		if view.menuPinned {
			if view.callbacks.Navigate != nil {
				view.callbacks.Navigate(view.menuCursor)
			}
			view.menuPinned = false
			view.menuHover = false
			view.Redraw()
			return true
		}
		// Space plays or pauses from any control except the star, which it toggles.
		if view.state.Focus == focusFollow {
			view.activate(focusFollow)
			return true
		}
		view.activate(focusPlayPause)
		return true
	}
	if key == fltk.ENTER_KEY || key == 13 || key == 0xff8d {
		if view.speedOpen {
			view.chooseSpeed(view.speedCursor)
			view.Redraw()
			return true
		}
		if view.episodesOpen {
			view.selectEpisode(view.episodeCursor)
			view.episodesOpen = false
			view.Redraw()
			return true
		}
		if view.menuPinned {
			if view.callbacks.Navigate != nil {
				view.callbacks.Navigate(view.menuCursor)
			}
			view.menuPinned = false
			view.menuHover = false
			view.Redraw()
			return true
		}
		if view.state.Focus == focusProgress {
			return true
		}
		view.activate(view.state.Focus)
		return true
	}
	if key == 0xff51 || key == 0xff53 || key == 0xff52 || key == 0xff54 {
		if view.speedOpen {
			if key == 0xff52 {
				view.moveSpeedCursor(-1)
			}
			if key == 0xff54 {
				view.moveSpeedCursor(1)
			}
			view.Redraw()
			return true
		}
		if view.episodesOpen {
			if key == 0xff52 && view.episodeCursor > 0 {
				view.episodeCursor--
			}
			if key == 0xff54 && view.episodeCursor < len(view.episodes)-1 {
				view.episodeCursor++
			}
			if view.episodeCursor < view.episodeStart {
				view.episodeStart = view.episodeCursor
			}
			if view.episodeCursor >= view.episodeStart+episodeRows {
				view.episodeStart = view.episodeCursor - (episodeRows - 1)
			}
			view.Redraw()
			return true
		}
		if view.menuPinned {
			if key == 0xff52 && view.menuCursor > 0 {
				view.menuCursor--
			}
			if key == 0xff54 && view.menuCursor < len(view.menuItems)-1 {
				view.menuCursor++
			}
			view.Redraw()
			return true
		}
		switch view.state.Focus {
		case focusProgress:
			if key == 0xff51 || key == 0xff53 {
				step := -5
				if key == 0xff53 {
					step = 5
				}
				if view.callbacks.Seek != nil {
					view.callbacks.Seek(step)
				}
				view.showControls()
			}
		case focusVolume:
			step := 5
			if key == 0xff51 || key == 0xff54 {
				step = -5
			}
			view.setVolume(view.state.Volume + step)
		}
		return true
	}
	return false
}

func (view *View) setVolume(volume int) {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	view.state.Volume = volume
	if volume > 0 {
		view.lastVolume = volume
	}
	if view.callbacks.Volume != nil {
		view.callbacks.Volume(volume)
	}
	view.showControls()
}

func (view *View) showControls() {
	view.visible = true
	view.lastMove = time.Now()
	view.Redraw()
}

var tabFocusOrder = [...]int{focusMenu, focusFollow, focusEpisodes, focusProgress, focusPlayPause, focusSeekBack, focusSeekForward, focusStop, focusVolume, focusSpeed, focusFullscreen}

// focusable reports whether a control exists and shows; the follow star shows only once
// the follow state is known.
func (state State) focusable(index int) bool {
	return index >= focusMenu && index <= focusSpeed && (index != focusFollow || state.FollowKnown)
}

func (view *View) moveTabFocus(reverse bool) {
	view.speedOpen = false
	next := nextTabFocus(view.state.Focus, reverse)
	if !view.state.focusable(next) {
		// The star is never first or last in the ring, so its neighbour exists.
		next = nextTabFocus(next, reverse)
	}
	view.SetFocus(next)
}

func nextTabFocus(current int, reverse bool) int {
	index := -1
	for position, focus := range tabFocusOrder {
		if current == focus {
			index = position
			break
		}
	}
	if reverse {
		if index < 0 {
			index = len(tabFocusOrder)
		}
		index = max(0, index-1)
	} else {
		index = min(len(tabFocusOrder)-1, index+1)
	}
	return tabFocusOrder[index]
}

func (view *View) menuVisible() bool { return view.menuPinned || view.menuHover }

func (view *View) selectEpisode(index int) {
	if index < 0 || index >= len(view.episodes) {
		return
	}
	view.episodeCursor = index
	if index == view.state.Episode && view.state.Playing && !view.state.Loading {
		return
	}
	view.state.Episode = index
	if view.callbacks.SelectEpisode != nil {
		view.callbacks.SelectEpisode(index)
	}
}

func (view *View) clampEpisodeStart() {
	view.episodeStart = clampEpisodeWindow(view.episodeStart, len(view.episodes))
}

func joinLines(values []string) string { return strings.Join(values, "\n") }
