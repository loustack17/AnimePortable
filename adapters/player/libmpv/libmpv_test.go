// SPDX-License-Identifier: MPL-2.0

package libmpv

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/core"
)

type fakeCapability struct {
	url        string
	mu         sync.Mutex
	closeCount int
}

func (capability *fakeCapability) URL() string { return capability.url }

func (capability *fakeCapability) Close() error {
	capability.mu.Lock()
	capability.closeCount++
	capability.mu.Unlock()
	return nil
}

func (capability *fakeCapability) closed() int {
	capability.mu.Lock()
	defer capability.mu.Unlock()
	return capability.closeCount
}

type fakeProxy struct {
	mu           sync.Mutex
	capabilities []*fakeCapability
	closeCount   int
	newErr       error
}

func (server *fakeProxy) NewSession(core.PlaybackSource) (capability, error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.newErr != nil {
		return nil, server.newErr
	}
	capability := &fakeCapability{url: "http://127.0.0.1/media/test-capability"}
	server.capabilities = append(server.capabilities, capability)
	return capability, nil
}

func (server *fakeProxy) Close() error {
	server.mu.Lock()
	server.closeCount++
	server.mu.Unlock()
	return nil
}

func (server *fakeProxy) caps() []*fakeCapability {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]*fakeCapability(nil), server.capabilities...)
}

type fakeBridge struct {
	events chan Event
	mu     sync.Mutex
	loads  []string
	loadFn func(context.Context, string, time.Duration, uint64) error
	snap   core.PlaybackSnapshot
	closed int
}

func newFakeBridge() *fakeBridge { return &fakeBridge{events: make(chan Event, 8)} }

func (bridge *fakeBridge) Load(ctx context.Context, mediaURL string, startAt time.Duration, generation uint64) error {
	bridge.mu.Lock()
	bridge.loads = append(bridge.loads, mediaURL)
	loadFn := bridge.loadFn
	bridge.mu.Unlock()
	if loadFn != nil {
		return loadFn(ctx, mediaURL, startAt, generation)
	}
	return ctx.Err()
}

func (bridge *fakeBridge) Events() <-chan Event { return bridge.events }

func (bridge *fakeBridge) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return bridge.snap, nil
}

func (bridge *fakeBridge) Close() error {
	bridge.mu.Lock()
	bridge.closed++
	bridge.mu.Unlock()
	return nil
}

func (bridge *fakeBridge) loadedURLs() []string {
	bridge.mu.Lock()
	defer bridge.mu.Unlock()
	return append([]string(nil), bridge.loads...)
}

type fakeRenderer struct {
	mu     sync.Mutex
	closed int
}

func (*fakeRenderer) Render(int, int) error { return nil }

func (renderer *fakeRenderer) Close() error {
	renderer.mu.Lock()
	renderer.closed++
	renderer.mu.Unlock()
	return nil
}

func testPlayer(proxy *fakeProxy, bridge *fakeBridge, renderer *fakeRenderer) *Player {
	return &Player{
		factory:  func(context.Context) (Bridge, Renderer, error) { return bridge, renderer, nil },
		newProxy: func() (proxyService, error) { return proxy, nil },
	}
}

func testRequest(id string) core.PlayRequest {
	return core.PlayRequest{
		AnimeID: core.AnimeID("anime"), EpisodeID: core.EpisodeID(id),
		Source: core.NewPlaybackSource("https://media.example/"+id+"?token=private", nil),
	}
}

