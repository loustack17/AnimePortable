// SPDX-License-Identifier: MPL-2.0

package bangumi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"animeportable/tests/contract"
)

const (
	testProviderID           = "bangumi"
	testAPIOrigin            = "https://api.bgm.tv"
	testSearchURL            = "https://api.bgm.tv/v0/search/subjects?limit=10&offset=0"
	testUserAgent            = "loustack17/AnimePortable/0.0.1 (https://github.com/loustack17/AnimePortable)"
	testCoverURL             = "https://lain.bgm.tv/pic/cover/l/12/34/1234_abc.jpg"
	testMaxResponseBodyBytes = 1 << 20
	testMaxTitleTextBytes    = 4 << 10
	testMaxDescriptionBytes  = 64 << 10
	testMaxCoverURLBytes     = 8 << 10
	testMaxJSONNestingDepth  = 64
	testMaxSearchResults     = 10
	testMaxEpisodeCount      = 100000
	testMaxYear              = 3000
	testMaxMetadataIDDigits  = 10
)

type fakeClient struct {
	handler  func(*http.Request, []byte) (*securehttp.Response, error)
	response *securehttp.Response
	err      error
	requests []*http.Request
	bodies   [][]byte
}

func (client *fakeClient) Do(request *http.Request) (*securehttp.Response, error) {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
	}
	client.requests = append(client.requests, request.Clone(request.Context()))
	client.bodies = append(client.bodies, body)
	if client.handler != nil {
		return client.handler(request, body)
	}
	return client.response, client.err
}

func jsonResponse(t *testing.T, value any) *securehttp.Response {
	t.Helper()
	return &securehttp.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		Body:       mustJSON(t, value),
	}
}

func rawJSONResponse(body []byte) *securehttp.Response {
	return &securehttp.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       body,
	}
}

func statusResponse(status int, body []byte) *securehttp.Response {
	return &securehttp.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       body,
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func validSubject() map[string]any {
	return map[string]any{
		"id":             123,
		"type":           2,
		"name":           "日本語タイトル",
		"name_cn":        "中文標題",
		"summary":        "Synopsis <strong>with a link</strong>",
		"date":           "2024-02-29",
		"eps":            12,
		"total_episodes": 24,
		"images": map[string]any{
			"common": testCoverURL,
		},
	}
}

func searchPayload(subjects ...any) map[string]any {
	if subjects == nil {
		subjects = []any{}
	}
	return map[string]any{
		"total":  len(subjects),
		"limit":  testMaxSearchResults,
		"offset": 0,
		"data":   subjects,
	}
}

func contractClient(t *testing.T) *fakeClient {
	t.Helper()
	return &fakeClient{handler: func(request *http.Request, _ []byte) (*securehttp.Response, error) {
		if request.Method == http.MethodPost {
			return jsonResponse(t, searchPayload(validSubject())), nil
		}
		if request.URL.Path == "/v0/subjects/999" {
			return statusResponse(http.StatusNotFound, []byte(`{"message":"remote-secret"}`)), nil
		}
		return jsonResponse(t, validSubject()), nil
	}}
}

func TestBangumiAdapterAcceptance(t *testing.T) {
	client := contractClient(t)
	contract.RunMetadataProvider(t, contract.MetadataProviderSuite{
		New: func(*testing.T) core.MetadataProvider { return newWithDo(client) },
		Search: contract.MetadataSearchCase{
			Supported: true,
			Query:     core.MetadataQuery{Title: "中文標題"},
			Expected:  []core.MetadataRef{{Provider: testProviderID, ID: "123"}},
		},
		Get: contract.MetadataGetCase{
			Supported: true,
			Ref:       core.MetadataRef{Provider: testProviderID, ID: "123"},
			Expected: core.AnimeMetadata{
				Ref:          core.MetadataRef{Provider: testProviderID, ID: "123"},
				Title:        "中文標題",
				NativeTitle:  "日本語タイトル",
				Description:  "Synopsis with a link",
				CoverURL:     testCoverURL,
				Year:         2024,
				EpisodeCount: 24,
			},
		},
		Missing:          &contract.MetadataMissingCase{Ref: core.MetadataRef{Provider: testProviderID, ID: "999"}, Expected: core.ErrNotFound},
		ForbiddenStrings: []string{"remote-secret", "<script>"},
	})

	secureClient, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins()})
	if err != nil {
		t.Fatal(err)
	}
	if New(secureClient) == nil {
		t.Fatal("Bangumi adapter composition returned nil")
	}
}

