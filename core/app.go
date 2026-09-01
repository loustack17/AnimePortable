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
