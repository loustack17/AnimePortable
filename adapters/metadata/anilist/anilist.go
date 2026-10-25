// SPDX-License-Identifier: MPL-2.0

package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	metadatainternal "animeportable/adapters/metadata/internal"
	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	graphqlURL           = "https://graphql.anilist.co"
	coverHost            = metadatapolicy.AniListCoverHost
	providerID           = "anilist"
	maxSearchResults     = 10
	maxResponseBodyBytes = 1 << 20
	maxJSONNestingDepth  = 64
	maxSeasonYear        = 3000
	maxEpisodeCount      = 100000
	searchDocument       = `query ($search: String!) { Page(page: 1, perPage: 10) { media(search: $search, type: ANIME) { id title { romaji native } seasonYear episodes } } }`
	malDocument          = `query ($id: Int!) { Media(id: $id, type: ANIME) { id idMal } }`
	maxMyAnimeListID     = 1<<31 - 1
	getDocument          = `query ($id: Int!) { Media(id: $id, type: ANIME) { id title { romaji native } seasonYear episodes description coverImage { extraLarge large medium } season studios(isMain: true) { nodes { name isAnimationStudio } } } }`
)

var (
	errMetadataUnavailable  = errors.New("anilist: metadata unavailable")
	errMetadataMalformed    = errors.New("anilist: malformed metadata response")
	errMetadataInvalidQuery = errors.New("anilist: invalid metadata query")
	errMetadataInvalidRef   = errors.New("anilist: invalid metadata reference")
	errMetadataProvider     = errors.New("anilist: provider error")
)

// responseRules are the answer limits and errors shared with the other metadata provider's
// checks in adapters/metadata/internal.
var responseRules = metadatainternal.Rules{
	MaxBodyBytes:    maxResponseBodyBytes,
	MaxNestingDepth: maxJSONNestingDepth,
	Unavailable:     errMetadataUnavailable,
	Malformed:       errMetadataMalformed,
}

type responseClient interface {
	Do(*http.Request) (*securehttp.Response, error)
}

type Client struct {
	do responseClient
}

func New(client *securehttp.Client) *Client {
	return newWithDo(client)
}

func newWithDo(do responseClient) *Client {
	return &Client{do: do}
}

func AllowedOrigins() []string {
	return []string{graphqlURL}
}

