// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
)

// ErrMetadataUnavailable means every metadata provider failed, so the lookup can be retried later.
var ErrMetadataUnavailable = errors.New("metadata unavailable")

// RefreshMetadata looks the anime up on every provider and stores the full record of the
// candidate MatchMetadata accepts. It returns false when no candidate is accepted, because
// no metadata is better than wrong metadata (ADR-009). One failing provider does not stop
// the others.
func (app *App) RefreshMetadata(ctx context.Context, animeID AnimeID, providers []MetadataProvider) (bool, error) {
	if ctx == nil || app == nil || app.store == nil {
		return false, ErrInvalidPlayback
	}
	anime, err := app.store.Anime(ctx, animeID)
	if err != nil {
		return false, err
	}
	query := MetadataQuery{Title: anime.Title, NativeTitle: anime.NativeTitle}
	var candidates []MetadataCandidate
	sources := make(map[string]MetadataProvider, len(providers))
	failed := 0
	for _, provider := range providers {
		found, err := provider.Search(ctx, query)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return false, ctxErr
			}
			failed++
			continue
		}
		for _, candidate := range found {
			sources[candidate.Ref.Provider] = provider
		}
		candidates = append(candidates, found...)
	}
	if len(providers) > 0 && failed == len(providers) {
		return false, ErrMetadataUnavailable
	}
	match := MatchMetadata(MetadataMatchRequest{Query: query, Candidates: candidates})
	if match.Decision != MetadataMatchAccepted {
		return false, nil
	}
	metadata, err := sources[match.Candidate.Ref.Provider].Get(ctx, match.Candidate.Ref)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return false, ctxErr
		}
		return false, ErrMetadataUnavailable
	}
	if err := app.store.SaveMetadata(ctx, animeID, metadata); err != nil {
		return false, err
	}
	return true, nil
}
