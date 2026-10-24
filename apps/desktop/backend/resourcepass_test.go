//go:build resourcepass

// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/adapters/player/libmpv"
	"animeportable/core"
	"animeportable/internal/resourcepass"
)

// resourceSource is an Anime1 stand-in with one anime of three episodes. Search answers at once
// for "fast" and otherwise waits until its caller gives up.
type resourceSource struct {
	searching chan struct{}
}

var resourceAnime = core.SourceRef{Provider: "anime1", ID: "show"}

func (*resourceSource) Catalog(context.Context) ([]core.SourceAnime, error) {
	return []core.SourceAnime{{Ref: resourceAnime, Title: "Show"}}, nil
}

func (source *resourceSource) Search(ctx context.Context, query string) ([]core.SourceAnime, error) {
	if query == "fast" {
		return []core.SourceAnime{{Ref: resourceAnime, Title: "Show"}}, nil
	}
	source.searching <- struct{}{}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (*resourceSource) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	episodes := make([]core.SourceEpisode, 3)
	for index, number := range []string{"1", "2", "3"} {
		episodes[index] = core.SourceEpisode{Ref: core.EpisodeRef{Anime: resourceAnime, ID: "episode-" + number}, Number: number}
	}
	return episodes, nil
}

func (*resourceSource) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.NewPlaybackSource("https://media.example/video.mp4", nil), nil
}

// resourceBridge stands in for the libmpv engine: it accepts every load and reports what it is
// told to, so the real player adapter, playback proxy, core tracking and SQLite run around it.
type resourceBridge struct {
	events     chan libmpv.Event
	mu         sync.Mutex
	generation uint64
	closeOnce  sync.Once
}

func (bridge *resourceBridge) Load(_ context.Context, _ string, _ time.Duration, _ bool, generation uint64) error {
	bridge.mu.Lock()
	bridge.generation = generation
	bridge.mu.Unlock()
	return nil
}

func (bridge *resourceBridge) Events() <-chan libmpv.Event { return bridge.events }

func (*resourceBridge) Snapshot(context.Context) (core.PlaybackSnapshot, error) {
	return core.PlaybackSnapshot{Position: 23 * time.Minute, Duration: 24 * time.Minute}, nil
}

func (bridge *resourceBridge) Close() error {
	bridge.closeOnce.Do(func() { close(bridge.events) })
	return nil
}

// end reports that the current file played to its end, as mpv does before autoplay.
func (bridge *resourceBridge) end(t *testing.T) {
	bridge.mu.Lock()
	generation := bridge.generation
	bridge.mu.Unlock()
	select {
	case bridge.events <- libmpv.Event{Generation: generation, Kind: libmpv.EventEnded, Position: 24 * time.Minute, Duration: 24 * time.Minute}:
	case <-time.After(5 * time.Second):
		t.Fatal("the session stopped reading player events")
	}
}

type resourceRenderer struct{}

func (resourceRenderer) Render(int, int) error { return nil }
func (resourceRenderer) Close() error          { return nil }

type resourceFixture struct {
	service  *Service
	source   *resourceSource
	animeID  string
	episodes []string
	mu       sync.Mutex
	bridge   *resourceBridge
}

func newResourceFixture(t *testing.T) *resourceFixture {
	t.Helper()
	ctx := context.Background()
	store, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "anime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := &resourceFixture{source: &resourceSource{searching: make(chan struct{})}}
	player := libmpv.NewPlayer(func(context.Context) (libmpv.Bridge, libmpv.Renderer, error) {
		// Unbuffered, so an end of file has been taken in by the session before end returns.
		bridge := &resourceBridge{events: make(chan libmpv.Event)}
		fixture.mu.Lock()
		fixture.bridge = bridge
		fixture.mu.Unlock()
		return bridge, resourceRenderer{}, nil
	}, nil, nil, nil)
	fixture.service = newWithDependencies(dependencies{store: store, source: fixture.source, newPlayer: func() (core.Player, error) { return player, nil }})
	catalog, err := fixture.service.Catalog(ctx)
	if err != nil || len(catalog) != 1 {
		t.Fatalf("catalog = %v, %v", catalog, err)
	}
	fixture.animeID = catalog[0].ID
	episodes, err := fixture.service.Episodes(ctx, fixture.animeID)
	if err != nil || len(episodes) != 3 {
		t.Fatalf("episodes = %v, %v", episodes, err)
	}
	for _, episode := range episodes {
		fixture.episodes = append(fixture.episodes, episode.ID)
	}
	return fixture
}

func (fixture *resourceFixture) play(t *testing.T, episode int, paused bool) {
	t.Helper()
	request := PlayRequest{AnimeID: fixture.animeID, EpisodeID: fixture.episodes[episode%len(fixture.episodes)], Paused: paused}
	if err := fixture.service.Play(context.Background(), request); err != nil {
		t.Fatal(err)
	}
}

func (fixture *resourceFixture) currentBridge() *resourceBridge {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.bridge
}

func (fixture *resourceFixture) close(t *testing.T) {
	t.Helper()
	if err := fixture.service.Close(); err != nil {
		t.Fatal(err)
	}
}

// Opening the player starts a playback proxy, a proxy session and core progress tracking;
// closing it must end all of them and save the position.
func TestResourcePassPlayerOpenClose(t *testing.T) {
	var fixture *resourceFixture
	resourcepass.Run(t, "player-open-close", 0, func() { fixture = newResourceFixture(t) }, func(i int) {
		fixture.play(t, i, false)
		if err := fixture.service.StopPlayback(context.Background()); err != nil {
			t.Fatal(err)
		}
	}, func() { fixture.close(t) })
}

// Switching episodes keeps the player and replaces the proxy session. Every other switch
// follows an end of file, as autoplay does, and some of those wait paused (自動播放 off).
func TestResourcePassEpisodeSwitches(t *testing.T) {
	var fixture *resourceFixture
	resourcepass.Run(t, "episode-switches", 0, func() {
		fixture = newResourceFixture(t)
		fixture.play(t, 0, false)
	}, func(i int) {
		autoplay := i%2 == 1
		if autoplay {
			fixture.currentBridge().end(t)
		}
		fixture.play(t, i+1, autoplay && i%4 == 3)
	}, func() {
		if err := fixture.service.StopPlayback(context.Background()); err != nil {
			t.Fatal(err)
		}
		fixture.close(t)
	})
}

// A search is cancelled when the user types again or leaves: before the source answers, before
// the search starts, and after it already answered.
func TestResourcePassCancelledSearches(t *testing.T) {
	var fixture *resourceFixture
	resourcepass.Run(t, "cancelled-searches", 0, func() { fixture = newResourceFixture(t) }, func(int) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			_, err := fixture.service.Search(ctx, "slow")
			done <- err
		}()
		select {
		case <-fixture.source.searching:
		case <-time.After(5 * time.Second):
			t.Fatal("the search never reached the source")
		}
		cancel()
		if err := <-done; err == nil {
			t.Fatal("a cancelled search succeeded")
		}

		ctx, cancel = context.WithCancel(context.Background())
		cancel()
		if _, err := fixture.service.Search(ctx, "fast"); err == nil {
			t.Fatal("a search cancelled before it started succeeded")
		}

		ctx, cancel = context.WithCancel(context.Background())
		if _, err := fixture.service.Search(ctx, "fast"); err != nil {
			t.Fatal(err)
		}
		cancel()
	}, func() { fixture.close(t) })
}
