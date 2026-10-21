//go:build windows && amd64 && cgo

package fltkengine

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"animeportable/adapters/player/libmpv"
	mpv "animeportable/apps/desktop/fltkengine/internal/mpvwin"
	"animeportable/core"
	"animeportable/internal/runtimepin"
	"github.com/ebitengine/purego"
	fltk "github.com/pwiecz/go-fltk"
)

var ErrClosed = errors.New("player is closed")
var errSeekPending = errors.New("seek has not completed")

var opengl = syscall.NewLazyDLL("opengl32.dll")
var wglGetProcAddress = opengl.NewProc("wglGetProcAddress")
var glGetStringProc = opengl.NewProc("glGetString")
var glGetStringOnce sync.Once
var glGetStringFunc func(uint32) *byte

type Engine struct {
	video            *fltk.GlWindow
	library          *mpv.Library
	core             *mpv.Mpv
	render           *mpv.RenderContext
	events           chan libmpv.Event
	mu               sync.Mutex
	closed           bool
	generation       uint64
	position         time.Duration
	duration         time.Duration
	paused           bool
	observed         observedProperties
	pending          bool
	ended            bool
	pendingStarted   bool
	failed           bool
	seekPending      bool
	seekStarted      bool
	diagnostics      *playbackDiagnostics
	diagnosticsTicks uint8
	onState          func(time.Duration, time.Duration, bool, int)
	onLoad           func(uint64)
	onFailure        func(uint64)
	onEnd            func(uint64)
	closeOnce        sync.Once
}

func New(ctx context.Context, video *fltk.GlWindow) (*Engine, error) {
	if video == nil {
		return nil, libmpv.ErrPlayerFailed
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, libmpv.ErrPlayerFailed
	}
	libraryPath, libraryHash, err := libraryLocation(executable, os.Getenv)
	if err != nil {
		return nil, libmpv.ErrPlayerFailed
	}
	library, err := mpv.Open(libraryPath, libraryHash)
	if err != nil {
		return nil, libmpv.ErrPlayerFailed
	}
	var result *Engine
	err = dispatch(ctx, func() error {
		core, err := library.New()
		if err != nil {
			return libmpv.ErrPlayerFailed
		}
		options := [][2]string{{"config", "no"}, {"vo", "libmpv"}, {"hwdec", "auto"}, {"demuxer-max-bytes", "16MiB"}, {"cache-secs", "5"}}
		for _, option := range options {
			if err := core.SetOptionString(option[0], option[1]); err != nil {
				core.TerminateDestroy()
				return libmpv.ErrPlayerFailed
			}
		}
		if err := core.Initialize(); err != nil {
			core.TerminateDestroy()
			return libmpv.ErrPlayerFailed
		}
		if err := observeProperties(core, playbackDiagnosticsEnabled(os.Getenv)); err != nil {
			core.TerminateDestroy()
			return libmpv.ErrPlayerFailed
		}
		video.MakeCurrent()
		render, err := core.NewRenderContextGL(glProc)
		if err != nil {
			core.TerminateDestroy()
			return libmpv.ErrPlayerFailed
		}
		engine := &Engine{video: video, library: library, core: core, render: render, events: make(chan libmpv.Event, 32)}
		if playbackDiagnosticsEnabled(os.Getenv) {
			engine.diagnostics = newPlaybackDiagnostics(true, filepath.Join(filepath.Dir(executable), "playback-diagnostics.json"))
		}
		render.SetUpdateCallback(func() {
			if engine.diagnostics != nil {
				engine.diagnostics.recordMPVUpdateCallback()
			}
			fltk.Awake(func() {
				if engine.diagnostics != nil {
					engine.diagnostics.recordFLTKAwakeCallback()
				}
				if !engine.isClosed() {
					video.Redraw()
				}
			})
		})
		result = engine
		fltk.AddTimeout(0.5, engine.tick)
		return nil
	})
	if err != nil {
		if result != nil {
			_ = result.Close()
		} else {
			_ = library.Close()
		}
		return nil, err
	}
	return result, err
}

