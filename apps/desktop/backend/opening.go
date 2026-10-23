// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"time"

	"animeportable/core"
)

// openingLookupTimeout bounds finding one opening: at most a MyAnimeList id lookup and one
// AniSkip request.
const openingLookupTimeout = core.MetadataRefreshTimeout

// OpeningTimes finds the opening of the episode playing, which is lengthMillis long. Found is
// false when there is nothing to skip. Each anime's MyAnimeList id is looked up once per app
// run; a failed or unsettled lookup is not remembered, so the next episode tries again. Two
// episodes starting at once may both look it up; the answers are the same.
func (service *Service) OpeningTimes(ctx context.Context, animeID, episode string, lengthMillis int64) (Opening, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return Opening{}, err
	}
	defer done()
	localAnimeID, err := localID(animeID)
	if err != nil {
		return Opening{}, err
	}
	localEpisodeID, err := episodeID(episode)
	if err != nil {
		return Opening{}, err
	}
	if lengthMillis <= 0 || !validMillis(lengthMillis) {
		return Opening{}, ErrInvalidInput
	}
	app, _, _, err := service.appSnapshot()
	if err != nil {
		return Opening{}, err
	}
	service.mu.Lock()
	myAnimeList, openings := service.myAnimeList, service.openings
	myAnimeListID, known := service.myAnimeListIDs[localAnimeID]
	service.mu.Unlock()
	if myAnimeList == nil || openings == nil {
		return Opening{}, ErrUnavailable
	}
	lookupCtx, cancel := context.WithTimeout(operationCtx, openingLookupTimeout)
	defer cancel()
	if !known {
		var settled bool
		myAnimeListID, settled, err = app.MyAnimeListID(lookupCtx, localAnimeID, myAnimeList)
		if err != nil {
			return Opening{}, safeError(err)
		}
		if settled {
			service.mu.Lock()
			if service.myAnimeListIDs == nil {
				service.myAnimeListIDs = make(map[core.AnimeID]int)
			}
			service.myAnimeListIDs[localAnimeID] = myAnimeListID
			service.mu.Unlock()
		}
	}
	if myAnimeListID == 0 {
		return Opening{}, nil
	}
	length := time.Duration(lengthMillis) * time.Millisecond
	opening, found, err := app.FindOpening(lookupCtx, localAnimeID, localEpisodeID, myAnimeListID, length, openings)
	if err != nil || !found {
		return Opening{}, safeError(err)
	}
	return Opening{Found: true, Start: opening.Start.Milliseconds(), End: opening.End.Milliseconds()}, nil
}
