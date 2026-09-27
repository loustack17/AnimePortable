// SPDX-License-Identifier: MPL-2.0

package libmpv

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"animeportable/adapters/playback/proxy"
	"animeportable/core"
)

var (
	ErrPlayerClosed   = errors.New("libmpv: playback session closed")
	ErrPlayerFailed   = errors.New("libmpv: playback failed")
	ErrPlayerCleanup  = errors.New("libmpv: playback cleanup failed")
	ErrInvalidStartAt = errors.New("libmpv: playback start position is invalid")
)

type EventKind uint8

const (
	EventProgress EventKind = iota + 1
	EventPaused
	EventEnded
	EventStopped
	EventFailed
)

type Event struct {
	Generation uint64
	Kind       EventKind
	Position   time.Duration
	Duration   time.Duration
	Err        error
}

type Bridge interface {
	Load(ctx context.Context, capabilityURL string, startAt time.Duration, generation uint64) error
	Events() <-chan Event
	Snapshot(ctx context.Context) (core.PlaybackSnapshot, error)
	Close() error
}

type Renderer interface {
	Render(width, height int) error
	Close() error
}

type Factory func(context.Context) (Bridge, Renderer, error)

type Player struct {
	factory  Factory
	newProxy func() (proxyService, error)
}

func NewPlayer(factory Factory) *Player {
	return &Player{factory: factory, newProxy: func() (proxyService, error) {
		server, err := proxy.New(proxy.Config{})
		if err != nil {
			return nil, err
		}
		return &proxyServiceAdapter{server: server}, nil
	}}
}

func (player *Player) Start(ctx context.Context, request core.PlayRequest) (core.PlaybackSession, error) {
	if player == nil || player.factory == nil || ctx == nil {
		return nil, ErrPlayerFailed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateStartAt(request.StartAt); err != nil {
		return nil, err
	}
	server, err := player.newProxy()
	if err != nil {
		return nil, ErrPlayerFailed
	}
	if isNil(server) {
		return nil, ErrPlayerFailed
	}
	capability, err := server.NewSession(request.Source)
	if err != nil {
		_ = server.Close()
		return nil, ErrPlayerFailed
	}
	bridge, renderer, err := player.factory(ctx)
	if err != nil || isNil(bridge) || isNil(renderer) {
		cleanupBridge(bridge, renderer, capability, server)
		return nil, sanitizeError(err)
	}
	session := newSession(bridge, renderer, server, request, capability)
	go session.runEvents()
	if err := session.load(ctx, request, capability, true); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

type capability interface {
	URL() string
	Close() error
}

type proxyService interface {
	NewSession(core.PlaybackSource) (capability, error)
	Close() error
}

type proxyServiceAdapter struct {
	server *proxy.Server
}

func (service *proxyServiceAdapter) NewSession(source core.PlaybackSource) (capability, error) {
	return service.server.NewSession(source)
}

func (service *proxyServiceAdapter) Close() error { return service.server.Close() }

type playbackSession struct {
	bridge   Bridge
	renderer Renderer
	server   proxyService

	opMu sync.Mutex
	mu   sync.Mutex

	current        core.PlayRequest
	currentProxy   capability
	operation      context.CancelFunc
	closed         bool
	closeOnce      sync.Once
	closeDone      chan struct{}
	stopEvents     chan struct{}
	workerDone     chan struct{}
	events         chan core.PlaybackEvent
	position       time.Duration
	duration       time.Duration
	paused         bool
	cleanupErr     error
	generation     uint64
	nextGeneration uint64
	loading        bool
}

func newSession(bridge Bridge, renderer Renderer, server proxyService, request core.PlayRequest, current capability) *playbackSession {
	return &playbackSession{
		bridge: bridge, renderer: renderer, server: server,
		current: request, currentProxy: current,
		closeDone: make(chan struct{}), stopEvents: make(chan struct{}),
		workerDone: make(chan struct{}), events: make(chan core.PlaybackEvent, 16),
	}
}

func (session *playbackSession) load(ctx context.Context, request core.PlayRequest, candidate capability, initial bool) error {
	if ctx == nil {
		if !initial && candidate != nil {
			_ = candidate.Close()
		}
		return ErrPlayerFailed
	}
	if err := ctx.Err(); err != nil {
		if !initial && candidate != nil {
			_ = candidate.Close()
		}
		return err
	}
	if err := validateStartAt(request.StartAt); err != nil {
		if !initial && candidate != nil {
			_ = candidate.Close()
		}
		return err
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		if !initial && candidate != nil {
			_ = candidate.Close()
		}
		return ErrPlayerClosed
	}
	operationCtx, cancel := context.WithCancel(ctx)
	session.operation = cancel
	session.nextGeneration++
	generation := session.nextGeneration
	session.loading = true
	session.mu.Unlock()
	defer func() {
		cancel()
		session.mu.Lock()
		session.operation = nil
		session.loading = false
		session.mu.Unlock()
	}()
	err := session.bridge.Load(operationCtx, candidate.URL(), request.StartAt, generation)
	if err != nil {
		if !initial {
			if closeErr := candidate.Close(); closeErr != nil {
				session.recordCleanupError()
				return ErrPlayerCleanup
			}
		}
		return sanitizeError(err)
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		if !initial {
			_ = candidate.Close()
		}
		return ErrPlayerClosed
	}
	old := session.currentProxy
	session.current = request
	session.currentProxy = candidate
	session.generation = generation
	session.position = request.StartAt
	session.duration = 0
	session.paused = false
	session.mu.Unlock()
	if !initial && old != nil {
		if err := old.Close(); err != nil {
			session.recordCleanupError()
			return ErrPlayerCleanup
		}
	}
	return nil
}

func (player *Player) String() string { return "libmpv.Player{redacted}" }

func (player *Player) GoString() string { return player.String() }

func (session *playbackSession) String() string { return "libmpv.playbackSession{redacted}" }

func (session *playbackSession) GoString() string { return session.String() }

func (session *playbackSession) Load(ctx context.Context, request core.PlayRequest) error {
	if session == nil || ctx == nil {
		return ErrPlayerClosed
	}
	if err := validateStartAt(request.StartAt); err != nil {
		return err
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return ErrPlayerClosed
	}
	server := session.server
	session.mu.Unlock()
	candidate, err := server.NewSession(request.Source)
	if err != nil {
		return ErrPlayerFailed
	}
	return session.load(ctx, request, candidate, false)
}

func (session *playbackSession) Events() <-chan core.PlaybackEvent {
	if session == nil {
		closed := make(chan core.PlaybackEvent)
		close(closed)
		return closed
	}
	return session.events
}

func (session *playbackSession) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if session == nil || ctx == nil {
		return core.PlaybackSnapshot{}, ErrPlayerClosed
	}
	if err := ctx.Err(); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return core.PlaybackSnapshot{}, ErrPlayerClosed
	}
	session.mu.Unlock()
	snapshot, err := session.bridge.Snapshot(ctx)
	if err != nil {
		return core.PlaybackSnapshot{}, sanitizeError(err)
	}
	if snapshot.Position < 0 || snapshot.Duration < 0 {
		return core.PlaybackSnapshot{}, ErrPlayerFailed
	}
	session.mu.Lock()
	if !session.closed {
		session.position = snapshot.Position
		session.duration = snapshot.Duration
		session.paused = snapshot.Paused
	}
	session.mu.Unlock()
	return snapshot, nil
}

