// SPDX-License-Identifier: MPL-2.0

package anime1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

const (
	resolverTestToken  = `%7B%22c%22%3A%221%22%2C%22e%22%3A%22episode%22%7D`
	resolverTestQuery  = "query-secret"
	resolverTestCookie = "cookie-secret"
)

type resolverRequest struct {
	method  string
	url     string
	headers http.Header
	body    string
}

type resolverClient struct {
	mu        sync.Mutex
	responses []*securehttp.Response
	errors    []error
	requests  []resolverRequest
	cancelAt  int
	cancel    context.CancelFunc
}

func (client *resolverClient) Do(request *http.Request) (*securehttp.Response, error) {
	var body []byte
	if request.Body != nil {
		var err error
		body, err = io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
	}
	client.mu.Lock()
	index := len(client.requests)
	client.requests = append(client.requests, resolverRequest{method: request.Method, url: request.URL.String(), headers: request.Header.Clone(), body: string(body)})
	if client.cancel != nil && index+1 == client.cancelAt {
		client.cancel()
	}
	var response *securehttp.Response
	if index < len(client.responses) {
		response = client.responses[index]
	}
	var responseErr error
	if index < len(client.errors) {
		responseErr = client.errors[index]
	}
	client.mu.Unlock()
	return response, responseErr
}

func resolverPage(token string) *securehttp.Response {
	body := `<html><body><main id="main"><div><article id="post-101"><video data-apireq="` + token + `"></video></article></div></main></body></html>`
	return &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: []byte(body)}
}

func resolverAPI(body string, cookies ...string) *securehttp.Response {
	header := http.Header{"Content-Type": {"application/json"}}
	for _, cookie := range cookies {
		header.Add("Set-Cookie", cookie)
	}
	return &securehttp.Response{StatusCode: http.StatusOK, Header: header, Body: []byte(body)}
}

func resolverSuccess() *securehttp.Response {
	return resolverAPI(`{"s":[{"src":"//cdn.v.anime1.me/video.mp4?sig=`+resolverTestQuery+`","type":"video/mp4"}]}`,
		"e="+resolverTestCookie+"-e; Path=/; Secure; HttpOnly",
		"h="+resolverTestCookie+"-h; Path=/; Secure; HttpOnly",
		"p="+resolverTestCookie+"-p; Path=/; Secure; HttpOnly")
}

func resolverContractResponses() []*securehttp.Response {
	return []*securehttp.Response{resolverPage(resolverTestToken), resolverSuccess()}
}

func resolverRef() core.EpisodeRef {
	return core.EpisodeRef{Anime: core.SourceRef{Provider: providerID, ID: "42"}, ID: "101"}
}

func TestResolveExactProtocolAndOpaqueOutput(t *testing.T) {
	fake := &resolverClient{responses: resolverContractResponses()}
	source, err := newWithDo(fake).Resolve(context.Background(), resolverRef())
	if err != nil {
		t.Fatal(err)
	}
	if source.URL() != "https://cdn.v.anime1.me/video.mp4?sig="+resolverTestQuery {
		t.Fatalf("URL host/path mismatch")
	}
	wantCookie := "e=" + resolverTestCookie + "-e; h=" + resolverTestCookie + "-h; p=" + resolverTestCookie + "-p"
	if got := source.Headers().Get("Cookie"); got != wantCookie {
		t.Fatalf("Cookie header shape mismatch")
	}
	if len(fake.requests) != 2 {
		t.Fatalf("requests = %d", len(fake.requests))
	}
	if got := fake.requests[0]; got.method != http.MethodGet || got.url != "https://anime1.me/101" || got.body != "" {
		t.Fatalf("GET request = %#v", got)
	}
	if got := fake.requests[1]; got.method != http.MethodPost || got.url != resolverAPIURL || got.body != "d="+resolverTestToken || got.headers.Get("Content-Type") != "application/x-www-form-urlencoded" || got.headers.Get("Referer") != resolverReferer {
		t.Fatalf("POST protocol mismatch")
	}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{fmt.Sprint(source), fmt.Sprintf("%#v", source), fmt.Sprintf("%+v", source), string(encoded)} {
		for _, secret := range []string{resolverTestToken, resolverTestCookie, resolverTestQuery} {
			if strings.Contains(output, secret) {
				t.Fatal("opaque output leaked resolver secret")
			}
		}
	}
	headers := source.Headers()
	headers.Set("Cookie", "mutated")
	if source.Headers().Get("Cookie") != wantCookie {
		t.Fatal("PlaybackSource headers were not cloned")
	}
}

