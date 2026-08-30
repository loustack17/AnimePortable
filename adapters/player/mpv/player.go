package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"

	"animeportable/adapters/playback/proxy"
	"animeportable/core"
)

var (
	ErrPlayerClosed  = errors.New("mpv: playback session closed")
	ErrPlayerFailed  = errors.New("mpv: playback session failed")
	ErrLoadRejected  = errors.New("mpv: episode load rejected")
	ErrPlayerCleanup = errors.New("mpv: playback cleanup failed")
)

const defaultLoadTimeout = 30 * time.Second

type rawPlaybackSession interface {
	PID() int
	LoadFile(context.Context, string) error
	currentMedia(context.Context) (string, error)
	Events() <-chan Event
	Close() error
}

type sequencedRawPlaybackSession interface {
	loadFileSequenced(context.Context, string) ipcLoadReceipt
	waitEventsThrough(context.Context) (uint64, error)
}

type proxyCapability interface {
	URL() string
	Close() error
}

type proxyService interface {
	NewSession(core.PlaybackSource) (proxyCapability, error)
	Close() error
}

type playerDeps struct {
	startRaw    func(context.Context, Executable) (rawPlaybackSession, error)
	newProxy    func() (proxyService, error)
	loadTimeout time.Duration
}

type Player struct {
	executable Executable
	deps       playerDeps
}

func NewPlayer(executable Executable) *Player {
	return &Player{executable: executable, deps: defaultPlayerDeps()}
}

func newPlayer(executable Executable, deps playerDeps) *Player {
	deps = normalizePlayerDeps(deps)
	return &Player{executable: executable, deps: deps}
}

func normalizePlayerDeps(deps playerDeps) playerDeps {
	if deps.startRaw == nil {
		deps.startRaw = func(ctx context.Context, executable Executable) (rawPlaybackSession, error) {
			return StartIPC(ctx, executable)
		}
	}
	if deps.newProxy == nil {
		deps.newProxy = func() (proxyService, error) {
			server, err := proxy.New(proxy.Config{})
			if err != nil {
				return nil, err
			}
			return &proxyServiceAdapter{server: server}, nil
		}
	}
	if deps.loadTimeout <= 0 {
		deps.loadTimeout = defaultLoadTimeout
	}
	return deps
}

func defaultPlayerDeps() playerDeps {
	return normalizePlayerDeps(playerDeps{})
}

func closeProxyService(service proxyService) func() error {
	if isNilDependency(service) {
		return nil
	}
	return service.Close
}

func closeProxyCapability(capability proxyCapability) func() error {
	if isNilDependency(capability) {
		return nil
	}
	return capability.Close
}

func closeRawSession(raw rawPlaybackSession) func() error {
	if isNilDependency(raw) {
		return nil
	}
	return raw.Close
}

func cleanupPlayerStart(fallback error, closers ...func() error) error {
	failed := false
	for _, close := range closers {
		if close != nil && close() != nil {
			failed = true
		}
	}
	if failed {
		return ErrPlayerCleanup
	}
	return sanitizePlayerError(fallback)
}

type proxyServiceAdapter struct {
	server *proxy.Server
}

func (service *proxyServiceAdapter) NewSession(source core.PlaybackSource) (proxyCapability, error) {
	if service == nil || service.server == nil {
		return nil, ErrPlayerClosed
	}
	return service.server.NewSession(source)
}

func (service *proxyServiceAdapter) Close() error {
	if service == nil || service.server == nil {
		return nil
	}
	return service.server.Close()
}

type managedProxy struct {
	proxyCapability
	closeOnce sync.Once
	err       error
}

func (capability *managedProxy) Close() error {
	if capability == nil {
		return nil
	}
	capability.closeOnce.Do(func() {
		if capability.proxyCapability != nil {
			capability.err = capability.proxyCapability.Close()
		}
	})
	return capability.err
}

func (capability *managedProxy) String() string { return "mpv.managedProxy{redacted}" }

func (capability *managedProxy) GoString() string { return "mpv.managedProxy{redacted}" }

func (capability *managedProxy) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Proxy string `json:"proxy"`
	}{Proxy: "redacted"})
}

