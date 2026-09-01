package anilist

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"animeportable/tests/contract"
)

type fakeClient struct {
	handler  func(*http.Request, []byte) (*securehttp.Response, error)
	response *securehttp.Response
	err      error
	requests []*http.Request
	bodies   [][]byte
}

func (client *fakeClient) Do(request *http.Request) (*securehttp.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
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

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func validMedia() map[string]any {
	return map[string]any{
		"id": 123,
		"title": map[string]any{
			"romaji": "Romaji Title",
			"native": "Native Title",
		},
		"seasonYear":  2024,
		"episodes":    12,
		"description": "# **A synopsis** <br><p>[with a link](https://example.test)</p>",
		"coverImage": map[string]any{
			"extraLarge": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/123.jpg",
			"large":      "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/123-large.jpg",
			"medium":     "https://s4.anilist.co/file/anilistcdn/media/anime/cover/medium/123.jpg",
		},
		"season": "WINTER",
		"studios": map[string]any{
			"nodes": []any{
				map[string]any{"name": "Non-animation", "isAnimationStudio": false},
				map[string]any{"name": "Animation Studio", "isAnimationStudio": true},
			},
		},
	}
}

func searchPayload(media ...any) map[string]any {
	return map[string]any{"data": map[string]any{"Page": map[string]any{"media": media}}}
}

func getPayload(media any) map[string]any {
	return map[string]any{"data": map[string]any{"Media": media}}
}

func TestAniListAdapterAcceptance(t *testing.T) {
	search := validMedia()
	get := validMedia()
	client := &fakeClient{handler: func(_ *http.Request, body []byte) (*securehttp.Response, error) {
		var request struct {
			Query     string                     `json:"query"`
			Variables map[string]json.RawMessage `json:"variables"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			return nil, err
		}
		if strings.Contains(request.Query, "Page(") {
			return jsonResponse(t, searchPayload(search)), nil
		}
		var id int
		if err := json.Unmarshal(request.Variables["id"], &id); err != nil {
			return nil, err
		}
		if id == 999 {
			return &securehttp.Response{StatusCode: http.StatusNotFound, Header: http.Header{"Content-Type": {"application/json"}}}, nil
		}
		return jsonResponse(t, getPayload(get)), nil
	}}

	contract.RunMetadataProvider(t, contract.MetadataProviderSuite{
		New: func(*testing.T) core.MetadataProvider { return newWithDo(client) },
		Search: contract.MetadataSearchCase{
			Supported: true,
			Query:     core.MetadataQuery{Title: "Romaji Title"},
			Expected:  []core.MetadataRef{{Provider: providerID, ID: "123"}},
		},
		Get: contract.MetadataGetCase{
			Supported: true,
			Ref:       core.MetadataRef{Provider: providerID, ID: "123"},
			Expected: core.AnimeMetadata{
				Ref:          core.MetadataRef{Provider: providerID, ID: "123"},
				Title:        "Romaji Title",
				NativeTitle:  "Native Title",
				Description:  "A synopsis with a link",
				CoverURL:     "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/123.jpg",
				Season:       "WINTER",
				Year:         2024,
				Studio:       "Animation Studio",
				EpisodeCount: 12,
			},
		},
		Missing:          &contract.MetadataMissingCase{Ref: core.MetadataRef{Provider: providerID, ID: "999"}, Expected: core.ErrNotFound},
		ForbiddenStrings: []string{"remote-secret", "<script>"},
	})

	secureClient, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins()})
	if err != nil {
		t.Fatal(err)
	}
	if New(secureClient) == nil {
		t.Fatal("AniList adapter composition returned nil")
	}
}

func TestAniListRequestShape(t *testing.T) {
	searchClient := &fakeClient{response: jsonResponse(t, searchPayload())}
	provider := newWithDo(searchClient)
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	if _, err := provider.Search(ctx, core.MetadataQuery{Title: "Title", NativeTitle: "Native", Year: 2020, EpisodeCount: 12}); err != nil {
		t.Fatal(err)
	}
	if len(searchClient.requests) != 1 {
		t.Fatalf("requests = %d", len(searchClient.requests))
	}
	assertRequest(t, searchClient.requests[0], searchDocument)
	var searchBody struct {
		Query     string                     `json:"query"`
		Variables map[string]json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(searchClient.bodies[0], &searchBody); err != nil {
		t.Fatal(err)
	}
	if searchBody.Query != searchDocument {
		t.Fatalf("search query = %q", searchBody.Query)
	}
	var search string
	if err := json.Unmarshal(searchBody.Variables["search"], &search); err != nil {
		t.Fatal(err)
	}
	if search != "Title" || len(searchBody.Variables) != 1 {
		t.Fatalf("search variables = %#v", searchBody.Variables)
	}

	getClient := &fakeClient{response: jsonResponse(t, getPayload(validMedia()))}
	if _, err := newWithDo(getClient).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"}); err != nil {
		t.Fatal(err)
	}
	if len(getClient.requests) != 1 {
		t.Fatalf("requests = %d", len(getClient.requests))
	}
	assertRequest(t, getClient.requests[0], getDocument)
	var getBody struct {
		Query     string                     `json:"query"`
		Variables map[string]json.RawMessage `json:"variables"`
	}
	if err := json.Unmarshal(getClient.bodies[0], &getBody); err != nil {
		t.Fatal(err)
	}
	if getBody.Query != getDocument {
		t.Fatalf("get query = %q", getBody.Query)
	}
	var id int
	if err := json.Unmarshal(getBody.Variables["id"], &id); err != nil {
		t.Fatal(err)
	}
	if id != 123 || len(getBody.Variables) != 1 {
		t.Fatalf("get variables = %#v", getBody.Variables)
	}
}

func assertRequest(t *testing.T, request *http.Request, document string) {
	t.Helper()
	if request.Method != http.MethodPost || request.URL.String() != graphqlURL {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}
	if request.Header.Get("Content-Type") != "application/json" || request.Header.Get("Accept") != "application/json" {
		t.Fatalf("headers = %#v", request.Header)
	}
}

func TestAniListSearchUsesNativeFallbackAndRejectsEmpty(t *testing.T) {
	client := &fakeClient{response: jsonResponse(t, searchPayload())}
	provider := newWithDo(client)
	if _, err := provider.Search(context.Background(), core.MetadataQuery{NativeTitle: "Native"}); err != nil {
		t.Fatal(err)
	}
	var request struct {
		Variables map[string]string `json:"variables"`
	}
	if err := json.Unmarshal(client.bodies[0], &request); err != nil {
		t.Fatal(err)
	}
	if request.Variables["search"] != "Native" {
		t.Fatalf("variables = %#v", request.Variables)
	}
	if _, err := provider.Search(context.Background(), core.MetadataQuery{}); !errors.Is(err, errMetadataInvalidQuery) {
		t.Fatalf("error = %v", err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("empty query made a request: %d", len(client.requests))
	}
}

func TestAniListGetMapsMinimalNullOptionals(t *testing.T) {
	minimal := map[string]any{
		"id":          7,
		"title":       nil,
		"seasonYear":  nil,
		"episodes":    nil,
		"description": nil,
		"coverImage":  nil,
		"season":      nil,
		"studios":     nil,
	}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(minimal))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata != (core.AnimeMetadata{Ref: core.MetadataRef{Provider: providerID, ID: "7"}}) {
		t.Fatalf("metadata = %#v", metadata)
	}

}

func TestAniListGetSanitizesDescriptionMarkup(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
		forbidden   string
	}{
		{name: "raw script", description: "Synopsis <script>alert(1)</script> end", want: "Synopsis end", forbidden: "alert(1)"},
		{name: "encoded script", description: "Synopsis &lt;script&gt;alert(1)&lt;/script&gt; end", want: "Synopsis end", forbidden: "alert(1)"},
		{name: "double encoded script", description: "Synopsis &amp;lt;script&amp;gt;alert(1)&amp;lt;/script&amp;gt; end", want: "Synopsis end", forbidden: "alert(1)"},
		{name: "style block", description: "Synopsis <style>body{color:red}</style> end", want: "Synopsis end", forbidden: "body{color:red}"},
		{name: "unclosed script", description: "Synopsis <script>never closed", want: "Synopsis", forbidden: "never closed"},
		{name: "unclosed tag", description: "Synopsis <img src=x", want: "Synopsis", forbidden: "img"},
		{name: "unclosed comment", description: "Synopsis <!--remote-secret", want: "Synopsis", forbidden: "remote-secret"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			media := validMedia()
			media["description"] = test.description
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Description != test.want {
				t.Fatalf("description = %q, want %q", metadata.Description, test.want)
			}
			if strings.Contains(metadata.Description, "<") || strings.Contains(strings.ToLower(metadata.Description), "<script") || strings.Contains(metadata.Description, "</") {
				t.Fatalf("description retained markup: %q", metadata.Description)
			}
			if strings.Contains(metadata.Description, test.forbidden) {
				t.Fatalf("description retained blocked content: %q", metadata.Description)
			}
		})
	}
}

func TestAniListGetDropsInvalidCoverAndNonAnimationStudios(t *testing.T) {
	media := validMedia()
	media["coverImage"] = map[string]any{"extraLarge": "https://evil.example/cover.jpg"}
	media["studios"] = map[string]any{"nodes": []any{
		map[string]any{"name": "Wrong Studio", "isAnimationStudio": false},
		map[string]any{"name": "", "isAnimationStudio": true},
	}}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.CoverURL != "" || metadata.Studio != "" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestAniListGetDropsOversizedStudio(t *testing.T) {
	media := validMedia()
	media["studios"] = map[string]any{"nodes": []any{
		map[string]any{"name": strings.Repeat("x", maxStudioTextBytes+1), "isAnimationStudio": true},
	}}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Studio != "" {
		t.Fatalf("studio = %q", metadata.Studio)
	}
}

func TestAniListGetRejectsInvalidValuesAtomically(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value any
	}{
		{name: "negative year", field: "seasonYear", value: -1},
		{name: "oversized year", field: "seasonYear", value: maxSeasonYear + 1},
		{name: "negative episodes", field: "episodes", value: -1},
		{name: "oversized episodes", field: "episodes", value: maxEpisodeCount + 1},
		{name: "unknown season", field: "season", value: "UNKNOWN"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			media := validMedia()
			media[test.field] = test.value
			metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
			if !errors.Is(err, errMetadataMalformed) {
				t.Fatalf("error = %v", err)
			}
			if metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v", metadata)
			}
		})
	}
}

func TestAniListGetRejectsOversizedTitleAtomically(t *testing.T) {
	media := validMedia()
	media["title"] = map[string]any{"romaji": strings.Repeat("x", maxTitleTextBytes+1)}
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
		t.Fatalf("metadata = %#v, error = %v", metadata, err)
	}
}

func TestAniListSearchRejectsInvalidCandidatesAtomically(t *testing.T) {
	valid := validMedia()
	invalid := validMedia()
	invalid["id"] = -1
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(invalid, valid))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestAniListSearchRejectsOutOfRangeCandidateAtomically(t *testing.T) {
	invalid := validMedia()
	invalid["episodes"] = maxEpisodeCount + 1
	items, err := newWithDo(&fakeClient{response: jsonResponse(t, searchPayload(invalid))}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataMalformed) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestAniListGetHandlesMissingAndGraphQLErrors(t *testing.T) {
	missing, err := newWithDo(&fakeClient{response: &securehttp.Response{StatusCode: http.StatusNotFound, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"message":"remote-secret"}`)}}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if !errors.Is(err, core.ErrNotFound) || missing != (core.AnimeMetadata{}) {
		t.Fatalf("missing = %#v, %v", missing, err)
	}
	graphql, err := newWithDo(&fakeClient{response: jsonResponse(t, map[string]any{
		"errors": []any{map[string]any{"message": "remote-secret"}},
		"data":   nil,
	})}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if !errors.Is(err, errMetadataProvider) || graphql != (core.AnimeMetadata{}) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("graphql = %#v, %v", graphql, err)
	}
}

func TestAniListResponseValidation(t *testing.T) {
	tests := []struct {
		name     string
		response *securehttp.Response
		want     error
	}{
		{name: "bad MIME", response: &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: mustJSON(t, getPayload(validMedia()))}, want: errMetadataMalformed},
		{name: "malformed JSON", response: rawJSONResponse([]byte(`{"data":`)), want: errMetadataMalformed},
		{name: "oversized", response: rawJSONResponse([]byte(strings.Repeat("x", maxResponseBodyBytes+1))), want: errMetadataMalformed},
		{name: "server failure", response: &securehttp.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte(`{"message":"remote-secret"}`)}, want: errMetadataUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata, err := newWithDo(&fakeClient{response: test.response}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
			if !errors.Is(err, test.want) || metadata != (core.AnimeMetadata{}) {
				t.Fatalf("metadata = %#v, error = %v", metadata, err)
			}
			if strings.Contains(err.Error(), "remote-secret") {
				t.Fatalf("error leaked remote message: %v", err)
			}
		})
	}
}

