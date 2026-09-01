package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"animeportable/core"
)

func TestEpisodeMappingsPersistSortAndEnforceIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime-a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime-b"}); err != nil {
		t.Fatal(err)
	}
	firstRef := episodeRef("provider-a", "source-anime", "source-episode-1")
	secondRef := episodeRef("provider-b", "source-anime", "source-episode-1")
	for _, ref := range []core.SourceRef{firstRef.Anime, secondRef.Anime} {
		if err := store.SaveSourceRef(ctx, core.AnimeID("anime-a"), ref); err != nil {
			t.Fatal(err)
		}
	}
	first := mapping("anime-a", "episode-1", firstRef)
	second := mapping("anime-a", "episode-1", secondRef)
	if err := store.SaveEpisodeMapping(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEpisodeMapping(ctx, first); err != nil {
		t.Fatalf("idempotent mapping save: %v", err)
	}
	if err := store.SaveEpisodeMapping(ctx, second); err != nil {
		t.Fatalf("multiple provider refs for one canonical episode: %v", err)
	}
	got, err := store.EpisodeMappings(ctx, core.AnimeID("anime-a"))
	if err != nil {
		t.Fatal(err)
	}
	want := []core.EpisodeMapping{first, second}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mappings = %#v, want %#v", got, want)
	}
	conflictingEpisode := mapping("anime-a", "episode-2", firstRef)
	if err := store.SaveEpisodeMapping(ctx, conflictingEpisode); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("provider episode remap error = %v, want ErrIdentityConflict", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	if got, err := store.EpisodeMappings(ctx, core.AnimeID("anime-a")); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened mappings = %#v, %v, want %#v", got, err, want)
	}
}

func TestEpisodeMappingsValidateReferencesAndReturnSafeEmptyLists(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	ref := episodeRef("provider", "source-anime", "source-episode")
	mappingValue := mapping("anime", "episode", ref)
	if err := store.SaveEpisodeMapping(ctx, mappingValue); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing source ref error = %v, want ErrInvalidInput", err)
	}
	if err := store.SaveSourceRef(ctx, core.AnimeID("anime"), ref.Anime); err != nil {
		t.Fatal(err)
	}
	if got, err := store.EpisodeMappings(ctx, core.AnimeID("missing")); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("missing anime mappings = %#v, %v, want nonnil empty", got, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := store.EpisodeMappings(cancelled, core.AnimeID("anime")); got == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled mappings = %#v, %v", got, err)
	}
	if err := store.SaveEpisodeMapping(cancelled, mappingValue); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled mapping save error = %v", err)
	}
	tooLong := strings.Repeat("x", maxIdentityBytes+1)
	invalid := []core.EpisodeMapping{
		{AnimeID: "", EpisodeID: "episode", Ref: ref},
		{AnimeID: "anime", EpisodeID: "", Ref: ref},
		{AnimeID: "anime", EpisodeID: "episode", Ref: core.EpisodeRef{Anime: core.SourceRef{Provider: "", ID: "id"}, ID: "episode"}},
		{AnimeID: "anime", EpisodeID: "episode", Ref: core.EpisodeRef{Anime: core.SourceRef{Provider: "provider", ID: "source-anime"}, ID: ""}},
		{AnimeID: "anime", EpisodeID: core.EpisodeID(tooLong), Ref: ref},
	}
	for index, item := range invalid {
		if err := store.SaveEpisodeMapping(ctx, item); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("invalid mapping %d error = %v, want ErrInvalidInput", index, err)
		}
	}
}

func TestEpisodeMappingsUpgradeFromInitialSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	initial, err := migrationFS.ReadFile("migrations/0001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	legacy := migrationSource{migrations: []migration{{version: 1, name: "0001_initial.sql", sql: string(initial)}}}
	store, err := open(context.Background(), path, legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	var migrations int
	if err := store.db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&migrations); err != nil {
		t.Fatal(err)
	}
	if migrations != 2 {
		t.Fatalf("upgraded migration count = %d, want 2", migrations)
	}
	if _, err := store.db.Exec("SELECT 1 FROM episode_mappings LIMIT 1"); err != nil {
		t.Fatalf("upgraded mapping table unavailable: %v", err)
	}
}

func TestEpisodeMappingsConcurrentWritesKeepProviderIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	first := openLibraryStore(t, path)
	second := openLibraryStore(t, path)
	defer first.Close()
	defer second.Close()
	ctx := context.Background()
	if err := first.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	ref := episodeRef("provider", "source-anime", "source-episode")
	if err := first.SaveSourceRef(ctx, core.AnimeID("anime"), ref.Anime); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		<-start
		results <- first.SaveEpisodeMapping(ctx, mapping("anime", "episode-1", ref))
	}()
	go func() {
		defer group.Done()
		<-start
		results <- second.SaveEpisodeMapping(ctx, mapping("anime", "episode-2", ref))
	}()
	close(start)
	group.Wait()
	successes := 0
	conflicts := 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			successes++
		case errors.Is(err, ErrIdentityConflict):
			conflicts++
		default:
			t.Fatalf("concurrent mapping error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d, want 1 each", successes, conflicts)
	}
	if got, err := first.EpisodeMappings(ctx, core.AnimeID("anime")); err != nil || len(got) != 1 {
		t.Fatalf("concurrent mappings = %#v, %v, want one row", got, err)
	}
}

func episodeRef(provider, animeID, episodeID string) core.EpisodeRef {
	return core.EpisodeRef{Anime: core.SourceRef{Provider: provider, ID: animeID}, ID: episodeID}
}

func mapping(animeID, episodeID string, ref core.EpisodeRef) core.EpisodeMapping {
	return core.EpisodeMapping{AnimeID: core.AnimeID(animeID), EpisodeID: core.EpisodeID(episodeID), Ref: ref}
}
