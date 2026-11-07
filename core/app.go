// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"errors"
	"time"
)

type App struct {
	source   AnimeSource
	player   Player
	store    Store
	tracking playbackTrackingConfig
}

func NewApp(source AnimeSource, player Player, store Store) *App {
	return &App{
		source: source,
		player: player,
		store:  store,
		tracking: playbackTrackingConfig{
			now:       time.Now,
			newTicker: newRealPlaybackTicker,
			interval:  defaultCheckpointInterval,
			timeout:   defaultCheckpointTimeout,
		},
	}
}

// PlayEpisode starts a player for the episode. paused makes it wait at its start position
// instead of playing.
func (app *App) PlayEpisode(ctx context.Context, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration, paused bool) (PlaybackSession, error) {
	request, err := app.preparePlayRequest(ctx, animeID, episodeID, ref, startAt)
	if err != nil {
		return nil, err
	}
	request.Paused = paused
	raw, err := app.player.Start(ctx, request)
	if err != nil {
		return nil, err
	}
	tracked, err := newTrackedPlaybackSession(raw, app.store, request, app.tracking)
	if err != nil {
		closeErr := raw.Close()
		if closeErr != nil {
			return nil, ErrPlaybackTracking
		}
		return nil, err
	}
	return tracked, nil
}

// SwitchEpisode loads another episode into session. paused makes it wait at its start
// position instead of playing; only the autoplay switch asks for that.
func (app *App) SwitchEpisode(ctx context.Context, session PlaybackSession, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration, paused bool) error {
	tracked, ok := session.(*trackedPlaybackSession)
	if !ok || tracked == nil {
		return ErrPlaybackTracking
	}
	return tracked.switchEpisode(ctx, func(operationCtx context.Context) (PlayRequest, error) {
		request, err := app.preparePlayRequest(operationCtx, animeID, episodeID, ref, startAt)
		request.Paused = paused
		return request, err
	})
}

// ready reports whether ctx is set and app has a store, which the store-backed operations need.
func (app *App) ready(ctx context.Context) bool {
	return ctx != nil && app != nil && app.store != nil
}

func (app *App) preparePlayRequest(ctx context.Context, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration) (PlayRequest, error) {
	if !app.ready(ctx) || app.source == nil || app.player == nil || startAt < 0 {
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
	if settings.Resolved().ResumePlayback != ToggleEnabled {
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
