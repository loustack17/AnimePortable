package anime1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"golang.org/x/net/html"
)

type fakeClient struct {
	mu               sync.Mutex
	response         *securehttp.Response
	err              error
	episodeResponses []*securehttp.Response
	episodeIndex     int
	calls            int
	requests         []*http.Request
	started          chan struct{}
	blockDone        bool
}

func (client *fakeClient) Do(request *http.Request) (*securehttp.Response, error) {
	client.mu.Lock()
	client.calls++
	client.requests = append(client.requests, request.Clone(request.Context()))
	started := client.started
	response := client.response
	err := client.err
	blockDone := client.blockDone
	if request.URL.Path != "/animelist.json" && client.episodeIndex < len(client.episodeResponses) {
		response = client.episodeResponses[client.episodeIndex]
		client.episodeIndex++
	}
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

func episodeResponse(body string) *securehttp.Response {
	return &securehttp.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/html; charset=utf-8"}},
		Body:       []byte(body),
	}
}

func episodeFixture(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func episodeRef(id string) core.EpisodeRef {
	return core.EpisodeRef{Anime: core.SourceRef{Provider: providerID, ID: "42"}, ID: id}
}

type sequenceEpisodeClient struct {
	responses []*securehttp.Response
	errors    []error
	index     int
}

func (client *sequenceEpisodeClient) Do(*http.Request) (*securehttp.Response, error) {
	index := client.index
	client.index++
	if index < len(client.errors) && client.errors[index] != nil {
		return nil, client.errors[index]
	}
	if index < len(client.responses) {
		return client.responses[index], nil
	}
	return nil, nil
}

type cancelAfterPageClient struct {
	response *securehttp.Response
	cancel   context.CancelFunc
	calls    int
}

func (client *cancelAfterPageClient) Do(*http.Request) (*securehttp.Response, error) {
	client.calls++
	if client.calls == 2 {
		client.cancel()
	}
	return client.response, nil
}

type cancelAfterChecksContext struct {
	checks   int
	cancelAt int
}

func (ctx *cancelAfterChecksContext) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (ctx *cancelAfterChecksContext) Done() <-chan struct{} {
	return nil
}

func (ctx *cancelAfterChecksContext) Err() error {
	ctx.checks++
	if ctx.checks >= ctx.cancelAt {
		return context.Canceled
	}
	return nil
}

func (ctx *cancelAfterChecksContext) Value(any) any {
	return nil
}

func episodeBodyWithCount(count int) string {
	var builder strings.Builder
	builder.WriteString(`<html><body><main id="main">`)
	for index := 0; index < count; index++ {
		postID := 10000 + index
		fmt.Fprintf(&builder, `<article id="post-%d" class="category-42"><h2 class="entry-title"><a href="/%d">Show [%d]</a></h2></article>`, postID, postID, index+1)
	}
	builder.WriteString(`</main></body></html>`)
	return builder.String()
}

func episodePageBody(postID, page, nextPage int) string {
	navigation := ""
	if nextPage > 0 {
		navigation = fmt.Sprintf(`<nav class="navigation"><div class="nav-previous"><a href="/category/example/page/%d">Older</a></div></nav>`, nextPage)
	}
	return fmt.Sprintf(`<html><body><main id="main"><article id="post-%d" class="category-42"><h2 class="entry-title"><a href="/%d">Show [%d]</a></h2></article></main>%s</body></html>`, postID, postID, page, navigation)
}

func episodeTextNode(value string) *html.Node {
	root := &html.Node{Type: html.ElementNode, Data: "a"}
	root.AppendChild(&html.Node{Type: html.TextNode, Data: value})
	return root
}

func documentWithNodeCount(count int) *html.Node {
	root := &html.Node{Type: html.DocumentNode}
	main := &html.Node{Type: html.ElementNode, Data: "main", Attr: []html.Attribute{{Key: "id", Val: "main"}}}
	root.AppendChild(main)
	current := main
	for index := 2; index < count; index++ {
		child := &html.Node{Type: html.ElementNode, Data: "div"}
		current.AppendChild(child)
		current = child
	}
	return root
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

func TestEpisodesFixtureReturnsOldestFirstAndUsesArchivePagination(t *testing.T) {
	fake := &fakeClient{episodeResponses: []*securehttp.Response{
		episodeResponse(episodeFixture(t, "episodes-page-1.html")),
		episodeResponse(episodeFixture(t, "episodes-page-2.html")),
	}}
	items, err := newWithDo(fake).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceEpisode{
		{Ref: episodeRef("98"), Number: "PV02"},
		{Ref: episodeRef("99"), Number: "00"},
		{Ref: episodeRef("100"), Number: "01"},
		{Ref: episodeRef("101"), Number: "02"},
	}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("items = %#v, want %#v", items, expected)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.calls != 2 || len(fake.requests) != 2 {
		t.Fatalf("calls = %d requests = %d", fake.calls, len(fake.requests))
	}
	if got := fake.requests[0].URL.String(); got != "https://anime1.me/?cat=42" {
		t.Fatalf("first URL = %q", got)
	}
	if got := fake.requests[0].Method; got != http.MethodGet {
		t.Fatalf("first method = %q", got)
	}
	if got := fake.requests[1].URL.String(); got != "https://anime1.me/category/anime/%E7%95%AA/page/2" {
		t.Fatalf("second URL = %q", got)
	}
}

func TestEpisodesRejectsInvalidRefsBeforeRequest(t *testing.T) {
	fake := &fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(episodeFixture(t, "episodes-page-1.html"))}}
	client := newWithDo(fake)
	for _, ref := range []core.SourceRef{
		{},
		{Provider: "other", ID: "42"},
		{Provider: providerID, ID: "0"},
		{Provider: providerID, ID: "01"},
		{Provider: providerID, ID: "+1"},
		{Provider: providerID, ID: " 1"},
		{Provider: providerID, ID: "12345678901"},
	} {
		if _, err := client.Episodes(context.Background(), ref); !errors.Is(err, errEpisodesInvalidRef) {
			t.Fatalf("ref %#v error = %v", ref, err)
		}
	}
	fake.mu.Lock()
	calls := fake.calls
	fake.mu.Unlock()
	if calls != 0 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestEpisodesPropagatesCancellationAndSanitizesFailures(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	client := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(episodeFixture(t, "episodes-page-1.html"))}})
	if _, err := client.Episodes(canceled, core.SourceRef{Provider: providerID, ID: "42"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	if _, err := newWithDo(&fakeClient{err: errors.New("raw token https://private.example")}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"}); !errors.Is(err, errEpisodesUnavailable) || strings.Contains(err.Error(), "raw token") {
		t.Fatalf("network error = %v", err)
	}
	status := &securehttp.Error{Kind: securehttp.KindStatus}
	if _, err := newWithDo(&fakeClient{err: status}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"}); !errors.Is(err, status) {
		t.Fatalf("secure error = %v", err)
	}
}

func TestEpisodesRejectsMalformedPagesAndDoesNotReturnPartialResults(t *testing.T) {
	base := episodeFixture(t, "episodes-page-1.html")
	malformedTitle := strings.Replace(base, `Example Show [02]`, `Example Show []`, 1)
	malformedTitle = strings.Replace(malformedTitle, `Example Show [01]`, `Example Show []`, 1)
	duplicatePost := strings.Replace(base, `id="post-101"`, `id="post-100"`, 1)
	duplicatePost = strings.Replace(duplicatePost, `href="https://anime1.me/101"`, `href="/100"`, 1)
	invalidLink := strings.Replace(base, `href="https://anime1.me/101"`, `href="https://anime1.me/999"`, 1)
	invalidLink = strings.Replace(invalidLink, `href="/100"`, `href="https://anime1.me/998"`, 1)
	for _, body := range []string{
		strings.Replace(base, `id="main"`, `id="other"`, 1),
		base + `<main id="main"></main>`,
		base + `<nav class="navigation"><div class="nav-previous"><a href="/category/other-show/page/2">Older</a></div></nav>`,
		strings.Replace(base, `category-42`, `category-41`, 1),
		duplicatePost,
		invalidLink,
		malformedTitle,
	} {
		items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
		if !errors.Is(err, errEpisodesMalformed) || items != nil {
			t.Fatalf("items = %#v error = %v", items, err)
		}
	}
	secondPageError := &fakeClient{episodeResponses: []*securehttp.Response{
		episodeResponse(base),
		{StatusCode: http.StatusForbidden, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte("secret")},
	}}
	items, err := newWithDo(secondPageError).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if _, ok := err.(*securehttp.Error); !ok || items != nil {
		t.Fatalf("second page items = %#v error = %v", items, err)
	}
}

func TestEpisodesAcceptsXHTMLAndExactLabels(t *testing.T) {
	body := `<html><body><main id="main"><article id="post-123" class="category-42"><h2 class="entry-title"><a href="/123">Show <span>[BD特典SP]</span></a></h2></article></main></body></html>`
	response := episodeResponse(body)
	response.Header.Set("Content-Type", "application/xhtml+xml")
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{response}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceEpisode{{Ref: episodeRef("123"), Number: "BD特典SP", Title: ""}}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("items = %#v, want %#v", items, expected)
	}
}