func (client *Client) Search(ctx context.Context, query core.MetadataQuery) ([]core.MetadataCandidate, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	search := strings.TrimSpace(query.Title)
	if search == "" {
		search = strings.TrimSpace(query.NativeTitle)
	}
	if !metadatainternal.ValidSearchQuery(search) {
		return nil, errMetadataInvalidQuery
	}
	response, err := client.doRequest(ctx, searchDocument, struct {
		Search string `json:"search"`
	}{Search: search}, false)
	if err != nil {
		return nil, err
	}
	var payload searchResponse
	if err := responseRules.Decode(response.Body, &payload); err != nil {
		return nil, err
	}
	if len(payload.Errors) > 0 {
		return nil, errMetadataProvider
	}
	if payload.Data == nil || payload.Data.Page == nil {
		return nil, errMetadataMalformed
	}
	if payload.Data.Page.Media == nil {
		return []core.MetadataCandidate{}, nil
	}
	if len(*payload.Data.Page.Media) > maxSearchResults {
		return nil, errMetadataMalformed
	}
	results := make([]core.MetadataCandidate, 0, len(*payload.Data.Page.Media))
	for _, media := range *payload.Data.Page.Media {
		candidate, ok := mapCandidate(media)
		if !ok {
			return nil, errMetadataMalformed
		}
		results = append(results, candidate)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (client *Client) Get(ctx context.Context, ref core.MetadataRef) (core.AnimeMetadata, error) {
	if err := checkContext(ctx); err != nil {
		return core.AnimeMetadata{}, err
	}
	if ref.Provider != providerID {
		return core.AnimeMetadata{}, errMetadataInvalidRef
	}
	id, numericID, ok := metadatainternal.ParseID(ref.ID)
	if !ok {
		return core.AnimeMetadata{}, errMetadataInvalidRef
	}
	response, err := client.doRequest(ctx, getDocument, struct {
		ID int `json:"id"`
	}{ID: numericID}, true)
	if err != nil {
		return core.AnimeMetadata{}, err
	}
	var payload getResponse
	if err := responseRules.Decode(response.Body, &payload); err != nil {
		return core.AnimeMetadata{}, err
	}
	if len(payload.Errors) > 0 {
		return core.AnimeMetadata{}, errMetadataProvider
	}
	if payload.Data == nil || payload.Data.Media == nil {
		return core.AnimeMetadata{}, core.ErrNotFound
	}
	metadata, ok := mapMetadata(*payload.Data.Media)
	if !ok || metadata.Ref.ID != id {
		return core.AnimeMetadata{}, errMetadataMalformed
	}
	if err := checkContext(ctx); err != nil {
		return core.AnimeMetadata{}, err
	}
	return metadata, nil
}

// MyAnimeListID returns the MyAnimeList id of an AniList anime; core.ErrNotFound means AniList
// has none.
func (client *Client) MyAnimeListID(ctx context.Context, ref core.MetadataRef) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	if ref.Provider != providerID {
		return 0, errMetadataInvalidRef
	}
	id, numericID, ok := metadatainternal.ParseID(ref.ID)
	if !ok {
		return 0, errMetadataInvalidRef
	}
	response, err := client.doRequest(ctx, malDocument, struct {
		ID int `json:"id"`
	}{ID: numericID}, true)
	if err != nil {
		return 0, err
	}
	var payload malResponse
	if err := responseRules.Decode(response.Body, &payload); err != nil {
		return 0, err
	}
	if len(payload.Errors) > 0 {
		return 0, errMetadataProvider
	}
	if payload.Data == nil || payload.Data.Media == nil {
		return 0, core.ErrNotFound
	}
	media := payload.Data.Media
	if mediaID, _, ok := metadatainternal.ParseJSONID(media.ID); !ok || mediaID != id {
		return 0, errMetadataMalformed
	}
	if media.MyAnimeListID == nil {
		return 0, core.ErrNotFound
	}
	if *media.MyAnimeListID <= 0 || *media.MyAnimeListID > maxMyAnimeListID {
		return 0, errMetadataMalformed
	}
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	return *media.MyAnimeListID, nil
}

func (client *Client) doRequest(ctx context.Context, document string, variables any, notFoundIsMissing bool) (*securehttp.Response, error) {
	if client == nil || client.do == nil {
		return nil, errMetadataUnavailable
	}
	body, err := json.Marshal(struct {
		Query     string `json:"query"`
		Variables any    `json:"variables"`
	}{Query: document, Variables: variables})
	if err != nil {
		return nil, errMetadataUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, graphqlURL, bytes.NewReader(body))
	if err != nil {
		return nil, errMetadataUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := client.do.Do(request)
	return responseRules.CheckResponse(ctx, response, err, notFoundIsMissing)
}

func mapCandidate(media mediaResponse) (core.MetadataCandidate, bool) {
	id, _, ok := metadatainternal.ParseJSONID(media.ID)
	if !ok {
		return core.MetadataCandidate{}, false
	}
	title, nativeTitle, ok := mapTitles(media.Title)
	if !ok || !validOptionalNumber(media.SeasonYear, maxSeasonYear) || !validOptionalNumber(media.Episodes, maxEpisodeCount) {
		return core.MetadataCandidate{}, false
	}
	candidate := core.MetadataCandidate{
		Ref:          core.MetadataRef{Provider: providerID, ID: id},
		Title:        title,
		NativeTitle:  nativeTitle,
		Year:         numericValue(media.SeasonYear),
		EpisodeCount: numericValue(media.Episodes),
	}
	return candidate, true
}

func mapMetadata(media mediaResponse) (core.AnimeMetadata, bool) {
	id, _, ok := metadatainternal.ParseJSONID(media.ID)
	if !ok {
		return core.AnimeMetadata{}, false
	}
	title, nativeTitle, ok := mapTitles(media.Title)
	if !ok || !validOptionalNumber(media.SeasonYear, maxSeasonYear) || !validOptionalNumber(media.Episodes, maxEpisodeCount) {
		return core.AnimeMetadata{}, false
	}
	description, ok := mapDescription(media.Description)
	if !ok {
		return core.AnimeMetadata{}, false
	}
	metadata := core.AnimeMetadata{
		Ref:          core.MetadataRef{Provider: providerID, ID: id},
		Title:        title,
		NativeTitle:  nativeTitle,
		Description:  description,
		Year:         numericValue(media.SeasonYear),
		EpisodeCount: numericValue(media.Episodes),
	}
	if media.Season != nil {
		season, ok := metadatapolicy.NormalizePlainText(*media.Season, metadatapolicy.SeasonLimits())
		if !ok || !validSeason(season) {
			return core.AnimeMetadata{}, false
		}
		metadata.Season = season
	}
	if media.CoverImage != nil {
		metadata.CoverURL = mapCoverURL(*media.CoverImage)
	}
	if media.Studios != nil {
		metadata.Studio = mapStudio(*media.Studios)
	}
	return metadata, true
}

func mapTitles(title *mediaTitle) (string, string, bool) {
	if title == nil {
		return "", "", true
	}
	romaji, ok := metadatainternal.BoundedText(title.Romaji, metadatapolicy.TitleLimits())
	if !ok {
		return "", "", false
	}
	native, ok := metadatainternal.BoundedText(title.Native, metadatapolicy.TitleLimits())
	if !ok {
		return "", "", false
	}
	return romaji, native, true
}

func mapDescription(description *string) (string, bool) {
	if description == nil {
		return "", true
	}
	return metadatapolicy.NormalizePlainText(*description, metadatapolicy.DescriptionLimits())
}

func mapCoverURL(image coverImage) string {
	for _, value := range []*string{image.ExtraLarge, image.Large, image.Medium} {
		if value == nil || *value == "" {
			continue
		}
		if metadatainternal.ValidCoverURL(*value, coverHost) {
			return *value
		}
		return ""
	}
	return ""
}

func mapStudio(studios studioConnection) string {
	for _, studio := range studios.Nodes {
		if !studio.IsAnimationStudio || studio.Name == nil {
			continue
		}
		name, ok := metadatainternal.BoundedText(studio.Name, metadatapolicy.StudioLimits())
		if ok && name != "" {
			return name
		}
	}
	return ""
}

func validSeason(season string) bool {
	switch season {
	case "WINTER", "SPRING", "SUMMER", "FALL":
		return true
	default:
		return false
	}
}

func validOptionalNumber(value *int, maximum int) bool {
	return value == nil || *value >= 0 && *value <= maximum
}

func numericValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errMetadataUnavailable
	}
	return ctx.Err()
}