func TestBangumiRequestShape(t *testing.T) {
	searchClient := &fakeClient{response: jsonResponse(t, searchPayload())}
	provider := newWithDo(searchClient)
	if _, err := provider.Search(context.Background(), core.MetadataQuery{Title: "中文標題", NativeTitle: "日本語タイトル", Year: 2024, EpisodeCount: 24}); err != nil {
		t.Fatal(err)
	}
	if len(searchClient.requests) != 1 {
		t.Fatalf("search requests = %d", len(searchClient.requests))
	}
	assertSearchRequest(t, searchClient.requests[0])
	assertSearchBody(t, searchClient.bodies[0], "中文標題")

	getClient := &fakeClient{response: jsonResponse(t, validSubject())}
	if _, err := newWithDo(getClient).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"}); err != nil {
		t.Fatal(err)
	}
	if len(getClient.requests) != 1 {
		t.Fatalf("get requests = %d", len(getClient.requests))
	}
	assertGetRequest(t, getClient.requests[0], "123")
	if len(getClient.bodies[0]) != 0 {
		t.Fatalf("get request body = %q", getClient.bodies[0])
	}
}

func assertSearchRequest(t *testing.T, request *http.Request) {
	t.Helper()
	assertCommonRequest(t, request)
	if request.Method != http.MethodPost || request.URL.String() != testSearchURL {
		t.Fatalf("search request = %s %s", request.Method, request.URL)
	}
	if request.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("search content type = %#v", request.Header)
	}
}

func assertGetRequest(t *testing.T, request *http.Request, id string) {
	t.Helper()
	assertCommonRequest(t, request)
	want := testAPIOrigin + "/v0/subjects/" + id
	if request.Method != http.MethodGet || request.URL.String() != want {
		t.Fatalf("get request = %s %s", request.Method, request.URL)
	}
	if request.Header.Get("Content-Type") != "" {
		t.Fatalf("get content type = %#v", request.Header)
	}
}

func assertCommonRequest(t *testing.T, request *http.Request) {
	t.Helper()
	if request == nil {
		t.Fatal("request is nil")
	}
	if request.Header.Get("Accept") != "application/json" {
		t.Fatalf("accept = %#v", request.Header)
	}
	if request.Header.Get("User-Agent") != testUserAgent {
		t.Fatalf("user agent = %q", request.Header.Get("User-Agent"))
	}
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer"} {
		if request.Header.Get(header) != "" {
			t.Fatalf("unexpected %s header", header)
		}
	}
}

func assertSearchBody(t *testing.T, body []byte, wantKeyword string) {
	t.Helper()
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != 3 {
		t.Fatalf("search body keys = %#v", payload)
	}
	var keyword string
	if err := json.Unmarshal(payload["keyword"], &keyword); err != nil || keyword != wantKeyword {
		t.Fatalf("keyword = %q, %v", keyword, err)
	}
	var sort string
	if err := json.Unmarshal(payload["sort"], &sort); err != nil || sort != "match" {
		t.Fatalf("sort = %q, %v", sort, err)
	}
	var filter map[string]json.RawMessage
	if err := json.Unmarshal(payload["filter"], &filter); err != nil {
		t.Fatal(err)
	}
	if len(filter) != 1 {
		t.Fatalf("filter keys = %#v", filter)
	}
	var types []int
	if err := json.Unmarshal(filter["type"], &types); err != nil || !reflect.DeepEqual(types, []int{2}) {
		t.Fatalf("filter types = %#v, %v", types, err)
	}
}

