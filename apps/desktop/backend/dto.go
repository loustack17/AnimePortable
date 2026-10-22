// SPDX-License-Identifier: MPL-2.0

package backend

type Anime struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	NativeTitle string `json:"nativeTitle"`
	Description string `json:"description"`
}

type Episode struct {
	ID      string `json:"id"`
	AnimeID string `json:"animeId"`
	Number  string `json:"number"`
	Title   string `json:"title"`
}

type Metadata struct {
	Title        string `json:"title"`
	NativeTitle  string `json:"nativeTitle"`
	Description  string `json:"description"`
	Season       string `json:"season"`
	Year         int    `json:"year"`
	Studio       string `json:"studio"`
	EpisodeCount int    `json:"episodeCount"`
}

type Detail struct {
	Anime     Anime     `json:"anime"`
	Metadata  *Metadata `json:"metadata,omitempty"`
	Following bool      `json:"following"`
}

// Following is one followed anime. Episode fields are local episode IDs and the
// source's episode labels; the numbers are empty when not known yet.
type Following struct {
	AnimeID               string `json:"animeId"`
	LatestAvailable       string `json:"latestAvailable"`
	LatestAvailableNumber string `json:"latestAvailableNumber"`
	LatestWatched         string `json:"latestWatched"`
	LatestWatchedNumber   string `json:"latestWatchedNumber"`
	HasAvailable          bool   `json:"hasAvailable"`
	HasWatched            bool   `json:"hasWatched"`
	NewEpisode            bool   `json:"newEpisode"`
	// RecentlyUpdated marks Home's 近期更新: a latest episode first seen in the last 7 days,
	// after the anime was followed, and not yet watched. LatestSeenAt is RFC 3339 or empty.
	RecentlyUpdated bool   `json:"recentlyUpdated"`
	LatestSeenAt    string `json:"latestSeenAt"`
}

// History is one played episode. EpisodeNumber is the stored episode label, empty when
// not known yet.
type History struct {
	AnimeID       string `json:"animeId"`
	EpisodeID     string `json:"episodeId"`
	EpisodeNumber string `json:"episodeNumber"`
	Position      int64  `json:"position"`
	Duration      int64  `json:"duration"`
	Completed     bool   `json:"completed"`
	UpdatedAt     string `json:"updatedAt"`
	LastPlayed    string `json:"lastPlayed"`
}

type Settings struct {
	Appearance     string `json:"appearance"`
	AutoplayNext   string `json:"autoplayNext"`
	AutoPlay       string `json:"autoPlay"`
	ResumePlayback string `json:"resumePlayback"`
	Language       string `json:"language"`
	// PlaybackSpeed is a percent (100 is normal). It is read-only here: SaveSettings keeps the
	// stored speed and only SavePlaybackSpeed changes it, because the player owns it.
	PlaybackSpeed    int    `json:"playbackSpeed"`
	HardwareDecoding string `json:"hardwareDecoding"`
	ProxyMode        string `json:"proxyMode"`
	ProxyAddress     string `json:"proxyAddress"`
}

// ScheduleItem is one show in the weekly airing schedule. AnimeID is the local anime it
// matches, empty when the app does not know it; Query is what to search for then. Weekday
// is 0 (Sunday) to 6.
type ScheduleItem struct {
	AnimeID string `json:"animeId"`
	Title   string `json:"title"`
	Query   string `json:"query"`
	Weekday int    `json:"weekday"`
}

type PlayRequest struct {
	AnimeID   string `json:"animeId"`
	EpisodeID string `json:"episodeId"`
	StartAt   int64  `json:"startAt"`
	// Paused asks to load the episode waiting at its start, whether the request switches an
	// open player or starts a new one.
	Paused bool `json:"paused"`
}

type Cover struct {
	Bytes     []byte `json:"bytes"`
	MediaType string `json:"mediaType"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// ProxyTestResult is the outcome of one request to Anime1 through the proxy being tested.
// Reason is one of the Reason codes (network.go), empty when OK.
type ProxyTestResult struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason"`
}

// UpdateInfo compares this build with the newest GitHub release. Versions have no "v" prefix.
// Latest and URL are empty when no release was found; URL is a GitHub release page or empty.
type UpdateInfo struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Newer   bool   `json:"newer"`
	URL     string `json:"url"`
}