func TestEpisodesRejectsPaginationViolations(t *testing.T) {
	base := episodeFixture(t, "episodes-page-1.html")
	for _, href := range []string{
		`/category/example-show/page/1/`,
		`/category/example-show/page/3/`,
		`/category/example-show/page/02/`,
		`/category/example-show/page/2/?secret=yes`,
		`/category/example-show/page/2/#secret`,
		`https://other.example/category/example-show/page/2/`,
		`https://anime1.me:443/category/example-show/page/2/`,
		`/category/./page/2/`,
		`/category/example%2Fshow/page/2/`,
		`/category/%65xample-show/page/2/`,
	} {
		body := strings.Replace(base, `/category/anime/%e7%95%aa/page/2`, href, 1)
		items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
		if !errors.Is(err, errEpisodesMalformed) || items != nil {
			t.Fatalf("href %q items = %#v error = %v", href, items, err)
		}
	}
	pageTwo := episodeFixture(t, "episodes-page-2.html") + `<nav class="navigation"><div class="nav-previous"><a href="/category/other-show/page/3/">Older</a></div></nav>`
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(base), episodeResponse(pageTwo)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("changed prefix items = %#v error = %v", items, err)
	}
}

func TestEpisodesRejectsBoundsAndMIMEViolations(t *testing.T) {
	valid := episodeFixture(t, "episodes-page-2.html")
	tooLarge := strings.Repeat("x", maxEpisodeBodyBytes+1)
	largeResponse := episodeResponse(tooLarge)
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{largeResponse}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("large body items = %#v error = %v", items, err)
	}
	wrongMIME := episodeResponse(valid)
	wrongMIME.Header.Set("Content-Type", "application/json")
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{wrongMIME}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("wrong MIME items = %#v error = %v", items, err)
	}
	nodes := `<html><body>` + strings.Repeat(`<span></span>`, maxEpisodeDOMNodes) + `</body></html>`
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(nodes)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("large DOM items = %#v error = %v", items, err)
	}
}

