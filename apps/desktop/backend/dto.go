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
	Anime    Anime     `json:"anime"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

type Following struct {
	AnimeID         string `json:"animeId"`
	LatestAvailable string `json:"latestAvailable"`
	LatestWatched   string `json:"latestWatched"`
	HasAvailable    bool   `json:"hasAvailable"`
	HasWatched      bool   `json:"hasWatched"`
	NewEpisode      bool   `json:"newEpisode"`
}

type History struct {
	AnimeID    string `json:"animeId"`
	EpisodeID  string `json:"episodeId"`
	Position   int64  `json:"position"`
	Duration   int64  `json:"duration"`
	Completed  bool   `json:"completed"`
	UpdatedAt  string `json:"updatedAt"`
	LastPlayed string `json:"lastPlayed"`
}

type Settings struct {
	Appearance     string `json:"appearance"`
	MPVPath        string `json:"mpvPath"`
	AutoplayNext   string `json:"autoplayNext"`
	ResumePlayback string `json:"resumePlayback"`
	Language       string `json:"language"`
}

type ScheduleItem struct {
	AnimeID   string `json:"animeId"`
	EpisodeID string `json:"episodeId"`
	AirsAt    string `json:"airsAt"`
	Precision string `json:"precision"`
}

type PlayRequest struct {
	AnimeID   string `json:"animeId"`
	EpisodeID string `json:"episodeId"`
	StartAt   int64  `json:"startAt"`
}

type Cover struct {
	Bytes     []byte `json:"bytes"`
	MediaType string `json:"mediaType"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}