type graphqlErrors struct {
	Errors []json.RawMessage `json:"errors"`
}

type searchResponse struct {
	graphqlErrors
	Data *searchData `json:"data"`
}

type searchData struct {
	Page *searchPage `json:"Page"`
}

type searchPage struct {
	Media *[]mediaResponse `json:"media"`
}

type getResponse struct {
	graphqlErrors
	Data *getData `json:"data"`
}

type getData struct {
	Media *mediaResponse `json:"Media"`
}

type mediaResponse struct {
	ID          json.RawMessage   `json:"id"`
	Title       *mediaTitle       `json:"title"`
	SeasonYear  *int              `json:"seasonYear"`
	Episodes    *int              `json:"episodes"`
	Description *string           `json:"description"`
	CoverImage  *coverImage       `json:"coverImage"`
	Season      *string           `json:"season"`
	Studios     *studioConnection `json:"studios"`
}

type malResponse struct {
	graphqlErrors
	Data *struct {
		Media *struct {
			ID            json.RawMessage `json:"id"`
			MyAnimeListID *int            `json:"idMal"`
		} `json:"Media"`
	} `json:"data"`
}

type mediaTitle struct {
	Romaji *string `json:"romaji"`
	Native *string `json:"native"`
}

type coverImage struct {
	ExtraLarge *string `json:"extraLarge"`
	Large      *string `json:"large"`
	Medium     *string `json:"medium"`
}

type studioConnection struct {
	Nodes []studioNode `json:"nodes"`
}

type studioNode struct {
	Name              *string `json:"name"`
	IsAnimationStudio bool    `json:"isAnimationStudio"`
}

var (
	_ core.MetadataProvider  = (*Client)(nil)
	_ core.MyAnimeListSource = (*Client)(nil)
)