func TestEpisodesEnforcesLabelLimitsAndSkipsMalformedEntries(t *testing.T) {
	base := `<html><body><main id="main">%s</main></body></html>`
	article := func(postID, label string) string {
		return `<article id="post-` + postID + `" class="category-42"><h2 class="entry-title"><a href="/` + postID + `">Show [` + label + `]</a></h2></article>`
	}
	exact := strings.Repeat("é", maxEpisodeLabelRunes)
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(fmt.Sprintf(base, article("1", exact)))}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil || len(items) != 1 || items[0].Number != exact {
		t.Fatalf("exact label items = %#v error = %v", items, err)
	}
	tooManyBytes := strings.Repeat("é", maxEpisodeLabelRunes-1) + "界"
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(fmt.Sprintf(base, article("2", tooManyBytes)))}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("large label items = %#v error = %v", items, err)
	}
	validArticle := article("3", "1")
	malformedArticle := strings.Replace(article("4", ""), "Show []", "Show [", 1)
	body := fmt.Sprintf(base, malformedArticle+validArticle)
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil || len(items) != 1 || items[0].Ref.ID != "3" {
		t.Fatalf("skipped malformed items = %#v error = %v", items, err)
	}
}

func TestEpisodeBodyAndDOMExactLimits(t *testing.T) {
	base := episodeBodyWithCount(1)
	exactBody := base + strings.Repeat(" ", maxEpisodeBodyBytes-len(base))
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(exactBody)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil || len(items) != 1 {
		t.Fatalf("exact body items = %d error = %v", len(items), err)
	}
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(exactBody + " ")}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("body +1 items = %#v error = %v", items, err)
	}
	if _, err := inspectEpisodeDocument(context.Background(), documentWithNodeCount(maxEpisodeDOMNodes)); err != nil {
		t.Fatalf("exact DOM error = %v", err)
	}
	if _, err := inspectEpisodeDocument(context.Background(), documentWithNodeCount(maxEpisodeDOMNodes+1)); !errors.Is(err, errEpisodesMalformed) {
		t.Fatalf("DOM +1 error = %v", err)
	}
}