func TestResolveValidatesInputsBeforeRequest(t *testing.T) {
	for _, ref := range []core.EpisodeRef{
		{},
		{Anime: core.SourceRef{Provider: "other", ID: "42"}, ID: "101"},
		{Anime: core.SourceRef{Provider: providerID, ID: "0"}, ID: "101"},
		{Anime: core.SourceRef{Provider: providerID, ID: "42"}, ID: "0"},
		{Anime: core.SourceRef{Provider: providerID, ID: "42"}, ID: "01"},
		{Anime: core.SourceRef{Provider: providerID, ID: "42"}, ID: strings.Repeat("9", maxEpisodeIDDigits+1)},
	} {
		fake := &resolverClient{}
		if _, err := newWithDo(fake).Resolve(context.Background(), ref); !errors.Is(err, errResolverInvalidRef) || len(fake.requests) != 0 {
			t.Fatalf("ref %#v requests=%d error=%v", ref, len(fake.requests), err)
		}
	}
	if _, err := newWithDo(&resolverClient{}).Resolve(nil, resolverRef()); !errors.Is(err, errResolverUnavailable) {
		t.Fatalf("nil context error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newWithDo(&resolverClient{}).Resolve(ctx, resolverRef()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled error = %v", err)
	}
	var client *Client
	if _, err := client.Resolve(context.Background(), resolverRef()); !errors.Is(err, errResolverUnavailable) {
		t.Fatalf("nil client error = %v", err)
	}
}

func TestResolveRejectsMalformedEpisodePages(t *testing.T) {
	valid := string(resolverPage(resolverTestToken).Body)
	cases := map[string]string{
		"missing main":       strings.Replace(valid, `id="main"`, `id="other"`, 1),
		"duplicate main":     valid + `<main id="main"></main>`,
		"wrong post":         strings.Replace(valid, `post-101`, `post-102`, 1),
		"duplicate token":    strings.Replace(valid, `data-apireq="`+resolverTestToken+`"`, `data-apireq="`+resolverTestToken+`" data-apireq="`+resolverTestToken+`"`, 1),
		"duplicate main id":  strings.Replace(valid, `id="main"`, `id="main" id="main"`, 1),
		"duplicate post id":  strings.Replace(valid, `id="post-101"`, `id="post-101" id="post-101"`, 1),
		"multiple videos":    strings.Replace(valid, `</article>`, `<video data-apireq="`+resolverTestToken+`"></video></article>`, 1),
		"raw text only":      strings.Replace(valid, `<video data-apireq="`+resolverTestToken+`"></video>`, `<script><video data-apireq="`+resolverTestToken+`"></video></script>`, 1),
		"lowercase escape":   strings.Replace(valid, `%7B`, `%7b`, 1),
		"form delimiter":     strings.Replace(valid, resolverTestToken, resolverTestToken+`&x=y`, 1),
		"non-object token":   strings.Replace(valid, resolverTestToken, `%5B%5D`, 1),
		"duplicate JSON key": strings.Replace(valid, resolverTestToken, `%7B%22x%22%3A1%2C%22x%22%3A2%7D`, 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &resolverClient{responses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: []byte(body)}}}
			source, err := newWithDo(fake).Resolve(context.Background(), resolverRef())
			if !errors.Is(err, errResolverMalformed) || source.URL() != "" || len(fake.requests) != 1 {
				t.Fatalf("source=%v requests=%d error=%v", source, len(fake.requests), err)
			}
		})
	}
}

