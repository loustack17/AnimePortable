// SPDX-License-Identifier: MPL-2.0

package core

import "context"

type MetadataQuery struct {
	Title        string
	NativeTitle  string
	Season       string
	Year         int
	EpisodeCount int
}

type MetadataCandidate struct {
	Ref          MetadataRef
	Title        string
	NativeTitle  string
	Season       string
	Year         int
	EpisodeCount int
}

type MetadataProvider interface {
	Search(ctx context.Context, query MetadataQuery) ([]MetadataCandidate, error)
	Get(ctx context.Context, ref MetadataRef) (AnimeMetadata, error)
}