func libraryLocation(executable string, getenv func(string) string) (string, string, error) {
	path := getenv("ANIMEPORTABLE_LIBMPV_OVERRIDE")
	hash := getenv("ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256")
	if path == "" && hash == "" {
		return filepath.Join(filepath.Dir(executable), "libmpv-2.dll"), runtimepin.LibMPVSHA256, nil
	}
	if !filepath.IsAbs(path) || len(hash) != 64 {
		return "", "", mpv.ErrInvalidLibrary
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return "", "", mpv.ErrInvalidLibrary
	}
	return path, hash, nil
}

func glProc(name string) uintptr {
	bytes, err := syscall.BytePtrFromString(name)
	if err != nil {
		return 0
	}
	address, _, _ := wglGetProcAddress.Call(uintptr(unsafe.Pointer(bytes)))
	if address > 3 && address != ^uintptr(0) {
		return address
	}
	proc := opengl.NewProc(name)
	if proc.Find() != nil {
		return 0
	}
	return proc.Addr()
}

func openGLString(name uint32) string {
	glGetStringOnce.Do(func() {
		if glGetStringProc.Find() == nil {
			purego.RegisterFunc(&glGetStringFunc, glGetStringProc.Addr())
		}
	})
	if glGetStringFunc == nil {
		return ""
	}
	address := glGetStringFunc(name)
	if address == nil {
		return ""
	}
	bytes := make([]byte, 0, 128)
	for index := 0; index < 128; index++ {
		value := *(*byte)(unsafe.Add(unsafe.Pointer(address), uintptr(index)))
		if value == 0 {
			break
		}
		bytes = append(bytes, value)
	}
	return boundedDiagnosticText(string(bytes), 128)
}

