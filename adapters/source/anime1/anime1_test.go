package anime1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"animeportable/tests/contract"
)

type fakeClient struct {
	mu        sync.Mutex
	response  *securehttp.Response
	err       error
	calls     int
	requests  []*http.Request
	started   chan struct{}
	blockDone bool
}

func (client *fakeClient) Do(request *http.Request) (*securehttp.Response, error) {
	client.mu.Lock()
	client.calls++
	client.requests = append(client.requests, request.Clone(request.Context()))
	started := client.started
	response := client.response
	err := client.err
	blockDone := client.blockDone
	client.mu.Unlock()
	if started != nil {
		select {
		case <-started:
		default:
			close(started)
		}
	}
	if blockDone {
		<-request.Context().Done()
		return nil, request.Context().Err()
	}
	return response, err
}

func catalogFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func jsonResponse(body []byte) *securehttp.Response {
	return &securehttp.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json; charset=utf-8"}},
		Body:       body,
	}
}

func TestParseCatalogFixtureExact(t *testing.T) {
	items, err := parseCatalog(context.Background(), catalogFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceAnime{
		{Ref: core.SourceRef{Provider: providerID, ID: "42"}, Title: "Alpha Show"},
		{Ref: core.SourceRef{Provider: providerID, ID: "7"}, Title: "Beta & Gamma"},
	}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("items = %#v, want %#v", items, expected)
	}
}

func TestNormalizeTitleStripsMarkupExcludesSubtreesAndDecodesEntities(t *testing.T) {
	title, err := normalizeTitle(context.Background(), "before <b>middle&nbsp;&amp;</b> <script>secret</script><style>style</style><template>template</template><iframe>iframe</iframe> after")
	if err != nil {
		t.Fatal(err)
	}
	if title != "before middle & after" {
		t.Fatalf("title = %q", title)
	}
}

func TestNormalizeTitleExcludesRawTextAndLegacyContainers(t *testing.T) {
	const sentinel = "script-looking-sentinel"
	containers := []struct {
		name  string
		open  string
		close string
	}{
		{name: "script", open: "<script>", close: "</script>"},
		{name: "style", open: "<style>", close: "</style>"},
		{name: "template", open: "<template>", close: "</template>"},
		{name: "iframe", open: "<iframe>", close: "</iframe>"},
		{name: "textarea", open: "<textarea>", close: "</textarea>"},
		{name: "title", open: "<title>", close: "</title>"},
		{name: "xmp", open: "<xmp>", close: "</xmp>"},
		{name: "noembed", open: "<noembed>", close: "</noembed>"},
		{name: "noframes", open: "<noframes>", close: "</noframes>"},
		{name: "noscript", open: "<noscript>", close: "</noscript>"},
		{name: "plaintext", open: "<plaintext>", close: "</plaintext>"},
	}
	for _, container := range containers {
		t.Run(container.name, func(t *testing.T) {
			fragment := "before " + container.open + "<script>" + sentinel + "</script>" + container.close + " after"
			title, err := normalizeTitle(context.Background(), fragment)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(title, sentinel) {
				t.Fatalf("title = %q", title)
			}
		})
	}
}

func TestParseCatalogRejectsInvalidRowsAndRequiresOneValidRow(t *testing.T) {
	body := []byte(`[["1", "string id"], [2.5, "fractional"], [3e1, "exponent"], [0, "zero"], [-4, "negative"], [5, null], [6], "not row"]`)
	if _, err := parseCatalog(context.Background(), body); !errors.Is(err, errCatalogMalformed) {
		t.Fatalf("error = %v", err)
	}
	valid := []byte(`[["0007", "leading zero string"], [8, "<em>valid</em>"], [8, "duplicate"]]`)
	items, err := parseCatalog(context.Background(), valid)
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceAnime{{Ref: core.SourceRef{Provider: providerID, ID: "8"}, Title: "valid"}}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("items = %#v, want %#v", items, expected)
	}
}

func TestParseCatalogRejectsMalformedTopLevelAndExcessRows(t *testing.T) {
	for _, body := range [][]byte{[]byte(`{}`), []byte(`null`), []byte(`[]`), []byte(`not-json`)} {
		if _, err := parseCatalog(context.Background(), body); !errors.Is(err, errCatalogMalformed) {
			t.Fatalf("body %q error = %v", body, err)
		}
	}
	rows := make([][]any, maxCatalogRows+1)
	for index := range rows {
		rows[index] = []any{index + 1, "title"}
	}
	body, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseCatalog(context.Background(), body); !errors.Is(err, errCatalogMalformed) {
		t.Fatalf("excess rows error = %v", err)
	}
}

func TestParseRowEnforcesRawAndNormalizedTitleLimits(t *testing.T) {
	for _, fragment := range []string{
		strings.Repeat("x", maxTitleFragmentBytes+1),
		"<span>" + strings.Repeat("x", maxTitleTextBytes+1) + "</span>",
	} {
		row, err := json.Marshal([]any{1, fragment})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := parseRow(context.Background(), row); ok {
			t.Fatal("over-limit row accepted")
		}
	}
}

