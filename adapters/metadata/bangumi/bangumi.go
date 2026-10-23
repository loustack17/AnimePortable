// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	maxAliases           = 16
	browsePageSize       = 50
	// A season holds about 130-150 anime; the bound keeps a broken listing from paging forever.
	maxBrowseSubjects = 400
	// browseFirstPages covers a usual season (about 130-150 anime) in one round of requests.
	browseFirstPages = 3
	maxCachedSeasons = 16
)

var regionTag = regexp.MustCompile(`\s*[(（](?:港|台|港/台|台/港|港台)[)）]\s*$`)

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

	seasonsMu sync.Mutex
	seasons   map[seasonKey]*seasonFetch
}

type seasonKey struct{ year, month int }

func New(client *securehttp.Client) *Client {
	return newWithDo(client)
}

func newWithDo(do responseClient) *Client {
	return &Client{do: do, seasons: make(map[seasonKey]*seasonFetch)}
}

func AllowedOrigins() []string {
	return []string{apiOrigin}
}

func (client *Client) Search(ctx context.Context, query core.MetadataQuery) ([]core.MetadataCandidate, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// Bangumi's Chinese names are Simplified, and its search does not match a Taiwan title
	// written in Traditional characters (判處勇者刑 finds nothing, 判处勇者刑 finds it; Loop
	// 37d), so the Chinese title is searched in Simplified. A Japanese native title is searched
	// as it is: folding would turn 日本語 into 日本语.
	keyword := core.FoldToSimplified(strings.TrimSpace(query.Title))
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

// seasonStartMonths maps an airing season to the month Bangumi dates its new shows in.
var seasonStartMonths = map[string]int{"winter": 1, "spring": 4, "summer": 7, "fall": 10}

// BrowseSeason lists the anime Bangumi dates in the first month of the season, in pages of
// browsePageSize. Each season is fetched once per app run; several anime share it.
func (client *Client) BrowseSeason(ctx context.Context, year int, season string) ([]core.MetadataCandidate, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	month, ok := seasonStartMonths[season]
	if !ok || year < 1900 || year > maxYear {
		return nil, errMetadataInvalidQuery
	}
	key := seasonKey{year: year, month: month}
	for {
		candidates, err := client.sharedSeason(ctx, key)
		// The shared fetch ran under the context of the lookup that started it. If that one was
		// cancelled (its page closed) while this lookup still runs, it fetches the season itself.
		if errors.Is(err, context.Canceled) && ctx.Err() == nil {
			continue
		}
		return candidates, err
	}
}

// sharedSeason returns the season's listing, fetching it or waiting for the fetch another
// lookup started. Lookups of anime from the same season often run at once (追蹤 refresh).
func (client *Client) sharedSeason(ctx context.Context, key seasonKey) ([]core.MetadataCandidate, error) {
	client.seasonsMu.Lock()
	fetch, found := client.seasons[key]
	if !found {
		if len(client.seasons) >= maxCachedSeasons {
			for other, entry := range client.seasons {
				if entry.finished() {
					delete(client.seasons, other)
				}
			}
		}
		fetch = &seasonFetch{done: make(chan struct{})}
		client.seasons[key] = fetch
	}
	client.seasonsMu.Unlock()
	if found {
		select {
		case <-fetch.done:
			return fetch.candidates, fetch.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	fetch.candidates, fetch.err = client.fetchSeason(ctx, key.year, key.month)
	if fetch.err != nil {
		// A failed fetch is not kept: the next lookup tries again.
		client.seasonsMu.Lock()
		delete(client.seasons, key)
		client.seasonsMu.Unlock()
	}
	close(fetch.done)
	return fetch.candidates, fetch.err
}

// seasonFetch is one season's listing, shared by the lookups that ask for it while or after it
// is fetched. candidates and err are set before done is closed.
type seasonFetch struct {
	done       chan struct{}
	candidates []core.MetadataCandidate
	err        error
}

func (fetch *seasonFetch) finished() bool {
	select {
	case <-fetch.done:
		return true
	default:
		return false
	}
}

func (client *Client) fetchSeason(ctx context.Context, year, month int) ([]core.MetadataCandidate, error) {
	// Bangumi takes seconds per page, so the pages a season usually fills are asked for at once,
	// and any further ones the first page's total calls for after that.
	pages, total, err := client.browsePages(ctx, year, month, 0, browseFirstPages)
	if err != nil {
		return nil, err
	}
	if more := min(total, maxBrowseSubjects) - browseFirstPages*browsePageSize; more > 0 {
		rest, _, err := client.browsePages(ctx, year, month, browseFirstPages, (more+browsePageSize-1)/browsePageSize)
		if err != nil {
			return nil, err
		}
		pages = append(pages, rest...)
	}
	var candidates []core.MetadataCandidate
	for _, page := range pages {
		for _, subject := range page {
			// One odd entry in a list of a hundred must not hide the others; it is left out.
			if candidate, ok := mapCandidate(subject); ok {
				candidates = append(candidates, candidate)
			}
		}
	}
	return candidates, nil
}

// browsePages fetches count browse pages from page first on, all at once, and returns their
// subjects in page order and the total the first of them reports. The first failure cancels
// the other requests and is returned.
func (client *Client) browsePages(ctx context.Context, year, month, first, count int) ([][]subjectResponse, int, error) {
	pages := make([][]subjectResponse, count)
	totals := make([]int, count)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var failOnce sync.Once
	var failure error
	fail := func(err error) {
		failOnce.Do(func() { failure = err; cancel() })
	}
	var wait sync.WaitGroup
	for index := range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			offset := (first + index) * browsePageSize
			target := fmt.Sprintf("%s/v0/subjects?type=2&year=%d&month=%d&limit=%d&offset=%d", apiOrigin, year, month, browsePageSize, offset)
			response, err := client.doRequest(ctx, http.MethodGet, target, nil, false)
			if err != nil {
				fail(err)
				return
			}
			var page browseResponse
			if err := decodeResponse(response.Body, &page); err != nil {
				fail(err)
				return
			}
			if len(page.Data) > browsePageSize || page.Total < 0 {
				fail(errMetadataMalformed)
				return
			}
			pages[index], totals[index] = page.Data, page.Total
		}()
	}
	wait.Wait()
	if failure != nil {
		return nil, 0, failure
	}
	return pages, totals[0], nil
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
		Aliases:      subjectAliases(subject.Infobox),
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

// subjectAliases returns the subject's alternative names (别名). They often include the Taiwan
// title, which lets an Anime1 name such as 間諜家家酒 match Bangumi's 间谍过家家. Malformed
// entries are skipped, never fatal.
func subjectAliases(raw json.RawMessage) []string {
	var entries []struct {
		Key   string          `json:"key"`
		Value json.RawMessage `json:"value"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &entries) != nil {
		return nil
	}
	var aliases []string
	for _, entry := range entries {
		if entry.Key != "别名" {
			continue
		}
		var values []string
		var single string
		var list []struct {
			V string `json:"v"`
		}
		if json.Unmarshal(entry.Value, &single) == nil {
			values = append(values, single)
		} else if json.Unmarshal(entry.Value, &list) == nil {
			for _, item := range list {
				values = append(values, item.V)
			}
		}
		for _, value := range values {
			alias, ok := boundedText(&value, maxTitleTextBytes, maxTitleTextRunes)
			if !ok || alias == "" || len(aliases) >= maxAliases {
				continue
			}
			// Some aliases carry a region tag such as "(港/台)"; script differences are folded by
			// the core matcher.
			if alias = strings.TrimSpace(regionTag.ReplaceAllString(alias, "")); alias != "" {
				aliases = append(aliases, alias)
			}
		}
	}
	return aliases
}

func mapTitles(nameCN, name *string) (string, string, bool) {
	if nameCN != nil {
		// Size is checked before conversion so oversized remote text never reaches the converter.
		if len(*nameCN) > maxTitleTextBytes {
			return "", "", false
		}
		converted, ok := toTraditional(*nameCN)
		if !ok {
			return "", "", false
		}
		nameCN = &converted
	}
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
	if len(*summary) > maxDescriptionBytes {
		return "", false
	}
	// Some summaries append the Japanese original after this marker; only the Chinese part is shown.
	chinese, _, _ := strings.Cut(*summary, "[简介原文]")
	if isJapanese(chinese) {
		// Some subjects only have the Japanese original. It is shown as is (owner decision
		// 2026-10-14): converting it would turn Japanese kanji such as 国 into Chinese forms.
		return metadatapolicy.NormalizePlainText(chinese, metadatapolicy.DescriptionLimits())
	}
	converted, ok := toTraditional(chinese)
	if !ok {
		return "", false
	}
	return metadatapolicy.NormalizePlainText(converted, metadatapolicy.DescriptionLimits())
}

// isJapanese reports Japanese prose: hiragana (particles such as の, を, に) make up over a tenth
// of its Han and hiragana characters. Chinese summaries may quote katakana names but hardly
// use hiragana.
func isJapanese(value string) bool {
	hiragana, han := 0, 0
	for _, character := range value {
		switch {
		case unicode.Is(unicode.Hiragana, character):
			hiragana++
		case unicode.Is(unicode.Han, character):
			han++
		}
	}
	return hiragana > 0 && hiragana*10 > hiragana+han
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
			return resizedCoverURL(*value)
		}
		return ""
	}
	return ""
}

// Bangumi's large cover is the full-size original (often around 2000x2800), which is over the
// cover loader's pixel limit. Bangumi's own resize path serves the same image 400 px wide.
func resizedCoverURL(value string) string {
	const original = "https://lain.bgm.tv/pic/cover/l/"
	if strings.HasPrefix(value, original) {
		return "https://lain.bgm.tv/r/400/pic/cover/l/" + strings.TrimPrefix(value, original)
	}
	return value
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

type browseResponse struct {
	Total int               `json:"total"`
	Data  []subjectResponse `json:"data"`
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
	// Infobox is decoded leniently by subjectAliases; a malformed infobox must not reject the subject.
	Infobox json.RawMessage `json:"infobox"`
}

type subjectImages struct {
	Large  *string `json:"large"`
	Common *string `json:"common"`
}

var (
	_ core.MetadataProvider      = (*Client)(nil)
	_ core.MetadataSeasonBrowser = (*Client)(nil)
)