type loadAttempt struct {
	request   core.PlayRequest
	old       *managedProxy
	candidate *managedProxy

	mu                 sync.Mutex
	receiptSet         bool
	sequenced          bool
	barrier            uint64
	loadedPending      bool
	pendingSequence    uint64
	ackSet             bool
	ackErr             error
	loaded             bool
	loadedObserved     bool
	observedSequence   uint64
	validationInFlight int
	cancel             context.CancelFunc
	canceled           bool
	queryCtx           context.Context
	ready              chan struct{}
	readyOnce          sync.Once
}

func newLoadAttempt(request core.PlayRequest, old, candidate *managedProxy) *loadAttempt {
	return &loadAttempt{request: request, old: old, candidate: candidate, ready: make(chan struct{})}
}

func (attempt *loadAttempt) setReceipt(receipt ipcLoadReceipt) {
	attempt.mu.Lock()
	attempt.receiptSet = true
	attempt.sequenced = receipt.barrier != 0 || receipt.ack != 0
	attempt.barrier = receipt.barrier
	if attempt.loadedPending && attempt.sequenced && attempt.pendingSequence <= attempt.barrier {
		attempt.loadedPending = false
	}
	if attempt.loadedObserved && attempt.sequenced && attempt.observedSequence <= attempt.barrier && attempt.validationInFlight == 0 {
		attempt.loadedObserved = false
	}
	if attempt.loadedPending && (!attempt.sequenced || attempt.pendingSequence > attempt.barrier) {
		attempt.loaded = true
		attempt.loadedPending = false
	}
	loaded := attempt.loaded
	ackSet := attempt.ackSet
	attempt.mu.Unlock()
	if loaded && ackSet {
		attempt.signal()
	}
}

func (attempt *loadAttempt) markLoaded(sequence uint64) (error, bool) {
	attempt.mu.Lock()
	if attempt.loaded {
		ackErr, ackSet := attempt.ackErr, attempt.ackSet
		attempt.mu.Unlock()
		return ackErr, ackSet
	}
	if attempt.sequenced && attempt.receiptSet && sequence <= attempt.barrier {
		attempt.mu.Unlock()
		return nil, false
	}
	if !attempt.receiptSet {
		attempt.loadedPending = true
		attempt.pendingSequence = sequence
		attempt.mu.Unlock()
		return nil, false
	}
	attempt.loaded = true
	ackErr, ackSet := attempt.ackErr, attempt.ackSet
	ready := ackSet
	attempt.mu.Unlock()
	if ready {
		attempt.signal()
	}
	return ackErr, ackSet
}

func (attempt *loadAttempt) setAck(err error) bool {
	attempt.mu.Lock()
	if attempt.ackSet {
		loaded := attempt.loaded
		attempt.mu.Unlock()
		return loaded
	}
	attempt.ackSet = true
	attempt.ackErr = err
	loaded := attempt.loaded
	ready := err != nil || attempt.loaded
	attempt.mu.Unlock()
	if ready {
		attempt.signal()
	}
	return loaded
}

func (attempt *loadAttempt) observeLoadedAt(sequence uint64) bool {
	attempt.mu.Lock()
	if attempt.sequenced && attempt.receiptSet && sequence <= attempt.barrier {
		attempt.mu.Unlock()
		return false
	}
	attempt.loadedObserved = true
	attempt.observedSequence = sequence
	attempt.validationInFlight++
	attempt.mu.Unlock()
	return true
}

func (attempt *loadAttempt) finishLoadedValidation(sequence uint64) {
	attempt.mu.Lock()
	if attempt.validationInFlight > 0 {
		attempt.validationInFlight--
	}
	if attempt.validationInFlight == 0 && attempt.sequenced && attempt.receiptSet && sequence <= attempt.barrier {
		attempt.loadedObserved = false
	}
	attempt.mu.Unlock()
}

func (attempt *loadAttempt) loadedObservationState() bool {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	return attempt.loadedObserved || attempt.validationInFlight > 0
}

func (attempt *loadAttempt) signal() {
	attempt.readyOnce.Do(func() { close(attempt.ready) })
}

