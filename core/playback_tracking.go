package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

const (
	defaultCheckpointInterval = 15 * time.Second
	defaultCheckpointTimeout  = 5 * time.Second
	trackedEventQueueCapacity = 32
)

type playbackTicker interface {
	C() <-chan time.Time
	Stop()
}

type playbackTickerFactory func(time.Duration) playbackTicker

type realPlaybackTicker struct {
	*time.Ticker
}

func newRealPlaybackTicker(interval time.Duration) playbackTicker {
	return realPlaybackTicker{Ticker: time.NewTicker(interval)}
}

func (ticker realPlaybackTicker) C() <-chan time.Time {
	return ticker.Ticker.C
}

type playbackTrackingConfig struct {
	now       func() time.Time
	newTicker playbackTickerFactory
	interval  time.Duration
	timeout   time.Duration
}

func normalizePlaybackTrackingConfig(config playbackTrackingConfig) playbackTrackingConfig {
	if config.now == nil {
		config.now = time.Now
	}
	if config.newTicker == nil {
		config.newTicker = newRealPlaybackTicker
	}
	if config.interval <= 0 {
		config.interval = defaultCheckpointInterval
	}
	if config.timeout <= 0 {
		config.timeout = defaultCheckpointTimeout
	}
	return config
}

type trackedPlaybackState struct {
	request    PlayRequest
	snapshot   PlaybackSnapshot
	completed  bool
	terminal   bool
	owner      uint64
	generation uint64
	persisted  uint64
	pending    *trackedPendingPlayback
}

type trackedPendingPlayback struct {
	request    PlayRequest
	snapshot   PlaybackSnapshot
	completed  bool
	terminal   bool
	owner      uint64
	events     []PlaybackEvent
	checkpoint bool
}

type trackedPlaybackSession struct {
	raw    PlaybackSnapshotter
	store  Store
	config playbackTrackingConfig
	relay  *trackedEventRelay

	opGate      chan struct{}
	persistGate chan struct{}
	lifecycle   context.Context
	cancel      context.CancelFunc
	deliveryMu  sync.Mutex
	stateMu     sync.Mutex
	state       trackedPlaybackState
	closing     bool

	stopOnce sync.Once
	stop     chan struct{}
	runDone  chan struct{}

	closeOnce sync.Once
	closeDone chan struct{}
	closeErr  error
}

func newTrackedPlaybackSession(raw PlaybackSession, store Store, request PlayRequest, config playbackTrackingConfig) (*trackedPlaybackSession, error) {
	snapshotter, ok := raw.(PlaybackSnapshotter)
	if !ok || snapshotter == nil || store == nil || raw.Events() == nil {
		return nil, ErrPlaybackTracking
	}
	config = normalizePlaybackTrackingConfig(config)
	lifecycle, cancel := context.WithCancel(context.Background())
	session := &trackedPlaybackSession{
		raw:         snapshotter,
		store:       store,
		config:      config,
		opGate:      make(chan struct{}, 1),
		persistGate: make(chan struct{}, 1),
		lifecycle:   lifecycle,
		cancel:      cancel,
		stop:        make(chan struct{}),
		runDone:     make(chan struct{}),
		closeDone:   make(chan struct{}),
		state: trackedPlaybackState{
			request: request,
			snapshot: PlaybackSnapshot{
				Position: request.StartAt,
			},
			owner:      1,
			generation: 1,
		},
	}
	session.relay = newTrackedEventRelay(session.stop)
	go session.run()
	return session, nil
}

func (session *trackedPlaybackSession) Events() <-chan PlaybackEvent {
	if session == nil || session.relay == nil {
		closed := make(chan PlaybackEvent)
		close(closed)
		return closed
	}
	return session.relay.events
}

func (session *trackedPlaybackSession) Snapshot(ctx context.Context) (PlaybackSnapshot, error) {
	if session == nil || ctx == nil {
		return PlaybackSnapshot{}, ErrPlaybackTracking
	}
	operationCtx, end, err := session.beginOperation(ctx)
	if err != nil {
		return PlaybackSnapshot{}, err
	}
	defer end()
	return session.refreshSnapshot(operationCtx)
}

