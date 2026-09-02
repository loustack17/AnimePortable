// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"animeportable/core"
)

func TestLibraryStatePersistsAcrossAnimeUpdatesAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	store := openLibraryStore(t, path)
	anime := core.Anime{ID: "anime-a", Title: "Before", NativeTitle: "原題", Description: "description"}
	ref := core.SourceRef{Provider: "source", ID: "external-a"}
	metadata := core.AnimeMetadata{
		Ref:          core.MetadataRef{Provider: "metadata", ID: "external-a"},
		Title:        "Metadata",
		NativeTitle:  "原題",
		Description:  "description",
		CoverURL:     "https://example.test/cover.jpg",
		Season:       "spring",
		Year:         2026,
		Studio:       "studio",
		EpisodeCount: 12,
	}
	ctx := context.Background()
	for _, save := range []func() error{
		func() error { return store.SaveAnime(ctx, anime) },
		func() error { return store.SaveSourceRef(ctx, anime.ID, ref) },
		func() error { return store.SaveMetadata(ctx, anime.ID, metadata) },
		func() error { return store.SetFollowing(ctx, anime.ID, true) },
	} {
		if err := save(); err != nil {
			t.Fatal(err)
		}
	}
	anime.Title = "After"
	if err := store.SaveAnime(ctx, anime); err != nil {
		t.Fatal(err)
	}
	assertLibraryState(t, store, anime, ref, metadata)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLibraryStore(t, path)
	defer store.Close()
	assertLibraryState(t, store, anime, ref, metadata)
}

