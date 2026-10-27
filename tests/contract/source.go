// SPDX-License-Identifier: MPL-2.0

package contract

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"animeportable/core"
)

type SourceListCase struct {
	Expected []core.SourceRef
}

type SourceSearchCase struct {
	Query    string
	Expected []core.SourceRef
}

type SourceEpisodesCase struct {
	Anime    core.SourceRef
	Expected []core.EpisodeRef
}

type SourceResolveCase struct {
	Episode core.EpisodeRef
}

type AnimeSourceSuite struct {
	New              func(t *testing.T) core.AnimeSource
	Catalog          SourceListCase
	Search           SourceSearchCase
	Episodes         SourceEpisodesCase
	Resolve          SourceResolveCase
	ForbiddenStrings []string
}

func RunAnimeSource(t *testing.T, suite AnimeSourceSuite) {
	t.Helper()
	validateAnimeSourceSuite(t, suite)
	t.Run("catalog", func(t *testing.T) {
		source := suite.New(t)
		items, err := source.Catalog(context.Background())
		requireNoError(t, err, suite.ForbiddenStrings)
		refs := make([]core.SourceRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, item.Ref)
		}
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateSourceMembership(refs, suite.Catalog.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Catalog(ctx); return err })
	})
	t.Run("search", func(t *testing.T) {
		source := suite.New(t)
		items, err := source.Search(context.Background(), suite.Search.Query)
		requireNoError(t, err, suite.ForbiddenStrings)
		refs := make([]core.SourceRef, 0, len(items))
		for _, item := range items {
			refs = append(refs, item.Ref)
		}
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateSourceMembership(refs, suite.Search.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Search(ctx, suite.Search.Query); return err })
	})
	t.Run("episodes", func(t *testing.T) {
		source := suite.New(t)
		items, err := source.Episodes(context.Background(), suite.Episodes.Anime)
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateEpisodeOrder(items, suite.Episodes.Anime, suite.Episodes.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Episodes(ctx, suite.Episodes.Anime); return err })
	})
	t.Run("resolve", func(t *testing.T) {
		source := suite.New(t)
		resolved, err := source.Resolve(context.Background(), suite.Resolve.Episode)
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateAbsoluteURL(resolved.URL()))
		requireValid(t, validateForbidden(resolved, suite.ForbiddenStrings))
		encoded, err := json.Marshal(resolved)
		if err != nil {
			t.Fatal(err)
		}
		requireValid(t, validateForbidden(string(encoded), suite.ForbiddenStrings))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Resolve(ctx, suite.Resolve.Episode); return err })
	})
}

func validateAnimeSourceSuite(t *testing.T, suite AnimeSourceSuite) {
	t.Helper()
	if suite.New == nil {
		t.Fatal("source factory is nil")
	}
	for _, count := range []int{len(suite.Catalog.Expected), len(suite.Search.Expected), len(suite.Episodes.Expected)} {
		if count == 0 {
			t.Fatal("source case has no expected fixtures")
		}
	}
	validateSourceFixtures(t, suite.Catalog.Expected)
	validateSourceFixtures(t, suite.Search.Expected)
	if err := validateSourceRef(suite.Episodes.Anime); err != nil {
		t.Fatal(err)
	}
	validateEpisodeFixtures(t, suite.Episodes.Expected, suite.Episodes.Anime)
	validateEpisodeFixtures(t, []core.EpisodeRef{suite.Resolve.Episode}, suite.Resolve.Episode.Anime)
}

func validateSourceFixtures(t *testing.T, refs []core.SourceRef) {
	t.Helper()
	seen := map[core.SourceRef]struct{}{}
	for _, ref := range refs {
		if err := validateSourceRef(ref); err != nil {
			t.Fatal(err)
		}
		if _, exists := seen[ref]; exists {
			t.Fatalf("duplicate source fixture: %#v", ref)
		}
		seen[ref] = struct{}{}
	}
}

func validateEpisodeFixtures(t *testing.T, refs []core.EpisodeRef, parent core.SourceRef) {
	t.Helper()
	seen := map[core.EpisodeRef]struct{}{}
	for _, ref := range refs {
		if err := validateSourceRef(ref.Anime); err != nil || strings.TrimSpace(ref.ID) == "" {
			t.Fatalf("invalid episode fixture: %#v", ref)
		}
		if parent != (core.SourceRef{}) && ref.Anime != parent {
			t.Fatalf("episode fixture has wrong parent: %#v", ref)
		}
		if _, exists := seen[ref]; exists {
			t.Fatalf("duplicate episode fixture: %#v", ref)
		}
		seen[ref] = struct{}{}
	}
}

func checkCanceled(t *testing.T, forbidden []string, call func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := call(ctx)
	requireErrorSafe(t, err, forbidden)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("canceled call did not return context.Canceled")
	}
}

func requireNoError(t *testing.T, err error, forbidden []string) {
	t.Helper()
	if err == nil {
		return
	}
	requireErrorSafe(t, err, forbidden)
	t.Fatalf("adapter call failed with %T", err)
}

func requireErrorSafe(t *testing.T, err error, forbidden []string) {
	t.Helper()
	if validateForbidden(err, forbidden) != nil {
		t.Fatal("adapter error contains forbidden content")
	}
}

func requireValid(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