func TestBangumiSearchUsesNativeFallbackAndRejectsEmpty(t *testing.T) {
	client := &fakeClient{response: jsonResponse(t, searchPayload())}
	provider := newWithDo(client)
	if _, err := provider.Search(context.Background(), core.MetadataQuery{NativeTitle: "日本語タイトル"}); err != nil {
		t.Fatal(err)
	}
	assertSearchBody(t, client.bodies[0], "日本語タイトル")
	if _, err := provider.Search(context.Background(), core.MetadataQuery{}); !errors.Is(err, errMetadataInvalidQuery) {
		t.Fatalf("empty query error = %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("empty query made request: %d", len(client.requests))
	}
	if _, err := provider.Search(context.Background(), core.MetadataQuery{Title: strings.Repeat("x", testMaxTitleTextBytes+1)}); !errors.Is(err, errMetadataInvalidQuery) {
		t.Fatalf("oversized query error = %v", err)
	}
	if _, err := provider.Search(context.Background(), core.MetadataQuery{Title: "bad\x00query"}); !errors.Is(err, errMetadataInvalidQuery) {
		t.Fatalf("control query error = %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("invalid query made request: %d", len(client.requests))
	}
}

func TestBangumiMapsTitleDateAndEpisodePolicies(t *testing.T) {
	cases := []struct {
		name         string
		nameCN       any
		nameOriginal any
		date         any
		eps          any
		total        any
		wantTitle    string
		wantNative   string
		wantYear     int
		wantEpisodes int
	}{
		{name: "primary chinese title and total episodes", nameCN: " 中文標題 ", nameOriginal: " 日本語タイトル ", date: "2024-02-29", eps: 12, total: 24, wantTitle: "中文標題", wantNative: "日本語タイトル", wantYear: 2024, wantEpisodes: 24},
		{name: "native fallback and eps", nameCN: " ", nameOriginal: " 日本語タイトル ", date: "", eps: 12, total: 0, wantTitle: "日本語タイトル", wantNative: "日本語タイトル", wantEpisodes: 12},
		{name: "nil chinese title fallback", nameCN: nil, nameOriginal: " 日本語タイトル ", date: "", eps: 12, total: 0, wantTitle: "日本語タイトル", wantNative: "日本語タイトル", wantEpisodes: 12},
		{name: "null total uses eps", nameCN: "中文標題", nameOriginal: "日本語タイトル", date: nil, eps: 12, total: nil, wantTitle: "中文標題", wantNative: "日本語タイトル", wantEpisodes: 12},
		{name: "zero optionals", nameCN: "中文標題", nameOriginal: "日本語タイトル", date: nil, eps: 0, total: 0, wantTitle: "中文標題", wantNative: "日本語タイトル"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject["name_cn"] = test.nameCN
			subject["name"] = test.nameOriginal
			subject["date"] = test.date
			subject["eps"] = test.eps
			subject["total_episodes"] = test.total
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Title != test.wantTitle || metadata.NativeTitle != test.wantNative || metadata.Year != test.wantYear || metadata.EpisodeCount != test.wantEpisodes {
				t.Fatalf("metadata = %#v", metadata)
			}
			if metadata.Season != "" || metadata.Studio != "" {
				t.Fatalf("inferred unsupported fields: %#v", metadata)
			}
		})
	}
}

func TestBangumiGetMapsPlainTextAndNullableOptionals(t *testing.T) {
	subject := validSubject()
	subject["summary"] = "<p>Synopsis <em>text</em></p>"
	subject["date"] = nil
	subject["eps"] = nil
	subject["total_episodes"] = nil
	subject["images"] = nil
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	want := core.AnimeMetadata{Ref: core.MetadataRef{Provider: testProviderID, ID: "123"}, Title: "中文標題", NativeTitle: "日本語タイトル", Description: "Synopsis text"}
	if metadata != want {
		t.Fatalf("metadata = %#v, want %#v", metadata, want)
	}

	subject = validSubject()
	subject["summary"] = ""
	subject["images"] = map[string]any{"common": ""}
	metadata, err = newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Description != "" || metadata.CoverURL != "" {
		t.Fatalf("empty optional values = %#v", metadata)
	}

	subject = validSubject()
	subject["summary"] = nil
	subject["images"] = nil
	metadata, err = newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Description != "" || metadata.CoverURL != "" {
		t.Fatalf("nil optional values = %#v", metadata)
	}
}

func TestBangumiRejectsBothTitlesEmptyAtomically(t *testing.T) {
	invalid := validSubject()
	invalid["name"] = "\t"
	invalid["name_cn"] = "  "
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(invalid, validSubject()))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, invalid)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
		t.Fatalf("metadata = %#v, error = %v", metadata, err)
	}
}

