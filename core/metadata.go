// SPDX-License-Identifier: MPL-2.0

package core

import "context"

type MetadataQuery struct {
	Title        string
	NativeTitle  string
	Year         int
	EpisodeCount int
}

type MetadataCandidate struct {
	Ref         MetadataRef
	Title       string
	NativeTitle string
	// Aliases are the provider's alternative names; they often include the Taiwan title.
	Aliases      []string
	Year         int
	EpisodeCount int
}

type MetadataProvider interface {
	Search(ctx context.Context, query MetadataQuery) ([]MetadataCandidate, error)
	Get(ctx context.Context, ref MetadataRef) (AnimeMetadata, error)
}

// MetadataSeasonBrowser is a MetadataProvider that can also list every anime that began airing
// in one season. A title search misses some anime (a Taiwan name the provider does not know,
// or a sequel that search ranks below other works); the season list still holds them.
type MetadataSeasonBrowser interface {
	// BrowseSeason lists the anime that began airing in year and season ("winter", "spring",
	// "summer" or "fall").
	BrowseSeason(ctx context.Context, year int, season string) ([]MetadataCandidate, error)
}