func (attempt *loadAttempt) setCancel(cancel context.CancelFunc) {
	attempt.mu.Lock()
	if attempt.canceled {
		attempt.mu.Unlock()
		cancel()
		return
	}
	attempt.cancel = cancel
	attempt.mu.Unlock()
}

func (attempt *loadAttempt) setQueryContext(ctx context.Context) {
	attempt.mu.Lock()
	attempt.queryCtx = ctx
	attempt.mu.Unlock()
}

func (attempt *loadAttempt) queryContext() context.Context {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	return attempt.queryCtx
}

func (attempt *loadAttempt) cancelOperation() {
	attempt.mu.Lock()
	attempt.canceled = true
	cancel := attempt.cancel
	attempt.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (attempt *loadAttempt) result() (error, bool) {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	return attempt.ackErr, attempt.loaded
}

func (attempt *loadAttempt) loadedState() bool {
	attempt.mu.Lock()
	defer attempt.mu.Unlock()
	return attempt.loaded
}

const playbackRelayCapacity = 64

type playbackRelay struct {
	mu       sync.Mutex
	queue    []core.PlaybackEvent
	terminal *core.PlaybackEvent
	wake     chan struct{}
	stop     <-chan struct{}
	events   chan core.PlaybackEvent
	done     chan struct{}
}

func newPlaybackRelay(stop <-chan struct{}) *playbackRelay {
	relay := &playbackRelay{
		queue:  make([]core.PlaybackEvent, 0, playbackRelayCapacity),
		wake:   make(chan struct{}, 1),
		stop:   stop,
		events: make(chan core.PlaybackEvent, 1),
		done:   make(chan struct{}),
	}
	go relay.run()
	return relay
}

func (relay *playbackRelay) publishTerminal(event core.PlaybackEvent) {
	if relay == nil {
		return
	}
	relay.mu.Lock()
	if relay.terminal == nil {
		relay.terminal = &event
		relay.queue = nil
	}
	relay.mu.Unlock()
	relay.wakeRelay()
}

func (relay *playbackRelay) publish(event core.PlaybackEvent) {
	select {
	case <-relay.stop:
		return
	default:
	}
	relay.mu.Lock()
	if isCoalesciblePlaybackEvent(event.Kind) {
		for index := range relay.queue {
			queued := relay.queue[index]
			if queued.AnimeID == event.AnimeID && queued.EpisodeID == event.EpisodeID && isCoalesciblePlaybackEvent(queued.Kind) {
				relay.queue[index] = event
				relay.mu.Unlock()
				relay.wakeRelay()
				return
			}
		}
		if len(relay.queue) >= playbackRelayCapacity {
			relay.mu.Unlock()
			return
		}
	} else if len(relay.queue) >= playbackRelayCapacity {
		if !relay.dropCoalescible() {
			relay.mu.Unlock()
			return
		}
	}
	relay.queue = append(relay.queue, event)
	relay.mu.Unlock()
	relay.wakeRelay()
}

func isCoalesciblePlaybackEvent(kind core.PlaybackEventKind) bool {
	return kind == core.PlaybackEventProgress || kind == core.PlaybackEventPaused
}

func (relay *playbackRelay) dropCoalescible() bool {
	for index := range relay.queue {
		if isCoalesciblePlaybackEvent(relay.queue[index].Kind) {
			relay.queue = append(relay.queue[:index], relay.queue[index+1:]...)
			return true
		}
	}
	return false
}

func (relay *playbackRelay) wakeRelay() {
	select {
	case relay.wake <- struct{}{}:
	default:
	}
}

func (relay *playbackRelay) run() {
	defer close(relay.done)
	defer close(relay.events)
	for {
		relay.mu.Lock()
		if relay.terminal != nil {
			event := *relay.terminal
			relay.terminal = nil
			relay.mu.Unlock()
			relay.deliverTerminal(event)
			return
		}
		if len(relay.queue) > 0 {
			event := relay.queue[0]
			relay.queue = relay.queue[1:]
			relay.mu.Unlock()
			select {
			case relay.events <- event:
			case <-relay.stop:
				continue
			}
			continue
		}
		relay.mu.Unlock()
		select {
		case <-relay.wake:
		case <-relay.stop:
			relay.mu.Lock()
			terminal := relay.terminal != nil
			relay.mu.Unlock()
			if terminal {
				continue
			}
			return
		}
	}
}

func (relay *playbackRelay) deliverTerminal(event core.PlaybackEvent) {
	select {
	case relay.events <- event:
		return
	default:
	}
	select {
	case <-relay.events:
	default:
	}
	select {
	case relay.events <- event:
	default:
	}
}

type playbackSession struct {
	raw     rawPlaybackSession
	server  proxyService
	timeout time.Duration

	opMu sync.Mutex
	mu   sync.Mutex

	closed       bool
	failed       bool
	current      core.PlayRequest
	currentProxy *managedProxy
	pending      *loadAttempt

	stopEvents    chan struct{}
	relay         *playbackRelay
	eventDone     chan struct{}
	shutdownDone  chan struct{}
	shutdownOnce  sync.Once
	processedMu   sync.Mutex
	processed     uint64
	processedWake chan struct{}
	cleanupErr    error
	position      time.Duration
	duration      time.Duration
	paused        bool
}

func (session *playbackSession) String() string { return "mpv.playbackSession{redacted}" }

func (session *playbackSession) GoString() string { return "mpv.playbackSession{redacted}" }

func (session *playbackSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Session string `json:"session"`
	}{Session: "redacted"})
}