func dispatch(ctx context.Context, action func() error) error {
	if ctx == nil {
		return context.Canceled
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	if !fltk.Awake(func() {
		if err := ctx.Err(); err != nil {
			done <- err
			return
		}
		done <- action()
	}) {
		return libmpv.ErrPlayerFailed
	}
	err := <-done
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (engine *Engine) isClosed() bool {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.closed
}

func (engine *Engine) Load(ctx context.Context, url string, startAt time.Duration, generation uint64) error {
	return dispatch(ctx, func() error {
		if engine.isClosed() {
			return ErrClosed
		}
		// Drop events of the previous file, but keep the property cache current.
		for index := 0; index < 128; index++ {
			event := engine.core.WaitEvent(0)
			if event == nil || event.EventID == mpv.EventNone {
				break
			}
			engine.mu.Lock()
			engine.applyProperty(event)
			engine.mu.Unlock()
		}
		command := []string{"loadfile", url, "replace"}
		if startAt > 0 {
			command = append(command, "-1", "start="+strconv.FormatFloat(startAt.Seconds(), 'f', 3, 64))
		}
		if err := engine.core.Command(command); err != nil {
			return libmpv.ErrPlayerFailed
		}
		engine.mu.Lock()
		engine.generation = generation
		engine.position = startAt
		engine.duration = 0
		engine.pending = true
		engine.pendingStarted = false
		engine.ended = false
		engine.failed = false
		engine.seekPending = false
		engine.seekStarted = false
		engine.forgetObservedPlayback()
		onLoad := engine.onLoad
		engine.mu.Unlock()
		if onLoad != nil {
			onLoad(generation)
		}
		engine.video.Redraw()
		return nil
	})
}

func (engine *Engine) Events() <-chan libmpv.Event { return engine.events }

func (engine *Engine) SetStateHandler(handler func(time.Duration, time.Duration, bool, int)) {
	engine.mu.Lock()
	engine.onState = handler
	engine.mu.Unlock()
}

func (engine *Engine) SetFailureHandler(handler func(uint64)) {
	engine.mu.Lock()
	engine.onFailure = handler
	engine.mu.Unlock()
}

// SetEndHandler sets the handler called on the UI thread, once, when the current file
// reaches a normal end of file. Stop, quit, redirect and error ends do not call it.
func (engine *Engine) SetEndHandler(handler func(uint64)) {
	engine.mu.Lock()
	engine.onEnd = handler
	engine.mu.Unlock()
}

func (engine *Engine) SetLoadHandler(handler func(uint64)) {
	engine.mu.Lock()
	engine.onLoad = handler
	engine.mu.Unlock()
}

func (engine *Engine) FailedGeneration(generation uint64) bool {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return !engine.closed && engine.failed && engine.generation == generation
}

func (engine *Engine) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if ctx == nil {
		return core.PlaybackSnapshot{}, context.Canceled
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		snapshot, err := engine.snapshot(ctx)
		if !errors.Is(err, errSeekPending) {
			return snapshot, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			// mpv never confirmed the seek (for example an unseekable stream); stop waiting for it
			// so later snapshots and position updates are not blocked until the next load.
			engine.mu.Lock()
			engine.seekPending, engine.seekStarted = false, false
			engine.mu.Unlock()
			return core.PlaybackSnapshot{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func (engine *Engine) snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	var snapshot core.PlaybackSnapshot
	err := dispatch(ctx, func() error {
		if engine.isClosed() {
			return ErrClosed
		}
		engine.pollEvents()
		engine.mu.Lock()
		if engine.closed {
			engine.mu.Unlock()
			return ErrClosed
		}
		if engine.seekPending {
			engine.mu.Unlock()
			return errSeekPending
		}
		snapshot = core.PlaybackSnapshot{Position: engine.position, Duration: engine.duration, Paused: engine.paused}
		// While a load is pending the observed values still describe the previous file; after a
		// normal end mpv unloads the file and reports them as unavailable.
		positionKnown := engine.observed.positionKnown && !engine.pending && !engine.ended
		durationKnown := engine.observed.durationKnown && !engine.pending && !engine.ended
		positionSeconds, durationSeconds := engine.observed.positionSeconds, engine.observed.durationSeconds
		engine.mu.Unlock()
		var err error
		if positionKnown {
			if snapshot.Position, err = secondsToDuration(positionSeconds); err != nil {
				return err
			}
		}
		if durationKnown {
			if snapshot.Duration, err = secondsToDuration(durationSeconds); err != nil {
				return err
			}
		}
		engine.mu.Lock()
		engine.position, engine.duration = snapshot.Position, snapshot.Duration
		engine.mu.Unlock()
		return nil
	})
	if err != nil {
		return core.PlaybackSnapshot{}, err
	}
	return snapshot, nil
}

func secondsToDuration(seconds float64) (time.Duration, error) {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds < 0 || seconds >= float64(1<<63)/float64(time.Second) {
		return 0, libmpv.ErrPlayerFailed
	}
	return time.Duration(seconds * float64(time.Second)), nil
}

// observedProperties caches the values mpv pushes through property-change events,
// so the UI thread never has to ask the core synchronously.
type observedProperties struct {
	positionSeconds float64
	positionKnown   bool
	durationSeconds float64
	durationKnown   bool
	videoHeight     int
	diagnostics     playbackDiagnosticMPVProperties
}

const observePropertyReply = 1

var coreObservedProperties = []struct {
	name   string
	format mpv.Format
}{
	{"time-pos", mpv.FormatDouble},
	{"duration", mpv.FormatDouble},
	{"pause", mpv.FormatFlag},
	{"video-params/h", mpv.FormatString},
}

var diagnosticObservedProperties = []string{
	"frame-drop-count", "decoder-frame-drop-count", "hwdec-current", "display-fps",
	"estimated-display-fps", "vo-delayed-frame-count", "estimated-vf-fps",
}

func observeProperties(core *mpv.Mpv, diagnostics bool) error {
	for _, property := range coreObservedProperties {
		if err := core.ObserveProperty(observePropertyReply, property.name, property.format); err != nil {
			return err
		}
	}
	if !diagnostics {
		return nil
	}
	for _, name := range diagnosticObservedProperties {
		if err := core.ObserveProperty(observePropertyReply, name, mpv.FormatString); err != nil {
			return err
		}
	}
	return nil
}

// applyProperty stores a property-change event in the cache; the caller holds engine.mu.
func (engine *Engine) applyProperty(event *mpv.Event) {
	if event.EventID != mpv.EventPropertyChange || event.Property == nil {
		return
	}
	change := event.Property
	observed := &engine.observed
	switch change.Name {
	case "time-pos":
		// Positions queued before mpv starts the seek predate it; the seek target is cached instead.
		if engine.seekPending && !engine.seekStarted {
			return
		}
		observed.positionSeconds, observed.positionKnown = change.Double, change.Format == mpv.FormatDouble
	case "duration":
		observed.durationSeconds, observed.durationKnown = change.Double, change.Format == mpv.FormatDouble
	case "pause":
		if change.Format == mpv.FormatFlag {
			engine.paused = change.Flag
		}
	case "video-params/h":
		observed.videoHeight, _ = strconv.Atoi(propertyText(change))
	case "frame-drop-count":
		observed.diagnostics.FrameDropCount = parseDiagnosticCount(propertyText(change))
	case "decoder-frame-drop-count":
		observed.diagnostics.DecoderFrameDropCount = parseDiagnosticCount(propertyText(change))
	case "hwdec-current":
		observed.diagnostics.HWDecCurrent = propertyText(change)
	case "display-fps":
		observed.diagnostics.DisplayFPS = parseDiagnosticFPS(propertyText(change))
	case "estimated-display-fps":
		observed.diagnostics.EstimatedDisplayFPS = parseDiagnosticFPS(propertyText(change))
	case "vo-delayed-frame-count":
		observed.diagnostics.VODelayedFrameCount = parseDiagnosticCount(propertyText(change))
	case "estimated-vf-fps":
		observed.diagnostics.VideoFPS = parseDiagnosticFPS(propertyText(change))
	}
}

// forgetObservedPlayback drops cached time-pos and duration so the previous file's
// values are never reported for a new one. The caller holds engine.mu.
func (engine *Engine) forgetObservedPlayback() {
	engine.observed.positionKnown, engine.observed.durationKnown = false, false
	engine.observed.positionSeconds, engine.observed.durationSeconds = 0, 0
}

// advanceSeek clears a pending seek once mpv restarts playback after it. seek() already
// cached the target position, so Snapshot does not need to wait for a time-pos change,
// which mpv omits when the target equals the current position. The caller holds engine.mu.
func (engine *Engine) advanceSeek(event *mpv.Event) {
	if !engine.seekPending {
		return
	}
	switch event.EventID {
	case mpv.EventSeek:
		engine.seekStarted = true
	case mpv.EventPlaybackRestart:
		if engine.seekStarted {
			engine.seekPending, engine.seekStarted = false, false
		}
	case mpv.EventEnd:
		engine.seekPending, engine.seekStarted = false, false
	}
}

// endTolerance separates a file that played to its real end from a network stream that mpv
// reports as eof after it was cut short. A position from the last tick is at most 0.5 s old
// and the cache is 5 s, so a real end is always well inside 30 s of the duration.
const endTolerance = 30 * time.Second

// endsNormally reports whether an end-file event is the current file reaching its end.
// An EOF that arrives while a load is pending belongs to the file being replaced.
func endsNormally(reason mpv.EndFileReason, generation uint64, pending bool, position, duration time.Duration) bool {
	return generation != 0 && !pending && reason == mpv.EndFileEOF &&
		duration > 0 && duration-position <= endTolerance
}

// endsEarly reports an eof of the current file far from its end: a stream that stopped early,
// which the user must hear about like any other playback failure.
func endsEarly(reason mpv.EndFileReason, generation uint64, pending bool, position, duration time.Duration) bool {
	return generation != 0 && !pending && reason == mpv.EndFileEOF &&
		!endsNormally(reason, generation, pending, position, duration)
}

func (engine *Engine) pollEvents() {
	for index := 0; index < 128; index++ {
		event := engine.core.WaitEvent(0)
		if event == nil || event.EventID == mpv.EventNone {
			break
		}
		engine.mu.Lock()
		generation := engine.generation
		pending := engine.pending
		started := engine.pendingStarted
		engine.applyProperty(event)
		engine.advanceSeek(event)
		if event.EventID == mpv.EventStart && pending {
			engine.pendingStarted = true
			// Property changes of the previous file may still have been queued behind loadfile.
			engine.forgetObservedPlayback()
		}
		if event.EventID == mpv.EventFileLoaded {
			engine.pending = false
			engine.pendingStarted = false
		}
		if event.EventID == mpv.EventEnd && pending && started && event.EndFile().Reason == mpv.EndFileError {
			engine.pending = false
			engine.pendingStarted = false
		}
		var notifyEnd func(uint64)
		normalEnd, cutShort := false, false
		if event.EventID == mpv.EventEnd {
			// Prefer what mpv last reported for the file; the last tick covers the rest.
			position, duration := engine.position, engine.duration
			if engine.observed.positionKnown {
				position = time.Duration(math.Max(engine.observed.positionSeconds, 0) * float64(time.Second))
			}
			if engine.observed.durationKnown {
				duration = time.Duration(math.Max(engine.observed.durationSeconds, 0) * float64(time.Second))
			}
			normalEnd = endsNormally(event.EndFile().Reason, generation, pending, position, duration)
			if normalEnd {
				// Keep reporting the finished file's end: mpv unloads it, so its position and
				// duration are unavailable from now on.
				engine.duration = duration
				engine.position = duration
				engine.ended = true
				notifyEnd = engine.onEnd
			}
			cutShort = endsEarly(event.EndFile().Reason, generation, pending, position, duration)
		}
		var notifyFailure func(uint64)
		if event.EventID == mpv.EventEnd && (event.EndFile().Reason == mpv.EndFileError && (!pending || started) || cutShort) && !engine.failed {
			engine.failed = true
			notifyFailure = engine.onFailure
		}
		endPosition, endDuration := engine.position, engine.duration
		engine.mu.Unlock()
		if generation == 0 || event.EventID != mpv.EventEnd {
			continue
		}
		ended := event.EndFile()
		if pending && (!started || ended.Reason != mpv.EndFileError) {
			continue
		}
		kind := libmpv.EventStopped
		if normalEnd {
			kind = libmpv.EventEnded
		} else if ended.Reason == mpv.EndFileError || cutShort {
			kind = libmpv.EventFailed
		}
		select {
		case engine.events <- libmpv.Event{Generation: generation, Kind: kind, Position: endPosition, Duration: endDuration}:
		default:
		}
		if notifyFailure != nil {
			fltk.Awake(func() { notifyFailure(generation) })
		}
		if notifyEnd != nil {
			fltk.Awake(func() { notifyEnd(generation) })
		}
	}
}

func (engine *Engine) tick() {
	if engine.isClosed() {
		return
	}
	engine.pollEvents()
	engine.mu.Lock()
	pending := engine.pending
	failed := engine.failed
	engine.mu.Unlock()
	if pending || failed {
		fltk.RepeatTimeout(0.5, engine.tick)
		return
	}
	engine.mu.Lock()
	sample := engine.observed.diagnostics
	// Until mpv reports a position, keep the requested start so resume progress is not reset to 0.
	position, duration := engine.position.Seconds(), 0.0
	if engine.ended {
		duration = engine.duration.Seconds()
		position = duration
	} else {
		if engine.observed.positionKnown {
			position = math.Max(engine.observed.positionSeconds, 0)
		}
		if engine.observed.durationKnown {
			duration = math.Max(engine.observed.durationSeconds, 0)
		}
	}
	paused := engine.paused
	resolution := engine.observed.videoHeight
	engine.position = time.Duration(position * float64(time.Second))
	engine.duration = time.Duration(duration * float64(time.Second))
	onState := engine.onState
	event := libmpv.Event{Generation: engine.generation, Kind: libmpv.EventProgress, Position: engine.position, Duration: engine.duration}
	if paused {
		event.Kind = libmpv.EventPaused
	}
	engine.mu.Unlock()
	if engine.diagnostics != nil {
		engine.diagnosticsTicks++
		if engine.diagnosticsTicks >= 10 {
			engine.diagnosticsTicks = 0
			engine.diagnostics.recordMPVProperties(sample)
		}
	}
	if onState != nil && event.Generation != 0 {
		onState(event.Position, event.Duration, paused, resolution)
	}
	if event.Generation != 0 {
		select {
		case engine.events <- event:
		default:
		}
	}
	fltk.RepeatTimeout(0.5, engine.tick)
}

func (engine *Engine) Render(width, height int) error {
	if engine.isClosed() {
		return nil
	}
	diagnostics := engine.diagnostics
	if diagnostics != nil {
		diagnostics.renderStarted(time.Now())
		diagnostics.captureOpenGL(func() (string, string) {
			return openGLString(0x1F00), openGLString(0x1F01)
		})
	}
	flags := engine.render.Update()
	if diagnostics != nil {
		diagnostics.recordRenderUpdate(flags&mpv.RenderUpdateFrame != 0)
	}
	started := time.Time{}
	if diagnostics != nil {
		started = time.Now()
	}
	err := engine.render.RenderGL(0, width, height, true)
	if diagnostics != nil {
		diagnostics.renderFinished(started, time.Now(), err != nil)
	}
	if err != nil {
		return libmpv.ErrPlayerFailed
	}
	return nil
}

func (engine *Engine) Control(ctx context.Context, command string, value int) error {
	return dispatch(ctx, func() error {
		if engine.isClosed() {
			return ErrClosed
		}
		switch command {
		case "pause":
			engine.pollEvents()
			return engine.setPaused(!engine.pausedState())
		case "seek":
			return engine.seek(value, "relative")
		case "seek_absolute":
			if value < 0 {
				value = 0
			}
			return engine.seek(value, "absolute")
		case "stop":
			if err := engine.setPaused(true); err != nil {
				return err
			}
			return engine.seek(0, "absolute")
		case "volume":
			return engine.core.SetProperty("volume", mpv.FormatDouble, float64(value))
		default:
			return libmpv.ErrPlayerFailed
		}
	})
}

func (engine *Engine) pausedState() bool {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	return engine.paused
}

// setPaused also updates the cache at once: mpv confirms through a property-change
// event later, and callers (toggle, Snapshot) must see their own write meanwhile.
func (engine *Engine) setPaused(paused bool) error {
	if err := engine.core.SetProperty("pause", mpv.FormatFlag, paused); err != nil {
		return err
	}
	engine.mu.Lock()
	engine.paused = paused
	engine.mu.Unlock()
	return nil
}

func propertyText(change *mpv.PropertyChange) string {
	if change.Format != mpv.FormatString {
		return ""
	}
	return change.String
}

// cacheSeekTarget records where a seek will land so progress reads stay correct even when
// mpv sends no time-pos change for it. The caller holds engine.mu.
func (engine *Engine) cacheSeekTarget(value int, mode string) {
	target := float64(value)
	if mode != "absolute" {
		current := engine.position.Seconds()
		if engine.observed.positionKnown {
			current = engine.observed.positionSeconds
		}
		target += current
	}
	target = math.Max(target, 0)
	if engine.observed.durationKnown {
		target = math.Min(target, engine.observed.durationSeconds)
	}
	engine.observed.positionSeconds, engine.observed.positionKnown = target, true
}

func (engine *Engine) seek(value int, mode string) error {
	engine.pollEvents()
	engine.mu.Lock()
	engine.seekPending = true
	engine.seekStarted = false
	previousSeconds, previousKnown := engine.observed.positionSeconds, engine.observed.positionKnown
	engine.cacheSeekTarget(value, mode)
	engine.mu.Unlock()
	if err := engine.core.Command([]string{"seek", strconv.Itoa(value), mode}); err != nil {
		engine.mu.Lock()
		engine.seekPending, engine.seekStarted = false, false
		engine.observed.positionSeconds, engine.observed.positionKnown = previousSeconds, previousKnown
		engine.mu.Unlock()
		return err
	}
	return nil
}

func (engine *Engine) Close() error {
	var err error
	engine.closeOnce.Do(func() {
		err = dispatch(context.Background(), func() error {
			engine.mu.Lock()
			engine.closed = true
			engine.mu.Unlock()
			engine.render.SetUpdateCallback(nil)
			engine.video.MakeCurrent()
			freeErr := engine.render.Free()
			_ = engine.diagnostics.persist()
			coreErr := engine.core.TerminateDestroy()
			libraryErr := engine.library.Close()
			close(engine.events)
			engine.video.Redraw()
			if freeErr != nil || coreErr != nil || libraryErr != nil {
				return libmpv.ErrPlayerCleanup
			}
			return nil
		})
	})
	return err
}

var _ libmpv.Bridge = (*Engine)(nil)
var _ libmpv.Renderer = (*Engine)(nil)
