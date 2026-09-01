package mpv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

type fakeProxyService struct {
	mu         sync.Mutex
	next       int
	caps       []*fakeProxyCapability
	newErrors  []error
	newResults []fakeProxyResult
	closed     bool
	closeCount int
	closeErr   error
	afterNew   func(*fakeProxyCapability)
}

type fakeProxyResult struct {
	capability *fakeProxyCapability
	err        error
}

func (service *fakeProxyService) NewSession(core.PlaybackSource) (proxyCapability, error) {
	service.mu.Lock()
	if service.closed {
		service.mu.Unlock()
		return nil, ErrPlayerClosed
	}
	if len(service.newErrors) > 0 {
		err := service.newErrors[0]
		service.newErrors = service.newErrors[1:]
		service.mu.Unlock()
		return nil, err
	}
	if len(service.newResults) > 0 {
		result := service.newResults[0]
		service.newResults = service.newResults[1:]
		if result.capability != nil {
			service.caps = append(service.caps, result.capability)
		}
		hook := service.afterNew
		service.mu.Unlock()
		if hook != nil {
			hook(result.capability)
		}
		return result.capability, result.err
	}
	service.next++
	capability := &fakeProxyCapability{url: fmt.Sprintf("http://127.0.0.1:43210/media/%d", service.next)}
	service.caps = append(service.caps, capability)
	hook := service.afterNew
	service.mu.Unlock()
	if hook != nil {
		hook(capability)
	}
	return capability, nil
}

func (service *fakeProxyService) Close() error {
	service.mu.Lock()
	service.closed = true
	service.closeCount++
	service.mu.Unlock()
	return service.closeErr
}

type fakeProxyCapability struct {
	mu       sync.Mutex
	url      string
	closed   bool
	closes   int
	closeErr error
}

func (capability *fakeProxyCapability) URL() string { return capability.url }

func (capability *fakeProxyCapability) Close() error {
	capability.mu.Lock()
	capability.closed = true
	capability.closes++
	err := capability.closeErr
	capability.mu.Unlock()
	return err
}

func (capability *fakeProxyCapability) isClosed() bool {
	capability.mu.Lock()
	defer capability.mu.Unlock()
	return capability.closed
}

type fakeLoadPlan struct {
	ack          error
	event        bool
	eventBefore  bool
	block        <-chan struct{}
	started      chan<- struct{}
	whileLoad    func()
	beforePath   func()
	currentMedia func() (string, error)
	eventHandled <-chan struct{}
}

type fakeRawSession struct {
	mu             sync.Mutex
	events         chan Event
	closed         chan struct{}
	closeOnce      sync.Once
	pid            int
	plans          []fakeLoadPlan
	urls           []string
	closeCount     int
	closeErr       error
	currentPath    string
	currentMediaFn func() (string, error)
	seekPositions  []time.Duration
	seekErr        error
}

type sequencedLoadPlan struct {
	receipt   ipcLoadReceipt
	event     *Event
	holdEvent bool
}

type sequencedFakeRawSession struct {
	mu            sync.Mutex
	events        chan Event
	closed        chan struct{}
	closeOnce     sync.Once
	pid           int
	plans         []sequencedLoadPlan
	urls          []string
	currentPath   string
	queuedEvents  []Event
	dispatched    uint64
	waitCalls     int
	lastWaitCount uint64
	closeCount    int
}

var _ rawPlaybackSession = (*sequencedFakeRawSession)(nil)
var _ sequencedRawPlaybackSession = (*sequencedFakeRawSession)(nil)

func newSequencedFakeRaw(plans ...sequencedLoadPlan) *sequencedFakeRawSession {
	return &sequencedFakeRawSession{
		events: make(chan Event, 16),
		closed: make(chan struct{}),
		pid:    7002,
		plans:  plans,
	}
}

func (raw *sequencedFakeRawSession) PID() int { return raw.pid }

func (raw *sequencedFakeRawSession) Events() <-chan Event { return raw.events }

func (raw *sequencedFakeRawSession) currentMedia(context.Context) (string, error) {
	raw.mu.Lock()
	defer raw.mu.Unlock()
	return raw.currentPath, nil
}

func (raw *sequencedFakeRawSession) LoadFile(ctx context.Context, mediaURL string) error {
	return raw.loadFileSequenced(ctx, mediaURL).err
}

func (raw *sequencedFakeRawSession) loadFileSequenced(ctx context.Context, mediaURL string) ipcLoadReceipt {
	if ctx == nil {
		return ipcLoadReceipt{err: ErrIPC}
	}
	if err := ctx.Err(); err != nil {
		return ipcLoadReceipt{err: err}
	}
	raw.mu.Lock()
	defer raw.mu.Unlock()
	select {
	case <-raw.closed:
		return ipcLoadReceipt{err: ErrIPCClosed}
	default:
	}
	raw.urls = append(raw.urls, mediaURL)
	raw.currentPath = mediaURL
	plan := sequencedLoadPlan{}
	if len(raw.plans) > 0 {
		plan = raw.plans[0]
		raw.plans = raw.plans[1:]
	}
	if plan.event != nil {
		if plan.holdEvent {
			raw.queuedEvents = append(raw.queuedEvents, *plan.event)
		} else {
			raw.events <- *plan.event
			raw.dispatched++
		}
	}
	return plan.receipt
}