func (player *Player) String() string { return "mpv.Player{redacted}" }

func (player *Player) GoString() string { return "mpv.Player{redacted}" }

func (player *Player) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Player string `json:"player"`
	}{Player: "redacted"})
}

var _ core.Player = (*Player)(nil)
var _ core.PlaybackSession = (*playbackSession)(nil)

func (player *Player) Start(ctx context.Context, request core.PlayRequest) (core.PlaybackSession, error) {
	if player == nil {
		return nil, ErrPlayerFailed
	}
	if ctx == nil {
		return nil, ErrStart
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deps := normalizePlayerDeps(player.deps)
	server, err := deps.newProxy()
	if err != nil {
		return nil, cleanupPlayerStart(err, closeProxyService(server))
	}
	if isNilDependency(server) {
		return nil, ErrPlayerFailed
	}
	current, err := server.NewSession(request.Source)
	if err != nil {
		return nil, cleanupPlayerStart(err, closeProxyCapability(current), server.Close)
	}
	if isNilDependency(current) {
		return nil, cleanupPlayerStart(ErrPlayerFailed, server.Close)
	}
	managedCurrent := &managedProxy{proxyCapability: current}
	raw, err := deps.startRaw(ctx, player.executable)
	if err != nil {
		return nil, cleanupPlayerStart(err, closeRawSession(raw), managedCurrent.Close, server.Close)
	}
	if isNilDependency(raw) {
		return nil, cleanupPlayerStart(ErrPlayerFailed, managedCurrent.Close, server.Close)
	}
	session := newPlaybackSession(raw, server, deps.loadTimeout)
	if err := session.start(ctx, request, managedCurrent); err != nil {
		if cleanupErr := session.Close(); cleanupErr != nil {
			return nil, cleanupErr
		}
		return nil, err
	}
	return session, nil
}

func newPlaybackSession(raw rawPlaybackSession, server proxyService, timeout time.Duration) *playbackSession {
	if timeout <= 0 {
		timeout = defaultLoadTimeout
	}
	session := &playbackSession{
		raw:           raw,
		server:        server,
		timeout:       timeout,
		stopEvents:    make(chan struct{}),
		eventDone:     make(chan struct{}),
		shutdownDone:  make(chan struct{}),
		processedWake: make(chan struct{}),
	}
	session.relay = newPlaybackRelay(session.stopEvents)
	go session.runEvents()
	return session
}

func (session *playbackSession) start(ctx context.Context, request core.PlayRequest, current *managedProxy) error {
	session.opMu.Lock()
	defer session.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return ErrPlayerClosed
	}
	attempt := newLoadAttempt(request, nil, current)
	session.pending = attempt
	session.mu.Unlock()
	return session.dispatch(ctx, attempt)
}

