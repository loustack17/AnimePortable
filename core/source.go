// SPDX-License-Identifier: MPL-2.0

package core

import "context"

type SourceAnime struct {
	Ref         SourceRef
	Title       string
	NativeTitle string
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
