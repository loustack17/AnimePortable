// SPDX-License-Identifier: MPL-2.0

package core

import "context"

type Store interface {
	SaveAnime(ctx context.Context, anime Anime) error
	Anime(ctx context.Context, id AnimeID) (Anime, error)
	ListAnime(ctx context.Context) ([]Anime, error)
	SaveSourceRef(ctx context.Context, animeID AnimeID, ref SourceRef) error
	SourceRefs(ctx context.Context, animeID AnimeID) ([]SourceRef, error)
	SaveEpisodeMapping(ctx context.Context, mapping EpisodeMapping) error
	EpisodeMappings(ctx context.Context, animeID AnimeID) ([]EpisodeMapping, error)
	SaveMetadata(ctx context.Context, animeID AnimeID, metadata AnimeMetadata) error
	Metadata(ctx context.Context, animeID AnimeID) (AnimeMetadata, error)
	SetFollowing(ctx context.Context, animeID AnimeID, following bool) error
	Following(ctx context.Context) ([]FollowedAnime, error)
	History(ctx context.Context) ([]HistoryEntry, error)
	// RemoveHistory also deletes the anime's saved resume positions.
	RemoveHistory(ctx context.Context, animeID AnimeID) error
	// SavePlaybackCheckpoint records the playing position in both the resume positions and
	// the watch history.
	SavePlaybackCheckpoint(ctx context.Context, entry HistoryEntry) error
	Progress(ctx context.Context, animeID AnimeID, episodeID EpisodeID) (PlaybackProgress, error)
	SaveSettings(ctx context.Context, settings Settings) error
	Settings(ctx context.Context) (Settings, error)
	// ReplaceSchedule replaces the whole stored weekly schedule with the latest fetch. Entry
	// AnimeID values are not stored.
	ReplaceSchedule(ctx context.Context, entries []ScheduleEntry) error
	// Schedule lists the stored weekly schedule by weekday, in source order within a day.
	// AnimeID is set when an anime's stored metadata has the entry's Ref.
	Schedule(ctx context.Context) ([]ScheduleEntry, error)
}
