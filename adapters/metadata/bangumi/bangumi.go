// SPDX-License-Identifier: MPL-2.0

package bangumi

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
	"time"
	"unicode"
	"unicode/utf8"

	metadatainternal "animeportable/adapters/metadata/internal"
	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	apiOrigin            = "https://api.bgm.tv"
	searchURL            = apiOrigin + "/v0/search/subjects?limit=10&offset=0"
	userAgent            = "loustack17/AnimePortable/0.0.1 (https://github.com/loustack17/AnimePortable)"
	coverHost            = "lain.bgm.tv"
	providerID           = "bangumi"
	maxSearchResults     = 10
	maxResponseBodyBytes = 1 << 20
	maxTitleTextBytes    = metadatapolicy.MaxTitleTextBytes
	maxTitleTextRunes    = metadatapolicy.MaxTitleTextRunes
	maxDescriptionBytes  = metadatapolicy.MaxDescriptionTextBytes
	maxDescriptionRunes  = metadatapolicy.MaxDescriptionTextRunes
	maxCoverURLBytes     = metadatapolicy.MaxCoverURLBytes
	maxJSONNestingDepth  = 64
	maxEpisodeCount      = 100000
	maxYear              = 3000
	maxMetadataIDDigits  = 10
)

var (
	errMetadataUnavailable  = errors.New("bangumi: metadata unavailable")
	errMetadataMalformed    = errors.New("bangumi: malformed metadata response")
	errMetadataInvalidQuery = errors.New("bangumi: invalid metadata query")
	errMetadataInvalidRef   = errors.New("bangumi: invalid metadata reference")
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
	return []string{apiOrigin}
}

