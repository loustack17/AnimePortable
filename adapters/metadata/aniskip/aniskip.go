// SPDX-License-Identifier: MPL-2.0

// Package aniskip asks the AniSkip API for community opening times. A request carries only the
// MyAnimeList id, the episode number and the episode length (docs/03_SECURITY.md).
package aniskip

import (
	"context"
	"errors"
	"fmt"
	"math"
	"mime"
	"net/http"
	"strings"
	"time"

	metadatainternal "animeportable/adapters/metadata/internal"
	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"animeportable/internal/release"
)

const (
	apiOrigin = "https://api.aniskip.com"
	// MaxResponseBytes bounds the answer the client reads.
	MaxResponseBytes = 64 << 10
	maxResults       = 20
	// maxJSONNestingDepth is well above the answer's own depth of four.
	maxJSONNestingDepth = 16
	// maxEpisodeSeconds bounds the length sent; no episode is longer than a day.
	maxEpisodeSeconds = 24 * 60 * 60
)

var errUnavailable = errors.New("aniskip: unavailable")

// AllowedOrigins is the only origin the opening lookup may contact.
func AllowedOrigins() []string { return []string{apiOrigin} }

type responseClient interface {
	Do(*http.Request) (*securehttp.Response, error)
}

type Client struct {
	do      responseClient
	version string
}

// New returns a Client that identifies itself with the app version.
func New(client *securehttp.Client) *Client { return newWithDo(client, release.Version) }

func newWithDo(do responseClient, version string) *Client {
	return &Client{do: do, version: version}
}

type skipTimes struct {
	Found   bool         `json:"found"`
	Results []skipResult `json:"results"`
}

type skipResult struct {
	Interval struct {
		StartTime *float64 `json:"startTime"`
		EndTime   *float64 `json:"endTime"`
	} `json:"interval"`
	SkipType      string   `json:"skipType"`
	EpisodeLength *float64 `json:"episodeLength"`
}

// Openings returns the opening submissions for one episode of length. "Not found" is no
// submissions, not an error; a malformed answer is an error.
func (client *Client) Openings(ctx context.Context, myAnimeListID, episode int, length time.Duration) ([]core.OpeningCandidate, error) {
	if client == nil || client.do == nil || ctx == nil {
		return nil, errUnavailable
	}
	seconds := int(math.Round(length.Seconds()))
	if myAnimeListID <= 0 || episode <= 0 || seconds <= 0 || seconds > maxEpisodeSeconds {
		return nil, errUnavailable
	}
	address := fmt.Sprintf("%s/v2/skip-times/%d/%d?types=op&types=mixed-op&episodeLength=%d", apiOrigin, myAnimeListID, episode, seconds)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, errUnavailable
	}
	request.Header.Set("User-Agent", "AnimePortable/"+client.version)
	request.Header.Set("Accept", "application/json")
	response, err := client.do.Do(request)
	if err != nil {
		return nil, err
	}
	// AniSkip answers 404 when it has no times for the episode.
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if err := response.RequireSuccess(); err != nil {
		return nil, err
	}
	if !isJSON(response.Header.Get("Content-Type")) {
		return nil, errUnavailable
	}
	var payload skipTimes
	if !metadatainternal.DecodeObject(response.Body, &payload, metadatainternal.JSONLimits{MaxBytes: MaxResponseBytes, MaxNestingDepth: maxJSONNestingDepth}) || len(payload.Results) > maxResults {
		return nil, errUnavailable
	}
	if !payload.Found {
		return nil, nil
	}
	candidates := make([]core.OpeningCandidate, 0, len(payload.Results))
	for _, result := range payload.Results {
		if candidate, ok := openingCandidate(result); ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates, nil
}

// openingCandidate converts one result; anything but a complete opening of finite times is
// dropped. core.ChooseOpening checks the times against the episode.
func openingCandidate(result skipResult) (core.OpeningCandidate, bool) {
	if result.SkipType != "op" && result.SkipType != "mixed-op" {
		return core.OpeningCandidate{}, false
	}
	start, startOK := seconds(result.Interval.StartTime)
	end, endOK := seconds(result.Interval.EndTime)
	length, lengthOK := seconds(result.EpisodeLength)
	if !startOK || !endOK || !lengthOK {
		return core.OpeningCandidate{}, false
	}
	return core.OpeningCandidate{Opening: core.Opening{Start: start, End: end}, EpisodeLength: length}, true
}

func seconds(value *float64) (time.Duration, bool) {
	if value == nil || math.IsNaN(*value) || *value < 0 || *value > maxEpisodeSeconds {
		return 0, false
	}
	return time.Duration(*value * float64(time.Second)), true
}

func isJSON(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

var _ core.OpeningSource = (*Client)(nil)