func (session *trackedPlaybackSession) Load(ctx context.Context, request PlayRequest) error {
	if session == nil || ctx == nil || request.StartAt < 0 {
		return ErrInvalidPlayback
	}
	operationCtx, end, err := session.beginOperation(ctx)
	if err != nil {
		return err
	}
	defer end()
	if err := session.snapshotAndCheckpoint(operationCtx); err != nil {
		return err
	}
	request, err = session.prepareDirectRequest(operationCtx, request)
	if err != nil {
		return err
	}
	if err := session.snapshotAndCheckpoint(operationCtx); err != nil {
		return err
	}
	return session.loadPending(operationCtx, request)
}

func (session *trackedPlaybackSession) switchEpisode(ctx context.Context, resolve func(context.Context) (PlayRequest, error)) error {
	if session == nil || ctx == nil || resolve == nil {
		return ErrInvalidPlayback
	}
	operationCtx, end, err := session.beginOperation(ctx)
	if err != nil {
		return err
	}
	defer end()
	if err := session.snapshotAndCheckpoint(operationCtx); err != nil {
		return err
	}
	request, err := resolve(operationCtx)
	if err != nil {
		return err
	}
	if err := session.snapshotAndCheckpoint(operationCtx); err != nil {
		return err
	}
	return session.loadPending(operationCtx, request)
}

func (session *trackedPlaybackSession) beginOperation(ctx context.Context) (context.Context, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	operationCtx, cancel := context.WithCancel(ctx)
	stopLifecycle := context.AfterFunc(session.lifecycle, cancel)
	select {
	case session.opGate <- struct{}{}:
	case <-operationCtx.Done():
		stopLifecycle()
		cancel()
		if session.isClosing() {
			return nil, nil, ErrPlaybackTracking
		}
		return nil, nil, operationCtx.Err()
	}
	if session.isClosing() {
		<-session.opGate
		stopLifecycle()
		cancel()
		return nil, nil, ErrPlaybackTracking
	}
	end := func() {
		<-session.opGate
		stopLifecycle()
		cancel()
	}
	return operationCtx, end, nil
}

func (session *trackedPlaybackSession) prepareDirectRequest(ctx context.Context, request PlayRequest) (PlayRequest, error) {
	if _, err := session.store.Anime(ctx, request.AnimeID); err != nil {
		return PlayRequest{}, err
	}
	startAt, err := playbackResumePosition(ctx, session.store, request.AnimeID, request.EpisodeID, request.StartAt)
	if err != nil {
		return PlayRequest{}, err
	}
	request.StartAt = startAt
	return request, nil
}

func (session *trackedPlaybackSession) loadPending(ctx context.Context, request PlayRequest) error {
	session.beginPending(request)
	if err := session.raw.Load(ctx, request); err != nil {
		session.discardPending()
		return err
	}
	checkpoint := session.commitPending()
	if checkpoint {
		session.checkpointWithTimeout()
	}
	return nil
}

func (session *trackedPlaybackSession) beginPending(request PlayRequest) {
	session.stateMu.Lock()
	session.state.pending = &trackedPendingPlayback{
		request: request, snapshot: PlaybackSnapshot{Position: request.StartAt},
		owner: session.state.owner + 1,
	}
	session.stateMu.Unlock()
}

func (session *trackedPlaybackSession) discardPending() {
	session.deliveryMu.Lock()
	defer session.deliveryMu.Unlock()
	session.stateMu.Lock()
	session.state.pending = nil
	session.stateMu.Unlock()
}

func (session *trackedPlaybackSession) commitPending() bool {
	session.deliveryMu.Lock()
	defer session.deliveryMu.Unlock()
	session.stateMu.Lock()
	pending := session.state.pending
	if pending == nil {
		session.stateMu.Unlock()
		return false
	}
	session.state.request = pending.request
	session.state.snapshot = pending.snapshot
	session.state.completed = pending.completed
	session.state.terminal = pending.terminal
	session.state.owner = pending.owner
	session.state.generation++
	session.state.pending = nil
	events := append([]PlaybackEvent(nil), pending.events...)
	checkpoint := pending.checkpoint
	session.stateMu.Unlock()
	for _, event := range events {
		session.relay.publish(event, pending.owner)
	}
	return checkpoint
}

