package backend

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/core"
)

type advancingSession struct {
	*playbackTestSession
	snapshotMu sync.Mutex
	snapshot   core.PlaybackSnapshot
}

func (session *advancingSession) Snapshot(ctx context.Context) (core.PlaybackSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return core.PlaybackSnapshot{}, err
	}
	session.snapshotMu.Lock()
	defer session.snapshotMu.Unlock()
	return session.snapshot, nil
}

type advancingPlayer struct{ session *advancingSession }

func (player advancingPlayer) Start(ctx context.Context, request core.PlayRequest) (core.PlaybackSession, error) {
	if err := player.session.Load(ctx, request); err != nil {
		return nil, err
	}
	return player.session, nil
}

func TestStopPlaybackPersistsFinalPositionAndResumesAfterRestart(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "resume.db")
	store, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	anime := core.Anime{ID: "anime", Title: "Resume test"}
	ref := core.EpisodeRef{Anime: core.SourceRef{Provider: "anime1", ID: "show"}, ID: "episode"}
	for _, err := range []error{
		store.SaveAnime(ctx, anime),
		store.SaveSourceRef(ctx, anime.ID, ref.Anime),
		store.SaveEpisodeMapping(ctx, core.EpisodeMapping{AnimeID: anime.ID, EpisodeID: "episode", Ref: ref}),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := store.SavePlaybackCheckpoint(ctx, core.HistoryEntry{Progress: core.PlaybackProgress{
		AnimeID: anime.ID, EpisodeID: "episode", Position: 10 * time.Second, Duration: time.Minute, UpdatedAt: oldTime,
	}, LastPlayedAt: oldTime}); err != nil {
		t.Fatal(err)
	}
	raw := &advancingSession{playbackTestSession: newPlaybackTestSession(), snapshot: core.PlaybackSnapshot{Position: 10 * time.Second, Duration: time.Minute}}
	service := newWithDependencies(dependencies{store: store,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: anime.Title}},
		newPlayer: func() (core.Player, error) { return advancingPlayer{raw}, nil },
	})
	t.Cleanup(func() { _ = service.Close() })
	if err := service.Play(ctx, PlayRequest{AnimeID: "anime", EpisodeID: "episode", StartAt: 10000}); err != nil {
		t.Fatal(err)
	}
	raw.snapshotMu.Lock()
	raw.snapshot.Position = 42750 * time.Millisecond
	raw.snapshotMu.Unlock()
	if err := service.StopPlayback(ctx); err != nil {
		t.Fatal(err)
	}
	history, err := service.History(ctx)
	if err != nil || len(history) != 1 || history[0].Position != 42750 || history[0].UpdatedAt == oldTime.Format(time.RFC3339Nano) {
		t.Fatalf("final checkpoint = %#v, error=%v", history, err)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(ctx, database)
	if err != nil {
		t.Fatal(err)
	}
	player := &playbackTestPlayer{}
	restarted := newWithDependencies(dependencies{store: reopened,
		source:    &persistentSource{item: core.SourceAnime{Ref: ref.Anime, Title: anime.Title}},
		newPlayer: func() (core.Player, error) { return player, nil },
	})
	t.Cleanup(func() { _ = restarted.Close() })
	history, err = restarted.History(ctx)
	if err != nil || len(history) != 1 || history[0].Position != 42750 {
		t.Fatalf("restarted history=%#v, error=%v", history, err)
	}
	item := history[0]
	if err := restarted.Play(ctx, PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: item.Position}); err != nil {
		t.Fatal(err)
	}
	player.session.mu.Lock()
	startAt := player.session.request.StartAt
	player.session.mu.Unlock()
	if startAt != 42750*time.Millisecond {
		t.Fatalf("resume start=%v", startAt)
	}
}