func TestLibraryIdentityAndForeignKeyGuards(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	a := core.Anime{ID: "anime-a"}
	b := core.Anime{ID: "anime-b"}
	for _, anime := range []core.Anime{a, b} {
		if err := store.SaveAnime(ctx, anime); err != nil {
			t.Fatal(err)
		}
	}
	ref := core.SourceRef{Provider: "source", ID: "identity"}
	if err := store.SaveSourceRef(ctx, a.ID, ref); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceRef(ctx, a.ID, ref); err != nil {
		t.Fatalf("idempotent source save: %v", err)
	}
	if err := store.SaveSourceRef(ctx, b.ID, ref); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("source rebind error = %v, want ErrIdentityConflict", err)
	}
	metadata := core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: "identity"}}
	if err := store.SaveMetadata(ctx, a.ID, metadata); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveMetadata(ctx, a.ID, metadata); err != nil {
		t.Fatalf("idempotent metadata save: %v", err)
	}
	if err := store.SaveMetadata(ctx, b.ID, metadata); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("metadata rebind error = %v, want ErrIdentityConflict", err)
	}
	missing := core.AnimeID("missing")
	for name, save := range map[string]func() error{
		"source": func() error { return store.SaveSourceRef(ctx, missing, core.SourceRef{Provider: "new", ID: "source"}) },
		"metadata": func() error {
			return store.SaveMetadata(ctx, missing, core.AnimeMetadata{Ref: core.MetadataRef{Provider: "new", ID: "metadata"}})
		},
		"following": func() error { return store.SetFollowing(ctx, missing, true) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := save(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("unknown anime error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLibraryValidationAndParameterizedInput(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	injection := "anime'); DROP TABLE anime; --"
	anime := core.Anime{ID: core.AnimeID(injection), Title: injection, NativeTitle: injection, Description: injection}
	if err := store.SaveAnime(ctx, anime); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Anime(ctx, anime.ID); err != nil || got != anime {
		t.Fatalf("injection-like anime = %#v, %v", got, err)
	}
	if _, err := store.ListAnime(ctx); err != nil {
		t.Fatalf("anime table was not preserved: %v", err)
	}
	invalidUTF8 := string([]byte{0xff})
	tooLarge := string(make([]byte, maxIdentityBytes+1))
	for name, save := range map[string]func() error{
		"invalid UTF-8": func() error { return store.SaveAnime(ctx, core.Anime{ID: "valid", Title: invalidUTF8}) },
		"oversize":      func() error { return store.SaveAnime(ctx, core.Anime{ID: core.AnimeID(tooLarge)}) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := save(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("validation error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLibraryReferenceValidationBounds(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	if err := store.SaveAnime(ctx, core.Anime{ID: "anime"}); err != nil {
		t.Fatal(err)
	}
	tooLargeIdentity := strings.Repeat("x", maxIdentityBytes+1)
	tooLargeTitle := strings.Repeat("x", maxTitleBytes+1)
	tooLargeDescription := strings.Repeat("x", maxDescriptionBytes+1)
	tooLargeCoverURL := strings.Repeat("x", maxCoverURLBytes+1)
	tooLargeSeason := strings.Repeat("x", maxSeasonBytes+1)
	for name, save := range map[string]func() error{
		"source empty provider": func() error { return store.SaveSourceRef(ctx, "anime", core.SourceRef{ID: "id"}) },
		"source empty ID":       func() error { return store.SaveSourceRef(ctx, "anime", core.SourceRef{Provider: "provider"}) },
		"source provider bounds": func() error {
			return store.SaveSourceRef(ctx, "anime", core.SourceRef{Provider: tooLargeIdentity, ID: "id"})
		},
		"source ID bounds": func() error {
			return store.SaveSourceRef(ctx, "anime", core.SourceRef{Provider: "provider", ID: tooLargeIdentity})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := save(); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
	base := core.AnimeMetadata{Ref: core.MetadataRef{Provider: "provider", ID: "id"}}
	for name, mutate := range map[string]func(*core.AnimeMetadata){
		"empty provider":      func(metadata *core.AnimeMetadata) { metadata.Ref.Provider = "" },
		"empty ID":            func(metadata *core.AnimeMetadata) { metadata.Ref.ID = "" },
		"provider bounds":     func(metadata *core.AnimeMetadata) { metadata.Ref.Provider = tooLargeIdentity },
		"ID bounds":           func(metadata *core.AnimeMetadata) { metadata.Ref.ID = tooLargeIdentity },
		"title bounds":        func(metadata *core.AnimeMetadata) { metadata.Title = tooLargeTitle },
		"native title bounds": func(metadata *core.AnimeMetadata) { metadata.NativeTitle = tooLargeTitle },
		"description bounds":  func(metadata *core.AnimeMetadata) { metadata.Description = tooLargeDescription },
		"cover URL bounds":    func(metadata *core.AnimeMetadata) { metadata.CoverURL = tooLargeCoverURL },
		"season bounds":       func(metadata *core.AnimeMetadata) { metadata.Season = tooLargeSeason },
		"studio bounds":       func(metadata *core.AnimeMetadata) { metadata.Studio = tooLargeIdentity },
		"negative year":       func(metadata *core.AnimeMetadata) { metadata.Year = -1 },
		"year bounds":         func(metadata *core.AnimeMetadata) { metadata.Year = 10000 },
		"negative episode count": func(metadata *core.AnimeMetadata) {
			metadata.EpisodeCount = -1
		},
	} {
		t.Run("metadata "+name, func(t *testing.T) {
			metadata := base
			mutate(&metadata)
			if err := store.SaveMetadata(ctx, "anime", metadata); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLibraryIdentityWritesAreAtomic(t *testing.T) {
	for _, handles := range []struct {
		name     string
		separate bool
	}{
		{name: "same handle"},
		{name: "separate handles", separate: true},
	} {
		t.Run(handles.name, func(t *testing.T) {
			t.Run("source ref", func(t *testing.T) {
				exerciseIdentityWrite(t, handles.separate, func(store *Store, id core.AnimeID) error {
					return store.SaveSourceRef(context.Background(), id, core.SourceRef{Provider: "source", ID: "shared"})
				})
			})
			t.Run("metadata", func(t *testing.T) {
				exerciseIdentityWrite(t, handles.separate, func(store *Store, id core.AnimeID) error {
					return store.SaveMetadata(context.Background(), id, core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: "shared"}})
				})
			})
		})
	}
}

func TestMetadataIdentityReleaseNeverLeaksStorageError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	first := openLibraryStore(t, path)
	defer first.Close()
	second := openLibraryStore(t, path)
	defer second.Close()
	ctx := context.Background()
	for _, id := range []core.AnimeID{"anime-a", "anime-b"} {
		if err := first.SaveAnime(ctx, core.Anime{ID: id}); err != nil {
			t.Fatal(err)
		}
	}

	for iteration := range 20 {
		shared := core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: "shared"}}
		if err := first.SaveMetadata(ctx, "anime-a", shared); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-start
			results <- first.SaveMetadata(ctx, "anime-a", core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: fmt.Sprintf("released-a-%d", iteration)}})
		}()
		go func() {
			<-start
			results <- second.SaveMetadata(ctx, "anime-b", shared)
		}()
		close(start)
		for range 2 {
			err := <-results
			if err != nil && !errors.Is(err, ErrIdentityConflict) {
				t.Fatalf("iteration %d returned %v, want nil or ErrIdentityConflict", iteration, err)
			}
		}
		if err := second.SaveMetadata(ctx, "anime-b", core.AnimeMetadata{Ref: core.MetadataRef{Provider: "metadata", ID: fmt.Sprintf("released-b-%d", iteration)}}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLibraryEmptyListsAreNonNil(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	for name, list := range map[string]func() (any, error){
		"anime": func() (any, error) { return store.ListAnime(ctx) },
		"source refs": func() (any, error) {
			return store.SourceRefs(ctx, "missing")
		},
		"following": func() (any, error) { return store.Following(ctx) },
	} {
		t.Run(name, func(t *testing.T) {
			items, err := list()
			if err != nil {
				t.Fatal(err)
			}
			if reflect.ValueOf(items).IsNil() || reflect.ValueOf(items).Len() != 0 {
				t.Fatalf("items = %#v, want nonnil empty list", items)
			}
		})
	}
}

func TestLibraryListsAreDeterministicNonNilAndHonorContext(t *testing.T) {
	store := openLibraryStore(t, filepath.Join(t.TempDir(), "anime.sqlite"))
	defer store.Close()
	ctx := context.Background()
	for _, id := range []core.AnimeID{"anime-c", "anime-a", "anime-b"} {
		if err := store.SaveAnime(ctx, core.Anime{ID: id}); err != nil {
			t.Fatal(err)
		}
		if err := store.SetFollowing(ctx, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveSourceRef(ctx, "anime-a", core.SourceRef{Provider: "z", ID: "1"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSourceRef(ctx, "anime-a", core.SourceRef{Provider: "a", ID: "2"}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.ListAnime(ctx); err != nil || !reflect.DeepEqual(got, []core.Anime{{ID: "anime-a"}, {ID: "anime-b"}, {ID: "anime-c"}}) {
		t.Fatalf("anime list = %#v, %v", got, err)
	}
	if got, err := store.SourceRefs(ctx, "anime-a"); err != nil || !reflect.DeepEqual(got, []core.SourceRef{{Provider: "a", ID: "2"}, {Provider: "z", ID: "1"}}) {
		t.Fatalf("source list = %#v, %v", got, err)
	}
	if got, err := store.Following(ctx); err != nil || !reflect.DeepEqual(got, []core.AnimeID{"anime-a", "anime-b", "anime-c"}) {
		t.Fatalf("following list = %#v, %v", got, err)
	}
	for name, list := range map[string]func(context.Context) ([]core.AnimeID, error){
		"following": store.Following,
	} {
		t.Run(name, func(t *testing.T) {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			got, err := list(cancelled)
			if got == nil || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled list = %#v, %v", got, err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.SaveAnime(cancelled, core.Anime{ID: "later"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled save error = %v, want context.Canceled", err)
	}
}

func openLibraryStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func assertLibraryState(t *testing.T, store *Store, anime core.Anime, ref core.SourceRef, metadata core.AnimeMetadata) {
	t.Helper()
	ctx := context.Background()
	if got, err := store.Anime(ctx, anime.ID); err != nil || got != anime {
		t.Fatalf("anime = %#v, %v", got, err)
	}
	if got, err := store.SourceRefs(ctx, anime.ID); err != nil || !reflect.DeepEqual(got, []core.SourceRef{ref}) {
		t.Fatalf("source refs = %#v, %v", got, err)
	}
	if got, err := store.Metadata(ctx, anime.ID); err != nil || got != metadata {
		t.Fatalf("metadata = %#v, %v", got, err)
	}
	if got, err := store.Following(ctx); err != nil || !reflect.DeepEqual(got, []core.AnimeID{anime.ID}) {
		t.Fatalf("following = %#v, %v", got, err)
	}
}

func exerciseIdentityWrite(t *testing.T, separate bool, write func(*Store, core.AnimeID) error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "anime.sqlite")
	first := openLibraryStore(t, path)
	second := first
	if separate {
		second = openLibraryStore(t, path)
	}
	defer first.Close()
	if separate {
		defer second.Close()
	}
	ctx := context.Background()
	for _, id := range []core.AnimeID{"anime-a", "anime-b"} {
		if err := first.SaveAnime(ctx, core.Anime{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	ready := sync.WaitGroup{}
	ready.Add(2)
	results := make(chan error, 2)
	for _, attempt := range []struct {
		store *Store
		id    core.AnimeID
	}{
		{store: first, id: "anime-a"},
		{store: second, id: "anime-b"},
	} {
		store, id := attempt.store, attempt.id
		go func(store *Store, id core.AnimeID) {
			ready.Done()
			<-start
			results <- write(store, id)
		}(store, id)
	}
	ready.Wait()
	close(start)
	successes := 0
	conflicts := 0
	for range 2 {
		err := <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrIdentityConflict):
			conflicts++
		default:
			t.Fatalf("concurrent write error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d, want 1 each", successes, conflicts)
	}
}
