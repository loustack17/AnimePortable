// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"math"
	"time"
)

const (
	// MaxOpeningLength bounds an opening; a longer interval is a bad submission and would skip
	// part of the story.
	MaxOpeningLength = 4 * time.Minute
	// openingLengthTolerance is how far the length of the cut a submission was timed on may be
	// from the episode playing; on another cut the same times would skip the wrong part.
	openingLengthTolerance = 5 * time.Second
	maxOpeningEpisode      = 10000
)

// Opening is an episode's opening song, as times from the start of the episode.
type Opening struct {
	Start time.Duration
	End   time.Duration
}

// OpeningCandidate is one community submission of an opening with the length of the episode
// it was timed on.
type OpeningCandidate struct {
	Opening
	EpisodeLength time.Duration
}

// OpeningSource looks up community opening times by MyAnimeList id, episode number and
// episode length; nothing else about the user or the anime is sent.
type OpeningSource interface {
	Openings(ctx context.Context, myAnimeListID, episode int, length time.Duration) ([]OpeningCandidate, error)
}

// MyAnimeListSource finds MyAnimeList ids through a metadata provider that has them.
type MyAnimeListSource interface {
	Search(ctx context.Context, query MetadataQuery) ([]MetadataCandidate, error)
	MyAnimeListID(ctx context.Context, ref MetadataRef) (int, error)
}

// MyAnimeListID finds the anime's MyAnimeList id, or 0 when no candidate is accepted. Anime1
// titles are Chinese, which providers with MyAnimeList ids rarely know, so the search uses the
// Japanese title of the stored metadata when there is one. settled is false for a miss without
// that title: metadata stored later (a detail page refreshes it in the background) may still
// find the id, so such an answer is not worth remembering.
func (app *App) MyAnimeListID(ctx context.Context, animeID AnimeID, source MyAnimeListSource) (id int, settled bool, err error) {
	if ctx == nil || app == nil || app.store == nil || source == nil {
		return 0, false, ErrInvalidPlayback
	}
	anime, err := app.store.Anime(ctx, animeID)
	if err != nil {
		return 0, false, err
	}
	query := metadataQueryFor(anime)
	stored, err := app.store.Metadata(ctx, animeID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return 0, false, err
	}
	japanese := err == nil && stored.NativeTitle != ""
	if japanese {
		query = MetadataQuery{NativeTitle: stored.NativeTitle, Year: stored.Year, EpisodeCount: stored.EpisodeCount}
	}
	candidates, err := source.Search(ctx, query)
	if err != nil {
		return 0, false, err
	}
	match := MatchMetadata(MetadataMatchRequest{Query: query, Candidates: candidates})
	if match.Decision != MetadataMatchAccepted {
		return 0, japanese, nil
	}
	id, err = source.MyAnimeListID(ctx, match.Candidate.Ref)
	if errors.Is(err, ErrNotFound) {
		return 0, true, nil
	}
	return id, err == nil, err
}

// FindOpening finds the opening of the episode playing, which is length long. It reports
// false when the episode has no plain whole number or no submission fits this cut.
func (app *App) FindOpening(ctx context.Context, animeID AnimeID, episodeID EpisodeID, myAnimeListID int, length time.Duration, source OpeningSource) (Opening, bool, error) {
	if ctx == nil || app == nil || app.store == nil || source == nil || myAnimeListID <= 0 || length <= 0 {
		return Opening{}, false, ErrInvalidPlayback
	}
	mappings, err := app.store.EpisodeMappings(ctx, animeID)
	if err != nil {
		return Opening{}, false, err
	}
	number := 0
	for _, mapping := range mappings {
		if mapping.EpisodeID == episodeID {
			number = wholeEpisodeNumber(mapping.Number)
			break
		}
	}
	if number == 0 {
		return Opening{}, false, nil
	}
	candidates, err := source.Openings(ctx, myAnimeListID, number, length)
	if err != nil {
		return Opening{}, false, err
	}
	opening, ok := ChooseOpening(candidates, length)
	return opening, ok, nil
}

// ChooseOpening picks the valid submission timed on the cut closest in length to the episode,
// or reports false when none is within the tolerance. Submissions are untrusted.
func ChooseOpening(candidates []OpeningCandidate, length time.Duration) (Opening, bool) {
	var best OpeningCandidate
	bestGap := time.Duration(math.MaxInt64)
	for _, candidate := range candidates {
		if !validOpening(candidate.Opening, length) || candidate.EpisodeLength <= 0 {
			continue
		}
		gap := (candidate.EpisodeLength - length).Abs()
		if gap <= openingLengthTolerance && gap < bestGap {
			best, bestGap = candidate, gap
		}
	}
	return best.Opening, bestGap <= openingLengthTolerance
}

func validOpening(opening Opening, length time.Duration) bool {
	return opening.Start >= 0 && opening.End > opening.Start && opening.End <= length && opening.End-opening.Start <= MaxOpeningLength
}

// wholeEpisodeNumber is the episode number when it is a plain whole number, else 0: AniSkip
// counts episodes, so "12.5" or a special has no entry.
func wholeEpisodeNumber(number string) int {
	value, ok := EpisodeNumberValue(number)
	if !ok || value < 1 || value > maxOpeningEpisode || value != math.Trunc(value) {
		return 0
	}
	return int(value)
}
