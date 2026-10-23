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
		candidates = addSeasonCandidates(ctx, query, anime.Catalog, providers, candidates, sources)
		if err := ctx.Err(); err != nil {
			return false, err
		}
		match = MatchMetadata(MetadataMatchRequest{Query: query, Candidates: candidates})
	}
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

// addSeasonCandidates adds the anime of the catalog's airing season, from every provider that
// can list one, to a search that found no accepted match. The same matching rules apply; the
// season list only brings in works the search did not return, and only those whose names
// match the query at all (the rest would score nothing and only count against the candidate
// limit). A provider that cannot list the season adds nothing, and the search result stands.
func addSeasonCandidates(ctx context.Context, query MetadataQuery, catalog CatalogInfo, providers []MetadataProvider, candidates []MetadataCandidate, sources map[string]MetadataProvider) []MetadataCandidate {
	if catalog.Year == 0 || catalog.AiringSeason == "" {
		return candidates
	}
	queryTitle, queryNative, _ := metadataFields(query.Title, query.NativeTitle, query.Season)
	series := metadataSeriesNames(queryTitle, queryNative, candidates)
	known := make(map[MetadataRef]bool, len(candidates))
	for _, candidate := range candidates {
		known[candidate.Ref] = true
	}
	for _, provider := range providers {
		browser, ok := provider.(MetadataSeasonBrowser)
		if !ok {
			continue
		}
		found, err := browser.BrowseSeason(ctx, catalog.Year, catalog.AiringSeason)
		if err != nil {
			continue
		}
		for _, candidate := range found {
			// MatchMetadata takes a bounded number of candidates; past that, the rest are left out.
			if known[candidate.Ref] || len(candidates) >= metadataMaxCandidates || !metadataNamesMatch(query, candidate, series) {
				continue
			}
			known[candidate.Ref] = true
			sources[candidate.Ref.Provider] = provider
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

var bracketedName = regexp.MustCompile(`^(.+?)\s*[(（]([^()（）]+)[)）]\s*$`)

// metadataQueryFor builds the lookup query. Anime1 sometimes gives a Latin-script title with its
// Chinese name in brackets, as in "RELEASE THE SPYCE (月影特工)"; the bracketed name is then
// matched as the alternative title. Brackets on a Chinese title are left alone: they hold edition
// markers such as "(劇場版)" or "(第二季)", and dropping those would match the wrong work.
func metadataQueryFor(anime Anime) MetadataQuery {
	// The airing season stays out: only AniList candidates carry one, and a season on one side
	// only counts against every Bangumi candidate.
	query := MetadataQuery{Title: anime.Title, NativeTitle: anime.NativeTitle, Year: anime.Catalog.Year, EpisodeCount: anime.Catalog.EpisodeCount}
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