func TestResolverTokenExactLimit(t *testing.T) {
	prefix := `%7B%22x%22%3A%22`
	suffix := `%22%7D`
	exact := prefix + strings.Repeat("a", maxResolverTokenBytes-len(prefix)-len(suffix)) + suffix
	if len(exact) != maxResolverTokenBytes || !validResolverToken(exact) {
		t.Fatal("exact token limit rejected")
	}
	if validResolverToken(exact + "a") {
		t.Fatal("token limit +1 accepted")
	}
	if validResolverToken(`%7B%22x%22%3A%22%FF%22%7D`) {
		t.Fatal("invalid decoded UTF-8 accepted")
	}
}

func TestResolverBodyAndDOMExactLimits(t *testing.T) {
	exactBody := make([]byte, maxResolverBodyBytes)
	client := newWithDo(&resolverClient{responses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: exactBody}}})
	if _, err := client.fetchResolver(context.Background(), http.MethodGet, episodesOrigin+"/101", "", "", ""); err != nil {
		t.Fatalf("exact body rejected: %v", err)
	}
	client = newWithDo(&resolverClient{responses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Body: append(exactBody, 0)}}})
	if _, err := client.fetchResolver(context.Background(), http.MethodGet, episodesOrigin+"/101", "", "", ""); !errors.Is(err, errResolverMalformed) {
		t.Fatalf("body limit +1 error = %v", err)
	}
	if _, err := resolverMain(context.Background(), documentWithNodeCount(maxResolverDOMNodes)); err != nil {
		t.Fatalf("exact DOM rejected: %v", err)
	}
	if _, err := resolverMain(context.Background(), documentWithNodeCount(maxResolverDOMNodes+1)); !errors.Is(err, errResolverMalformed) {
		t.Fatalf("DOM limit +1 error = %v", err)
	}
}

func TestResolveFailureMatrixAndSanitization(t *testing.T) {
	wrongMIME := resolverSuccess()
	wrongMIME.Header.Set("Content-Type", "text/html")
	status := resolverSuccess()
	status.StatusCode = http.StatusForbidden
	status.Header.Set("Content-Type", "text/html")
	tooLarge := resolverSuccess()
	tooLarge.Body = make([]byte, maxResolverBodyBytes+1)
	cases := []struct {
		name      string
		responses []*securehttp.Response
		errors    []error
		want      error
	}{
		{name: "nil first response", responses: []*securehttp.Response{nil}, want: errResolverUnavailable},
		{name: "unknown first network", errors: []error{errors.New("raw-network-secret")}, want: errResolverUnavailable},
		{name: "status before MIME", responses: []*securehttp.Response{status}, want: &securehttp.Error{Kind: securehttp.KindStatus}},
		{name: "page MIME", responses: []*securehttp.Response{{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: resolverPage(resolverTestToken).Body}}, want: errResolverMalformed},
		{name: "nil API response", responses: []*securehttp.Response{resolverPage(resolverTestToken), nil}, want: errResolverUnavailable},
		{name: "API MIME", responses: []*securehttp.Response{resolverPage(resolverTestToken), wrongMIME}, want: errResolverMalformed},
		{name: "API body limit", responses: []*securehttp.Response{resolverPage(resolverTestToken), tooLarge}, want: errResolverMalformed},
		{name: "malformed JSON", responses: []*securehttp.Response{resolverPage(resolverTestToken), resolverAPI(`{`)}, want: errResolverMalformed},
		{name: "duplicate key", responses: []*securehttp.Response{resolverPage(resolverTestToken), resolverAPI(`{"s":[],"s":[]}`)}, want: errResolverMalformed},
		{name: "explicit failure", responses: []*securehttp.Response{resolverPage(resolverTestToken), resolverAPI(`{"success":false,"s":[]}`)}, want: errResolverMalformed},
		{name: "empty sources", responses: []*securehttp.Response{resolverPage(resolverTestToken), resolverAPI(`{"s":[]}`)}, want: errResolverMalformed},
		{name: "unknown API field with no sources", responses: []*securehttp.Response{resolverPage(resolverTestToken), resolverAPI(`{"s":[],"token":"secret"}`)}, want: errResolverMalformed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fake := &resolverClient{responses: test.responses, errors: test.errors}
			source, err := newWithDo(fake).Resolve(context.Background(), resolverRef())
			if source.URL() != "" || !errors.Is(err, test.want) {
				var gotSecure, wantSecure *securehttp.Error
				if !errors.As(err, &gotSecure) || !errors.As(test.want, &wantSecure) || gotSecure.Kind != wantSecure.Kind {
					t.Fatalf("source=%v error=%v want=%v", source, err, test.want)
				}
			}
			if strings.Contains(fmt.Sprint(err), "raw-network-secret") {
				t.Fatalf("unsanitized error = %v", err)
			}
		})
	}
}

