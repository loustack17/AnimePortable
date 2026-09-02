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
	Supported bool
	Expected  []core.SourceRef
}

type SourceSearchCase struct {
	Supported bool
	Query     string
	Expected  []core.SourceRef
}

type SourceEpisodesCase struct {
	Supported bool
	Anime     core.SourceRef
	Expected  []core.EpisodeRef
}

type SourceResolveCase struct {
	Supported bool
	Episode   core.EpisodeRef
}

type SourceScheduleCase struct {
	Supported bool
	Query     core.ScheduleQuery
	Expected  []core.SourceScheduleItem
}

type AnimeSourceSuite struct {
	New              func(t *testing.T) core.AnimeSource
	Catalog          SourceListCase
	Search           SourceSearchCase
	Episodes         SourceEpisodesCase
	Resolve          SourceResolveCase
	Schedule         SourceScheduleCase
	ForbiddenStrings []string
}

func RunAnimeSource(t *testing.T, suite AnimeSourceSuite) {
	t.Helper()
	validateAnimeSourceSuite(t, suite)
	t.Run("catalog", func(t *testing.T) {
		source := suite.New(t)
		items, err := source.Catalog(context.Background())
		if !suite.Catalog.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
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
		if !suite.Search.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
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
		if !suite.Episodes.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateEpisodeOrder(items, suite.Episodes.Anime, suite.Episodes.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Episodes(ctx, suite.Episodes.Anime); return err })
	})
	t.Run("resolve", func(t *testing.T) {
		source := suite.New(t)
		resolved, err := source.Resolve(context.Background(), suite.Resolve.Episode)
		if !suite.Resolve.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
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
	t.Run("schedule", func(t *testing.T) {
		source := suite.New(t)
		items, err := source.Schedule(context.Background(), suite.Schedule.Query)
		if !suite.Schedule.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateSchedule(items, suite.Schedule.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := source.Schedule(ctx, suite.Schedule.Query); return err })
	})
}

func validateAnimeSourceSuite(t *testing.T, suite AnimeSourceSuite) {
	t.Helper()
	if suite.New == nil {
		t.Fatal("source factory is nil")
	}
	for _, contractCase := range []struct {
		supported bool
		count     int
	}{
		{suite.Catalog.Supported, len(suite.Catalog.Expected)},
		{suite.Search.Supported, len(suite.Search.Expected)},
		{suite.Episodes.Supported, len(suite.Episodes.Expected)},
		{suite.Schedule.Supported, len(suite.Schedule.Expected)},
	} {
		if contractCase.supported && contractCase.count == 0 {
			t.Fatal("supported source case has no expected fixtures")
		}
		if !contractCase.supported && contractCase.count != 0 {
			t.Fatal("unsupported source case has expected fixtures")
		}
	}
	validateSourceFixtures(t, suite.Catalog.Expected)
	validateSourceFixtures(t, suite.Search.Expected)
	if suite.Episodes.Supported {
		if err := validateSourceRef(suite.Episodes.Anime); err != nil {
			t.Fatal(err)
		}
		validateEpisodeFixtures(t, suite.Episodes.Expected, suite.Episodes.Anime)
	} else if suite.Episodes.Anime != (core.SourceRef{}) {
		t.Fatal("unsupported episodes case has anime fixture")
	}
	if suite.Resolve.Supported {
		validateEpisodeFixtures(t, []core.EpisodeRef{suite.Resolve.Episode}, suite.Resolve.Episode.Anime)
	} else if suite.Resolve.Episode != (core.EpisodeRef{}) {
		t.Fatal("unsupported resolve case has episode fixture")
	}
	if suite.Schedule.Supported {
		for _, item := range suite.Schedule.Expected {
			if err := validateScheduleItem(item); err != nil {
				t.Fatal(err)
			}
		}
	}
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

func requireUnsupported(t *testing.T, err error, forbidden []string) {
	t.Helper()
	requireErrorSafe(t, err, forbidden)
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatal("unsupported call did not return ErrUnsupported")
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
