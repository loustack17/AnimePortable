// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

// ListFollowing builds the followed-anime list from local data only, so it loads
// without the network and still shows when the source is unreachable. Episode lists
// are refreshed separately; this reports the last stored state.
func (app *App) ListFollowing(ctx context.Context) ([]FollowingEntry, error) {
	if ctx == nil || app == nil || app.store == nil {
		return nil, ErrInvalidPlayback
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids, err := app.store.Following(ctx)
	if err != nil {
		return nil, err
	}
	history, err := app.store.History(ctx)
	if err != nil {
		return nil, err
	}
	watched, recent := indexPlaybackHistory(history)
	entries := make([]FollowingEntry, 0, len(ids))
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := app.store.Anime(ctx, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		mappings, err := app.store.EpisodeMappings(ctx, id)
		if err != nil {
			return nil, err
		}
		entries = append(entries, followingEntry(id, episodeNumbers(mappings), watched[id], recent[id]))
	}
	return entries, nil
}

// followingEntry applies the indicator rules: the latest episode is the highest
// numbered one, the watched episode is the highest numbered one with any history
// (so rewatching an older episode does not lower it), and a series the user has started
// has a new episode when no watched episode is numbered at least the latest.
func followingEntry(id AnimeID, numbers map[EpisodeID]string, watched map[EpisodeID]struct{}, recent EpisodeID) FollowingEntry {
	entry := FollowingEntry{AnimeID: id, HasWatched: len(watched) > 0}
	entry.LatestAvailable, entry.HasAvailable = highestNumbered(numbers, nil)
	watchedNumbered, ok := highestNumbered(numbers, watched)
	if !ok {
		// No watched episode has a known number yet; show the most recently played one.
		watchedNumbered = recent
	}
	if entry.HasWatched {
		entry.LatestWatched = watchedNumbered
		entry.LatestWatchedNumber = numbers[watchedNumbered]
	}
	if entry.HasAvailable {
		entry.LatestAvailableNumber = numbers[entry.LatestAvailable]
		entry.NewEpisode = entry.HasWatched && !watchedUpTo(numbers, watched, entry.LatestAvailableNumber)
	}
	return entry
}

// watchedUpTo reports whether an episode numbered at least latest has history. It
// compares numbers, not IDs, because one episode number can have several local IDs
// (a fallback source ref, or a re-uploaded post).
func watchedUpTo(numbers map[EpisodeID]string, watched map[EpisodeID]struct{}, latest string) bool {
	latestValue, _ := EpisodeNumberValue(latest)
	for id := range watched {
		if value, ok := EpisodeNumberValue(numbers[id]); ok && value >= latestValue {
			return true
		}
	}
	return false
}

// highestNumbered returns the episode with the highest plain number, limited to
// the given set when it is not nil. Ties go to the larger ID so the result is stable.
func highestNumbered(numbers map[EpisodeID]string, within map[EpisodeID]struct{}) (EpisodeID, bool) {
	var best EpisodeID
	var bestValue float64
	found := false
	for id, number := range numbers {
		if within != nil {
			if _, ok := within[id]; !ok {
				continue
			}
		}
		value, ok := EpisodeNumberValue(number)
		if !ok {
			continue
		}
		if !found || value > bestValue || value == bestValue && id > best {
			best, bestValue, found = id, value, true
		}
	}
	return best, found
}

// episodeNumbers maps each canonical episode to its known number. An episode can
// have several provider mappings; any one with a number is enough.
func episodeNumbers(mappings []EpisodeMapping) map[EpisodeID]string {
	numbers := make(map[EpisodeID]string, len(mappings))
	for _, mapping := range mappings {
		if mapping.EpisodeID == "" {
			continue
		}
		if number := strings.TrimSpace(mapping.Number); number != "" || numbers[mapping.EpisodeID] == "" {
			numbers[mapping.EpisodeID] = number
		}
	}
	return numbers
}

func indexPlaybackHistory(history []HistoryEntry) (map[AnimeID]map[EpisodeID]struct{}, map[AnimeID]EpisodeID) {
	watched := make(map[AnimeID]map[EpisodeID]struct{})
	recent := make(map[AnimeID]EpisodeID)
	recentAt := make(map[AnimeID]time.Time)
	for _, entry := range history {
		animeID := entry.Progress.AnimeID
		episodeID := entry.Progress.EpisodeID
		if animeID == "" || episodeID == "" {
			continue
		}
		if watched[animeID] == nil {
			watched[animeID] = make(map[EpisodeID]struct{})
		}
		watched[animeID][episodeID] = struct{}{}
		playedAt, found := recentAt[animeID]
		if !found || !entry.LastPlayedAt.Before(playedAt) {
			recent[animeID] = episodeID
			recentAt[animeID] = entry.LastPlayedAt
		}
	}
	return watched, recent
}

// EpisodeNumberValue parses a plain episode number such as "12" or "12.5".
// Labels like "SP" or "OVA" are not plain numbers.
func EpisodeNumberValue(number string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}
