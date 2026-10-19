// SPDX-License-Identifier: MPL-2.0

package core

import "context"

// WatchedEpisode is a history entry with the episode's stored number, empty when the
// number is not known yet.
type WatchedEpisode struct {
	HistoryEntry
	EpisodeNumber string
}

// ListHistory returns watch history, newest first, with episode numbers from stored
// episode mappings. It reads only the store, so history never needs the network.
func (app *App) ListHistory(ctx context.Context) ([]WatchedEpisode, error) {
	if ctx == nil || app == nil || app.store == nil {
		return nil, ErrInvalidPlayback
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	history, err := app.store.History(ctx)
	if err != nil {
		return nil, err
	}
	numbers := make(map[AnimeID]map[EpisodeID]string)
	entries := make([]WatchedEpisode, 0, len(history))
	for _, entry := range history {
		id := entry.Progress.AnimeID
		if _, loaded := numbers[id]; !loaded {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			mappings, err := app.store.EpisodeMappings(ctx, id)
			if err != nil {
				return nil, err
			}
			numbers[id] = episodeNumbers(mappings)
		}
		entries = append(entries, WatchedEpisode{HistoryEntry: entry, EpisodeNumber: numbers[id][entry.Progress.EpisodeID]})
	}
	return entries, nil
}
