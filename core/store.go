package core

import "context"

type Store interface {
	SaveAnime(ctx context.Context, anime Anime) error
	Anime(ctx context.Context, id AnimeID) (Anime, error)
	ListAnime(ctx context.Context) ([]Anime, error)
	SaveSourceRef(ctx context.Context, animeID AnimeID, ref SourceRef) error
	SourceRefs(ctx context.Context, animeID AnimeID) ([]SourceRef, error)
	SaveMetadata(ctx context.Context, animeID AnimeID, metadata AnimeMetadata) error
	Metadata(ctx context.Context, animeID AnimeID) (AnimeMetadata, error)
	SetFollowing(ctx context.Context, animeID AnimeID, following bool) error
	Following(ctx context.Context) ([]AnimeID, error)
	AddHistory(ctx context.Context, entry HistoryEntry) error
	History(ctx context.Context) ([]HistoryEntry, error)
	RemoveHistory(ctx context.Context, animeID AnimeID) error
	SaveProgress(ctx context.Context, progress PlaybackProgress) error
	Progress(ctx context.Context, animeID AnimeID, episodeID EpisodeID) (PlaybackProgress, error)
	SaveSettings(ctx context.Context, settings Settings) error
	Settings(ctx context.Context) (Settings, error)
}
