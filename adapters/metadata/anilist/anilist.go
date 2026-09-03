// SPDX-License-Identifier: MPL-2.0

package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	metadatainternal "animeportable/adapters/metadata/internal"
	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	graphqlURL           = "https://graphql.anilist.co"
	coverHost            = "s4.anilist.co"
	providerID           = "anilist"
	maxSearchResults     = 10
	maxResponseBodyBytes = 1 << 20
	maxTitleTextBytes    = metadatapolicy.MaxTitleTextBytes
	maxTitleTextRunes    = metadatapolicy.MaxTitleTextRunes
	maxDescriptionBytes  = metadatapolicy.MaxDescriptionTextBytes
	maxDescriptionRunes  = metadatapolicy.MaxDescriptionTextRunes
	maxCoverURLBytes     = metadatapolicy.MaxCoverURLBytes
	maxMetadataIDDigits  = 10
	maxStudioTextBytes   = metadatapolicy.MaxStudioTextBytes
	maxJSONNestingDepth  = 64
	maxSeasonYear        = 3000
	maxEpisodeCount      = 100000
	searchDocument       = `query ($search: String!) { Page(page: 1, perPage: 10) { media(search: $search, type: ANIME) { id title { romaji native } seasonYear episodes } } }`
	getDocument          = `query ($id: Int!) { Media(id: $id, type: ANIME) { id title { romaji native } seasonYear episodes description coverImage { extraLarge large medium } season studios(isMain: true) { nodes { name isAnimationStudio } } } }`
)

var (
	errMetadataUnavailable  = errors.New("anilist: metadata unavailable")
	errMetadataMalformed    = errors.New("anilist: malformed metadata response")
	errMetadataInvalidQuery = errors.New("anilist: invalid metadata query")
	errMetadataInvalidRef   = errors.New("anilist: invalid metadata reference")
	errMetadataProvider     = errors.New("anilist: provider error")
)

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
	if !validSearchQuery(search) {
		return nil, errMetadataInvalidQuery
	}
	response, err := client.doRequest(ctx, searchDocument, struct {
		Search string `json:"search"`
	}{Search: search}, false)
	if err != nil {
		return nil, err
	}
	var payload searchResponse
	if err := decodeResponse(response.Body, &payload); err != nil {
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
	id, numericID, ok := parseMetadataID(ref.ID)
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
	if err := decodeResponse(response.Body, &payload); err != nil {
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
	if err != nil {
		return nil, sanitizeRequestError(err)
	}
	if response == nil {
		return nil, errMetadataUnavailable
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusNotFound && notFoundIsMissing {
		return nil, core.ErrNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, errMetadataUnavailable
	}
	if len(response.Body) > maxResponseBodyBytes || !isJSONResponse(response.Header.Get("Content-Type")) {
		return nil, errMetadataMalformed
	}
	return response, nil
}

func mapCandidate(media mediaResponse) (core.MetadataCandidate, bool) {
	id, _, ok := parseJSONMetadataID(media.ID)
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
	id, _, ok := parseJSONMetadataID(media.ID)
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
	romaji, ok := boundedText(title.Romaji, maxTitleTextBytes, maxTitleTextRunes)
	if !ok {
		return "", "", false
	}
	native, ok := boundedText(title.Native, maxTitleTextBytes, maxTitleTextRunes)
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
		if validCoverURL(*value) {
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
		name, ok := boundedText(studio.Name, maxStudioTextBytes, metadatapolicy.MaxStudioTextRunes)
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

func boundedText(value *string, maxBytes, maxRunes int) (string, bool) {
	if value == nil {
		return "", true
	}
	return metadatapolicy.NormalizePlainText(*value, metadatapolicy.PlainTextLimits{MaxInputBytes: maxBytes, MaxOutputBytes: maxBytes, MaxOutputRunes: maxRunes})
}

func validSearchQuery(value string) bool {
	return value != "" && len(value) <= maxTitleTextBytes && utf8.RuneCountInString(value) <= maxTitleTextRunes && utf8.ValidString(value) && !containsDisallowedControl(value)
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

func parseMetadataID(value string) (string, int, bool) {
	if len(value) == 0 || len(value) > maxMetadataIDDigits || value[0] == '0' {
		return "", 0, false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", 0, false
		}
	}
	numeric, err := strconv.ParseInt(value, 10, 32)
	if err != nil || numeric <= 0 {
		return "", 0, false
	}
	return value, int(numeric), true
}

func parseJSONMetadataID(value json.RawMessage) (string, int, bool) {
	if len(value) == 0 {
		return "", 0, false
	}
	return parseMetadataID(string(value))
}

func validCoverURL(value string) bool {
	if value == "" || !metadatapolicy.IsSafeCoverURL(value) {
		return false
	}
	target, err := url.Parse(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(target.Hostname(), coverHost)
}

func containsDisallowedControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return true
		}
	}
	return false
}

func isJSONResponse(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errMetadataUnavailable
	}
	return ctx.Err()
}

func sanitizeRequestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var secureError *securehttp.Error
	if errors.As(err, &secureError) {
		return secureError
	}
	return errMetadataUnavailable
}

func decodeResponse(body []byte, value any) error {
	if !metadatainternal.DecodeObject(body, value, metadatainternal.JSONLimits{
		MaxBytes:        maxResponseBodyBytes,
		MaxNestingDepth: maxJSONNestingDepth,
	}) {
		return errMetadataMalformed
	}
	return nil
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

var _ core.MetadataProvider = (*Client)(nil)
