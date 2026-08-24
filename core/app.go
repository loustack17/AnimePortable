package core

import (
	"context"
	"time"
)

type App struct {
	source AnimeSource
	player Player
	store  Store
}

func NewApp(source AnimeSource, player Player, store Store) *App {
	return &App{source: source, player: player, store: store}
}

func (app *App) Search(ctx context.Context, query string) ([]SourceAnime, error) {
	return app.source.Search(ctx, query)
}

func (app *App) PlayEpisode(ctx context.Context, animeID AnimeID, episodeID EpisodeID, ref EpisodeRef, startAt time.Duration) (PlaybackSession, error) {
	source, err := app.source.Resolve(ctx, ref)
	if err != nil {
		return nil, err
	}
	return app.player.Start(ctx, PlayRequest{
		AnimeID:   animeID,
		EpisodeID: episodeID,
		Source:    source,
		StartAt:   startAt,
	})
}

func (app *App) Library(ctx context.Context) ([]Anime, error) {
	return app.store.ListAnime(ctx)
}