func TestEpisodePageAndCountExactLimits(t *testing.T) {
	responses := make([]*securehttp.Response, 0, maxEpisodePages)
	for page := 1; page <= maxEpisodePages; page++ {
		next := 0
		if page < maxEpisodePages {
			next = page + 1
		}
		responses = append(responses, episodeResponse(episodePageBody(10000+page, page, next)))
	}
	items, err := newWithDo(&sequenceEpisodeClient{responses: responses}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil || len(items) != maxEpisodePages {
		t.Fatalf("exact pages items = %d error = %v", len(items), err)
	}
	responses[maxEpisodePages-1] = episodeResponse(episodePageBody(10000+maxEpisodePages, maxEpisodePages, maxEpisodePages+1))
	items, err = newWithDo(&sequenceEpisodeClient{responses: responses}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("pages +1 items = %#v error = %v", items, err)
	}
	exactEpisodes := episodeBodyWithCount(maxEpisodeCount)
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(exactEpisodes)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil || len(items) != maxEpisodeCount {
		t.Fatalf("exact episodes items = %d error = %v", len(items), err)
	}
	items, err = newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(episodeBodyWithCount(maxEpisodeCount + 1))}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, errEpisodesMalformed) || items != nil {
		t.Fatalf("episodes +1 items = %#v error = %v", items, err)
	}
}

