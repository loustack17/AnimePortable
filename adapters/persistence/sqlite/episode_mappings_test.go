// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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
	if migrations != 5 {
		t.Fatalf("upgraded migration count = %d, want 5", migrations)
	}
	if _, err := store.db.Exec("SELECT 1 FROM episode_mappings LIMIT 1"); err != nil {
		t.Fatalf("upgraded mapping table unavailable: %v", err)
	}
}

func TestEpisodeMappingNumberIsStoredFilledInAndNeverErased(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	ref := episodeRef("provider", "source-anime", "source-episode")
	if err := store.SaveSourceRef(ctx, core.AnimeID("anime"), ref.Anime); err != nil {
		t.Fatal(err)
	}
	unnumbered := mapping("anime", "episode", ref)
	if err := store.SaveEpisodeMapping(ctx, unnumbered); err != nil {
		t.Fatal(err)
	}
	numbered := unnumbered
	numbered.Number = "12"
	if err := store.SaveEpisodeMapping(ctx, numbered); err != nil {
		t.Fatalf("fill in number: %v", err)
	}
	if err := store.SaveEpisodeMapping(ctx, unnumbered); err != nil {
		t.Fatalf("save without number: %v", err)
	}
	tooLong := numbered
	tooLong.Number = strings.Repeat("9", maxEpisodeNumberBytes+1)
	if err := store.SaveEpisodeMapping(ctx, tooLong); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("too long number error = %v, want ErrInvalidInput", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	if got, err := store.EpisodeMappings(ctx, core.AnimeID("anime")); err != nil || !reflect.DeepEqual(got, []core.EpisodeMapping{numbered}) {
		t.Fatalf("mappings = %#v, %v, want %#v", got, err, []core.EpisodeMapping{numbered})
	}
}

func TestEpisodeNumberMigrationKeepsExistingMappings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql"} {
		sql, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		previous = append(previous, migration{version: version + 1, name: name, sql: string(sql)})
	}
	legacy, err := open(context.Background(), path, migrationSource{migrations: previous})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO anime (id, title, native_title, description) VALUES ('anime', '', '', '')`,
		`INSERT INTO source_refs (anime_id, provider, external_id) VALUES ('anime', 'provider', 'source-anime')`,
		`INSERT INTO episode_mappings (anime_id, episode_id, provider, provider_anime_id, provider_episode_id) VALUES ('anime', 'episode', 'provider', 'source-anime', 'source-episode')`,
	}
	for _, statement := range statements {
		if _, err := legacy.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	store := openLibraryStore(t, path)
	defer store.Close()
	want := []core.EpisodeMapping{mapping("anime", "episode", episodeRef("provider", "source-anime", "source-episode"))}
	if got, err := store.EpisodeMappings(context.Background(), core.AnimeID("anime")); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("upgraded mappings = %#v, %v, want %#v", got, err, want)
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

func TestEpisodeFirstSeenIsKeptAndFollowTimeIsRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	ref := episodeRef("provider", "source-anime", "source-episode")
	if err := store.SaveSourceRef(ctx, core.AnimeID("anime"), ref.Anime); err != nil {
		t.Fatal(err)
	}
	first := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	saved := mapping("anime", "episode", ref)
	saved.Number, saved.FirstSeenAt = "1", first
	if err := store.SaveEpisodeMapping(ctx, saved); err != nil {
		t.Fatal(err)
	}
	again := saved
	again.Number, again.FirstSeenAt = "01", first.Add(48*time.Hour)
	if err := store.SaveEpisodeMapping(ctx, again); err != nil {
		t.Fatal(err)
	}
	got, err := store.EpisodeMappings(ctx, core.AnimeID("anime"))
	if err != nil || len(got) != 1 || !got[0].FirstSeenAt.Equal(first) || got[0].Number != "01" {
		t.Fatalf("mappings = %#v, %v, want first seen %v kept and number updated", got, err, first)
	}

	before := time.Now().Add(-time.Second)
	if err := store.SetFollowing(ctx, core.AnimeID("anime"), true); err != nil {
		t.Fatal(err)
	}
	followed, err := store.Following(ctx)
	if err != nil || len(followed) != 1 || followed[0].FollowedAt.Before(before) {
		t.Fatalf("following = %#v, %v, want a follow time after %v", followed, err, before)
	}
	original := followed[0].FollowedAt
	if err := store.SetFollowing(ctx, core.AnimeID("anime"), true); err != nil {
		t.Fatal(err)
	}
	if again, err := store.Following(ctx); err != nil || len(again) != 1 || !again[0].FollowedAt.Equal(original) {
		t.Fatalf("following again = %#v, %v, want the original time %v", again, err, original)
	}
}

func TestRecentUpdateMigrationStampsExistingFollows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	var previous []migration
	for version, name := range []string{"0001_initial.sql", "0002_episode_mappings.sql", "0003_drop_settings_mpv_path.sql", "0004_episode_number.sql"} {
		sql, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		previous = append(previous, migration{version: version + 1, name: name, sql: string(sql)})
	}
	legacy, err := open(context.Background(), path, migrationSource{migrations: previous})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`INSERT INTO anime (id, title, native_title, description) VALUES ('anime', '', '', '')`,
		`INSERT INTO following (anime_id) VALUES ('anime')`,
	} {
		if _, err := legacy.db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Minute)
	store := openLibraryStore(t, path)
	defer store.Close()
	followed, err := store.Following(context.Background())
	if err != nil || len(followed) != 1 || followed[0].FollowedAt.Before(before) {
		t.Fatalf("following after upgrade = %#v, %v, want the migration time", followed, err)
	}
}