func TestNormalizeTitleEnforcesRuneLimitBelowByteLimit(t *testing.T) {
	fragment := strings.Repeat("界", maxTitleTextRunes+1)
	if len(fragment) >= maxTitleTextBytes {
		t.Fatalf("fragment bytes = %d", len(fragment))
	}
	if _, err := normalizeTitle(context.Background(), fragment); !errors.Is(err, errCatalogMalformed) {
		t.Fatalf("error = %v", err)
	}
}

func TestCatalogUsesFixedGETAndSanitizesFailures(t *testing.T) {
	fake := &fakeClient{response: jsonResponse(catalogFixture(t))}
	client := newWithDo(fake)
	items, err := client.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	fake.mu.Lock()
	if fake.calls != 1 || len(fake.requests) != 1 {
		t.Fatalf("calls = %d requests = %d", fake.calls, len(fake.requests))
	}
	request := fake.requests[0]
	fake.mu.Unlock()
	if request.Method != http.MethodGet || request.URL.String() != catalogURL {
		t.Fatalf("request = %s %s", request.Method, request.URL)
	}

	statusFake := &fakeClient{response: &securehttp.Response{StatusCode: http.StatusForbidden, Header: http.Header{"Content-Type": {"application/json"}}, Body: catalogFixture(t)}}
	_, err = newWithDo(statusFake).Catalog(context.Background())
	var statusError *securehttp.Error
	if !errors.As(err, &statusError) || statusError.Kind != securehttp.KindStatus {
		t.Fatalf("status error = %v", err)
	}
	networkFake := &fakeClient{err: errors.New("raw-secret network URL")}
	if _, err := newWithDo(networkFake).Catalog(context.Background()); err == nil || strings.Contains(err.Error(), "raw-secret") {
		t.Fatalf("network error = %v", err)
	}
	nonJSONFake := &fakeClient{response: &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: catalogFixture(t)}}
	if _, err := newWithDo(nonJSONFake).Catalog(context.Background()); !errors.Is(err, errCatalogMalformed) {
		t.Fatalf("content type error = %v", err)
	}
}

func TestCatalogPropagatesActiveCancellationToDo(t *testing.T) {
	started := make(chan struct{})
	fake := &fakeClient{started: started, blockDone: true}
	client := newWithDo(fake)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := client.Catalog(ctx)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request did not finish")
	}
}

func TestSearchIsLocalCaseInsensitiveSubstringAndBlankQueryDoesNotRequest(t *testing.T) {
	fake := &fakeClient{response: jsonResponse(catalogFixture(t))}
	client := newWithDo(fake)
	items, err := client.Search(context.Background(), "  beta &  ")
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceAnime{{Ref: core.SourceRef{Provider: providerID, ID: "7"}, Title: "Beta & Gamma"}}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("items = %#v, want %#v", items, expected)
	}
	fake.mu.Lock()
	calls := fake.calls
	fake.mu.Unlock()
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	blank, err := client.Search(context.Background(), " \t\n")
	if err != nil || len(blank) != 0 {
		t.Fatalf("blank = %#v, %v", blank, err)
	}
	fake.mu.Lock()
	calls = fake.calls
	fake.mu.Unlock()
	if calls != 1 {
		t.Fatalf("blank calls = %d", calls)
	}
}

func TestCanceledBlankSearchReturnsCancellationWithoutRequest(t *testing.T) {
	fake := &fakeClient{response: jsonResponse(catalogFixture(t))}
	client := newWithDo(fake)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	items, err := client.Search(ctx, " ")
	if !errors.Is(err, context.Canceled) || items != nil {
		t.Fatalf("items = %#v error = %v", items, err)
	}
	fake.mu.Lock()
	calls := fake.calls
	fake.mu.Unlock()
	if calls != 0 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestAnime1SourceContract(t *testing.T) {
	factory := func(*testing.T) core.AnimeSource {
		return newWithDo(&fakeClient{response: jsonResponse(catalogFixture(t))})
	}
	contract.RunAnimeSource(t, contract.AnimeSourceSuite{
		New:              factory,
		Catalog:          contract.SourceListCase{Supported: true, Expected: []core.SourceRef{{Provider: providerID, ID: "42"}, {Provider: providerID, ID: "7"}}},
		Search:           contract.SourceSearchCase{Supported: true, Query: "beta", Expected: []core.SourceRef{{Provider: providerID, ID: "7"}}},
		Episodes:         contract.SourceEpisodesCase{},
		Resolve:          contract.SourceResolveCase{},
		Schedule:         contract.SourceScheduleCase{},
		ForbiddenStrings: []string{"do-not-return", "secret", "<script>"},
	})
}

func TestUnsupportedMethods(t *testing.T) {
	client := New(nil)
	if _, err := client.Episodes(context.Background(), core.SourceRef{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := client.Resolve(context.Background(), core.EpisodeRef{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := client.Schedule(context.Background(), core.ScheduleQuery{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