func TestBangumiRejectsInvalidDatesAtomically(t *testing.T) {
	values := []string{"2024-1-01", "2024-01-1", "2024-02-30", "2024-00-01", "2024-13-01", "2024-01-01T00:00:00Z", strconv.Itoa(testMaxYear+1) + "-01-01"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			subject := validSubject()
			subject["date"] = value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func TestBangumiRejectsInvalidAnimeTypesAtomically(t *testing.T) {
	for _, value := range []any{nil, 0, 1, 3, 99, "2"} {
		t.Run(typeName(value), func(t *testing.T) {
			subject := validSubject()
			subject["type"] = value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func typeName(value any) string {
	if value == nil {
		return "null"
	}
	return strings.ReplaceAll(strings.TrimSpace(string(mustJSONForName(value))), "\"", "")
}

func mustJSONForName(value any) []byte {
	body, _ := json.Marshal(value)
	return body
}

func TestBangumiEpisodeValidationIsAtomic(t *testing.T) {
	cases := []struct {
		name  string
		field string
		value any
	}{
		{name: "negative total", field: "total_episodes", value: -1},
		{name: "oversized total", field: "total_episodes", value: testMaxEpisodeCount + 1},
		{name: "negative eps", field: "eps", value: -1},
		{name: "oversized eps", field: "eps", value: testMaxEpisodeCount + 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject[test.field] = test.value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func TestBangumiSearchRejectsInvalidCandidateAtomically(t *testing.T) {
	invalid := validSubject()
	invalid["id"] = 0
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(validSubject(), invalid))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}

	invalid = validSubject()
	invalid["type"] = 1
	items, err = newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(invalid))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("non-anime items = %#v, error = %v", items, err)
	}
}

func TestBangumiSearchRejectsExcessiveResultsAtomically(t *testing.T) {
	subjects := make([]any, 0, testMaxSearchResults+1)
	for index := 0; index < testMaxSearchResults+1; index++ {
		subject := validSubject()
		subject["id"] = index + 1
		subjects = append(subjects, subject)
	}
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(subjects...))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestBangumiSearchRequiresDataArray(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{name: "missing data", body: []byte(`{"total":0,"limit":10,"offset":0}`)},
		{name: "null data", body: []byte(`{"total":0,"limit":10,"offset":0,"data":null}`)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := newWithDo(&fakeClient{response: rawJSONResponse(test.body)}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
			if !errors.Is(err, errMetadataMalformed) || items != nil {
				t.Fatalf("items = %#v, error = %v", items, err)
			}
		})
	}

	items, err := newWithDo(&fakeClient{response: rawJSONResponse([]byte(`{"total":0,"limit":10,"offset":0,"data":[]}`))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty data result = %#v, error = %v", items, err)
	}
}

func TestBangumiAllowsUnknownFields(t *testing.T) {
	subject := validSubject()
	subject["future_field"] = map[string]any{"value": "ignored"}
	payload := searchPayload(subject)
	payload["future_response_field"] = "ignored"
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, payload)}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if err != nil || len(items) != 1 {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestBangumiResponseValidation(t *testing.T) {
	deep := nestedJSON(testMaxJSONNestingDepth + 1)
	duplicate := []byte(`{"id":123,"id":123,"type":2,"name":"日本語タイトル","name_cn":"中文標題","summary":"","date":"","eps":0,"total_episodes":0,"images":null}`)
	tests := []struct {
		name     string
		response *securehttp.Response
		want     error
	}{
		{name: "bad MIME", response: &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: mustJSON(t, validSubject())}, want: errMetadataMalformed},
		{name: "malformed JSON", response: rawJSONResponse([]byte(`{"id":`)), want: errMetadataMalformed},
		{name: "oversized body", response: rawJSONResponse([]byte(strings.Repeat("x", testMaxResponseBodyBytes+1))), want: errMetadataMalformed},
		{name: "duplicate keys", response: rawJSONResponse(duplicate), want: errMetadataMalformed},
		{name: "excessive nesting", response: rawJSONResponse(deep), want: errMetadataMalformed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := newWithDo(&fakeClient{response: test.response}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, test.want) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func nestedJSON(depth int) []byte {
	return []byte(`{"id":123,"type":2,"name":"日本語タイトル","name_cn":"中文標題","summary":"","date":"","eps":0,"total_episodes":0,"images":null,"future":` + strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth) + "}")
}

func TestBangumiRejectsOversizedTextAtomically(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{name: "title", field: "name_cn", value: strings.Repeat("x", testMaxTitleTextBytes+1)},
		{name: "summary", field: "summary", value: strings.Repeat("x", testMaxDescriptionBytes+1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject[test.field] = test.value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func TestBangumiDescriptionNormalization(t *testing.T) {
	tests := []struct {
		name      string
		summary   string
		want      string
		forbidden string
	}{
		{name: "raw script", summary: "Synopsis <script>alert(1)</script> end", want: "Synopsis end", forbidden: "alert(1)"},
		{name: "double encoded script", summary: "Synopsis &amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt; end", want: "Synopsis end", forbidden: "alert(1)"},
		{name: "unclosed tag", summary: "Synopsis <img src=x", want: "Synopsis", forbidden: "img"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject["summary"] = test.summary
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Description != test.want || strings.Contains(metadata.Description, "<") || strings.Contains(metadata.Description, "</") || strings.Contains(metadata.Description, test.forbidden) {
				t.Fatalf("description = %q, want %q", metadata.Description, test.want)
			}
		})
	}
}

func TestBangumiNormalizesRemoteDisplayFields(t *testing.T) {
	subject := validSubject()
	subject["name_cn"] = " <b>中文</b>\n 標題 "
	subject["name"] = " **日本語**\tタイトル "
	subject["summary"] = " <p>Plain <em>synopsis</em></p> "
	provider := newWithDo(&fakeClient{handler: func(request *http.Request, _ []byte) (*securehttp.Response, error) {
		if request.Method == http.MethodPost {
			return jsonResponse(t, searchPayload(subject)), nil
		}
		return jsonResponse(t, subject), nil
	}})
	items, err := provider.Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "中文 標題" || items[0].NativeTitle != "日本語 タイトル" {
		t.Fatalf("candidate = %#v", items)
	}
	metadata, err := provider.Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Title != "中文 標題" || metadata.NativeTitle != "日本語 タイトル" || metadata.Description != "Plain synopsis" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestBangumiRejectsControlCharactersAtomically(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		value string
	}{
		{name: "title", field: "name_cn", value: "中文\x00標題"},
		{name: "summary", field: "summary", value: "摘要\x00秘密"},
	} {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject[test.field] = test.value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func TestBangumiCoverHostValidation(t *testing.T) {
	for _, value := range []string{
		"http://lain.bgm.tv/pic/cover/l/12/34/1234_abc.jpg",
		"https://evil.example/pic/cover.jpg",
		"https://lain.bgm.tv.evil.example/pic/cover.jpg",
		"https://user:secret@lain.bgm.tv/pic/cover.jpg",
		"https://lain.bgm.tv:444/pic/cover.jpg",
	} {
		t.Run(value, func(t *testing.T) {
			subject := validSubject()
			subject["images"] = map[string]any{"common": value}
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.CoverURL != "" {
				t.Fatalf("invalid cover retained: %q", metadata.CoverURL)
			}
		})
	}

	subject := validSubject()
	subject["images"] = map[string]any{"common": "https://lain.bgm.tv/" + strings.Repeat("x", testMaxCoverURLBytes)}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.CoverURL != "" {
		t.Fatalf("oversized cover retained: %q", metadata.CoverURL)
	}
}

func TestBangumiCoverPriorityAndSafeFallback(t *testing.T) {
	largeURL := "https://lain.bgm.tv/pic/cover/l/56/78/5678_large.jpg"
	commonURL := testCoverURL
	cases := []struct {
		name      string
		images    map[string]any
		wantCover string
	}{
		{name: "large takes priority", images: map[string]any{"large": largeURL, "common": commonURL}, wantCover: largeURL},
		{name: "empty large falls back to common", images: map[string]any{"large": "", "common": commonURL}, wantCover: commonURL},
		{name: "invalid large does not fall back", images: map[string]any{"large": "https://evil.example/cover.jpg", "common": commonURL}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			subject := validSubject()
			subject["images"] = test.images
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.CoverURL != test.wantCover {
				t.Fatalf("cover = %q, want %q", metadata.CoverURL, test.wantCover)
			}
		})
	}
}

func TestBangumiHTTPStatusMappingAndRedaction(t *testing.T) {
	secretBody := []byte(`{"message":"remote-secret"}`)
	searchStatuses := []int{http.StatusNotFound, http.StatusTooManyRequests, http.StatusBadRequest, http.StatusInternalServerError}
	for _, status := range searchStatuses {
		t.Run("search-"+http.StatusText(status), func(t *testing.T) {
			items, err := newWithDo(&fakeClient{response: statusResponse(status, secretBody)}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
			if !errors.Is(err, errMetadataUnavailable) || errors.Is(err, core.ErrNotFound) || items != nil || strings.Contains(err.Error(), "remote-secret") {
				t.Fatalf("items = %#v, error = %v", items, err)
			}
		})
	}
	metadata, err := newWithDo(&fakeClient{response: statusResponse(http.StatusNotFound, secretBody)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if !errors.Is(err, core.ErrNotFound) || metadata != (core.AnimeMetadata{}) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("missing metadata = %#v, error = %v", metadata, err)
	}
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadRequest, http.StatusInternalServerError} {
		metadata, err = newWithDo(&fakeClient{response: statusResponse(status, secretBody)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
		if !errors.Is(err, errMetadataUnavailable) || metadata != (core.AnimeMetadata{}) || strings.Contains(err.Error(), "remote-secret") {
			t.Fatalf("status %d metadata = %#v, error = %v", status, metadata, err)
		}
	}
}

func TestBangumiReferenceValidation(t *testing.T) {
	client := &fakeClient{response: jsonResponse(t, validSubject())}
	provider := newWithDo(client)
	refs := []core.MetadataRef{
		{Provider: "other", ID: "123"},
		{Provider: testProviderID, ID: ""},
		{Provider: testProviderID, ID: "0"},
		{Provider: testProviderID, ID: "01"},
		{Provider: testProviderID, ID: "-1"},
		{Provider: testProviderID, ID: "12.3"},
		{Provider: testProviderID, ID: "not-an-id"},
		{Provider: testProviderID, ID: strings.Repeat("9", testMaxMetadataIDDigits+1)},
	}
	for _, ref := range refs {
		metadata, err := provider.Get(context.Background(), ref)
		if !errors.Is(err, errMetadataInvalidRef) || metadata != (core.AnimeMetadata{}) {
			t.Fatalf("ref %#v = %#v, %v", ref, metadata, err)
		}
	}
	if len(client.requests) != 0 {
		t.Fatalf("invalid refs made requests: %d", len(client.requests))
	}
}

func TestBangumiGetRejectsResponseIDMismatchAndInvalidIDs(t *testing.T) {
	mismatched := validSubject()
	mismatched["id"] = 124
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, mismatched)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
	if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
		t.Fatalf("mismatched metadata = %#v, error = %v", metadata, err)
	}

	for _, value := range []any{0, -1, int64(100000000000), "123"} {
		t.Run(typeName(value), func(t *testing.T) {
			subject := validSubject()
			subject["id"] = value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, subject)}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
		})
	}
}

func TestBangumiCancellationAndSanitizedRequestError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{response: jsonResponse(t, searchPayload())}
	if _, err := newWithDo(client).Search(ctx, core.MetadataQuery{Title: "Anime"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("search cancellation error = %v", err)
	}
	if _, err := newWithDo(client).Get(ctx, core.MetadataRef{Provider: testProviderID, ID: "123"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("get cancellation error = %v", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("canceled request count = %d", len(client.requests))
	}

	secret := errors.New("remote-secret network failure")
	if _, err := newWithDo(&fakeClient{err: secret}).Search(context.Background(), core.MetadataQuery{Title: "Anime"}); !errors.Is(err, errMetadataUnavailable) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("sanitized search error = %v", err)
	}
	if _, err := newWithDo(&fakeClient{err: secret}).Get(context.Background(), core.MetadataRef{Provider: testProviderID, ID: "123"}); !errors.Is(err, errMetadataUnavailable) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("sanitized get error = %v", err)
	}
}

func TestBangumiAllowedOriginsAreIndependent(t *testing.T) {
	origins := AllowedOrigins()
	if !reflect.DeepEqual(origins, []string{testAPIOrigin}) {
		t.Fatalf("origins = %#v", origins)
	}
	origins[0] = "https://evil.example"
	if !reflect.DeepEqual(AllowedOrigins(), []string{testAPIOrigin}) {
		t.Fatal("allowed origins expose internal storage")
	}
}

func TestLiveBangumiAdapter(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_BANGUMI_LIVE") != "1" {
		t.Skip("set ANIMEPORTABLE_BANGUMI_LIVE=1 to run the live adapter smoke test")
	}
	secureClient, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := New(secureClient)
	items, err := provider.Search(ctx, core.MetadataQuery{Title: "葬送的芙莉蓮"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("live search returned no candidates")
	}
	metadata, err := provider.Get(ctx, items[0].Ref)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Ref != items[0].Ref {
		t.Fatalf("metadata ref = %#v, candidate ref = %#v", metadata.Ref, items[0].Ref)
	}
}

var _ core.MetadataProvider = (*Client)(nil)
