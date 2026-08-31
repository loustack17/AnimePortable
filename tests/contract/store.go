package contract

import (
	"context"
	"errors"
	"testing"
	"time"

	"animeportable/core"
)

type StoreSuite struct {
	New func(t *testing.T) core.Store
}

func RunStore(t *testing.T, suite StoreSuite) {
	t.Helper()
	if suite.New == nil {
		t.Fatal("store factory is nil")
	}
	animeA := core.Anime{ID: "anime-a", Title: "Anime A"}
	animeB := core.Anime{ID: "anime-b", Title: "Anime B"}
	episodeA := core.EpisodeID("episode-a")
	episodeB := core.EpisodeID("episode-b")
	ctx := context.Background()
	t.Run("anime", func(t *testing.T) {
		store := suite.New(t)
		if _, err := store.Anime(ctx, animeA.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("empty anime error = %v", err)
		}
		if err := store.SaveAnime(ctx, animeA); err != nil {
			t.Fatal(err)
		}
		actual, err := store.Anime(ctx, animeA.ID)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, equal(actual, animeA))
		list, err := store.ListAnime(ctx)
		if err != nil || !containsAnime(list, animeA) {
			t.Fatalf("anime list = %#v, error = %v", list, err)
		}
	})
	t.Run("source refs", func(t *testing.T) {
		store := suite.New(t)
		seedAnime(t, ctx, store, animeA)
		ref := core.SourceRef{Provider: "source", ID: "external-a"}
		if err := store.SaveSourceRef(ctx, animeA.ID, ref); err != nil {
			t.Fatal(err)
		}
		refs, err := store.SourceRefs(ctx, animeA.ID)
		if err != nil || !containsSourceRef(refs, ref) {
			t.Fatalf("source refs = %#v, error = %v", refs, err)
		}
	})
	t.Run("metadata", func(t *testing.T) {
		store := suite.New(t)
		seedAnime(t, ctx, store, animeA)
		if _, err := store.Metadata(ctx, animeA.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("empty metadata error = %v", err)
		}
		metadata := core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: "external-a"}, Title: "Anime A"}
		if err := store.SaveMetadata(ctx, animeA.ID, metadata); err != nil {
			t.Fatal(err)
		}
		actual, err := store.Metadata(ctx, animeA.ID)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, equal(actual, metadata))
	})
	t.Run("following", func(t *testing.T) {
		store := suite.New(t)
		seedAnime(t, ctx, store, animeA)
		seedAnime(t, ctx, store, animeB)
		if err := store.SetFollowing(ctx, animeA.ID, true); err != nil {
			t.Fatal(err)
		}
		if err := store.SetFollowing(ctx, animeB.ID, true); err != nil {
			t.Fatal(err)
		}
		following, err := store.Following(ctx)
		if err != nil || !containsAnimeID(following, animeA.ID) || !containsAnimeID(following, animeB.ID) {
			t.Fatalf("following = %#v, error = %v", following, err)
		}
		if err := store.SetFollowing(ctx, animeA.ID, false); err != nil {
			t.Fatal(err)
		}
		following, err = store.Following(ctx)
		if err != nil || containsAnimeID(following, animeA.ID) || !containsAnimeID(following, animeB.ID) {
			t.Fatalf("following = %#v, error = %v", following, err)
		}
	})
	t.Run("history", func(t *testing.T) {
		store := suite.New(t)
		seedAnime(t, ctx, store, animeA)
		seedAnime(t, ctx, store, animeB)
		now := time.Now().UTC()
		entryA := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: animeA.ID, EpisodeID: episodeA}, LastPlayedAt: now}
		entryB := core.HistoryEntry{Progress: core.PlaybackProgress{AnimeID: animeB.ID, EpisodeID: episodeB}, LastPlayedAt: now.Add(time.Second)}
		if err := store.AddHistory(ctx, entryA); err != nil {
			t.Fatal(err)
		}
		if err := store.AddHistory(ctx, entryB); err != nil {
			t.Fatal(err)
		}
		history, err := store.History(ctx)
		if err != nil || !containsHistory(history, animeA.ID) || !containsHistory(history, animeB.ID) {
			t.Fatalf("history = %#v, error = %v", history, err)
		}
		if err := store.RemoveHistory(ctx, animeA.ID); err != nil {
			t.Fatal(err)
		}
		history, err = store.History(ctx)
		if err != nil || containsHistory(history, animeA.ID) || !containsHistory(history, animeB.ID) {
			t.Fatalf("history = %#v, error = %v", history, err)
		}
	})
	t.Run("progress", func(t *testing.T) {
		store := suite.New(t)
		seedAnime(t, ctx, store, animeA)
		if _, err := store.Progress(ctx, animeA.ID, episodeA); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("empty progress error = %v", err)
		}
		progress := core.PlaybackProgress{AnimeID: animeA.ID, EpisodeID: episodeA, Position: time.Minute, Duration: 24 * time.Minute}
		if err := store.SaveProgress(ctx, progress); err != nil {
			t.Fatal(err)
		}
		actual, err := store.Progress(ctx, animeA.ID, episodeA)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, equal(actual, progress))
	})
	t.Run("settings", func(t *testing.T) {
		store := suite.New(t)
		actual, err := store.Settings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, equal(actual, core.DefaultSettings()))
		settings := core.DefaultSettings()
		settings.Language = core.LanguageTraditionalChinese
		if err := store.SaveSettings(ctx, settings); err != nil {
			t.Fatal(err)
		}
		actual, err = store.Settings(ctx)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, equal(actual, settings))
	})
}

func seedAnime(t *testing.T, ctx context.Context, store core.Store, anime core.Anime) {
	t.Helper()
	if err := store.SaveAnime(ctx, anime); err != nil {
		t.Fatal(err)
	}
}

func containsAnime(items []core.Anime, expected core.Anime) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func containsSourceRef(items []core.SourceRef, expected core.SourceRef) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func containsAnimeID(items []core.AnimeID, expected core.AnimeID) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func containsHistory(items []core.HistoryEntry, animeID core.AnimeID) bool {
	for _, item := range items {
		if item.Progress.AnimeID == animeID {
			return true
		}
	}
	return false
}
