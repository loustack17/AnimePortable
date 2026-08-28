package core

import (
	"context"
	"time"
)

type SourceAnime struct {
	Ref         SourceRef
	Title       string
	NativeTitle string
}

type SourceEpisode struct {
	Ref    EpisodeRef
	Number string
	Title  string
}

type ScheduleQuery struct {
	From time.Time
	To   time.Time
}

type SchedulePrecision uint8

const (
	SchedulePrecisionDay SchedulePrecision = iota + 1
	SchedulePrecisionTime
)

type SourceScheduleItem struct {
	Anime     SourceAnime
	Episode   SourceEpisode
	AirsAt    time.Time
	Precision SchedulePrecision
}

type AnimeSource interface {
	Catalog(ctx context.Context) ([]SourceAnime, error)
	Search(ctx context.Context, query string) ([]SourceAnime, error)
	Episodes(ctx context.Context, ref SourceRef) ([]SourceEpisode, error)
	Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error)
	Schedule(ctx context.Context, query ScheduleQuery) ([]SourceScheduleItem, error)
}