func (session *trackedPlaybackSession) refreshSnapshot(ctx context.Context) (PlaybackSnapshot, error) {
	session.stateMu.Lock()
	if session.state.terminal {
		snapshot := session.state.snapshot
		session.stateMu.Unlock()
		return snapshot, nil
	}
	session.stateMu.Unlock()
	snapshot, err := session.raw.Snapshot(ctx)
	session.stateMu.Lock()
	if session.state.terminal {
		finalSnapshot := session.state.snapshot
		session.stateMu.Unlock()
		return finalSnapshot, nil
	}
	if err != nil {
		session.stateMu.Unlock()
		return PlaybackSnapshot{}, err
	}
	if !validTrackedSnapshot(snapshot) {
		session.stateMu.Unlock()
		return PlaybackSnapshot{}, ErrPlaybackTracking
	}
	if snapshot != session.state.snapshot {
		session.state.snapshot = snapshot
		session.state.completed = session.state.completed || completionThresholdReached(snapshot.Position, snapshot.Duration)
		session.state.generation++
	}
	session.stateMu.Unlock()
	return snapshot, nil
}

func (session *trackedPlaybackSession) snapshotAndCheckpoint(ctx context.Context) error {
	if _, err := session.refreshSnapshot(ctx); err != nil {
		return err
	}
	return session.checkpoint(ctx)
}

func (session *trackedPlaybackSession) checkpoint(ctx context.Context) error {
	if ctx == nil {
		return ErrPlaybackTracking
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case session.persistGate <- struct{}{}:
		defer func() { <-session.persistGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	session.stateMu.Lock()
	if session.state.generation == session.state.persisted {
		session.stateMu.Unlock()
		return nil
	}
	generation := session.state.generation
	request := session.state.request
	snapshot := session.state.snapshot
	completed := session.state.completed || completionThresholdReached(snapshot.Position, snapshot.Duration)
	session.stateMu.Unlock()
	if request.AnimeID == "" || request.EpisodeID == "" || !validTrackedSnapshot(snapshot) {
		return ErrPlaybackTracking
	}
	now := session.config.now().UTC()
	entry := HistoryEntry{
		Progress: PlaybackProgress{
			AnimeID: request.AnimeID, EpisodeID: request.EpisodeID,
			Position: snapshot.Position, Duration: snapshot.Duration,
			Completed: completed, UpdatedAt: now,
		},
		LastPlayedAt: now,
	}
	if err := session.store.SavePlaybackCheckpoint(ctx, entry); err != nil {
		return err
	}
	session.stateMu.Lock()
	if generation > session.state.persisted {
		session.state.persisted = generation
	}
	if completed {
		session.state.completed = true
	}
	session.stateMu.Unlock()
	return nil
}

func (session *trackedPlaybackSession) observe(event PlaybackEvent) (bool, uint64) {
	if event.Position < 0 || event.Duration < 0 {
		return false, 0
	}
	session.stateMu.Lock()
	defer session.stateMu.Unlock()
	pending := session.state.pending
	if pending != nil && event.AnimeID == pending.request.AnimeID && event.EpisodeID == pending.request.EpisodeID {
		if pending.terminal {
			return false, 0
		}
		snapshot := PlaybackSnapshot{Position: event.Position, Duration: event.Duration, Paused: event.Kind == PlaybackEventPaused}
		pending.snapshot = snapshot
		pending.completed = pending.completed || event.Kind == PlaybackEventEnded || completionThresholdReached(snapshot.Position, snapshot.Duration)
		pending.terminal = isTerminalPlaybackEvent(event.Kind)
		pending.checkpoint = pending.checkpoint || isCheckpointBoundary(event.Kind)
		pending.events = appendPendingPlaybackEvent(pending.events, event)
		return false, pending.owner
	}
	if event.AnimeID == session.state.request.AnimeID && event.EpisodeID == session.state.request.EpisodeID {
		if session.state.terminal {
			return false, 0
		}
		snapshot := PlaybackSnapshot{Position: event.Position, Duration: event.Duration, Paused: event.Kind == PlaybackEventPaused}
		session.state.snapshot = snapshot
		session.state.completed = session.state.completed || event.Kind == PlaybackEventEnded || completionThresholdReached(snapshot.Position, snapshot.Duration)
		session.state.terminal = isTerminalPlaybackEvent(event.Kind)
		session.state.generation++
		return isCheckpointBoundary(event.Kind), session.state.owner
	}
	return false, 0
}

func appendPendingPlaybackEvent(events []PlaybackEvent, event PlaybackEvent) []PlaybackEvent {
	if event.Kind == PlaybackEventProgress && len(events) > 0 {
		last := len(events) - 1
		if events[last].Kind == PlaybackEventProgress {
			events[last] = event
			return events
		}
	}
	if len(events) >= trackedEventQueueCapacity {
		for index, queued := range events {
			if queued.Kind == PlaybackEventProgress {
				events = append(events[:index], events[index+1:]...)
				return append(events, event)
			}
		}
		if !isTerminalPlaybackEvent(event.Kind) {
			return events
		}
		events = events[1:]
	}
	return append(events, event)
}

func (session *trackedPlaybackSession) handleRawEvent(event PlaybackEvent) {
	session.deliveryMu.Lock()
	checkpoint, owner := session.observe(event)
	if owner != 0 {
		session.stateMu.Lock()
		pending := session.state.pending != nil && session.state.pending.owner == owner
		session.stateMu.Unlock()
		if !pending {
			session.relay.publish(event, owner)
		}
	}
	session.deliveryMu.Unlock()
	if checkpoint {
		session.checkpointWithTimeout()
	}
}

func (session *trackedPlaybackSession) run() {
	ticker := session.config.newTicker(session.config.interval)
	defer func() {
		ticker.Stop()
		session.stopOnce.Do(func() { close(session.stop) })
		session.relay.shutdown()
		close(session.runDone)
	}()
	for {
		select {
		case event, ok := <-session.raw.Events():
			if !ok {
				session.checkpointWithTimeout()
				return
			}
			session.handleRawEvent(event)
		case <-ticker.C():
			session.checkpointWithTimeout()
		case <-session.stop:
			return
		}
	}
}

func (session *trackedPlaybackSession) checkpointWithTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), session.config.timeout)
	defer cancel()
	_ = session.checkpoint(ctx)
}

