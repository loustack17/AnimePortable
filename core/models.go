package core

import "time"

type AnimeID string

type EpisodeID string

type SourceRef struct {
	Provider string
	ID       string
}

type EpisodeRef struct {
	Anime SourceRef
	ID    string
}

type MetadataRef struct {
	Provider string
	ID       string
}

type Anime struct {
	ID          AnimeID
	Title       string
	NativeTitle string
	Description string
}

type Episode struct {
	ID      EpisodeID
	AnimeID AnimeID
	Number  string
	Title   string
}

type ScheduleItem struct {
	AnimeID   AnimeID
	EpisodeID EpisodeID
	AirsAt    time.Time
}

type AnimeMetadata struct {
	Ref          MetadataRef
	Title        string
	NativeTitle  string
	Description  string
	CoverURL     string
	Season       string
	Year         int
	Studio       string
	EpisodeCount int
}

type PlaybackProgress struct {
	AnimeID   AnimeID
	EpisodeID EpisodeID
	Position  time.Duration
	Duration  time.Duration
	Completed bool
	UpdatedAt time.Time
}

type PlaybackSnapshot struct {
	Position time.Duration
	Duration time.Duration
	Paused   bool
}

type HistoryEntry struct {
	Progress     PlaybackProgress
	LastPlayedAt time.Time
}

type Appearance uint8

const (
	AppearanceUnspecified Appearance = iota
	AppearanceSystem
	AppearanceLight
	AppearanceDark
)

type Toggle uint8

const (
	ToggleUnspecified Toggle = iota
	ToggleDisabled
	ToggleEnabled
)

type Language uint8

const (
	LanguageUnspecified Language = iota
	LanguageTraditionalChinese
	LanguageEnglish
)

type Settings struct {
	Appearance     Appearance
	MPVPath        string
	AutoplayNext   Toggle
	ResumePlayback Toggle
	Language       Language
}

func DefaultSettings() Settings {
	return Settings{AutoplayNext: ToggleEnabled}
}

type PlaybackEventKind uint8

const (
	PlaybackEventUnknown PlaybackEventKind = iota
	PlaybackEventProgress
	PlaybackEventPaused
	PlaybackEventEnded
	PlaybackEventFailed
	PlaybackEventStopped
)

type PlaybackEvent struct {
	AnimeID   AnimeID
	EpisodeID EpisodeID
	Kind      PlaybackEventKind
	Position  time.Duration
	Duration  time.Duration
	Err       error
}