func (session *playbackSession) Load(ctx context.Context, request core.PlayRequest) error {
	if session == nil {
		return ErrPlayerClosed
	}
	if ctx == nil {
		return ErrPlayerClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	session.opMu.Lock()
	defer session.opMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return ErrPlayerClosed
	}
	server := session.server
	old := session.currentProxy
	session.mu.Unlock()
	if server == nil || old == nil {
		return ErrPlayerFailed
	}
	proxySession, err := server.NewSession(request.Source)
	if err != nil {
		var candidate *managedProxy
		if !isNilDependency(proxySession) {
			candidate = &managedProxy{proxyCapability: proxySession}
		}
		return session.rejectCandidateBeforePending(candidate, sanitizePlayerError(err))
	}
	if isNilDependency(proxySession) {
		return ErrPlayerFailed
	}
	candidate := &managedProxy{proxyCapability: proxySession}
	if err := ctx.Err(); err != nil {
		return session.rejectCandidateBeforePending(candidate, err)
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return session.rejectCandidateBeforePending(candidate, ErrPlayerClosed)
	}
	attempt := newLoadAttempt(request, old, candidate)
	session.pending = attempt
	session.mu.Unlock()
	return session.dispatch(ctx, attempt)
}

func (session *playbackSession) dispatch(ctx context.Context, attempt *loadAttempt) error {
	if err := ctx.Err(); err != nil {
		session.preserve(attempt)
		return err
	}
	session.mu.Lock()
	if session.closed || session.pending != attempt {
		session.mu.Unlock()
		return ErrPlayerClosed
	}
	session.mu.Unlock()
	operationCtx, cancel := context.WithTimeout(ctx, session.timeout)
	attempt.setCancel(cancel)
	attempt.setQueryContext(operationCtx)
	defer cancel()
	if err := operationCtx.Err(); err != nil {
		session.preserve(attempt)
		return err
	}
	receipt := session.loadFile(operationCtx, attempt.candidate.URL())
	attempt.setReceipt(receipt)
	loaded := attempt.setAck(receipt.err)
	if receipt.err != nil {
		if receipt.ack != 0 {
			if waitErr := session.waitEventsThrough(operationCtx); waitErr != nil {
				session.failClosed()
				if !isDefiniteLoadRejection(receipt.err) {
					return sanitizePlayerError(waitErr)
				}
			}
		}
		if isDefiniteLoadRejection(receipt.err) {
			if loaded || attempt.loadedState() || attempt.loadedObservationState() {
				session.failClosed()
				return sanitizePlayerError(receipt.err)
			}
			session.preserve(attempt)
			return sanitizePlayerError(receipt.err)
		}
		session.failClosed()
		return sanitizePlayerError(receipt.err)
	}
	return session.finish(attempt, operationCtx)
}

func (session *playbackSession) loadFile(ctx context.Context, mediaURL string) ipcLoadReceipt {
	if sequenced, ok := session.raw.(sequencedRawPlaybackSession); ok {
		return sequenced.loadFileSequenced(ctx, mediaURL)
	}
	return ipcLoadReceipt{err: session.raw.LoadFile(ctx, mediaURL)}
}

func (session *playbackSession) waitEventsThrough(ctx context.Context) error {
	if sequenced, ok := session.raw.(sequencedRawPlaybackSession); ok {
		processed, err := sequenced.waitEventsThrough(ctx)
		if err != nil {
			return err
		}
		return session.waitProcessedThrough(ctx, processed)
	}
	return nil
}

func (session *playbackSession) markProcessed() {
	session.processedMu.Lock()
	session.processed++
	close(session.processedWake)
	session.processedWake = make(chan struct{})
	session.processedMu.Unlock()
}

func (session *playbackSession) waitProcessedThrough(ctx context.Context, count uint64) error {
	for {
		session.processedMu.Lock()
		if session.processed >= count {
			session.processedMu.Unlock()
			return nil
		}
		wake := session.processedWake
		session.processedMu.Unlock()
		select {
		case <-wake:
		case <-ctx.Done():
			return ctx.Err()
		case <-session.shutdownDone:
			return ErrPlayerClosed
		}
	}
}