func TestEpisodeTextLabelAndIDExactLimits(t *testing.T) {
	exactTitle := strings.Repeat("😀", maxEpisodeTitleRunes)
	if len(exactTitle) != maxEpisodeTitleBytes {
		t.Fatalf("exact title bytes = %d", len(exactTitle))
	}
	if got, err := normalizeEpisodeText(context.Background(), episodeTextNode(exactTitle)); err != nil || got != exactTitle {
		t.Fatalf("exact title length = %d error = %v", len(got), err)
	}
	for name, value := range map[string]string{
		"bytes +1": exactTitle + "x",
		"runes +1": strings.Repeat("x", maxEpisodeTitleRunes+1),
	} {
		if _, err := normalizeEpisodeText(context.Background(), episodeTextNode(value)); !errors.Is(err, errEpisodesMalformed) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
	exactLabel := strings.Repeat("é", maxEpisodeLabelRunes)
	if len(exactLabel) != maxEpisodeLabelBytes {
		t.Fatalf("exact label bytes = %d", len(exactLabel))
	}
	if got, ok, overLimit := episodeNumberResult("Show [" + exactLabel + "]"); !ok || overLimit || got != exactLabel {
		t.Fatalf("exact label = %q ok=%v over=%v", got, ok, overLimit)
	}
	for name, value := range map[string]string{
		"bytes +1": strings.Repeat("é", maxEpisodeLabelRunes-1) + "界",
		"runes +1": strings.Repeat("x", maxEpisodeLabelRunes+1),
	} {
		if _, ok, overLimit := episodeNumberResult("Show [" + value + "]"); ok || !overLimit {
			t.Fatalf("%s ok=%v over=%v", name, ok, overLimit)
		}
	}
	exactPostID := strings.Repeat("9", maxEpisodeIDDigits)
	if got, ok, overLimit := parsePostIDResult("post-" + exactPostID); !ok || overLimit || got != exactPostID {
		t.Fatalf("exact post ID = %q ok=%v over=%v", got, ok, overLimit)
	}
	if _, ok, overLimit := parsePostIDResult("post-" + exactPostID + "9"); ok || !overLimit {
		t.Fatalf("post ID +1 ok=%v over=%v", ok, overLimit)
	}
	exactCategory := strings.Repeat("9", maxCategoryIDDigits)
	if got, ok := parseEpisodeCategoryID(core.SourceRef{Provider: providerID, ID: exactCategory}); !ok || got != exactCategory {
		t.Fatalf("exact category = %q ok=%v", got, ok)
	}
	if _, ok := parseEpisodeCategoryID(core.SourceRef{Provider: providerID, ID: exactCategory + "9"}); ok {
		t.Fatal("category ID +1 accepted")
	}
}

func TestEpisodesRejectsOverLimitArticleBesideValidSibling(t *testing.T) {
	valid := `<article id="post-1" class="category-42"><h2 class="entry-title"><a href="/1">Show [01]</a></h2></article>`
	article := func(postID, title string) string {
		return `<article id="post-` + postID + `" class="category-42"><h2 class="entry-title"><a href="/` + postID + `">` + title + `</a></h2></article>`
	}
	cases := map[string]string{
		"title bytes +1": article("2", strings.Repeat("😀", maxEpisodeTitleRunes)+"x"),
		"title runes +1": article("3", strings.Repeat("x", maxEpisodeTitleRunes+1)),
		"label bytes +1": article("4", "Show ["+strings.Repeat("é", maxEpisodeLabelRunes-1)+"界]"),
		"label runes +1": article("5", "Show ["+strings.Repeat("x", maxEpisodeLabelRunes+1)+"]"),
		"post ID +1":     article(strings.Repeat("9", maxEpisodeIDDigits+1), "Show [02]"),
	}
	for name, overLimit := range cases {
		t.Run(name, func(t *testing.T) {
			body := `<html><body><main id="main">` + valid + overLimit + `</main></body></html>`
			items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
			if !errors.Is(err, errEpisodesMalformed) || items != nil {
				t.Fatalf("items = %#v error = %v", items, err)
			}
		})
	}
}

func TestEpisodeTitleExcludesRawTextTokens(t *testing.T) {
	const token = "playback-secret-token"
	excluded := ""
	for _, element := range []string{"script", "style", "template", "iframe", "textarea", "title", "xmp", "noembed", "noframes", "noscript"} {
		excluded += "<" + element + ">" + token + " [evil]</" + element + ">"
	}
	body := `<html><body><main id="main"><article id="post-123" class="category-42"><h2 class="entry-title"><a href="/123">Show ` + excluded + `[SP]</a></h2><video data-apireq="` + token + `"></video></article></main></body></html>`
	items, err := newWithDo(&fakeClient{episodeResponses: []*securehttp.Response{episodeResponse(body)}}).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	expected := []core.SourceEpisode{{Ref: episodeRef("123"), Number: "SP", Title: ""}}
	if !reflect.DeepEqual(items, expected) || strings.Contains(fmt.Sprintf("%#v", items), token) {
		t.Fatalf("items = %#v", items)
	}
}

func TestEpisodesRejectsNilInputsAndNilResponse(t *testing.T) {
	ref := core.SourceRef{Provider: providerID, ID: "42"}
	if items, err := newWithDo(&fakeClient{}).Episodes(nil, ref); !errors.Is(err, errEpisodesUnavailable) || items != nil {
		t.Fatalf("nil context items = %#v error = %v", items, err)
	}
	var client *Client
	if items, err := client.Episodes(context.Background(), ref); !errors.Is(err, errEpisodesUnavailable) || items != nil {
		t.Fatalf("nil client items = %#v error = %v", items, err)
	}
	if items, err := newWithDo(&fakeClient{}).Episodes(context.Background(), ref); !errors.Is(err, errEpisodesUnavailable) || items != nil {
		t.Fatalf("nil response items = %#v error = %v", items, err)
	}
}

func TestEpisodesSecondPageFailuresReturnNoPartialResult(t *testing.T) {
	first := episodeResponse(episodeFixture(t, "episodes-page-1.html"))
	status := &securehttp.Response{StatusCode: http.StatusForbidden, Header: http.Header{"Content-Type": {"application/json"}}, Body: []byte("secret")}
	wrongMIME := episodeResponse(episodeFixture(t, "episodes-page-2.html"))
	wrongMIME.Header.Set("Content-Type", "application/json")
	malformed := episodeResponse(`<html><body><main id="main"></main></body></html>`)
	cases := []struct {
		name      string
		client    responseClient
		wantKind  securehttp.Kind
		wantError error
	}{
		{name: "unknown network", client: &sequenceEpisodeClient{responses: []*securehttp.Response{first}, errors: []error{nil, errors.New("raw secret URL")}}, wantError: errEpisodesUnavailable},
		{name: "status before MIME", client: &sequenceEpisodeClient{responses: []*securehttp.Response{first, status}}, wantKind: securehttp.KindStatus},
		{name: "wrong MIME", client: &sequenceEpisodeClient{responses: []*securehttp.Response{first, wrongMIME}}, wantError: errEpisodesMalformed},
		{name: "malformed", client: &sequenceEpisodeClient{responses: []*securehttp.Response{first, malformed}}, wantError: errEpisodesMalformed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			items, err := newWithDo(test.client).Episodes(context.Background(), core.SourceRef{Provider: providerID, ID: "42"})
			if items != nil {
				t.Fatalf("partial items = %#v", items)
			}
			if test.wantError != nil && !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
			if test.wantKind != "" {
				var secureError *securehttp.Error
				if !errors.As(err, &secureError) || secureError.Kind != test.wantKind {
					t.Fatalf("secure error = %v", err)
				}
			}
			if strings.Contains(fmt.Sprint(err), "raw secret") {
				t.Fatalf("unsanitized error = %v", err)
			}
		})
	}
}

