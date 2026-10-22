// SPDX-License-Identifier: MPL-2.0

// Package github asks the GitHub API for the newest published release of the app. It only
// reads; it never downloads or runs a release.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"animeportable/adapters/network/securehttp"
	"animeportable/internal/release"
)

const (
	apiOrigin   = "https://api.github.com"
	releasesURL = apiOrigin + "/repos/loustack17/AnimePortable/releases?per_page=20"
	// MaxResponseBytes bounds the release list the client reads.
	MaxResponseBytes = 2 << 20
)

var errUnavailable = errors.New("github releases: unavailable")

// AllowedOrigins is the only origin the update check may contact.
func AllowedOrigins() []string { return []string{apiOrigin} }

// Release is a published release. Tag is the git tag as published; URL is its GitHub page, or
// empty when GitHub returned anything but a page of this repository's releases.
type Release struct {
	Tag string
	URL string
}

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

type apiRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	HTMLURL string `json:"html_url"`
}

// Newest returns the release with the highest version, pre-releases included and drafts
// ignored. found is false when no release has a version tag.
func (client *Client) Newest(ctx context.Context) (newest Release, found bool, err error) {
	if client == nil || client.do == nil || ctx == nil {
		return Release{}, false, errUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return Release{}, false, errUnavailable
	}
	request.Header.Set("User-Agent", "AnimePortable/"+client.version)
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.do.Do(request)
	if err != nil {
		return Release{}, false, err
	}
	if err := response.RequireSuccess(); err != nil {
		return Release{}, false, err
	}
	var releases []apiRelease
	if err := json.Unmarshal(response.Body, &releases); err != nil {
		return Release{}, false, errUnavailable
	}
	var best release.Parsed
	for _, item := range releases {
		version, ok := release.Parse(item.TagName)
		if item.Draft || !ok {
			continue
		}
		if !found || version.Compare(best) > 0 {
			best, newest, found = version, Release{Tag: item.TagName, URL: releasePage(item.HTMLURL)}, true
		}
	}
	return newest, found, nil
}

// releasePage keeps a URL only when it is a page of this repository's releases.
func releasePage(value string) string {
	page, _ := release.PageURL(value)
	return page
}
