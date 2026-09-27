//go:build windows && amd64 && cgo

package fltkengine

import (
	"context"
	"encoding/hex"
	"errors"
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
	fltk "github.com/pwiecz/go-fltk"
)

var ErrClosed = errors.New("player is closed")

var opengl = syscall.NewLazyDLL("opengl32.dll")
var wglGetProcAddress = opengl.NewProc("wglGetProcAddress")

type Engine struct {
	video          *fltk.GlWindow
	library        *mpv.Library
	core           *mpv.Mpv
	render         *mpv.RenderContext
	events         chan libmpv.Event
	mu             sync.Mutex
	closed         bool
	generation     uint64
	position       time.Duration
	duration       time.Duration
	paused         bool
	pending        bool
	pendingStarted bool
	failed         bool
	onState        func(time.Duration, time.Duration, bool, int)
	onLoad         func(uint64)
	onFailure      func(uint64)
	closeOnce      sync.Once
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
		for _, option := range [][2]string{{"config", "no"}, {"vo", "libmpv"}, {"hwdec", "auto"}, {"demuxer-max-bytes", "16MiB"}, {"cache-secs", "5"}} {
			if err := core.SetOptionString(option[0], option[1]); err != nil {
				core.TerminateDestroy()
				return libmpv.ErrPlayerFailed
			}
		}
		if err := core.Initialize(); err != nil {
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
		render.SetUpdateCallback(func() {
			fltk.Awake(func() {
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
		for index := 0; index < 128; index++ {
			if event := engine.core.WaitEvent(0); event == nil || event.EventID == mpv.EventNone {
				break
			}
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
		engine.paused = false
		engine.pending = true
		engine.pendingStarted = false
		engine.failed = false
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
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	if engine.closed {
		return core.PlaybackSnapshot{}, ErrClosed
	}
	return core.PlaybackSnapshot{Position: engine.position, Duration: engine.duration, Paused: engine.paused}, nil
}

func (engine *Engine) tick() {
	if engine.isClosed() {
		return
	}
	for index := 0; index < 128; index++ {
		event := engine.core.WaitEvent(0)
		if event == nil || event.EventID == mpv.EventNone {
			break
		}
		engine.mu.Lock()
		generation := engine.generation
		pending := engine.pending
		started := engine.pendingStarted
		if event.EventID == mpv.EventStart && pending {
			engine.pendingStarted = true
		}
		if event.EventID == mpv.EventFileLoaded {
			engine.pending = false
			engine.pendingStarted = false
		}
		if event.EventID == mpv.EventEnd && pending && started && event.EndFile().Reason == mpv.EndFileError {
			engine.pending = false
			engine.pendingStarted = false
		}
		var notifyFailure func(uint64)
		if event.EventID == mpv.EventEnd && event.EndFile().Reason == mpv.EndFileError && (!pending || started) && !engine.failed {
			engine.failed = true
			notifyFailure = engine.onFailure
		}
		engine.mu.Unlock()
		if generation == 0 || event.EventID != mpv.EventEnd {
			continue
		}
		ended := event.EndFile()
		if pending && (!started || ended.Reason != mpv.EndFileError) {
			continue
		}
		kind := libmpv.EventStopped
		if ended.Reason == mpv.EndFileEOF {
			kind = libmpv.EventEnded
		} else if ended.Reason == mpv.EndFileError {
			kind = libmpv.EventFailed
		}
		select {
		case engine.events <- libmpv.Event{Generation: generation, Kind: kind}:
		default:
		}
		if notifyFailure != nil {
			fltk.Awake(func() { notifyFailure(generation) })
		}
	}
	engine.mu.Lock()
	pending := engine.pending
	failed := engine.failed
	engine.mu.Unlock()
	if pending || failed {
		fltk.RepeatTimeout(0.5, engine.tick)
		return
	}
	position, _ := strconv.ParseFloat(engine.core.GetPropertyString("time-pos"), 64)
	duration, _ := strconv.ParseFloat(engine.core.GetPropertyString("duration"), 64)
	paused := engine.core.GetPropertyString("pause") == "yes"
	if position < 0 {
		position = 0
	}
	if duration < 0 {
		duration = 0
	}
	engine.mu.Lock()
	engine.position = time.Duration(position * float64(time.Second))
	engine.duration = time.Duration(duration * float64(time.Second))
	engine.paused = paused
	onState := engine.onState
	event := libmpv.Event{Generation: engine.generation, Kind: libmpv.EventProgress, Position: engine.position, Duration: engine.duration}
	if paused {
		event.Kind = libmpv.EventPaused
	}
	engine.mu.Unlock()
	if onState != nil && event.Generation != 0 {
		resolution, _ := strconv.Atoi(engine.core.GetPropertyString("video-params/h"))
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
	engine.render.Update()
	if err := engine.render.RenderGL(0, width, height, true); err != nil {
		return libmpv.ErrPlayerFailed
	}
	engine.render.ReportSwap()
	return nil
}

func (engine *Engine) Control(ctx context.Context, command string, value int) error {
	return dispatch(ctx, func() error {
		if engine.isClosed() {
			return ErrClosed
		}
		switch command {
		case "pause":
			return engine.core.SetProperty("pause", mpv.FormatFlag, engine.core.GetPropertyString("pause") != "yes")
		case "seek":
			return engine.core.Command([]string{"seek", strconv.Itoa(value), "relative"})
		case "seek_absolute":
			if value < 0 {
				value = 0
			}
			return engine.core.Command([]string{"seek", strconv.Itoa(value), "absolute"})
		case "stop":
			if err := engine.core.SetProperty("pause", mpv.FormatFlag, true); err != nil {
				return err
			}
			return engine.core.Command([]string{"seek", "0", "absolute"})
		case "volume":
			return engine.core.SetProperty("volume", mpv.FormatDouble, float64(value))
		default:
			return libmpv.ErrPlayerFailed
		}
	})
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
