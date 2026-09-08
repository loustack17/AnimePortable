// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"strings"

	"animeportable/core"
	metadata "animeportable/internal/metadata"
)

func (service *Service) ingestAnime(ctx context.Context, store core.Store, items []core.SourceAnime) ([]sourceIdentity, error) {
	service.identity.Lock()
	defer service.identity.Unlock()
	atomic, ok := store.(interface {
		IngestSourceAnime(context.Context, core.Anime, core.SourceRef) (core.Anime, error)
	})
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
		actual, ingestErr := atomic.IngestSourceAnime(ctx, core.Anime{ID: core.AnimeID(candidateID), Title: item.Title, NativeTitle: item.NativeTitle}, item.Ref)
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
	known := make(map[core.EpisodeRef]core.EpisodeID, len(mappings))
	for _, mapping := range mappings {
		known[mapping.Ref] = mapping.EpisodeID
	}
	result := make([]Episode, 0, len(items))
	for _, item := range items {
		if item.Ref.Anime != parent || item.Ref.ID == "" || strings.TrimSpace(item.Ref.Anime.Provider) == "" {
			return nil, ErrInvalidInput
		}
		number, numberOK := metadata.NormalizePlainText(item.Number, metadata.TitleLimits())
		title, titleOK := metadata.NormalizePlainText(item.Title, metadata.TitleLimits())
		if !numberOK || !titleOK || number == "" || len(number) > 128 || len(title) > 4096 {
			return nil, ErrInvalidInput
		}
		id, ok := known[item.Ref]
		if !ok {
			value, idErr := newLocalID("episode")
			if idErr != nil {
				return nil, idErr
			}
			id = core.EpisodeID(value)
			if err := store.SaveEpisodeMapping(ctx, core.EpisodeMapping{AnimeID: animeID, EpisodeID: id, Ref: item.Ref}); err != nil {
				return nil, err
			}
			known[item.Ref] = id
		}
		result = append(result, Episode{ID: string(id), AnimeID: string(animeID), Number: number, Title: title})
	}
	return result, nil
}
