package core

import (
	"context"
	"errors"
	"time"
)

type App struct {
	source             AnimeSource
	player             Player
	store              Store
	now                func() time.Time
	newTicker          playbackTickerFactory
	checkpointInterval time.Duration
	checkpointTimeout  time.Duration
}

func NewApp(source AnimeSource, player Player, store Store) *App {
	return &App{
		source:             source,
		player:             player,
		store:              store,
		now:                time.Now,
		newTicker:          newRealPlaybackTicker,
		checkpointInterval: defaultCheckpointInterval,
		checkpointTimeout:  defaultCheckpointTimeout,
	}
}

func (app *App) Search(ctx context.Context, query string) ([]SourceAnime, error) {
	return app.source.Search(ctx, query)
}

func (app *App) PlayEpisode(ctx context.Context, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration) (PlaybackSession, error) {
	request, err := app.preparePlayRequest(ctx, animeID, episodeID, ref, startAt)
	if err != nil {
		return nil, err
	}
	raw, err := app.player.Start(ctx, request)
	if err != nil {
		return nil, err
	}
	tracked, err := newTrackedPlaybackSession(raw, app.store, request, app.trackingConfig())
	if err != nil {
		closeErr := raw.Close()
		if closeErr != nil {
			return nil, ErrPlaybackTracking
		}
		return nil, err
	}
	return tracked, nil
}

func (app *App) SwitchEpisode(ctx context.Context, session PlaybackSession, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration) error {
	tracked, ok := session.(*trackedPlaybackSession)
	if !ok || tracked == nil {
		return ErrPlaybackTracking
	}
	return tracked.switchEpisode(ctx, func(operationCtx context.Context) (PlayRequest, error) {
		return app.preparePlayRequest(operationCtx, animeID, episodeID, ref, startAt)
	})
}

func (app *App) Library(ctx context.Context) ([]Anime, error) {
	return app.store.ListAnime(ctx)
}

func (app *App) Follow(ctx context.Context, animeID AnimeID) error {
	return app.setFollowing(ctx, animeID, true)
}

func (app *App) Unfollow(ctx context.Context, animeID AnimeID) error {
	return app.setFollowing(ctx, animeID, false)
}

func (app *App) setFollowing(ctx context.Context, animeID AnimeID, following bool) error {
	if ctx == nil || app == nil || app.store == nil {
		return ErrInvalidPlayback
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := app.store.Anime(ctx, animeID); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return app.store.SetFollowing(ctx, animeID, following)
}

func (app *App) ListFollowing(ctx context.Context) ([]FollowingEntry, error) {
	if ctx == nil || app == nil || app.store == nil || app.source == nil {
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
	if err := ctx.Err(); err != nil {
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
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		refs, err := app.store.SourceRefs(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mappings, err := app.store.EpisodeMappings(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		watchedEpisodes := watched[id]
		entry := FollowingEntry{
			AnimeID:       id,
			LatestWatched: recent[id],
			HasWatched:    len(watchedEpisodes) > 0,
		}
		episodes, loaded, err := app.followingEpisodes(ctx, refs)
		if err != nil {
			return nil, err
		}
		if loaded && len(episodes) > 0 {
			mappingByRef := episodeMappingIndex(mappings)
			entry.LatestAvailable = episodes[len(episodes)-1].Ref
			entry.HasAvailable = true
			entry.LatestWatched = latestWatchedEpisode(episodes, mappingByRef, watchedEpisodes, entry.LatestWatched)
			entry.NewEpisode = latestEpisodeIsNew(episodes[len(episodes)-1].Ref, mappingByRef, watchedEpisodes)
		}
		entries = append(entries, entry)
	}
	return entries, nil
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

func (app *App) followingEpisodes(ctx context.Context, refs []SourceRef) ([]SourceEpisode, bool, error) {
	for _, ref := range refs {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		episodes, err := app.source.Episodes(ctx, ref)
		if err == nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, false, ctxErr
			}
			return episodes, true, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, ctxErr
		}
	}
	return nil, false, nil
}

func latestWatchedEpisode(episodes []SourceEpisode, byRef map[EpisodeRef]EpisodeID, watched map[EpisodeID]struct{}, fallback EpisodeID) EpisodeID {
	latest := fallback
	for _, episode := range episodes {
		episodeID, ok := byRef[episode.Ref]
		if ok {
			if _, watched := watched[episodeID]; watched {
				latest = episodeID
			}
		}
	}
	return latest
}

func latestEpisodeIsNew(latest EpisodeRef, byRef map[EpisodeRef]EpisodeID, watched map[EpisodeID]struct{}) bool {
	episodeID, ok := byRef[latest]
	if !ok {
		return true
	}
	_, watchedEpisode := watched[episodeID]
	return !watchedEpisode
}

func episodeMappingIndex(mappings []EpisodeMapping) map[EpisodeRef]EpisodeID {
	byRef := make(map[EpisodeRef]EpisodeID, len(mappings))
	for _, mapping := range mappings {
		if mapping.EpisodeID == "" || mapping.Ref.ID == "" {
			continue
		}
		byRef[mapping.Ref] = mapping.EpisodeID
	}
	return byRef
}

func (app *App) preparePlayRequest(ctx context.Context, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration) (PlayRequest, error) {
	if ctx == nil || app == nil || app.source == nil || app.player == nil || app.store == nil || startAt < 0 {
		return PlayRequest{}, ErrInvalidPlayback
	}
	if err := ctx.Err(); err != nil {
		return PlayRequest{}, err
	}
	if _, err := app.store.Anime(ctx, animeID); err != nil {
		return PlayRequest{}, err
	}
	resolvedStart, err := playbackResumePosition(ctx, app.store, animeID, episodeID, startAt)
	if err != nil {
		return PlayRequest{}, err
	}
	playbackSource, err := app.source.Resolve(ctx, ref)
	if err != nil {
		return PlayRequest{}, err
	}
	if err := ctx.Err(); err != nil {
		return PlayRequest{}, err
	}
	if err := app.store.SaveSourceRef(ctx, animeID, ref.Anime); err != nil {
		return PlayRequest{}, err
	}
	if err := ctx.Err(); err != nil {
		return PlayRequest{}, err
	}
	if err := app.store.SaveEpisodeMapping(ctx, EpisodeMapping{AnimeID: animeID, EpisodeID: episodeID, Ref: ref}); err != nil {
		return PlayRequest{}, err
	}
	if err := ctx.Err(); err != nil {
		return PlayRequest{}, err
	}
	return PlayRequest{AnimeID: animeID, EpisodeID: episodeID, Source: playbackSource, StartAt: resolvedStart}, nil
}

func playbackResumePosition(ctx context.Context, store Store, animeID AnimeID, episodeID EpisodeID, startAt time.Duration) (time.Duration, error) {
	if startAt > 0 {
		return startAt, nil
	}
	settings, err := store.Settings(ctx)
	if err != nil {
		return 0, err
	}
	if settings.ResumePlayback != ToggleEnabled {
		return 0, nil
	}
	progress, err := store.Progress(ctx, animeID, episodeID)
	if errors.Is(err, ErrNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if progress.Completed || progress.Position <= 0 || completionThresholdReached(progress.Position, progress.Duration) {
		return 0, nil
	}
	return progress.Position, nil
}

func (app *App) trackingConfig() playbackTrackingConfig {
	return playbackTrackingConfig{
		now:       app.now,
		newTicker: app.newTicker,
		interval:  app.checkpointInterval,
		timeout:   app.checkpointTimeout,
	}
}