func (raw *sequencedFakeRawSession) waitEventsThrough(ctx context.Context) (uint64, error) {
	if ctx == nil {
		return 0, ErrIPC
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	raw.mu.Lock()
	defer raw.mu.Unlock()
	select {
	case <-raw.closed:
		return 0, ErrIPCClosed
	default:
	}
	raw.waitCalls++
	for _, event := range raw.queuedEvents {
		raw.events <- event
		raw.dispatched++
	}
	raw.queuedEvents = nil
	raw.lastWaitCount = raw.dispatched
	return raw.dispatched, nil
}

func (raw *sequencedFakeRawSession) Close() error {
	raw.closeOnce.Do(func() {
		raw.mu.Lock()
		raw.closeCount++
		close(raw.closed)
		close(raw.events)
		raw.mu.Unlock()
	})
	return nil
}

func (raw *sequencedFakeRawSession) loadURLs() []string {
	raw.mu.Lock()
	defer raw.mu.Unlock()
	return append([]string(nil), raw.urls...)
}

func newFakeRaw(plans ...fakeLoadPlan) *fakeRawSession {
	return &fakeRawSession{events: make(chan Event, 32), closed: make(chan struct{}), pid: 7001, plans: plans}
}

func (raw *fakeRawSession) PID() int { return raw.pid }

func (raw *fakeRawSession) Events() <-chan Event { return raw.events }

func (raw *fakeRawSession) currentMedia(context.Context) (string, error) {
	raw.mu.Lock()
	currentMediaFn := raw.currentMediaFn
	currentPath := raw.currentPath
	raw.mu.Unlock()
	if currentMediaFn != nil {
		return currentMediaFn()
	}
	return currentPath, nil
}

func (raw *fakeRawSession) LoadFile(ctx context.Context, mediaURL string) error {
	raw.mu.Lock()
	raw.urls = append(raw.urls, mediaURL)
	plan := fakeLoadPlan{event: true}
	if len(raw.plans) > 0 {
		plan = raw.plans[0]
		raw.plans = raw.plans[1:]
	}
	raw.mu.Unlock()
	if plan.beforePath != nil {
		plan.beforePath()
	}
	raw.mu.Lock()
	raw.currentPath = mediaURL
	raw.currentMediaFn = plan.currentMedia
	raw.mu.Unlock()
	if plan.started != nil {
		close(plan.started)
	}
	if plan.whileLoad != nil {
		plan.whileLoad()
	}
	if plan.eventBefore {
		raw.publish(Event{Kind: EventFileLoaded})
		if plan.eventHandled != nil {
			select {
			case <-plan.eventHandled:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	if plan.block != nil {
		select {
		case <-plan.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if plan.ack != nil {
		return plan.ack
	}
	if plan.event && !plan.eventBefore {
		raw.publish(Event{Kind: EventFileLoaded})
	}
	return nil
}

func (raw *fakeRawSession) Seek(ctx context.Context, position time.Duration) error {
	if ctx == nil {
		return ErrIPC
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	raw.mu.Lock()
	raw.seekPositions = append(raw.seekPositions, position)
	err := raw.seekErr
	raw.mu.Unlock()
	return err
}

func (raw *fakeRawSession) publish(event Event) {
	select {
	case <-raw.closed:
	case raw.events <- event:
	}
}

func (raw *fakeRawSession) Close() error {
	var err error
	raw.closeOnce.Do(func() {
		raw.mu.Lock()
		raw.closeCount++
		raw.mu.Unlock()
		close(raw.closed)
		close(raw.events)
		err = raw.closeErr
	})
	return err
}

func (raw *fakeRawSession) loadURLs() []string {
	raw.mu.Lock()
	defer raw.mu.Unlock()
	return append([]string(nil), raw.urls...)
}

func (raw *fakeRawSession) seekCalls() []time.Duration {
	raw.mu.Lock()
	defer raw.mu.Unlock()
	return append([]time.Duration(nil), raw.seekPositions...)
}

func testRequest(id string) core.PlayRequest {
	return core.PlayRequest{AnimeID: core.AnimeID("anime"), EpisodeID: core.EpisodeID(id), Source: core.NewPlaybackSource("https://media.example/"+id, http.Header{"Referer": {"https://media.example/"}})}
}

func newTestPlayer(raw *fakeRawSession, service *fakeProxyService) *Player {
	return newPlayer(Executable{path: "mpv"}, playerDeps{
		startRaw:    func(context.Context, Executable) (rawPlaybackSession, error) { return raw, nil },
		newProxy:    func() (proxyService, error) { return service, nil },
		loadTimeout: 100 * time.Millisecond,
	})
}

func newSequencedTestPlayer(raw *sequencedFakeRawSession, service *fakeProxyService, timeout time.Duration) *Player {
	return newPlayer(Executable{path: "mpv"}, playerDeps{
		startRaw:    func(context.Context, Executable) (rawPlaybackSession, error) { return raw, nil },
		newProxy:    func() (proxyService, error) { return service, nil },
		loadTimeout: timeout,
	})
}

func waitEvent(t *testing.T, events <-chan core.PlaybackEvent) core.PlaybackEvent {
	t.Helper()
	select {
	case event, ok := <-events:
		if !ok {
			t.Fatal("event channel closed")
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for playback event")
		return core.PlaybackEvent{}
	}
}

func TestPlayerSwitchesEpisodesOnOneProcessAndRotatesCapabilities(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{event: true}, fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	player := newTestPlayer(raw, service)
	session, err := player.Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if playback.PID() != raw.pid {
		t.Fatalf("pid = %d", playback.PID())
	}
	if err := playback.Load(context.Background(), testRequest("ep02")); err != nil {
		t.Fatal(err)
	}
	if err := playback.Load(context.Background(), testRequest("ep03")); err != nil {
		t.Fatal(err)
	}
	if playback.PID() != raw.pid {
		t.Fatalf("pid changed = %d", playback.PID())
	}
	urls := raw.loadURLs()
	if len(urls) != 3 || urls[0] == urls[1] || urls[1] == urls[2] || urls[0] == urls[2] {
		t.Fatalf("proxy urls = %q", urls)
	}
	service.mu.Lock()
	caps := append([]*fakeProxyCapability(nil), service.caps...)
	service.mu.Unlock()
	if len(caps) != 3 || !caps[0].isClosed() || !caps[1].isClosed() || caps[2].isClosed() {
		t.Fatalf("capability state = %t, %t, %t", caps[0].isClosed(), caps[1].isClosed(), caps[2].isClosed())
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
	if !caps[2].isClosed() || raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("cleanup = cap:%t raw:%d proxy:%d", caps[2].isClosed(), raw.closeCount, service.closeCount)
	}
}

func TestPlayerCommitsFileLoadedAndAckInEitherOrder(t *testing.T) {
	for _, eventBefore := range []bool{false, true} {
		t.Run(fmt.Sprintf("event-before-ack=%t", eventBefore), func(t *testing.T) {
			plan := fakeLoadPlan{event: true, eventBefore: eventBefore}
			raw := newFakeRaw(plan)
			service := &fakeProxyService{}
			session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
			if err != nil {
				t.Fatal(err)
			}
			if err := session.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlayerAppliesPositiveStartAtAfterLoad(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	request := testRequest("ep01")
	request.StartAt = 12*time.Second + 250*time.Millisecond
	session, err := newTestPlayer(raw, service).Start(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if calls := raw.seekCalls(); len(calls) != 1 || calls[0] != request.StartAt {
		t.Fatalf("seek calls = %v", calls)
	}
	snapshot, err := playback.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Position != request.StartAt {
		t.Fatalf("snapshot position = %s", snapshot.Position)
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerSeekFailureFailsClosed(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	raw.seekErr = ErrIPCCommand
	service := &fakeProxyService{}
	request := testRequest("ep01")
	request.StartAt = time.Second
	if _, err := newTestPlayer(raw, service).Start(context.Background(), request); !errors.Is(err, ErrLoadRejected) {
		t.Fatalf("seek failure = %v", err)
	}
	service.mu.Lock()
	serverClosed := service.closed
	service.mu.Unlock()
	if len(raw.seekCalls()) != 1 || raw.closeCount != 1 || !serverClosed {
		t.Fatalf("seek cleanup = calls:%d raw:%d server:%t", len(raw.seekCalls()), raw.closeCount, serverClosed)
	}
}

func TestPlayerRejectsNegativeStartAtBeforeStartingResources(t *testing.T) {
	raw := newFakeRaw()
	service := &fakeProxyService{}
	request := testRequest("ep01")
	request.StartAt = -time.Nanosecond
	if _, err := newTestPlayer(raw, service).Start(context.Background(), request); !errors.Is(err, ErrInvalidStartAt) {
		t.Fatalf("negative startAt = %v", err)
	}
	service.mu.Lock()
	capabilities := len(service.caps)
	service.mu.Unlock()
	if capabilities != 0 || raw.closeCount != 0 {
		t.Fatalf("negative startAt created resources: capabilities=%d raw=%d", capabilities, raw.closeCount)
	}
}

func TestPlayerMapsEndFileReasonsToProviderNeutralKinds(t *testing.T) {
	tests := []struct {
		reason string
		kind   core.PlaybackEventKind
		err    error
	}{
		{reason: "eof", kind: core.PlaybackEventEnded},
		{reason: "stop", kind: core.PlaybackEventStopped},
		{reason: "quit", kind: core.PlaybackEventStopped},
		{reason: "redirect", kind: core.PlaybackEventStopped},
		{reason: "unknown", kind: core.PlaybackEventStopped},
		{reason: "error", kind: core.PlaybackEventFailed, err: ErrPlayerFailed},
	}
	for _, test := range tests {
		t.Run(test.reason, func(t *testing.T) {
			raw := newFakeRaw(fakeLoadPlan{event: true})
			service := &fakeProxyService{}
			session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
			if err != nil {
				t.Fatal(err)
			}
			playback := session.(*playbackSession)
			raw.publish(Event{Kind: EventEndFile, Reason: test.reason})
			event := waitEvent(t, playback.Events())
			if event.Kind != test.kind || !errors.Is(event.Err, test.err) {
				t.Fatalf("event = %+v, want kind %d error %v", event, test.kind, test.err)
			}
			if err := playback.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlayerRawCloseTerminalCarriesLastCheckpoint(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyTimePos, Position: 42 * time.Second})
	if event := waitEvent(t, playback.Events()); event.Position != 42*time.Second {
		t.Fatalf("position event = %+v", event)
	}
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyDuration, Duration: 24 * time.Minute})
	if event := waitEvent(t, playback.Events()); event.Duration != 24*time.Minute {
		t.Fatalf("duration event = %+v", event)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	event := waitEvent(t, playback.Events())
	if event.Kind != core.PlaybackEventFailed || event.Position != 42*time.Second || event.Duration != 24*time.Minute || !errors.Is(event.Err, ErrIPCClosed) {
		t.Fatalf("terminal event = %+v", event)
	}
	_ = playback.Close()
}

func TestPlaybackRelayOnlyCoalescesProgress(t *testing.T) {
	stop := make(chan struct{})
	relay := newPlaybackRelay(stop)
	firstProgress := core.PlaybackEvent{AnimeID: "anime", EpisodeID: "ep01", Kind: core.PlaybackEventProgress, Position: time.Second}
	paused := core.PlaybackEvent{AnimeID: "anime", EpisodeID: "ep01", Kind: core.PlaybackEventPaused, Position: time.Second}
	progress := core.PlaybackEvent{AnimeID: "anime", EpisodeID: "ep01", Kind: core.PlaybackEventProgress, Position: 2 * time.Second}
	relay.publish(firstProgress)
	relay.publish(paused)
	relay.publish(progress)
	if event := waitEvent(t, relay.events); event.Kind != core.PlaybackEventProgress || event.Position != time.Second {
		t.Fatalf("first relay event = %+v", event)
	}
	if event := waitEvent(t, relay.events); event.Kind != core.PlaybackEventPaused || event.Position != time.Second {
		t.Fatalf("second relay event = %+v", event)
	}
	if event := waitEvent(t, relay.events); event.Kind != core.PlaybackEventProgress || event.Position != 2*time.Second {
		t.Fatalf("third relay event = %+v", event)
	}
	close(stop)
	select {
	case <-relay.done:
	case <-time.After(time.Second):
		t.Fatal("relay did not stop")
	}
}

func TestPlaybackRelayKeepsTerminalWhenPauseQueueIsFull(t *testing.T) {
	stop := make(chan struct{})
	relay := newPlaybackRelay(stop)
	for index := 0; index < playbackRelayCapacity+2; index++ {
		relay.publish(core.PlaybackEvent{
			AnimeID: "anime", EpisodeID: "episode", Kind: core.PlaybackEventPaused,
			Position: time.Duration(index) * time.Second,
		})
	}
	relay.publish(core.PlaybackEvent{AnimeID: "anime", EpisodeID: "episode", Kind: core.PlaybackEventEnded})
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-relay.events:
			if event.Kind == core.PlaybackEventEnded {
				close(stop)
				select {
				case <-relay.done:
				case <-time.After(time.Second):
					t.Fatal("relay did not stop")
				}
				return
			}
		case <-deadline:
			t.Fatal("terminal event was dropped")
		}
	}
}

func TestPlayerSuppressesSwitchingEventsAndKeepsCanonicalIDs(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{event: true, block: release, started: started})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	loaded := make(chan error, 1)
	go func() { loaded <- playback.Load(context.Background(), testRequest("ep02")) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("load did not start")
	}
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyTimePos, Position: 12 * time.Second})
	raw.publish(Event{Kind: EventEndFile, Reason: "eof"})
	select {
	case event := <-playback.Events():
		t.Fatalf("switching event was emitted: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-loaded; err != nil {
		t.Fatal(err)
	}
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyTimePos, Position: 20 * time.Second})
	event := waitEvent(t, playback.Events())
	if event.AnimeID != "anime" || event.EpisodeID != "ep02" || event.Position != 20*time.Second || event.Kind != core.PlaybackEventProgress {
		t.Fatalf("event = %+v", event)
	}
	_ = playback.Close()
}

func TestPlayerRelayDoesNotBlockRawPumpWhenPublicQueueIsFull(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	for index := range playbackRelayCapacity + 16 {
		playback.relay.publish(core.PlaybackEvent{AnimeID: core.AnimeID(fmt.Sprintf("anime-%d", index)), EpisodeID: core.EpisodeID("episode"), Kind: core.PlaybackEventProgress})
	}
	completed := make(chan error, 1)
	go func() { completed <- playback.Load(context.Background(), testRequest("ep02")) }()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("raw pump blocked by public event queue")
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
	if raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("cleanup counts = raw:%d proxy:%d", raw.closeCount, service.closeCount)
	}
}

func TestPlayerRawCloseDeliversTerminalFailureBeforeEventsClose(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-playback.eventDone:
	case <-time.After(time.Second):
		t.Fatal("raw close was not handled")
	}
	select {
	case <-playback.relay.done:
	case <-time.After(time.Second):
		t.Fatal("relay did not finish")
	}
	select {
	case event, ok := <-playback.Events():
		if !ok {
			t.Fatal("events closed before terminal failure")
		}
		if event.AnimeID != "anime" || event.EpisodeID != "ep01" || event.Kind != core.PlaybackEventFailed || !errors.Is(event.Err, ErrIPCClosed) {
			t.Fatalf("terminal event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal failure was not delivered")
	}
	select {
	case _, ok := <-playback.Events():
		if ok {
			t.Fatal("event channel remained open after terminal failure")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel did not close after terminal failure")
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerRawCloseReplacesBufferedProgressWithTerminalFailure(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	progress := core.PlaybackEvent{AnimeID: "anime", EpisodeID: "ep01", Kind: core.PlaybackEventProgress, Position: time.Second}
	select {
	case playback.relay.events <- progress:
	default:
		t.Fatal("public event buffer was not available")
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-playback.eventDone:
	case <-time.After(time.Second):
		t.Fatal("raw close was not handled")
	}
	select {
	case <-playback.relay.done:
	case <-time.After(time.Second):
		t.Fatal("relay did not finish")
	}
	select {
	case event, ok := <-playback.Events():
		if !ok {
			t.Fatal("events closed before terminal failure")
		}
		if event.AnimeID != "anime" || event.EpisodeID != "ep01" || event.Kind != core.PlaybackEventFailed || !errors.Is(event.Err, ErrIPCClosed) {
			t.Fatalf("terminal event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal failure was not delivered")
	}
	select {
	case _, ok := <-playback.Events():
		if ok {
			t.Fatal("event channel remained open after terminal failure")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel did not close after terminal failure")
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerCloseReturnsWithoutConsumerAfterRawClose(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-playback.eventDone:
	case <-time.After(time.Second):
		t.Fatal("raw close was not handled")
	}
	closed := make(chan error, 1)
	go func() { closed <- playback.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close blocked without an event consumer")
	}
	select {
	case event, ok := <-playback.Events():
		if !ok || event.Kind != core.PlaybackEventFailed || !errors.Is(event.Err, ErrIPCClosed) {
			t.Fatalf("terminal event = %+v, open=%t", event, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("terminal failure was not retained for a late consumer")
	}
	select {
	case _, ok := <-playback.Events():
		if ok {
			t.Fatal("event channel remained open after terminal failure")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel did not close after terminal failure")
	}
}

func TestPlayerDiagnosticsAreRedacted(t *testing.T) {
	secretURL := "https://media.example/video.mp4?token=source-token"
	secretHeader := "header-secret"
	secretExecutable := `C:\private\mpv-secret.exe`
	secretEndpoint := `\\.\pipe\mpv-secret`
	raw := newFakeRaw()
	raw.mu.Lock()
	raw.currentPath = secretEndpoint
	raw.mu.Unlock()
	managed := &managedProxy{proxyCapability: &fakeProxyCapability{url: secretURL}}
	player := &Player{executable: Executable{path: secretExecutable}}
	session := &playbackSession{
		raw:          raw,
		server:       &fakeProxyService{},
		current:      core.PlayRequest{Source: core.NewPlaybackSource(secretURL, http.Header{"Authorization": {"Bearer " + secretHeader}})},
		currentProxy: managed,
	}
	values := []struct {
		name     string
		value    any
		expected string
	}{
		{name: "player", value: player, expected: "mpv.Player{redacted}"},
		{name: "session", value: session, expected: "mpv.playbackSession{redacted}"},
		{name: "managed proxy", value: managed, expected: "mpv.managedProxy{redacted}"},
	}
	secrets := []string{secretURL, secretHeader, secretExecutable, secretEndpoint, "source-token"}
	for _, test := range values {
		t.Run(test.name, func(t *testing.T) {
			for _, format := range []string{"%v", "%+v", "%#v"} {
				got := fmt.Sprintf(format, test.value)
				if got != test.expected {
					t.Errorf("fmt %s = %q, want %q", format, got, test.expected)
				}
				for _, secret := range secrets {
					if strings.Contains(got, secret) {
						t.Errorf("fmt %s leaked %q: %q", format, secret, got)
					}
				}
			}
			encoded, err := json.Marshal(test.value)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range secrets {
				if strings.Contains(string(encoded), secret) {
					t.Errorf("JSON leaked %q: %s", secret, encoded)
				}
			}
		})
	}
}

func TestPlayerStaleLoadedEventWithoutRealEventFailsClosed(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	queried := make(chan struct{})
	var once sync.Once
	raw.mu.Lock()
	oldPath := raw.currentPath
	media := func() (string, error) {
		once.Do(func() { close(queried) })
		return oldPath, nil
	}
	raw.currentMediaFn = media
	raw.mu.Unlock()
	raw.plans = append(raw.plans, fakeLoadPlan{event: false, beforePath: func() {
		raw.publish(Event{Kind: EventFileLoaded})
		select {
		case <-queried:
		case <-time.After(time.Second):
			t.Error("stale event was not queried")
		}
	}, currentMedia: media})
	if err := playback.Load(context.Background(), testRequest("ep02")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stale-only load error = %v", err)
	}
	service.mu.Lock()
	caps := append([]*fakeProxyCapability(nil), service.caps...)
	serverClosed := service.closed
	service.mu.Unlock()
	if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || !serverClosed || raw.closeCount != 1 {
		t.Fatalf("stale-only cleanup = caps:%d closed:%t,%t server:%t raw:%d", len(caps), caps[0].isClosed(), caps[1].isClosed(), serverClosed, raw.closeCount)
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSequencedPlayerRejectsStaleLoadedEventAfterBarrier(t *testing.T) {
	for iteration := range 100 {
		raw := newSequencedFakeRaw(
			sequencedLoadPlan{
				receipt: ipcLoadReceipt{barrier: 1, ack: 2},
				event:   &Event{Kind: EventFileLoaded, sequence: 3},
			},
			sequencedLoadPlan{
				receipt: ipcLoadReceipt{barrier: 4, ack: 5},
				event:   &Event{Kind: EventFileLoaded, sequence: 4},
			},
		)
		service := &fakeProxyService{}
		session, err := newSequencedTestPlayer(raw, service, 10*time.Millisecond).Start(context.Background(), testRequest("ep01"))
		if err != nil {
			t.Fatalf("iteration %d start: %v", iteration, err)
		}
		playback := session.(*playbackSession)
		err = playback.Load(context.Background(), testRequest("ep02"))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("iteration %d stale load error = %v", iteration, err)
		}
		playback.mu.Lock()
		current := playback.current
		pending := playback.pending
		currentProxy := playback.currentProxy
		closed := playback.closed
		failed := playback.failed
		playback.mu.Unlock()
		if current.EpisodeID != "ep01" || pending != nil || currentProxy == nil || !closed || !failed {
			t.Fatalf("iteration %d state = current:%q pending:%t proxy:%t closed:%t failed:%t", iteration, current.EpisodeID, pending != nil, currentProxy != nil, closed, failed)
		}
		raw.mu.Lock()
		rawClosed := false
		select {
		case <-raw.closed:
			rawClosed = true
		default:
		}
		rawCloseCount := raw.closeCount
		raw.mu.Unlock()
		service.mu.Lock()
		caps := append([]*fakeProxyCapability(nil), service.caps...)
		serverClosed := service.closed
		service.mu.Unlock()
		if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || !serverClosed || !rawClosed || rawCloseCount != 1 {
			t.Fatalf("iteration %d cleanup = caps:%d closed:%t,%t server:%t raw:%t/%d", iteration, len(caps), caps[0].isClosed(), caps[1].isClosed(), serverClosed, rawClosed, rawCloseCount)
		}
		if err := playback.Close(); err != nil {
			t.Fatalf("iteration %d close: %v", iteration, err)
		}
	}
}

func TestSequencedPlayerDrainsQueuedLoadedEventBeforeRejecting(t *testing.T) {
	for iteration := range 100 {
		raw := newSequencedFakeRaw(
			sequencedLoadPlan{
				receipt: ipcLoadReceipt{barrier: 1, ack: 2},
				event:   &Event{Kind: EventFileLoaded, sequence: 3},
			},
			sequencedLoadPlan{
				receipt:   ipcLoadReceipt{barrier: 4, ack: 5, err: ErrIPCCommand},
				event:     &Event{Kind: EventFileLoaded, sequence: 6},
				holdEvent: true,
			},
		)
		service := &fakeProxyService{}
		session, err := newSequencedTestPlayer(raw, service, 100*time.Millisecond).Start(context.Background(), testRequest("ep01"))
		if err != nil {
			t.Fatalf("iteration %d start: %v", iteration, err)
		}
		playback := session.(*playbackSession)
		err = playback.Load(context.Background(), testRequest("ep02"))
		if !errors.Is(err, ErrLoadRejected) {
			t.Fatalf("iteration %d load error = %v", iteration, err)
		}
		raw.mu.Lock()
		waitCalls := raw.waitCalls
		waitCount := raw.lastWaitCount
		rawClosed := false
		select {
		case <-raw.closed:
			rawClosed = true
		default:
		}
		rawCloseCount := raw.closeCount
		raw.mu.Unlock()
		if waitCalls != 1 || waitCount != 2 {
			t.Fatalf("iteration %d event drain = calls:%d count:%d", iteration, waitCalls, waitCount)
		}
		playback.mu.Lock()
		current := playback.current
		pending := playback.pending
		closed := playback.closed
		failed := playback.failed
		playback.mu.Unlock()
		if current.EpisodeID != "ep01" || pending != nil || !closed || !failed {
			t.Fatalf("iteration %d state = current:%q pending:%t closed:%t failed:%t", iteration, current.EpisodeID, pending != nil, closed, failed)
		}
		service.mu.Lock()
		caps := append([]*fakeProxyCapability(nil), service.caps...)
		serverClosed := service.closed
		service.mu.Unlock()
		if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || !serverClosed || !rawClosed || rawCloseCount != 1 {
			t.Fatalf("iteration %d cleanup = caps:%d closed:%t,%t server:%t raw:%t/%d", iteration, len(caps), caps[0].isClosed(), caps[1].isClosed(), serverClosed, rawClosed, rawCloseCount)
		}
		if err := playback.Close(); err != nil {
			t.Fatalf("iteration %d close: %v", iteration, err)
		}
	}
}

func TestPlayerMaintainsCoherentSnapshotAndPauseSemantics(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	updates := []Event{
		{Kind: EventPropertyChange, Property: propertyDuration, Duration: 90 * time.Second},
		{Kind: EventPropertyChange, Property: propertyTimePos, Position: 12 * time.Second},
		{Kind: EventPropertyChange, Property: propertyDuration, Duration: 120 * time.Second},
		{Kind: EventPropertyChange, Property: propertyTimePos, Position: 20 * time.Second},
		{Kind: EventPropertyChange, Property: propertyPause, Paused: false},
		{Kind: EventPropertyChange, Property: propertyPause, Paused: true},
		{Kind: EventEndFile, Reason: "eof"},
	}
	for _, update := range updates {
		raw.publish(update)
	}
	var previousPosition, previousDuration time.Duration
	var sawSnapshot, sawPaused bool
	for {
		select {
		case event, ok := <-playback.Events():
			if !ok {
				t.Fatal("events closed before latest snapshot")
			}
			if event.AnimeID != "anime" || event.EpisodeID != "ep01" {
				t.Fatalf("event IDs = %+v", event)
			}
			if event.Position < previousPosition || event.Duration < previousDuration {
				t.Fatalf("snapshot regressed from %s/%s to %s/%s", previousPosition, previousDuration, event.Position, event.Duration)
			}
			if event.Position < 0 || event.Duration < 0 || event.Duration > 0 && event.Position > event.Duration {
				t.Fatalf("incoherent snapshot = %+v", event)
			}
			previousPosition = event.Position
			previousDuration = event.Duration
			switch event.Kind {
			case core.PlaybackEventProgress:
				sawSnapshot = true
			case core.PlaybackEventPaused:
				sawSnapshot = true
				sawPaused = true
			case core.PlaybackEventEnded:
				if event.Position != 20*time.Second || event.Duration != 120*time.Second {
					t.Fatalf("latest ended snapshot = %+v", event)
				}
				if !sawSnapshot || !sawPaused {
					t.Fatalf("pause/snapshot events missing before end: snapshot=%t paused=%t", sawSnapshot, sawPaused)
				}
				if err := playback.Close(); err != nil {
					t.Fatal(err)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("timed out consuming latest snapshot")
		}
	}
}

func TestPlayerIgnoresEventsBeforeCanonicalCommit(t *testing.T) {
	raw := newFakeRaw()
	service := &fakeProxyService{}
	session := newPlaybackSession(raw, service, 100*time.Millisecond)
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyTimePos, Position: 22 * time.Second})
	select {
	case event := <-session.Events():
		t.Fatalf("pre-commit event = %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	_ = session.Close()
}

func TestPlayerRejectAndPreCanceledLoadPreserveCurrent(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{ack: ErrIPCCommand}, fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if err := playback.Load(context.Background(), testRequest("ep02")); !errors.Is(err, ErrLoadRejected) {
		t.Fatalf("reject error = %v", err)
	}
	service.mu.Lock()
	first := service.caps[0]
	second := service.caps[1]
	service.mu.Unlock()
	if first.isClosed() || !second.isClosed() {
		t.Fatalf("reject capabilities = %t, %t", first.isClosed(), second.isClosed())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := playback.Load(ctx, testRequest("ep03")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	service.mu.Lock()
	count := len(service.caps)
	service.mu.Unlock()
	if count != 2 {
		t.Fatalf("pre-cancel created capability count = %d", count)
	}
	if err := playback.Load(context.Background(), testRequest("ep03")); err != nil {
		t.Fatal(err)
	}
	_ = playback.Close()
}

func TestPlayerLoadedEventThenRejectFailsClosedWhileValidationInflight(t *testing.T) {
	queryStarted := make(chan struct{})
	releaseQuery := make(chan struct{})
	var once sync.Once
	raw := newFakeRaw(fakeLoadPlan{event: true})
	raw.plans = append(raw.plans, fakeLoadPlan{eventBefore: true, ack: ErrIPCCommand, eventHandled: queryStarted, currentMedia: func() (string, error) {
		once.Do(func() { close(queryStarted) })
		<-releaseQuery
		return raw.currentPath, nil
	}})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Load(context.Background(), testRequest("ep02")); !errors.Is(err, ErrLoadRejected) {
		t.Fatalf("reject error = %v", err)
	}
	close(releaseQuery)
	service.mu.Lock()
	caps := append([]*fakeProxyCapability(nil), service.caps...)
	serverClosed := service.closed
	service.mu.Unlock()
	if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || !serverClosed || raw.closeCount != 1 {
		t.Fatalf("loaded reject cleanup = caps:%d closed:%t,%t server:%t raw:%d", len(caps), caps[0].isClosed(), caps[1].isClosed(), serverClosed, raw.closeCount)
	}
	_ = session.Close()
}

func TestPlayerAmbiguousLoadFailureClosesEverything(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{ack: context.DeadlineExceeded})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if err := playback.Load(context.Background(), testRequest("ep02")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("failure error = %v", err)
	}
	if err := playback.Load(context.Background(), testRequest("ep03")); !errors.Is(err, ErrPlayerClosed) {
		t.Fatalf("post-failure load error = %v", err)
	}
	service.mu.Lock()
	caps := append([]*fakeProxyCapability(nil), service.caps...)
	service.mu.Unlock()
	if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("failure cleanup = caps:%d closed:%t,%t raw:%d proxy:%d", len(caps), caps[0].isClosed(), caps[1].isClosed(), raw.closeCount, service.closeCount)
	}
	_ = playback.Close()
}

func TestPlayerTimeoutWaitingForFileLoadedFailsClosed(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{})
	service := &fakeProxyService{}
	player := newTestPlayer(raw, service)
	player.deps.loadTimeout = 10 * time.Millisecond
	session, err := player.Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	if err := playback.Load(context.Background(), testRequest("ep02")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
	if err := playback.Load(context.Background(), testRequest("ep03")); !errors.Is(err, ErrPlayerClosed) {
		t.Fatalf("post-timeout load error = %v", err)
	}
	service.mu.Lock()
	caps := append([]*fakeProxyCapability(nil), service.caps...)
	service.mu.Unlock()
	if len(caps) != 2 || !caps[0].isClosed() || !caps[1].isClosed() || raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("timeout cleanup = caps:%d closed:%t,%t raw:%d proxy:%d", len(caps), caps[0].isClosed(), caps[1].isClosed(), raw.closeCount, service.closeCount)
	}
	_ = playback.Close()
}

func TestPlayerIgnoresLateFileLoadedEvents(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	raw.publish(Event{Kind: EventFileLoaded})
	raw.publish(Event{Kind: EventPropertyChange, Property: propertyTimePos, Position: 21 * time.Second})
	event := waitEvent(t, playback.Events())
	if event.Kind != core.PlaybackEventProgress || event.EpisodeID != "ep01" || event.Position != 21*time.Second {
		t.Fatalf("event = %+v", event)
	}
	select {
	case event := <-playback.Events():
		t.Fatalf("late file-loaded was emitted: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
	_ = playback.Close()
}

func TestPlayerTimeoutAndCloseWakeBlockedLoad(t *testing.T) {
	started := make(chan struct{})
	block := make(chan struct{})
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{block: block, started: started})
	service := &fakeProxyService{}
	player := newTestPlayer(raw, service)
	session, err := player.Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	loaded := make(chan error, 1)
	go func() { loaded <- playback.Load(context.Background(), testRequest("ep02")) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("load did not start")
	}
	if err := playback.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-loaded:
		if !errors.Is(err, ErrPlayerClosed) && !errors.Is(err, context.Canceled) {
			t.Fatalf("blocked load error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not wake load")
	}
	if raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("close counts = raw:%d proxy:%d", raw.closeCount, service.closeCount)
	}
}

func TestPlayerSanitizesErrorsAndEvents(t *testing.T) {
	secret := "https://media.example/video.mp4?token=secret"
	raw := newFakeRaw(fakeLoadPlan{ack: errors.New(secret)})
	service := &fakeProxyService{}
	_, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("start error = %v", err)
	}
}

func TestPlayerStartClosesResourcesReturnedWithErrors(t *testing.T) {
	secret := errors.New("https://media.example/secret?token=secret")
	t.Run("proxy-factory", func(t *testing.T) {
		service := &fakeProxyService{}
		player := newPlayer(Executable{path: "mpv"}, playerDeps{
			newProxy: func() (proxyService, error) { return service, secret },
		})
		_, err := player.Start(context.Background(), testRequest("ep01"))
		if err == nil || strings.Contains(err.Error(), "secret") || service.closeCount != 1 {
			t.Fatalf("err=%v closes=%d", err, service.closeCount)
		}
	})
	t.Run("new-session", func(t *testing.T) {
		capability := &fakeProxyCapability{url: "http://127.0.0.1:43210/media/returned"}
		service := &fakeProxyService{newResults: []fakeProxyResult{{capability: capability, err: secret}}}
		player := newTestPlayer(newFakeRaw(), service)
		_, err := player.Start(context.Background(), testRequest("ep01"))
		if err == nil || strings.Contains(err.Error(), "secret") || !capability.isClosed() || service.closeCount != 1 {
			t.Fatalf("err=%v cap=%t closes=%d", err, capability.isClosed(), service.closeCount)
		}
	})
	t.Run("raw-factory", func(t *testing.T) {
		raw := newFakeRaw()
		service := &fakeProxyService{}
		player := newPlayer(Executable{path: "mpv"}, playerDeps{
			newProxy: func() (proxyService, error) { return service, nil },
			startRaw: func(context.Context, Executable) (rawPlaybackSession, error) { return raw, secret },
		})
		_, err := player.Start(context.Background(), testRequest("ep01"))
		service.mu.Lock()
		capability := service.caps[0]
		service.mu.Unlock()
		if err == nil || strings.Contains(err.Error(), "secret") || !capability.isClosed() || raw.closeCount != 1 || service.closeCount != 1 {
			t.Fatalf("err=%v cap=%t raw=%d proxy=%d", err, capability.isClosed(), raw.closeCount, service.closeCount)
		}
	})
	t.Run("nil-values", func(t *testing.T) {
		player := newPlayer(Executable{path: "mpv"}, playerDeps{
			newProxy: func() (proxyService, error) { return nil, nil },
		})
		if _, err := player.Start(context.Background(), testRequest("ep01")); !errors.Is(err, ErrPlayerFailed) {
			t.Fatalf("nil proxy error = %v", err)
		}
		service := &fakeProxyService{newResults: []fakeProxyResult{{capability: nil}}}
		player = newTestPlayer(newFakeRaw(), service)
		if _, err := player.Start(context.Background(), testRequest("ep01")); !errors.Is(err, ErrPlayerFailed) || service.closeCount != 1 {
			t.Fatalf("nil session error = %v closes=%d", err, service.closeCount)
		}
	})
}

func TestPlayerLoadClosesCapabilityReturnedWithError(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	returned := &fakeProxyCapability{url: "http://127.0.0.1:43210/media/returned"}
	service.mu.Lock()
	service.newResults = []fakeProxyResult{{capability: returned, err: errors.New("source secret")}}
	old := service.caps[0]
	service.mu.Unlock()
	if err := session.Load(context.Background(), testRequest("ep02")); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("load error = %v", err)
	}
	if !returned.isClosed() || old.isClosed() {
		t.Fatalf("capability state returned=%t old=%t", returned.isClosed(), old.isClosed())
	}
	_ = session.Close()
}

func TestPlayerPrePendingCandidateCleanupFailureFailsClosed(t *testing.T) {
	t.Run("resource-error", func(t *testing.T) {
		raw := newFakeRaw(fakeLoadPlan{event: true})
		service := &fakeProxyService{}
		session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
		if err != nil {
			t.Fatal(err)
		}
		candidate := &fakeProxyCapability{url: "http://127.0.0.1:43210/media/candidate", closeErr: errors.New("candidate secret")}
		service.mu.Lock()
		service.newResults = []fakeProxyResult{{capability: candidate, err: errors.New("source secret")}}
		old := service.caps[0]
		service.mu.Unlock()
		if err := session.Load(context.Background(), testRequest("ep02")); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("load error = %v", err)
		}
		service.mu.Lock()
		serverClosed := service.closed
		service.mu.Unlock()
		if !candidate.isClosed() || !old.isClosed() || !serverClosed || raw.closeCount != 1 {
			t.Fatalf("resource cleanup = candidate:%t old:%t server:%t raw:%d", candidate.isClosed(), old.isClosed(), serverClosed, raw.closeCount)
		}
		if err := session.Close(); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("close error = %v", err)
		}
	})

	t.Run("canceled", func(t *testing.T) {
		raw := newFakeRaw(fakeLoadPlan{event: true})
		service := &fakeProxyService{}
		session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
		if err != nil {
			t.Fatal(err)
		}
		candidate := &fakeProxyCapability{url: "http://127.0.0.1:43210/media/candidate", closeErr: errors.New("candidate secret")}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		service.mu.Lock()
		service.newResults = []fakeProxyResult{{capability: candidate}}
		service.afterNew = func(*fakeProxyCapability) { cancel() }
		old := service.caps[0]
		service.mu.Unlock()
		if err := session.Load(ctx, testRequest("ep02")); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("load error = %v", err)
		}
		service.mu.Lock()
		serverClosed := service.closed
		service.mu.Unlock()
		if !candidate.isClosed() || !old.isClosed() || !serverClosed || raw.closeCount != 1 {
			t.Fatalf("cancel cleanup = candidate:%t old:%t server:%t raw:%d", candidate.isClosed(), old.isClosed(), serverClosed, raw.closeCount)
		}
		if err := session.Close(); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("close error = %v", err)
		}
	})

	t.Run("session-closed", func(t *testing.T) {
		raw := newFakeRaw(fakeLoadPlan{event: true})
		service := &fakeProxyService{}
		session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
		if err != nil {
			t.Fatal(err)
		}
		playback := session.(*playbackSession)
		candidate := &fakeProxyCapability{url: "http://127.0.0.1:43210/media/candidate", closeErr: errors.New("candidate secret")}
		service.mu.Lock()
		service.newResults = []fakeProxyResult{{capability: candidate}}
		service.afterNew = func(*fakeProxyCapability) { _ = playback.Close() }
		old := service.caps[0]
		service.mu.Unlock()
		if err := session.Load(context.Background(), testRequest("ep02")); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("load error = %v", err)
		}
		service.mu.Lock()
		serverClosed := service.closed
		service.mu.Unlock()
		if !candidate.isClosed() || !old.isClosed() || !serverClosed || raw.closeCount != 1 {
			t.Fatalf("closed cleanup = candidate:%t old:%t server:%t raw:%d", candidate.isClosed(), old.isClosed(), serverClosed, raw.closeCount)
		}
		if err := session.Close(); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
			t.Fatalf("close error = %v", err)
		}
	})
}

func TestPlayerOldCapabilityRevokeFailureFailsClosed(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true}, fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	service.mu.Lock()
	old := service.caps[0]
	old.closeErr = errors.New("old capability secret")
	service.mu.Unlock()
	if err := session.Load(context.Background(), testRequest("ep02")); !errors.Is(err, ErrPlayerCleanup) {
		t.Fatalf("load error = %v", err)
	}
	service.mu.Lock()
	candidate := service.caps[1]
	serverClosed := service.closed
	service.mu.Unlock()
	if !old.isClosed() || !candidate.isClosed() || !serverClosed || raw.closeCount != 1 {
		t.Fatalf("cleanup old=%t candidate=%t server=%t raw=%d", old.isClosed(), candidate.isClosed(), serverClosed, raw.closeCount)
	}
	if err := session.Close(); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("close error = %v", err)
	}
}

func TestPlayerCloseCollectsSanitizedCleanupErrors(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	raw.closeErr = errors.New("raw endpoint secret")
	service := &fakeProxyService{closeErr: errors.New("proxy source secret")}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); !errors.Is(err, ErrPlayerCleanup) || strings.Contains(err.Error(), "secret") {
		t.Fatalf("close error = %v", err)
	}
}

func TestPlayerCloseIsIdempotentAndDoesNotLeakEvents(t *testing.T) {
	raw := newFakeRaw(fakeLoadPlan{event: true})
	service := &fakeProxyService{}
	session, err := newTestPlayer(raw, service).Start(context.Background(), testRequest("ep01"))
	if err != nil {
		t.Fatal(err)
	}
	playback := session.(*playbackSession)
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_ = playback.Close()
		}()
	}
	wait.Wait()
	select {
	case _, ok := <-playback.Events():
		if ok {
			t.Fatal("event channel remained open")
		}
	case <-time.After(time.Second):
		t.Fatal("event channel did not close")
	}
	if raw.closeCount != 1 || service.closeCount != 1 {
		t.Fatalf("cleanup counts = raw:%d proxy:%d", raw.closeCount, service.closeCount)
	}
}
