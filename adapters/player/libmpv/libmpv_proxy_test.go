// SPDX-License-Identifier: MPL-2.0

package libmpv

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"animeportable/core"
)

// proxySessionLimit is the real proxy's default session limit (adapters/playback/proxy).
const proxySessionLimit = 64

// TestEpisodeSwitchesDoNotLeakProxySessions uses the real proxy: a switch that left its old
// proxy session open would exhaust the limit well before the last of 200 switches.
func TestEpisodeSwitchesDoNotLeakProxySessions(t *testing.T) {
	baseline := runtime.NumGoroutine()
	bridge := newFakeBridge()
	player := NewPlayer(func(context.Context) (Bridge, Renderer, error) { return bridge, &fakeRenderer{}, nil }, nil)
	ctx := context.Background()
	// The proxy accepts any https source; nothing is fetched until the bridge opens the URL.
	request := func(index int) core.PlayRequest {
		return core.PlayRequest{
			AnimeID: "anime", EpisodeID: core.EpisodeID(fmt.Sprint("episode-", index)),
			Source: core.NewPlaybackSource(fmt.Sprintf("https://media.example/%d.mp4", index), nil),
		}
	}
	session, err := player.Start(ctx, request(0))
	if err != nil {
		t.Fatal(err)
	}
	const switches = 200
	if switches <= 3*proxySessionLimit {
		t.Fatalf("%d switches do not clearly exceed the %d-session limit", switches, proxySessionLimit)
	}
	for index := 1; index <= switches; index++ {
		if err := session.Load(ctx, request(index)); err != nil {
			t.Fatalf("switch %d failed: %v", index, err)
		}
	}
	if got := len(bridge.loadedURLs()); got != switches+1 {
		t.Fatalf("bridge loads = %d, want %d", got, switches+1)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > baseline {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines = %d after Close, baseline %d", runtime.NumGoroutine(), baseline)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