func TestResolverSourceURLPolicy(t *testing.T) {
	validSources := `{"s":[{"src":"https://bad.example/video.webm","type":"video/webm"},{"src":"//a.b.v.anime1.me/path/video.MP4?sig=opaque","type":"video/mp4; codecs=avc1"}]}`
	got, err := parseResolverAPI(context.Background(), []byte(validSources))
	if err != nil || got != "https://a.b.v.anime1.me/path/video.MP4?sig=opaque" {
		t.Fatalf("source = %q error = %v", got, err)
	}
	invalid := []string{
		"http://cdn.v.anime1.me/video.mp4",
		"https://v.anime1.me/video.mp4",
		"https://cdn.v.anime1.me:443/video.mp4",
		"https://user@cdn.v.anime1.me/video.mp4",
		"https://cdn.v.anime1.me/video.mp4#fragment",
		"https://CDN.v.anime1.me/video.mp4",
		"HTTPS://cdn.v.anime1.me/video.mp4",
		"https://cdn.v.anime1.me./video.mp4",
		"https://evilv.anime1.me/video.mp4",
		"https://-bad.v.anime1.me/video.mp4",
		"https://cdn.v.anime1.me/video.m3u8",
	}
	for _, value := range invalid {
		if _, ok := normalizeResolverURL(value); ok {
			t.Fatalf("invalid URL accepted")
		}
	}
	if _, ok := normalizeResolverURL("https://cdn.v.anime1.me/" + strings.Repeat("a", maxResolverURLBytes) + ".mp4"); ok {
		t.Fatal("URL limit +1 accepted")
	}
	base := "https://cdn.v.anime1.me/video.mp4?q="
	exact := base + strings.Repeat("a", maxResolverURLBytes-len(base))
	if len(exact) != maxResolverURLBytes {
		t.Fatal("bad URL fixture")
	}
	if _, ok := normalizeResolverURL(exact); !ok {
		t.Fatal("exact URL limit rejected")
	}
	if _, ok := normalizeResolverURL(exact + "a"); ok {
		t.Fatal("URL limit +1 accepted")
	}
	protocolBase := "//cdn.v.anime1.me/video.mp4?q="
	protocolExact := protocolBase + strings.Repeat("a", maxResolverURLBytes-len("https:")-len(protocolBase))
	if normalized, ok := normalizeResolverURL(protocolExact); !ok || len(normalized) != maxResolverURLBytes {
		t.Fatal("exact protocol-relative URL rejected")
	}
	if _, ok := normalizeResolverURL(protocolExact + "a"); ok {
		t.Fatal("protocol-relative URL limit +1 accepted")
	}
	exactHost := strings.Repeat("a", 61) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 51) + ".v.anime1.me"
	if len(exactHost) != 253 || !validResolverHost(exactHost) {
		t.Fatal("exact hostname limit rejected")
	}
	overHost := strings.Repeat("a", 61) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 52) + ".v.anime1.me"
	if len(overHost) != 254 || validResolverHost(overHost) {
		t.Fatal("hostname limit +1 accepted")
	}
}

