// SPDX-License-Identifier: MPL-2.0

package core

import (
	"context"
	"net/http"
	"time"
)

type PlaybackSource struct {
	url     string
	headers http.Header
}

func NewPlaybackSource(url string, headers http.Header) PlaybackSource {
	return PlaybackSource{url: url, headers: headers.Clone()}
}

func (source PlaybackSource) URL() string {
	return source.url
}

func (source PlaybackSource) Headers() http.Header {
	return source.headers.Clone()
}

func (PlaybackSource) String() string {
	return "PlaybackSource{redacted}"
}

func (PlaybackSource) GoString() string {
	return "PlaybackSource{redacted}"
}

type PlayRequest struct {
	AnimeID   AnimeID
	EpisodeID EpisodeID
	Source    PlaybackSource
	StartAt   time.Duration
}

type Player interface {
	Start(ctx context.Context, req PlayRequest) (PlaybackSession, error)
}

type PlaybackSession interface {
	Load(ctx context.Context, req PlayRequest) error
	Events() <-chan PlaybackEvent
	Close() error
}

type PlaybackSnapshotter interface {
	PlaybackSession
	Snapshot(ctx context.Context) (PlaybackSnapshot, error)
}
