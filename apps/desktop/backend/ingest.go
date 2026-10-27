// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"strings"
	"time"

	"animeportable/core"
	metadata "animeportable/internal/metadata"
)

// sourceIngester is the store's atomic write for source search results (SQLite has it).
type sourceIngester interface {
	IngestSourceAnime(context.Context, core.Anime, core.SourceRef) (core.Anime, error)
}

func (service *Service) ingestAnime(ctx context.Context, store core.Store, items []core.SourceAnime) ([]sourceIdentity, error) {
	service.identity.Lock()
	defer service.identity.Unlock()
	atomic, ok := store.(sourceIngester)
	if !ok {
		return nil, ErrUnavailable
	}
	result := make([]sourceIdentity, 0, len(items))
	for _, raw := range items {
		item, err := normalizeSourceAnime(raw)
		if err != nil {
			return nil, err
		}
		candidateID, idErr := newLocalID("anime")
		if idErr != nil {
			return nil, idErr
		}
		actual, ingestErr := atomic.IngestSourceAnime(ctx, core.Anime{ID: core.AnimeID(candidateID), Title: item.Title, NativeTitle: item.NativeTitle, Catalog: item.Catalog}, item.Ref)
		if ingestErr != nil {
			return nil, ingestErr
		}
		result = append(result, sourceIdentity{anime: actual, ref: item.Ref})
	}
	return result, nil
}

func (service *Service) ingestEpisodes(ctx context.Context, store core.Store, animeID core.AnimeID, parent core.SourceRef, items []core.SourceEpisode) ([]Episode, error) {
	service.identity.Lock()
	defer service.identity.Unlock()
	mappings, err := store.EpisodeMappings(ctx, animeID)
	if err != nil {
		return nil, err
	}
	known := make(map[core.EpisodeRef]core.EpisodeMapping, len(mappings))
	for _, mapping := range mappings {
		known[mapping.Ref] = mapping
	}
	// One time for the whole list, so a batch of newly seen episodes shares it.
	seenAt := time.Now()
	result := make([]Episode, 0, len(items))
	for _, item := range items {
		if item.Ref.Anime != parent || item.Ref.ID == "" || strings.TrimSpace(item.Ref.Anime.Provider) == "" {
			return nil, ErrInvalidInput
		}
		number, numberOK := metadata.NormalizePlainText(item.Number, metadata.TitleLimits())
		if !numberOK || number == "" || len(number) > 128 {
			return nil, ErrInvalidInput
		}
		mapping, ok := known[item.Ref]
		if !ok {
			value, idErr := newLocalID("episode")
			if idErr != nil {
				return nil, idErr
			}
			mapping = core.EpisodeMapping{AnimeID: animeID, EpisodeID: core.EpisodeID(value), Ref: item.Ref, FirstSeenAt: seenAt}
		}
		// Save new mappings, and known ones whose number is missing or changed, so
		// Following can show the latest episode without the source.
		if !ok || mapping.Number != number {
			mapping.Number = number
			if err := store.SaveEpisodeMapping(ctx, mapping); err != nil {
				return nil, err
			}

			known[item.Ref] = mapping
		}
		result = append(result, Episode{ID: string(mapping.EpisodeID), AnimeID: string(animeID), Number: number})
	}
	return result, nil
}