func TestStartLoadsOnlyProxyCapabilityAndRedactsEvents(t *testing.T) {
	proxy := &fakeProxy{}
	bridge := newFakeBridge()
	renderer := &fakeRenderer{}
	session, err := testPlayer(proxy, bridge, renderer).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	urls := bridge.loadedURLs()
	if len(urls) != 1 || !strings.HasPrefix(urls[0], "http://127.0.0.1/media/") {
		t.Fatalf("bridge received non-capability URL: %#v", urls)
	}
	if strings.Contains(urls[0], "media.example") || strings.Contains(urls[0], "private") {
		t.Fatalf("bridge received raw source data: %q", urls[0])
	}
	bridge.events <- Event{Generation: 1, Kind: EventFailed, Err: errors.New("remote token=private")}
	select {
	case event := <-session.Events():
		if event.Kind != core.PlaybackEventFailed || event.Err == nil || strings.Contains(event.Err.Error(), "private") {
			t.Fatalf("failure event was not sanitized: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to forward bridge event")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if caps := proxy.caps(); len(caps) != 1 || caps[0].closed() != 1 {
		t.Fatalf("active capability was not closed: %#v", caps)
	}
}

func TestSwitchClosesOldCapabilityOnlyAfterSuccessfulLoad(t *testing.T) {
	proxy := &fakeProxy{}
	bridge := newFakeBridge()
	session, err := testPlayer(proxy, bridge, &fakeRenderer{}).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	first := proxy.caps()[0]
	if err := session.Load(context.Background(), testRequest("ep2")); err != nil {
		t.Fatal(err)
	}
	caps := proxy.caps()
	if len(caps) != 2 || first.closed() != 1 || caps[1].closed() != 0 {
		t.Fatalf("capability rotation failed: first=%d second=%d", first.closed(), caps[1].closed())
	}
	bridge.mu.Lock()
	bridge.loadFn = func(context.Context, string, time.Duration, uint64) error { return errors.New("load secret") }
	bridge.mu.Unlock()
	if err := session.Load(context.Background(), testRequest("ep3")); !errors.Is(err, ErrPlayerFailed) {
		t.Fatalf("failed switch returned %v", err)
	}
	caps = proxy.caps()
	if len(caps) != 3 || caps[1].closed() != 0 || caps[2].closed() != 1 {
		t.Fatalf("failed switch did not preserve old media capability: second=%d candidate=%d", caps[1].closed(), caps[2].closed())
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if caps[1].closed() != 1 {
		t.Fatal("current media capability was not closed")
	}
}

func TestSwitchIgnoresLateProgressFromPreviousEpisode(t *testing.T) {
	server := &fakeProxy{}
	bridge := newFakeBridge()
	raw, err := testPlayer(server, bridge, &fakeRenderer{}).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if err := raw.Load(context.Background(), testRequest("ep2")); err != nil {
		t.Fatal(err)
	}
	session := raw.(*playbackSession)
	session.forwardEvent(Event{Generation: 1, Kind: EventProgress, Position: 30 * time.Second})
	session.forwardEvent(Event{Generation: 2, Kind: EventProgress, Position: 2 * time.Second})
	select {
	case event := <-raw.Events():
		if event.EpisodeID != "ep2" || event.Position != 2*time.Second {
			t.Fatalf("switched progress = %#v", event)
		}
	default:
		t.Fatal("current episode progress was dropped")
	}
	select {
	case event := <-raw.Events():
		t.Fatalf("stale episode event forwarded: %#v", event)
	default:
	}
}

func TestLoadCancellationRevokesCandidate(t *testing.T) {
	proxy := &fakeProxy{}
	bridge := newFakeBridge()
	session, err := testPlayer(proxy, bridge, &fakeRenderer{}).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	bridge.mu.Lock()
	bridge.loadFn = func(ctx context.Context, _ string, _ time.Duration, _ uint64) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	bridge.mu.Unlock()
	loadCtx, cancel := context.WithCancel(context.Background())
	loadDone := make(chan error, 1)
	go func() { loadDone <- session.Load(loadCtx, testRequest("ep2")) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("switch load did not start")
	}
	cancel()
	select {
	case err := <-loadDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled switch returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled switch did not return")
	}
	if got := proxy.caps(); len(got) != 2 || got[0].closed() != 0 || got[1].closed() != 1 {
		t.Fatalf("canceled switch capability cleanup mismatch: %#v", got)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCallerCancellationAndSnapshotValidation(t *testing.T) {
	proxy := &fakeProxy{}
	bridge := newFakeBridge()
	session, err := testPlayer(proxy, bridge, &fakeRenderer{}).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := session.Load(ctx, testRequest("ep2")); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled load returned %v", err)
	}
	if _, err := session.(core.PlaybackSnapshotter).Snapshot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot returned %v", err)
	}
	bridge.mu.Lock()
	bridge.snap.Position = -time.Second
	bridge.mu.Unlock()
	if _, err := session.(core.PlaybackSnapshotter).Snapshot(context.Background()); !errors.Is(err, ErrPlayerFailed) {
		t.Fatalf("invalid snapshot returned %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseCancelsInFlightLoadAndIsIdempotent(t *testing.T) {
	proxy := &fakeProxy{}
	bridge := newFakeBridge()
	started := make(chan struct{})
	session, err := testPlayer(proxy, bridge, &fakeRenderer{}).Start(context.Background(), testRequest("ep1"))
	if err != nil {
		t.Fatal(err)
	}
	bridge.loadFn = func(ctx context.Context, _ string, _ time.Duration, _ uint64) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}
	loadDone := make(chan error, 1)
	go func() { loadDone <- session.Load(context.Background(), testRequest("ep2")) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("switch bridge load did not start")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-loadDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight load returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("close did not cancel in-flight load")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}
