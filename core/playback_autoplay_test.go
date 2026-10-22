// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

const autoplayEpisodeLength = 24 * time.Minute

// finishedProgress returns the last saved checkpoint of episodeID, if any.
func finishedProgress(store *trackingStore, episodeID EpisodeID) (PlaybackProgress, bool) {
	entries := store.entries()
	for index := len(entries) - 1; index >= 0; index-- {
		if entries[index].Progress.EpisodeID == episodeID {
			return entries[index].Progress, true
		}
	}
	return PlaybackProgress{}, false
}

func requireFinishedEpisodeCompleted(t *testing.T, store *trackingStore) {
	t.Helper()
	requireTrackingEventually(t, func() bool {
		progress, ok := finishedProgress(store, "episode-1")
		return ok && progress.Completed
	})
	progress, _ := finishedProgress(store, "episode-1")
	if progress.Position != autoplayEpisodeLength || progress.Duration != autoplayEpisodeLength {
		t.Fatalf("finished episode saved at %s of %s, want the end", progress.Position, progress.Duration)
	}
}

func TestSwitchAfterEndedSavesFinishedEpisodeCompleted(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/two", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	raw.emit(PlaybackEvent{AnimeID: "anime", EpisodeID: "episode-1", Kind: PlaybackEventEnded, Position: autoplayEpisodeLength, Duration: autoplayEpisodeLength})
	requireTrackingEventually(t, func() bool {
		progress, ok := finishedProgress(store, "episode-1")
		return ok && progress.Completed
	})
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0, false); err != nil {
		t.Fatal(err)
	}
	requireFinishedEpisodeCompleted(t, store)
	if got := raw.loadCount(); got != 1 {
		t.Fatalf("loads = %d, want the next episode loaded once", got)
	}
	if got := raw.loads[0].EpisodeID; got != "episode-2" {
		t.Fatalf("loaded episode = %q, want episode-2", got)
	}
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchBeforeEndedEventStillSavesFinishedEpisodeCompleted(t *testing.T) {
	raw := newTrackingRawSession()
	store := newTrackingStore()
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/two", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	// The engine reports position = duration after a normal end; no Ended event was seen.
	raw.setSnapshot(PlaybackSnapshot{Position: autoplayEpisodeLength, Duration: autoplayEpisodeLength})
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0, false); err != nil {
		t.Fatal(err)
	}
	requireFinishedEpisodeCompleted(t, store)
	if got := raw.loadCount(); got != 1 {
		t.Fatalf("loads = %d, want the next episode loaded once", got)
	}
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSwitchWithFailingResolveKeepsSessionUsable(t *testing.T) {
	raw := newTrackingRawSession()
	raw.setSnapshot(PlaybackSnapshot{Position: autoplayEpisodeLength, Duration: autoplayEpisodeLength})
	store := newTrackingStore()
	var resolves atomic.Int32
	resolveErr := errors.New("source unavailable")
	source := &fakeSource{resolved: NewPlaybackSource("https://media.example/two", nil)}
	app := newTrackingTestApp(source, raw, store, newManualPlaybackTicker())
	started, err := app.PlayEpisode(context.Background(), "anime", "episode-1", EpisodeRef{ID: "one"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	source.resolve = func(context.Context, EpisodeRef) (PlaybackSource, error) {
		resolves.Add(1)
		return PlaybackSource{}, resolveErr
	}
	err = app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0, false)
	if !errors.Is(err, resolveErr) {
		t.Fatalf("switch error = %v, want the resolve error", err)
	}
	if got := resolves.Load(); got != 1 {
		t.Fatalf("resolve calls = %d, want 1", got)
	}
	if got := raw.loadCount(); got != 0 {
		t.Fatalf("loads = %d, want none", got)
	}
	if got := raw.closes(); got != 0 {
		t.Fatalf("raw closes = %d, want the session left open", got)
	}

	// The same session still switches once the source works again.
	source.resolve = nil
	if err := app.SwitchEpisode(context.Background(), started, "anime", "episode-2", EpisodeRef{ID: "two"}, 0, false); err != nil {
		t.Fatalf("switch after failure = %v", err)
	}
	if got := raw.loadCount(); got != 1 {
		t.Fatalf("loads after retry = %d, want 1", got)
	}
	if err := started.Close(); err != nil {
		t.Fatal(err)
	}
}