func isCheckpointBoundary(kind PlaybackEventKind) bool {
	return kind == PlaybackEventPaused || kind == PlaybackEventEnded ||
		kind == PlaybackEventStopped || kind == PlaybackEventFailed
}

func completionThresholdReached(position, duration time.Duration) bool {
	return position >= 0 && duration > 0 && float64(position)/float64(duration) >= 0.9
}

func validTrackedSnapshot(snapshot PlaybackSnapshot) bool {
	return snapshot.Position >= 0 && snapshot.Duration >= 0
}

func (session *trackedPlaybackSession) isClosing() bool {
	session.stateMu.Lock()
	defer session.stateMu.Unlock()
	return session.closing
}

func (session *trackedPlaybackSession) stopRun(ctx context.Context) error {
	session.stopOnce.Do(func() { close(session.stop) })
	select {
	case <-session.runDone:
		return nil
	case <-ctx.Done():
		return ErrPlaybackTracking
	}
}

func (session *trackedPlaybackSession) Close() error {
	if session == nil {
		return nil
	}
	session.closeOnce.Do(func() { go session.close() })
	<-session.closeDone
	return session.closeErr
}

func (session *trackedPlaybackSession) close() {
	session.stateMu.Lock()
	session.closing = true
	session.stateMu.Unlock()
	session.cancel()
	stopCtx, cancelStop := context.WithTimeout(context.Background(), session.config.timeout)
	runErr := session.stopRun(stopCtx)
	cancelStop()
	ctx, cancel := context.WithTimeout(context.Background(), session.config.timeout)
	var snapshotErr error
	var persistErr error
	acquired := false
	select {
	case session.opGate <- struct{}{}:
		acquired = true
	case <-ctx.Done():
		snapshotErr = ErrPlaybackTracking
	}
	if acquired {
		if _, err := session.refreshSnapshot(ctx); err != nil {
			snapshotErr = ErrPlaybackTracking
		}
		persistErr = session.checkpoint(ctx)
		<-session.opGate
	}
	rawErr := session.raw.Close()
	if !acquired {
		persistCtx, cancelPersist := context.WithTimeout(context.Background(), session.config.timeout)
		persistErr = session.checkpoint(persistCtx)
		cancelPersist()
	}
	cancel()
	<-session.runDone
	session.closeErr = errors.Join(runErr, snapshotErr, persistErr, rawErr)
	close(session.closeDone)
}