func TestEpisodesCancellationDuringTraversalAndPagination(t *testing.T) {
	ctx := &cancelAfterChecksContext{cancelAt: 100}
	if _, err := inspectEpisodeDocument(ctx, documentWithNodeCount(1000)); !errors.Is(err, context.Canceled) {
		t.Fatalf("traversal error = %v", err)
	}
	requestContext, cancel := context.WithCancel(context.Background())
	client := &cancelAfterPageClient{response: episodeResponse(episodeFixture(t, "episodes-page-1.html")), cancel: cancel}
	items, err := newWithDo(client).Episodes(requestContext, core.SourceRef{Provider: providerID, ID: "42"})
	if !errors.Is(err, context.Canceled) || items != nil || client.calls != 2 {
		t.Fatalf("pagination items = %#v error = %v calls = %d", items, err, client.calls)
	}
}

func TestAnime1SourceContract(t *testing.T) {
	factory := func(*testing.T) core.AnimeSource {
		return newWithDo(&fakeClient{
			response: jsonResponse(catalogFixture(t)),
			episodeResponses: []*securehttp.Response{
				episodeResponse(episodeFixture(t, "episodes-page-1.html")),
				episodeResponse(episodeFixture(t, "episodes-page-2.html")),
			},
		})
	}
	contract.RunAnimeSource(t, contract.AnimeSourceSuite{
		New:     factory,
		Catalog: contract.SourceListCase{Supported: true, Expected: []core.SourceRef{{Provider: providerID, ID: "42"}, {Provider: providerID, ID: "7"}}},
		Search:  contract.SourceSearchCase{Supported: true, Query: "beta", Expected: []core.SourceRef{{Provider: providerID, ID: "7"}}},
		Episodes: contract.SourceEpisodesCase{
			Supported: true,
			Anime:     core.SourceRef{Provider: providerID, ID: "42"},
			Expected:  []core.EpisodeRef{episodeRef("98"), episodeRef("99"), episodeRef("100"), episodeRef("101")},
		},
		Resolve:          contract.SourceResolveCase{},
		Schedule:         contract.SourceScheduleCase{},
		ForbiddenStrings: []string{"do-not-return", "secret", "<script>"},
	})
}

func TestUnsupportedMethods(t *testing.T) {
	client := New(nil)
	if _, err := client.Episodes(context.Background(), core.SourceRef{}); !errors.Is(err, errEpisodesInvalidRef) {
		t.Fatal(err)
	}
	if _, err := client.Resolve(context.Background(), core.EpisodeRef{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := client.Schedule(context.Background(), core.ScheduleQuery{}); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
