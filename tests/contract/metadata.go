package contract

import (
	"context"
	"errors"
	"testing"

	"animeportable/core"
)

type MetadataSearchCase struct {
	Supported bool
	Query     core.MetadataQuery
	Expected  []core.MetadataRef
}

type MetadataGetCase struct {
	Supported bool
	Ref       core.MetadataRef
	Expected  core.AnimeMetadata
}

type MetadataMissingCase struct {
	Ref      core.MetadataRef
	Expected error
}

type MetadataProviderSuite struct {
	New              func(t *testing.T) core.MetadataProvider
	Search           MetadataSearchCase
	Get              MetadataGetCase
	Missing          *MetadataMissingCase
	ForbiddenStrings []string
}

func RunMetadataProvider(t *testing.T, suite MetadataProviderSuite) {
	t.Helper()
	validateMetadataSuite(t, suite)
	t.Run("search", func(t *testing.T) {
		provider := suite.New(t)
		items, err := provider.Search(context.Background(), suite.Search.Query)
		if !suite.Search.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateForbidden(items, suite.ForbiddenStrings))
		requireValid(t, validateMetadataCandidates(items, suite.Search.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := provider.Search(ctx, suite.Search.Query); return err })
	})
	t.Run("get", func(t *testing.T) {
		provider := suite.New(t)
		metadata, err := provider.Get(context.Background(), suite.Get.Ref)
		if !suite.Get.Supported {
			requireUnsupported(t, err, suite.ForbiddenStrings)
			return
		}
		requireNoError(t, err, suite.ForbiddenStrings)
		requireValid(t, validateForbidden(metadata, suite.ForbiddenStrings))
		requireValid(t, equal(metadata, suite.Get.Expected))
		checkCanceled(t, suite.ForbiddenStrings, func(ctx context.Context) error { _, err := provider.Get(ctx, suite.Get.Ref); return err })
	})
	if suite.Missing != nil {
		t.Run("missing", func(t *testing.T) {
			provider := suite.New(t)
			_, err := provider.Get(context.Background(), suite.Missing.Ref)
			requireErrorSafe(t, err, suite.ForbiddenStrings)
			if !errors.Is(err, suite.Missing.Expected) {
				t.Fatal("missing lookup returned unexpected error")
			}
		})
	}
}

func validateMetadataSuite(t *testing.T, suite MetadataProviderSuite) {
	t.Helper()
	if suite.New == nil {
		t.Fatal("metadata provider factory is nil")
	}
	if suite.Search.Supported && len(suite.Search.Expected) == 0 {
		t.Fatal("supported metadata search has no expected fixtures")
	}
	if !suite.Search.Supported && len(suite.Search.Expected) != 0 {
		t.Fatal("unsupported metadata search has expected fixtures")
	}
	validateMetadataFixtures(t, suite.Search.Expected)
	if suite.Get.Supported {
		requireValid(t, validateMetadataRef(suite.Get.Ref))
		if suite.Get.Expected.Ref != suite.Get.Ref {
			t.Fatal("expected metadata ref does not match get ref")
		}
	} else if suite.Get.Ref != (core.MetadataRef{}) || suite.Get.Expected != (core.AnimeMetadata{}) {
		t.Fatal("unsupported metadata get has fixtures")
	}
	if suite.Missing != nil {
		requireValid(t, validateMetadataRef(suite.Missing.Ref))
		if suite.Missing.Expected == nil {
			t.Fatal("missing metadata expected error is nil")
		}
	}
}

func validateMetadataFixtures(t *testing.T, refs []core.MetadataRef) {
	t.Helper()
	seen := map[core.MetadataRef]struct{}{}
	for _, ref := range refs {
		requireValid(t, validateMetadataRef(ref))
		if _, exists := seen[ref]; exists {
			t.Fatalf("duplicate metadata fixture: %#v", ref)
		}
		seen[ref] = struct{}{}
	}
}