func (client *Client) Search(ctx context.Context, query core.MetadataQuery) ([]core.MetadataCandidate, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(query.Title)
	if keyword == "" {
		keyword = strings.TrimSpace(query.NativeTitle)
	}
	if !validSearchQuery(keyword) {
		return nil, errMetadataInvalidQuery
	}
	payload := struct {
		Keyword string `json:"keyword"`
		Sort    string `json:"sort"`
		Filter  struct {
			Type []int `json:"type"`
		} `json:"filter"`
	}{Keyword: keyword, Sort: "match"}
	payload.Filter.Type = []int{2}
	response, err := client.doRequest(ctx, http.MethodPost, searchURL, payload, false)
	if err != nil {
		return nil, err
	}
	var result searchResponse
	if err := decodeResponse(response.Body, &result); err != nil {
		return nil, err
	}
	if len(result.Data) > maxSearchResults {
		return nil, errMetadataMalformed
	}
	candidates := make([]core.MetadataCandidate, 0, len(result.Data))
	for _, subject := range result.Data {
		candidate, ok := mapCandidate(subject)
		if !ok {
			return nil, errMetadataMalformed
		}
		candidates = append(candidates, candidate)
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return candidates, nil
}

func (client *Client) Get(ctx context.Context, ref core.MetadataRef) (core.AnimeMetadata, error) {
	if err := checkContext(ctx); err != nil {
		return core.AnimeMetadata{}, err
	}
	if ref.Provider != providerID {
		return core.AnimeMetadata{}, errMetadataInvalidRef
	}
	id, _, ok := parseMetadataID(ref.ID)
	if !ok {
		return core.AnimeMetadata{}, errMetadataInvalidRef
	}
	response, err := client.doRequest(ctx, http.MethodGet, apiOrigin+"/v0/subjects/"+id, nil, true)
	if err != nil {
		return core.AnimeMetadata{}, err
	}
	var subject subjectResponse
	if err := decodeResponse(response.Body, &subject); err != nil {
		return core.AnimeMetadata{}, err
	}
	metadata, ok := mapMetadata(subject)
	if !ok || metadata.Ref.ID != id {
		return core.AnimeMetadata{}, errMetadataMalformed
	}
	if err := checkContext(ctx); err != nil {
		return core.AnimeMetadata{}, err
	}
	return metadata, nil
}

func (client *Client) doRequest(ctx context.Context, method, target string, body any, notFoundIsMissing bool) (*securehttp.Response, error) {
	if client == nil || client.do == nil {
		return nil, errMetadataUnavailable
	}
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, errMetadataUnavailable
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, errMetadataUnavailable
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
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

func mapCandidate(subject subjectResponse) (core.MetadataCandidate, bool) {
	mapped, ok := mapSubject(subject)
	if !ok {
		return core.MetadataCandidate{}, false
	}
	return core.MetadataCandidate{
		Ref:          core.MetadataRef{Provider: providerID, ID: mapped.id},
		Title:        mapped.title,
		NativeTitle:  mapped.nativeTitle,
		Year:         mapped.year,
		EpisodeCount: mapped.episodeCount,
	}, true
}

func mapMetadata(subject subjectResponse) (core.AnimeMetadata, bool) {
	mapped, ok := mapSubject(subject)
	if !ok {
		return core.AnimeMetadata{}, false
	}
	description, ok := mapDescription(subject.Summary)
	if !ok {
		return core.AnimeMetadata{}, false
	}
	return core.AnimeMetadata{
		Ref:          core.MetadataRef{Provider: providerID, ID: mapped.id},
		Title:        mapped.title,
		NativeTitle:  mapped.nativeTitle,
		Description:  description,
		CoverURL:     mapCoverURL(subject.Images),
		Year:         mapped.year,
		EpisodeCount: mapped.episodeCount,
	}, true
}

type mappedSubject struct {
	id           string
	title        string
	nativeTitle  string
	year         int
	episodeCount int
}

func mapSubject(subject subjectResponse) (mappedSubject, bool) {
	id, _, ok := parseJSONMetadataID(subject.ID)
	if !ok || subject.Type == nil || *subject.Type != 2 {
		return mappedSubject{}, false
	}
	title, nativeTitle, ok := mapTitles(subject.NameCN, subject.Name)
	if !ok {
		return mappedSubject{}, false
	}
	year, ok := mapDate(subject.Date)
	if !ok || !validEpisodeCount(subject.Eps) || !validEpisodeCount(subject.TotalEpisodes) {
		return mappedSubject{}, false
	}
	return mappedSubject{
		id:           id,
		title:        title,
		nativeTitle:  nativeTitle,
		year:         year,
		episodeCount: episodeCount(subject.TotalEpisodes, subject.Eps),
	}, true
}

func mapTitles(nameCN, name *string) (string, string, bool) {
	chinese, ok := boundedText(nameCN, maxTitleTextBytes, maxTitleTextRunes)
	if !ok {
		return "", "", false
	}
	native, ok := boundedText(name, maxTitleTextBytes, maxTitleTextRunes)
	if !ok || chinese == "" && native == "" {
		return "", "", false
	}
	if chinese == "" {
		chinese = native
	}
	return chinese, native, true
}

func mapDescription(summary *string) (string, bool) {
	if summary == nil {
		return "", true
	}
	return metadatapolicy.NormalizePlainText(*summary, metadatapolicy.DescriptionLimits())
}

func mapCoverURL(images *subjectImages) string {
	if images == nil {
		return ""
	}
	for _, value := range []*string{images.Large, images.Common} {
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

func boundedText(value *string, maxBytes, maxRunes int) (string, bool) {
	if value == nil {
		return "", true
	}
	return metadatapolicy.NormalizePlainText(*value, metadatapolicy.PlainTextLimits{MaxInputBytes: maxBytes, MaxOutputBytes: maxBytes, MaxOutputRunes: maxRunes})
}

func validSearchQuery(value string) bool {
	return value != "" && len(value) <= maxTitleTextBytes && utf8.RuneCountInString(value) <= maxTitleTextRunes && utf8.ValidString(value) && !containsDisallowedControl(value)
}

func validEpisodeCount(value *int) bool {
	return value == nil || *value >= 0 && *value <= maxEpisodeCount
}

func episodeCount(total, eps *int) int {
	if total != nil && *total > 0 {
		return *total
	}
	if eps != nil {
		return *eps
	}
	return 0
}

func mapDate(value *string) (int, bool) {
	if value == nil || *value == "" {
		return 0, true
	}
	if len(*value) != len("2006-01-02") {
		return 0, false
	}
	parsed, err := time.Parse("2006-01-02", *value)
	if err != nil || parsed.Format("2006-01-02") != *value || parsed.Year() <= 0 || parsed.Year() > maxYear {
		return 0, false
	}
	return parsed.Year(), true
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

type searchResponse struct {
	Data []subjectResponse `json:"data"`
}

func (response *searchResponse) UnmarshalJSON(body []byte) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	data := bytes.TrimSpace(envelope.Data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return errors.New("search data array required")
	}
	var subjects []subjectResponse
	if err := json.Unmarshal(data, &subjects); err != nil {
		return err
	}
	response.Data = subjects
	return nil
}

type subjectResponse struct {
	ID            json.RawMessage `json:"id"`
	Type          *int            `json:"type"`
	Name          *string         `json:"name"`
	NameCN        *string         `json:"name_cn"`
	Summary       *string         `json:"summary"`
	Date          *string         `json:"date"`
	Eps           *int            `json:"eps"`
	TotalEpisodes *int            `json:"total_episodes"`
	Images        *subjectImages  `json:"images"`
}

type subjectImages struct {
	Large  *string `json:"large"`
	Common *string `json:"common"`
}

var _ core.MetadataProvider = (*Client)(nil)