type trackedEventRelay struct {
	mu                sync.Mutex
	queue             []trackedRelayEvent
	priority          []trackedRelayEvent
	lastTerminalOwner uint64
	terminalSeen      bool
	wake              chan struct{}
	stop              <-chan struct{}
	events            chan PlaybackEvent
	done              chan struct{}
	once              sync.Once
}

type trackedRelayEvent struct {
	event PlaybackEvent
	owner uint64
}

func newTrackedEventRelay(stop <-chan struct{}) *trackedEventRelay {
	relay := &trackedEventRelay{
		queue:    make([]trackedRelayEvent, 0, trackedEventQueueCapacity),
		priority: make([]trackedRelayEvent, 0, trackedEventQueueCapacity),
		wake:     make(chan struct{}, 1),
		stop:     stop,
		events:   make(chan PlaybackEvent, 1),
		done:     make(chan struct{}),
	}
	go relay.run()
	return relay
}

func (relay *trackedEventRelay) publish(event PlaybackEvent, owner uint64) {
	if relay == nil {
		return
	}
	item := trackedRelayEvent{event: event, owner: owner}
	relay.mu.Lock()
	if isTerminalPlaybackEvent(event.Kind) {
		if relay.terminalSeen && relay.lastTerminalOwner == owner {
			relay.mu.Unlock()
			return
		}
		relay.terminalSeen = true
		relay.lastTerminalOwner = owner
		relay.queue = relay.queue[:0]
		if len(relay.priority) < trackedEventQueueCapacity {
			relay.priority = append(relay.priority, item)
		}
		relay.mu.Unlock()
		relay.wakeRelay()
		return
	}
	if relay.terminalSeen && relay.lastTerminalOwner == owner {
		relay.mu.Unlock()
		return
	}
	if event.Kind == PlaybackEventProgress {
		last := len(relay.queue) - 1
		if last >= 0 && relay.queue[last].owner == owner && relay.queue[last].event.Kind == PlaybackEventProgress {
			relay.queue[last] = item
			relay.mu.Unlock()
			relay.wakeRelay()
			return
		}
	}
	if len(relay.queue) >= trackedEventQueueCapacity && !relay.dropProgress() {
		relay.mu.Unlock()
		return
	}
	relay.queue = append(relay.queue, item)
	relay.mu.Unlock()
	relay.wakeRelay()
}

func isTerminalPlaybackEvent(kind PlaybackEventKind) bool {
	return kind == PlaybackEventEnded || kind == PlaybackEventStopped || kind == PlaybackEventFailed
}

func (relay *trackedEventRelay) dropProgress() bool {
	for index, item := range relay.queue {
		if item.event.Kind == PlaybackEventProgress {
			relay.queue = append(relay.queue[:index], relay.queue[index+1:]...)
			return true
		}
	}
	return false
}

func (relay *trackedEventRelay) wakeRelay() {
	select {
	case relay.wake <- struct{}{}:
	default:
	}
}

func (relay *trackedEventRelay) run() {
	defer close(relay.done)
	defer close(relay.events)
	for {
		relay.mu.Lock()
		if len(relay.priority) > 0 {
			item := relay.priority[0]
			relay.priority = relay.priority[1:]
			relay.mu.Unlock()
			select {
			case relay.events <- item.event:
			case <-relay.stop:
				return
			}
			continue
		}
		if len(relay.queue) > 0 {
			item := relay.queue[0]
			relay.queue = relay.queue[1:]
			relay.mu.Unlock()
			select {
			case relay.events <- item.event:
			case <-relay.stop:
				return
			}
			continue
		}
		relay.mu.Unlock()
		select {
		case <-relay.wake:
		case <-relay.stop:
			return
		}
	}
}

func (relay *trackedEventRelay) shutdown() {
	if relay == nil {
		return
	}
	relay.once.Do(func() {
		relay.wakeRelay()
		<-relay.done
	})
}
