package anilist

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stdhtml "html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
)

const (
	graphqlURL           = "https://graphql.anilist.co"
	coverHost            = "s4.anilist.co"
	providerID           = "anilist"
	maxSearchResults     = 10
	maxResponseBodyBytes = 1 << 20
	maxTitleTextBytes    = 4 << 10
	maxTitleTextRunes    = 1024
	maxDescriptionBytes  = 64 << 10
	maxDescriptionRunes  = 16384
	maxCoverURLBytes     = 8 << 10
	maxMetadataIDDigits  = 10
	maxStudioTextBytes   = 1 << 10
	maxJSONNestingDepth  = 64
	maxSeasonYear        = 3000
	maxEpisodeCount      = 100000
	searchDocument       = `query ($search: String!) { Page(page: 1, perPage: 10) { media(search: $search, type: ANIME) { id title { romaji native } seasonYear episodes } } }`
	getDocument          = `query ($id: Int!) { Media(id: $id, type: ANIME) { id title { romaji native } seasonYear episodes description coverImage { extraLarge large medium } season studios(isMain: true) { nodes { name isAnimationStudio } } } }`
)

var markdownLinkPattern = regexp.MustCompile(`\[([^\[\]\r\n]*)\]\([^()\r\n]*\)`)

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
		if !validSeason(*media.Season) {
			return core.AnimeMetadata{}, false
		}
		metadata.Season = *media.Season
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
	if len(*description) > maxDescriptionBytes || !utf8.ValidString(*description) {
		return "", false
	}
	plain := stripHTML(stdhtml.UnescapeString(*description))
	plain = stripHTML(plain)
	plain = strings.NewReplacer("<", " ", ">", " ").Replace(plain)
	plain = strings.TrimSpace(stripMarkdown(plain))
	if len(plain) > maxDescriptionBytes || utf8.RuneCountInString(plain) > maxDescriptionRunes || containsDisallowedControl(plain) {
		return "", false
	}
	return strings.Join(strings.Fields(plain), " "), true
}

func mapCoverURL(image coverImage) string {
	for _, value := range []*string{image.ExtraLarge, image.Large, image.Medium} {
		if value == nil || *value == "" {
			continue
		}
		if len(*value) <= maxCoverURLBytes && validCoverURL(*value) {
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
		name, ok := boundedText(studio.Name, maxStudioTextBytes, maxTitleTextRunes)
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
	if len(*value) > maxBytes || utf8.RuneCountInString(*value) > maxRunes || !utf8.ValidString(*value) || containsDisallowedControl(*value) {
		return "", false
	}
	return strings.TrimSpace(*value), true
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
	if len(value) == 0 || !utf8.ValidString(value) {
		return false
	}
	target, err := url.Parse(value)
	if err != nil || target == nil || !strings.EqualFold(target.Scheme, "https") || target.Opaque != "" || target.User != nil || target.Fragment != "" || target.Host == "" || target.Port() != "" {
		return false
	}
	return strings.EqualFold(target.Hostname(), coverHost)
}

func stripHTML(value string) string {
	var result strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(value))
	skippedElement := ""
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return result.String()
		case html.StartTagToken:
			name, _ := tokenizer.TagName()
			if skippedElement == "" && (string(name) == "script" || string(name) == "style") {
				skippedElement = string(name)
			}
			result.WriteByte(' ')
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if string(name) == skippedElement {
				skippedElement = ""
			}
			result.WriteByte(' ')
		case html.TextToken:
			if skippedElement == "" {
				result.Write(tokenizer.Text())
			}
		}
	}
}

func stripMarkdown(value string) string {
	value = stripMarkdownLinks(value)
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		for len(trimmed) > 0 && trimmed[0] == '#' {
			trimmed = trimmed[1:]
		}
		if len(trimmed) > 0 && (trimmed[0] == '>' || trimmed[0] == '-' || trimmed[0] == '+' || trimmed[0] == '*') {
			trimmed = trimmed[1:]
		}
		lines[index] = trimmed
	}
	value = strings.Join(lines, " ")
	var result strings.Builder
	for index := 0; index < len(value); index++ {
		switch value[index] {
		case '*', '_', '~', '`':
		case '\\':
			if index+1 < len(value) && strings.ContainsRune("\\`*_{}[]()#+-.!~", rune(value[index+1])) {
				continue
			}
			result.WriteByte(value[index])
		default:
			result.WriteByte(value[index])
		}
	}
	return result.String()
}

func stripMarkdownLinks(value string) string {
	return markdownLinkPattern.ReplaceAllString(value, "$1")
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
	if len(body) == 0 || len(body) > maxResponseBodyBytes || !jsonObjectWithoutDuplicateKeys(body) {
		return errMetadataMalformed
	}
	if err := json.Unmarshal(body, value); err != nil {
		return errMetadataMalformed
	}
	return nil
}

func jsonObjectWithoutDuplicateKeys(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := scanJSONValue(decoder, true, 0); err != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

func scanJSONValue(decoder *json.Decoder, requireObject bool, depth int) error {
	if depth > maxJSONNestingDepth {
		return errMetadataMalformed
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if requireObject && (!isDelimiter || delimiter != '{') {
		return errMetadataMalformed
	}
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errMetadataMalformed
			}
			if _, exists := seen[key]; exists {
				return errMetadataMalformed
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, false, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errMetadataMalformed
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, false, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errMetadataMalformed
		}
	default:
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