func (session *playbackSession) runEvents() {
	defer close(session.workerDone)
	defer close(session.events)
	for {
		select {
		case event, ok := <-session.bridge.Events():
			if !ok {
				return
			}
			session.forwardEvent(event)
		case <-session.stopEvents:
			return
		}
	}
}

func (session *playbackSession) forwardEvent(event Event) {
	session.mu.Lock()
	if session.closed || session.loading || event.Generation != session.generation || event.Position < 0 || event.Duration < 0 {
		session.mu.Unlock()
		return
	}
	request := session.current
	switch event.Kind {
	case EventProgress, EventPaused:
		session.position = event.Position
		session.duration = event.Duration
		session.paused = event.Kind == EventPaused
	}
	session.mu.Unlock()
	kind := core.PlaybackEventUnknown
	switch event.Kind {
	case EventProgress:
		kind = core.PlaybackEventProgress
	case EventPaused:
		kind = core.PlaybackEventPaused
	case EventEnded:
		kind = core.PlaybackEventEnded
	case EventStopped:
		kind = core.PlaybackEventStopped
	case EventFailed:
		kind = core.PlaybackEventFailed
	}
	if kind == core.PlaybackEventUnknown {
		return
	}
	output := core.PlaybackEvent{AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: kind, Position: event.Position, Duration: event.Duration}
	if kind == core.PlaybackEventFailed {
		output.Err = sanitizeError(event.Err)
	}
	select {
	case session.events <- output:
	default:
		if kind == core.PlaybackEventProgress {
			return
		}
		select {
		case <-session.events:
		default:
		}
		select {
		case session.events <- output:
		default:
		}
	}
}

func (session *playbackSession) Close() error {
	if session == nil {
		return nil
	}
	session.closeOnce.Do(func() {
		session.mu.Lock()
		session.closed = true
		cancel := session.operation
		close(session.stopEvents)
		session.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		session.opMu.Lock()
		bridgeErr := session.bridge.Close()
		rendererErr := session.renderer.Close()
		if session.currentProxy != nil && session.currentProxy.Close() != nil {
			session.cleanupErr = ErrPlayerCleanup
		}
		if session.server != nil && session.server.Close() != nil {
			session.cleanupErr = ErrPlayerCleanup
		}
		if bridgeErr != nil || rendererErr != nil {
			session.cleanupErr = ErrPlayerCleanup
		}
		session.opMu.Unlock()
		select {
		case <-session.workerDone:
		case <-time.After(2 * time.Second):
			session.cleanupErr = ErrPlayerCleanup
		}
		close(session.closeDone)
	})
	<-session.closeDone
	return session.cleanupErr
}

func (session *playbackSession) recordCleanupError() {
	session.mu.Lock()
	session.cleanupErr = ErrPlayerCleanup
	session.mu.Unlock()
}

func validateStartAt(position time.Duration) error {
	if position < 0 {
		return ErrInvalidStartAt
	}
	return nil
}

func sanitizeError(err error) error {
	switch {
	case err == nil:
		return ErrPlayerFailed
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, ErrInvalidStartAt):
		return ErrInvalidStartAt
	case errors.Is(err, ErrPlayerClosed):
		return ErrPlayerClosed
	case errors.Is(err, ErrPlayerCleanup):
		return ErrPlayerCleanup
	default:
		return ErrPlayerFailed
	}
}

func cleanupBridge(bridge Bridge, renderer Renderer, current capability, server proxyService) {
	if bridge != nil {
		_ = bridge.Close()
	}
	if renderer != nil {
		_ = renderer.Close()
	}
	if current != nil {
		_ = current.Close()
	}
	if server != nil {
		_ = server.Close()
	}
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

var _ core.Player = (*Player)(nil)
var _ core.PlaybackSnapshotter = (*playbackSession)(nil)