func (session *playbackSession) finish(attempt *loadAttempt, ctx context.Context) error {
	select {
	case <-attempt.ready:
	case <-ctx.Done():
		session.failClosed()
		return ctx.Err()
	case <-session.shutdownDone:
		return ErrPlayerClosed
	}
	ackErr, loaded := attempt.result()
	if ackErr != nil {
		session.preserve(attempt)
		return sanitizePlayerError(ackErr)
	}
	if !loaded {
		session.failClosed()
		return ErrPlayerFailed
	}
	session.mu.Lock()
	if session.closed || session.pending != attempt {
		session.mu.Unlock()
		return ErrPlayerClosed
	}
	session.pending = nil
	session.current = attempt.request
	session.currentProxy = attempt.candidate
	session.position = 0
	session.duration = 0
	session.paused = false
	session.mu.Unlock()
	if attempt.old != nil {
		if err := attempt.old.Close(); err != nil {
			session.collectCleanupError(err)
			session.failClosed()
			return ErrPlayerCleanup
		}
	}
	return nil
}

func (session *playbackSession) preserve(attempt *loadAttempt) {
	session.mu.Lock()
	if session.pending == attempt {
		session.pending = nil
	}
	session.mu.Unlock()
	if attempt != nil && attempt.candidate != nil {
		if err := attempt.candidate.Close(); err != nil {
			session.collectCleanupError(err)
			session.failClosed()
		}
	}
}

func (session *playbackSession) rejectCandidateBeforePending(candidate *managedProxy, fallback error) error {
	if candidate == nil {
		return fallback
	}
	if err := candidate.Close(); err != nil {
		session.collectCleanupError(err)
		session.failClosed()
		return ErrPlayerCleanup
	}
	return fallback
}

func isDefiniteLoadRejection(err error) bool {
	return errors.Is(err, ErrIPCCommand) || errors.Is(err, ErrInvalidMedia)
}

func (session *playbackSession) Events() <-chan core.PlaybackEvent {
	if session == nil || session.relay == nil {
		closed := make(chan core.PlaybackEvent)
		close(closed)
		return closed
	}
	return session.relay.events
}

func (session *playbackSession) PID() int {
	if session == nil || session.raw == nil {
		return 0
	}
	return session.raw.PID()
}

func (session *playbackSession) runEvents() {
	defer close(session.eventDone)
	if session.raw == nil {
		return
	}
	for {
		select {
		case event, ok := <-session.raw.Events():
			if !ok {
				session.handleRawClosed()
				return
			}
			session.handleEvent(event)
			session.markProcessed()
		case <-session.stopEvents:
			return
		}
	}
}

func (session *playbackSession) handleRawClosed() {
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}
	request := session.current
	hasRequest := request.AnimeID != "" || request.EpisodeID != ""
	session.mu.Unlock()
	if hasRequest {
		session.emitTerminal(core.PlaybackEvent{AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: core.PlaybackEventFailed, Err: ErrIPCClosed})
	}
	session.failClosed()
}

func (session *playbackSession) handleEvent(event Event) {
	session.mu.Lock()
	if session.closed || session.failed {
		session.mu.Unlock()
		return
	}
	if session.pending != nil {
		attempt := session.pending
		session.mu.Unlock()
		if event.Kind == EventFileLoaded {
			if attempt.observeLoadedAt(event.sequence) {
				session.latchLoaded(attempt, event.sequence)
			}
		}
		return
	}
	request := session.current
	if request.AnimeID == "" || request.EpisodeID == "" {
		session.mu.Unlock()
		return
	}
	position := session.position
	duration := session.duration
	session.mu.Unlock()
	switch event.Kind {
	case EventPropertyChange:
		kind := core.PlaybackEventProgress
		session.mu.Lock()
		switch event.Property {
		case propertyTimePos:
			session.position = event.Position
		case propertyDuration:
			session.duration = event.Duration
		case propertyPause:
			session.paused = event.Paused
		}
		position = session.position
		duration = session.duration
		paused := session.paused
		session.mu.Unlock()
		if event.Property == propertyPause && paused {
			kind = core.PlaybackEventPaused
		}
		session.emit(core.PlaybackEvent{
			AnimeID:   request.AnimeID,
			EpisodeID: request.EpisodeID,
			Kind:      kind,
			Position:  position,
			Duration:  duration,
		})
	case EventEndFile:
		kind := core.PlaybackEventEnded
		var eventErr error
		if event.Reason == "error" {
			kind = core.PlaybackEventFailed
			eventErr = ErrPlayerFailed
		}
		session.emit(core.PlaybackEvent{AnimeID: request.AnimeID, EpisodeID: request.EpisodeID, Kind: kind, Position: position, Duration: duration, Err: eventErr})
	}
}

