// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode"
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
	query := metadataQueryFor(anime)
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

var bracketedName = regexp.MustCompile(`^(.+?)\s*[(（]([^()（）]+)[)）]\s*$`)

// metadataQueryFor builds the lookup query. Anime1 sometimes gives a Latin-script title with its
// Chinese name in brackets, as in "RELEASE THE SPYCE (月影特工)"; the bracketed name is then
// matched as the alternative title. Brackets on a Chinese title are left alone: they hold edition
// markers such as "(劇場版)" or "(第二季)", and dropping those would match the wrong work.
func metadataQueryFor(anime Anime) MetadataQuery {
	query := MetadataQuery{Title: anime.Title, NativeTitle: anime.NativeTitle}
	match := bracketedName.FindStringSubmatch(anime.Title)
	if query.NativeTitle != "" || match == nil || containsHan(match[1]) || !containsHan(match[2]) {
		return query
	}
	query.Title, query.NativeTitle = strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
	return query
}

func containsHan(value string) bool {
	for _, character := range value {
		if unicode.Is(unicode.Han, character) {
			return true
		}
	}
	return false
}
