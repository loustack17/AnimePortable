// SPDX-License-Identifier: MPL-2.0

package core

import (
	"slices"
	"time"
)

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

type EpisodeMapping struct {
	AnimeID   AnimeID
	EpisodeID EpisodeID
	Ref       EpisodeRef
	// Number is the source's episode label ("12", "12.5", "SP"); empty when not yet known.
	Number string
	// FirstSeenAt is when the app first stored this episode; zero for episodes stored
	// before this was recorded.
	FirstSeenAt time.Time
}

// FollowedAnime is a followed anime and when it was followed (zero if unknown).
type FollowedAnime struct {
	AnimeID    AnimeID
	FollowedAt time.Time
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

type FollowingEntry struct {
	AnimeID               AnimeID
	LatestAvailable       EpisodeID
	LatestAvailableNumber string
	LatestWatched         EpisodeID
	LatestWatchedNumber   string
	HasAvailable          bool
	HasWatched            bool
	NewEpisode            bool
	// LatestSeenAt is when the app first saw the latest episode; zero if unknown.
	LatestSeenAt    time.Time
	RecentlyUpdated bool
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

// PlaybackSpeed is the speed every episode starts at, in percent (100 is normal). The player
// sets it and it is remembered across episodes and restarts.
type PlaybackSpeed int

const (
	PlaybackSpeedUnspecified PlaybackSpeed = 0
	PlaybackSpeedDefault     PlaybackSpeed = 100
)

// PlaybackSpeedSteps are the speeds the player offers, slowest first.
var PlaybackSpeedSteps = [...]PlaybackSpeed{75, 100, 125, 150, 200}

// Valid reports whether the speed is unspecified or one of PlaybackSpeedSteps.
func (speed PlaybackSpeed) Valid() bool {
	return speed == PlaybackSpeedUnspecified || slices.Contains(PlaybackSpeedSteps[:], speed)
}

type ProxyMode uint8

const (
	ProxyModeUnspecified ProxyMode = iota
	ProxyModeNone
	ProxyModeSystem
	ProxyModeManual
)

// Settings holds what the user chose. A zero field means the user has not chosen; use
// Resolved to read the effective value.
type Settings struct {
	Appearance   Appearance
	AutoplayNext Toggle
	// AutoPlay decides whether an episode loaded by AutoplayNext starts playing or waits paused.
	AutoPlay       Toggle
	ResumePlayback Toggle
	Language       Language
	PlaybackSpeed  PlaybackSpeed
	// HardwareDecoding off is the escape hatch for black screens or crashes; it applies the
	// next time the player opens.
	HardwareDecoding Toggle
	ProxyMode        ProxyMode
	// ProxyAddress is host:port and only matters in ProxyModeManual. Manual with an empty
	// address is storable so the user can pick Manual before typing; the network layer treats
	// it as no proxy. See ValidProxyAddress.
	ProxyAddress string
}

// Resolved replaces every unchosen field with its default. It is the only place defaults
// live, so every reader sees the same behavior. Language stays as stored until the language
// setting exists.
func (settings Settings) Resolved() Settings {
	if settings.Appearance == AppearanceUnspecified {
		settings.Appearance = AppearanceSystem
	}
	settings.AutoplayNext = toggleOrDefault(settings.AutoplayNext, ToggleEnabled)
	settings.AutoPlay = toggleOrDefault(settings.AutoPlay, ToggleEnabled)
	settings.ResumePlayback = toggleOrDefault(settings.ResumePlayback, ToggleEnabled)
	if settings.PlaybackSpeed == PlaybackSpeedUnspecified {
		settings.PlaybackSpeed = PlaybackSpeedDefault
	}
	settings.HardwareDecoding = toggleOrDefault(settings.HardwareDecoding, ToggleEnabled)
	if settings.ProxyMode == ProxyModeUnspecified {
		settings.ProxyMode = ProxyModeNone
	}
	return settings
}

func toggleOrDefault(value, fallback Toggle) Toggle {
	if value == ToggleUnspecified {
		return fallback
	}
	return value
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
