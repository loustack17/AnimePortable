// SPDX-License-Identifier: MPL-2.0

package core

import "context"

type SourceAnime struct {
	Ref         SourceRef
	Title       string
	NativeTitle string
	Catalog     CatalogInfo
}

// CatalogInfo is what the source catalog says about when an anime aired and how long it is.
// Each field is zero when the catalog leaves it unknown or unclear.
type CatalogInfo struct {
	Year int
	// AiringSeason is the season it began airing: "winter", "spring", "summer" or "fall".
	AiringSeason string
	EpisodeCount int
}

type SourceEpisode struct {
	Ref    EpisodeRef
	Number string
	Title  string
}

type AnimeSource interface {
	Catalog(ctx context.Context) ([]SourceAnime, error)
	Search(ctx context.Context, query string) ([]SourceAnime, error)
	Episodes(ctx context.Context, ref SourceRef) ([]SourceEpisode, error)
	Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error)
}