func (session *playbackSession) latchLoaded(attempt *loadAttempt, sequence uint64) {
	if attempt == nil || session.raw == nil {
		return
	}
	defer attempt.finishLoadedValidation(sequence)
	queryCtx := attempt.queryContext()
	var cancel context.CancelFunc
	if queryCtx == nil {
		queryCtx, cancel = context.WithTimeout(context.Background(), session.timeout)
	}
	if cancel != nil {
		defer cancel()
	}
	media, err := session.raw.currentMedia(queryCtx)
	if err != nil {
		return
	}
	session.mu.Lock()
	if session.closed || session.failed || session.pending != attempt || attempt.candidate == nil || media != attempt.candidate.URL() {
		session.mu.Unlock()
		return
	}
	ackErr, ackSet := attempt.markLoaded(sequence)
	session.mu.Unlock()
	if ackSet && isDefiniteLoadRejection(ackErr) {
		go session.failClosed()
	}
}

func (session *playbackSession) emit(event core.PlaybackEvent) {
	if session.relay != nil {
		session.relay.publish(event)
	}
}

func (session *playbackSession) emitTerminal(event core.PlaybackEvent) {
	if session.relay != nil {
		session.relay.publishTerminal(event)
	}
}

func (session *playbackSession) failClosed() {
	session.shutdown(true)
}

func (session *playbackSession) Close() error {
	if session == nil {
		return nil
	}
	session.shutdown(false)
	<-session.eventDone
	if session.relay != nil {
		<-session.relay.done
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.cleanupErr
}

func (session *playbackSession) shutdown(failed bool) {
	session.shutdownOnce.Do(func() {
		session.mu.Lock()
		session.closed = true
		session.failed = session.failed || failed
		current := session.currentProxy
		pending := session.pending
		var candidate *managedProxy
		if pending != nil {
			candidate = pending.candidate
			session.pending = nil
		}
		raw := session.raw
		server := session.server
		session.mu.Unlock()
		close(session.stopEvents)
		if pending != nil {
			pending.cancelOperation()
			pending.signal()
		}
		if raw != nil {
			session.collectCleanupError(raw.Close())
		}
		if candidate != nil {
			session.collectCleanupError(candidate.Close())
		}
		if current != nil {
			session.collectCleanupError(current.Close())
		}
		if server != nil {
			session.collectCleanupError(server.Close())
		}
		close(session.shutdownDone)
	})
}

func (session *playbackSession) collectCleanupError(err error) {
	if err == nil {
		return
	}
	session.mu.Lock()
	session.cleanupErr = ErrPlayerCleanup
	session.mu.Unlock()
}

func sanitizePlayerError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	case errors.Is(err, ErrIPCCommand):
		return ErrLoadRejected
	case errors.Is(err, ErrInvalidMedia):
		return ErrInvalidMedia
	case errors.Is(err, ErrIPCClosed):
		return ErrIPCClosed
	case errors.Is(err, ErrIPC):
		return ErrIPC
	case errors.Is(err, ErrStart):
		return ErrStart
	case errors.Is(err, ErrPlayerCleanup):
		return ErrPlayerCleanup
	default:
		return ErrPlayerFailed
	}
}

func isNilDependency(value any) bool {
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