func TestResolverSourceCountExactLimit(t *testing.T) {
	sources := make([]string, 0, maxResolverSources)
	for index := 0; index < maxResolverSources-1; index++ {
		sources = append(sources, `{"src":"https://bad.example/video.webm","type":"video/webm"}`)
	}
	sources = append(sources, `{"src":"https://cdn.v.anime1.me/video.mp4","type":"video/mp4"}`)
	body := []byte(`{"metadata":"ignored","s":[` + strings.Join(sources, ",") + `]}`)
	if _, err := parseResolverAPI(context.Background(), body); err != nil {
		t.Fatalf("exact source count rejected: %v", err)
	}
	sources = append(sources, `{"src":"https://cdn.v.anime1.me/extra.mp4","type":"video/mp4"}`)
	if _, err := parseResolverAPI(context.Background(), []byte(`{"s":[`+strings.Join(sources, ",")+`]}`)); !errors.Is(err, errResolverMalformed) {
		t.Fatalf("source count +1 error = %v", err)
	}
}

func TestResolverCookiesExactLimitsAndFailures(t *testing.T) {
	exact := strings.Repeat("a", maxResolverCookieBytes)
	header := http.Header{"Set-Cookie": {"e=" + exact, "h=" + exact, "p=" + exact, "ignored=value"}}
	value, err := resolverCookieHeader(header)
	if err != nil || !strings.HasPrefix(value, "e=") || !strings.Contains(value, "; h=") || !strings.Contains(value, "; p=") {
		t.Fatalf("exact cookies rejected: %v", err)
	}
	cases := []http.Header{
		{"Set-Cookie": {"e=" + strings.Repeat("a", maxResolverCookieBytes+1), "h=h", "p=p"}},
		{"Set-Cookie": {"e=e", "e=duplicate", "h=h", "p=p"}},
		{"Set-Cookie": {`e="quoted"`, "h=h", "p=p"}},
		{"Set-Cookie": {"e=e; Max-Age=0", "h=h", "p=p"}},
		{"Set-Cookie": {"e=e; Expires=" + time.Now().UTC().Format(http.TimeFormat), "h=h", "p=p"}},
		{"Set-Cookie": {"e", "e=valid", "h=h", "p=p"}},
		{"Set-Cookie": {"e=e; Max-Age=abc", "h=h", "p=p"}},
		{"Set-Cookie": {"e=e; Expires=garbage", "h=h", "p=p"}},
		{"Set-Cookie": {"e=e", "h=h"}},
		{"Set-Cookie": {"e=bad,value", "h=h", "p=p"}},
	}
	for _, candidate := range cases {
		if _, err := resolverCookieHeader(candidate); !errors.Is(err, errResolverMalformed) {
			t.Fatalf("invalid cookie accepted: %#v", candidate)
		}
	}
}

func TestResolveCancellationDuringAPIRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fake := &resolverClient{responses: resolverContractResponses(), cancelAt: 2, cancel: cancel}
	source, err := newWithDo(fake).Resolve(ctx, resolverRef())
	if !errors.Is(err, context.Canceled) || source.URL() != "" || len(fake.requests) != 2 {
		t.Fatalf("source=%v requests=%d error=%v", source, len(fake.requests), err)
	}
}

func TestAllowedOriginsExactAndCloned(t *testing.T) {
	want := []string{episodesOrigin, resolverOrigin}
	got := AllowedOrigins()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("origins = %#v", got)
	}
	got[0] = "mutated"
	if !reflect.DeepEqual(AllowedOrigins(), want) {
		t.Fatal("origins were not cloned")
	}
}