func TestAniListRejectsExcessiveJSONNesting(t *testing.T) {
	body := `{"data":` + strings.Repeat("[", maxJSONNestingDepth+1) + `null` + strings.Repeat("]", maxJSONNestingDepth+1) + `}`
	metadata, err := newWithDo(&fakeClient{response: rawJSONResponse([]byte(body))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if !errors.Is(err, errMetadataMalformed) || metadata != (core.AnimeMetadata{}) {
		t.Fatalf("metadata = %#v, error = %v", metadata, err)
	}
}

func TestAniListDescriptionWithManyOpenBracketsIsBounded(t *testing.T) {
	media := validMedia()
	media["description"] = strings.Repeat("[", maxDescriptionRunes)
	metadata, err := newWithDo(&fakeClient{response: jsonResponse(t, getPayload(media))}).Get(context.Background(), core.MetadataRef{Provider: providerID, ID: "123"})
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Description != media["description"] {
		t.Fatalf("description length = %d", len(metadata.Description))
	}
}

func TestAniListSearchDoesNotTreatHTTPNotFoundAsMissingMetadata(t *testing.T) {
	response := &securehttp.Response{StatusCode: http.StatusNotFound, Header: http.Header{"Content-Type": {"application/json"}}}
	items, err := newWithDo(&fakeClient{response: response}).Search(context.Background(), core.MetadataQuery{Title: "Anime"})
	if !errors.Is(err, errMetadataUnavailable) || errors.Is(err, core.ErrNotFound) || items != nil {
		t.Fatalf("items = %#v, error = %v", items, err)
	}
}

func TestAniListReferenceValidation(t *testing.T) {
	client := &fakeClient{response: jsonResponse(t, getPayload(validMedia()))}
	provider := newWithDo(client)
	refs := []core.MetadataRef{
		{Provider: "other", ID: "123"},
		{Provider: providerID, ID: ""},
		{Provider: providerID, ID: "0"},
		{Provider: providerID, ID: "01"},
		{Provider: providerID, ID: "-1"},
		{Provider: providerID, ID: "not-an-id"},
		{Provider: providerID, ID: "99999999999"},
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

func TestAniListCancellationAndSanitizedRequestError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{response: jsonResponse(t, searchPayload())}
	if _, err := newWithDo(client).Search(ctx, core.MetadataQuery{Title: "Anime"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if len(client.requests) != 0 {
		t.Fatalf("canceled request count = %d", len(client.requests))
	}

	secret := errors.New("remote-secret network failure")
	if _, err := newWithDo(&fakeClient{err: secret}).Search(context.Background(), core.MetadataQuery{Title: "Anime"}); !errors.Is(err, errMetadataUnavailable) || strings.Contains(err.Error(), "remote-secret") {
		t.Fatalf("sanitized error = %v", err)
	}
}

func TestAniListAllowedOriginsAreIndependent(t *testing.T) {
	origins := AllowedOrigins()
	if !reflect.DeepEqual(origins, []string{graphqlURL}) {
		t.Fatalf("origins = %#v", origins)
	}
	origins[0] = "https://evil.example"
	if AllowedOrigins()[0] != graphqlURL {
		t.Fatal("allowed origins expose internal storage")
	}
}

func TestLiveAniListAdapter(t *testing.T) {
	if os.Getenv("ANIMEPORTABLE_ANILIST_LIVE") != "1" {
		t.Skip("set ANIMEPORTABLE_ANILIST_LIVE=1 to run the live adapter smoke test")
	}
	secureClient, err := securehttp.New(securehttp.Config{AllowedOrigins: AllowedOrigins()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	provider := New(secureClient)
	items, err := provider.Search(ctx, core.MetadataQuery{Title: "Sousou no Frieren"})
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
