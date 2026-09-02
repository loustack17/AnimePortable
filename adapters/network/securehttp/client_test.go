// SPDX-License-Identifier: MPL-2.0

package securehttp

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsInvalidConfigAndSecuresTransport(t *testing.T) {
	invalid := []struct {
		name   string
		config Config
	}{
		{"missing origins", Config{}},
		{"non HTTPS origin", Config{AllowedOrigins: []string{"http://example.com"}}},
		{"origin with path", Config{AllowedOrigins: []string{"https://example.com/path"}}},
		{"negative timeout", Config{AllowedOrigins: []string{"https://example.com"}, ConnectTimeout: -1}},
		{"overflowing body limit", Config{AllowedOrigins: []string{"https://example.com"}, MaxResponseBytes: math.MaxInt64}},
		{"invalid sensitive header", Config{AllowedOrigins: []string{"https://example.com"}, ExtraSensitiveHeaders: []string{"bad header"}}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.config); !isKind(err, KindConfig) {
				t.Fatal(err)
			}
		})
	}
	c, err := New(Config{AllowedOrigins: []string{"https://EXAMPLE.com.:443/"}, ExtraSensitiveHeaders: []string{"X-Token", "x-token"}})
	if err != nil || c.maxBody != defaultMaxResponseBytes || c.maxRedirects != defaultMaxRedirects || len(c.extraSensitiveHeaders) != 1 {
		t.Fatal(err)
	}
	transport := c.httpClient.Transport.(*http.Transport)
	if transport.Proxy != nil || transport.DialTLSContext != nil || transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("unsafe transport")
	}
}

func TestDoRejectsInvalidURLBeforeTransport(t *testing.T) {
	calls := 0
	c := newRoundTripClientFunc(t, func(*http.Request) (*http.Response, error) {
		calls++
		return httpResponse(http.StatusOK, "ok", make(http.Header)), nil
	})
	requests := []*http.Request{
		mustRequest(t, "http://example.com/a"),
		mustRequest(t, "file://example.com/a"),
		mustRequest(t, "https://unapproved.example/a"),
		mustRequest(t, "https://example.com/a#fragment"),
		{Method: http.MethodGet, URL: &url.URL{Scheme: "https", Host: "bad host"}, Header: make(http.Header)},
	}
	for _, request := range requests {
		if _, err := c.Do(request); err == nil {
			t.Fatal(request.URL)
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
}

func TestURLAndOriginValidation(t *testing.T) {
	c := newTestClient(t, []string{"https://example.com", "https://[2001:4860:4860::8888]"}, nil, nil)
	for _, value := range []string{"http://example.com", "ftp://example.com", "file://example.com/a", "gopher://example.com", "data:text/plain,x", "javascript:alert(1)", "smb://example.com", "nfs://example.com", "custom://example.com", "https://user@example.com", "https://example.com/#x", "https://unapproved.example", "https://example.com:", "https://"} {
		u, _ := url.Parse(value)
		if err := c.validateURL(u); err == nil {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"https://example.com/path?q=1", "HTTPS://EXAMPLE.COM./", "https://[2001:4860:4860::8888]/"} {
		u, err := url.Parse(value)
		if err != nil || c.validateURL(u) != nil {
			t.Fatal(value, err)
		}
	}
	value, err := canonicalOriginString("HTTPS://[2001:4860:4860::8888]:443/")
	if err != nil || value != "https://[2001:4860:4860::8888]" {
		t.Fatal(value, err)
	}
	if _, err := canonicalOriginString("https://[::1"); err == nil {
		t.Fatal("malformed origin accepted")
	}
	if err := c.validateURL(&url.URL{Scheme: "https", Host: "bad host"}); err == nil {
		t.Fatal("malformed request URL accepted")
	}
}

func TestBlockedAddressPolicy(t *testing.T) {
	for _, value := range []string{"0.0.0.0", "127.0.0.1", "10.0.0.1", "100.64.0.1", "169.254.1.1", "169.254.169.254", "192.168.1.1", "198.18.0.1", "192.0.2.1", "224.0.0.1", "::", "::1", "fc00::1", "fec0::1", "fe80::1", "ff00::1", "2001:db8::1", "2002::1", "64:ff9b:1::1", "::ffff:10.0.0.1"} {
		if !blockedAddress(netip.MustParseAddr(value)) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"1.1.1.1", "2001:4860:4860::8888", "2606:4700:4700::1111"} {
		if blockedAddress(netip.MustParseAddr(value)) {
			t.Fatal(value)
		}
	}
}

func TestDialRejectsEveryBlockedAnswerBeforeLowerDial(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1", "fe80::1", "fec0::1", "::ffff:192.168.1.1"} {
		t.Run(value, func(t *testing.T) {
			dialed := false
			c := newTestClient(t, []string{"https://example.com"}, func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr(value)}, nil
			}, func(context.Context, string, string) (net.Conn, error) {
				dialed = true
				return nil, nil
			})
			if _, err := c.dialContext(context.Background(), "tcp", "example.com:443"); !isKind(err, KindAddress) || dialed {
				t.Fatal(err, dialed)
			}
		})
	}
}

func TestDialPinsPublicLiteralAndRejectsMixedAnswers(t *testing.T) {
	var got string
	c := newTestClient(t, []string{"https://example.com:8443"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("::ffff:1.1.1.1")}, nil
	}, func(_ context.Context, _, address string) (net.Conn, error) {
		got = address
		left, right := net.Pipe()
		right.Close()
		return left, nil
	})
	connection, err := c.dialContext(context.Background(), "tcp", "example.com:8443")
	if err != nil || got != "1.1.1.1:8443" {
		t.Fatal(got, err)
	}
	connection.Close()
	dialed := false
	c = newTestClient(t, []string{"https://example.com"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.1")}, nil
	}, func(context.Context, string, string) (net.Conn, error) { dialed = true; return nil, nil })
	if _, err := c.dialContext(context.Background(), "tcp", "example.com:443"); !isKind(err, KindAddress) || dialed {
		t.Fatal(err, dialed)
	}
}

func TestRedirectsValidateDestinationAndStripCrossOriginHeaders(t *testing.T) {
	lookups := 0
	c := newTestClient(t, []string{"https://one.example", "https://two.example"}, func(context.Context, string) ([]netip.Addr, error) {
		lookups++
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}, nil)
	previous := mustRequest(t, "https://one.example/a")
	next := mustRequest(t, "https://two.example/b")
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer", "X-Token"} {
		next.Header.Set(header, "secret")
	}
	c.extraSensitiveHeaders = []string{"X-Token"}
	if err := c.checkRedirect(next, []*http.Request{previous}); err != nil || lookups != 1 {
		t.Fatal(err, lookups)
	}
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Referer", "X-Token"} {
		if next.Header.Get(header) != "" {
			t.Fatal(header)
		}
	}
	same := mustRequest(t, "https://one.example/b")
	same.Header.Set("Authorization", "secret")
	if err := c.checkRedirect(same, []*http.Request{previous}); err != nil || same.Header.Get("Authorization") == "" {
		t.Fatal(err)
	}
	for _, value := range []string{"http://one.example", "https://three.example"} {
		if err := c.checkRedirect(mustRequest(t, value), []*http.Request{previous}); !isKind(err, KindRedirect) {
			t.Fatal(value, err)
		}
	}
	c.maxRedirects = 1
	if err := c.checkRedirect(mustRequest(t, "https://one.example/a"), []*http.Request{previous, previous}); !isKind(err, KindRedirect) {
		t.Fatal(err)
	}
	dialed := false
	c = newTestClient(t, []string{"https://one.example", "https://private.example"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, nil
	})
	if err := c.checkRedirect(mustRequest(t, "https://private.example/a"), []*http.Request{previous}); !isKind(err, KindRedirect) || dialed {
		t.Fatal(err, dialed)
	}
}

func TestDoRejectsBlockedRedirectBeforeSecondRequest(t *testing.T) {
	for _, test := range []struct {
		host    string
		address string
	}{
		{"private.example", "10.0.0.1"},
		{"localhost", "127.0.0.1"},
	} {
		t.Run(test.host, func(t *testing.T) {
			target := "https://" + test.host
			c := newTestClient(t, []string{"https://one.example", target}, func(_ context.Context, host string) ([]netip.Addr, error) {
				if host == test.host {
					return []netip.Addr{netip.MustParseAddr(test.address)}, nil
				}
				return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
			}, nil)
			calls := 0
			c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {target + "/a"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			if _, err := c.Do(mustRequest(t, "https://one.example/a")); !isKind(err, KindRedirect) || calls != 1 {
				t.Fatal(err, calls)
			}
		})
	}
}

func TestDoNeverRedirectsNonIdempotentRequests(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, location := range []string{"https://one.example/other", "https://two.example/other"} {
			c := newTestClient(t, []string{"https://one.example", "https://two.example"}, func(context.Context, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
			}, nil)
			calls := 0
			c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Header: http.Header{"Location": {location}}, Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			request, err := http.NewRequest(http.MethodPost, "https://one.example/api", strings.NewReader("d=secret"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Do(request); !isKind(err, KindRedirect) || calls != 1 {
				t.Fatalf("status=%d location=%s error=%v calls=%d", status, location, err, calls)
			}
		}
	}
}

func TestRedirectResolutionPreservesContextErrors(t *testing.T) {
	for _, test := range []struct {
		cause error
		kind  Kind
	}{
		{context.Canceled, KindCanceled},
		{context.DeadlineExceeded, KindTimeout},
	} {
		c := newTestClient(t, []string{"https://one.example", "https://two.example"}, func(_ context.Context, host string) ([]netip.Addr, error) {
			if host == "two.example" {
				return nil, test.cause
			}
			return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
		}, nil)
		c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://two.example/a"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		})
		_, err := c.Do(mustRequest(t, "https://one.example/a"))
		if !isKind(err, test.kind) || !errors.Is(err, test.cause) {
			t.Fatal(err)
		}
	}
}

func TestDoPreservesNonSuccessAndCopiesData(t *testing.T) {
	sourceHeader := http.Header{"X-Value": {"one"}}
	body := &recordingBody{Reader: strings.NewReader("body")}
	c := newRoundTripClient(t, &http.Response{StatusCode: http.StatusTeapot, ContentLength: 4, Header: sourceHeader, Body: body})
	request := mustRequest(t, "https://example.com/a")
	request.Header.Set("Authorization", "secret")
	response, err := c.Do(request)
	if err != nil || response.StatusCode != http.StatusTeapot || string(response.Body) != "body" || response.RequireSuccess() == nil {
		t.Fatal(response, err)
	}
	response.Header.Set("X-Value", "changed")
	response.Body[0] = 'B'
	if !body.closed || request.Header.Get("Authorization") != "secret" || sourceHeader.Get("X-Value") != "one" || response.Header.Get("X-Value") != "changed" || string(response.Body) != "Body" {
		t.Fatal("input or response mutation")
	}
}

func TestDoBoundsAndClosesBodies(t *testing.T) {
	for _, test := range []struct {
		name     string
		response *http.Response
		kind     Kind
	}{
		{"declared", &http.Response{StatusCode: 200, ContentLength: 4, Header: make(http.Header), Body: &recordingBody{Reader: strings.NewReader("long")}}, KindBodyTooLarge},
		{"streamed", &http.Response{StatusCode: 200, ContentLength: -1, Header: make(http.Header), Body: &recordingBody{Reader: strings.NewReader("long")}}, KindBodyTooLarge},
		{"decompressed", &http.Response{StatusCode: 200, ContentLength: -1, Uncompressed: true, Header: make(http.Header), Body: &recordingBody{Reader: strings.NewReader("long")}}, KindBodyTooLarge},
		{"read", &http.Response{StatusCode: 200, ContentLength: -1, Header: make(http.Header), Body: &recordingBody{Reader: errorReader{}}}, KindNetwork},
		{"close", &http.Response{StatusCode: 200, ContentLength: -1, Header: make(http.Header), Body: &recordingBody{Reader: strings.NewReader("ok"), closeErr: errors.New("secret close")}}, KindNetwork},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := test.response.Body.(*recordingBody)
			c := newRoundTripClient(t, test.response)
			c.maxBody = 3
			_, err := c.Do(mustRequest(t, "https://example.com/a"))
			if !isKind(err, test.kind) || !body.closed {
				t.Fatal(err, body.closed)
			}
		})
	}
}

func TestSanitizedErrors(t *testing.T) {
	for _, source := range []error{context.Canceled, context.DeadlineExceeded, errors.New("secret network tls url")} {
		err := sanitizeError(source)
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "tls") || strings.Contains(err.Error(), "url") {
			t.Fatal(err)
		}
	}
	if !errors.Is(sanitizeError(context.Canceled), context.Canceled) || !errors.Is(sanitizeError(context.DeadlineExceeded), context.DeadlineExceeded) {
		t.Fatal("safe causes missing")
	}
}

func TestTLSVerification(t *testing.T) {
	server, roots := trustedTLSServer(t, "example.test")
	defer server.Close()
	dialed := ""
	c := newTestClient(t, []string{"https://example.test"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}, func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = address
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	if _, err := c.Do(mustRequest(t, "https://example.test/")); !isKind(err, KindNetwork) || dialed != "1.1.1.1:443" {
		t.Fatal(err, dialed)
	}
	c = trustedTestClient(t, "example.test", server, roots)
	response, err := c.Do(mustRequest(t, "https://example.test/"))
	if err != nil || response.StatusCode != http.StatusOK || string(response.Body) != "ok" {
		t.Fatal(response, err)
	}
	c = trustedTestClient(t, "wrong.test", server, roots)
	if _, err := c.Do(mustRequest(t, "https://wrong.test/")); !isKind(err, KindNetwork) {
		t.Fatal(err)
	}
}

func TestCanceledDial(t *testing.T) {
	dialed := false
	c := newTestClient(t, []string{"https://example.com"}, func(ctx context.Context, _ string) ([]netip.Addr, error) {
		return nil, ctx.Err()
	}, func(context.Context, string, string) (net.Conn, error) {
		dialed = true
		return nil, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.dialContext(ctx, "tcp", "example.com:443")
	if !isKind(err, KindCanceled) || !errors.Is(err, context.Canceled) || dialed {
		t.Fatal(err, dialed)
	}
}

func TestOverallTimeout(t *testing.T) {
	c := newRoundTripClientFunc(t, func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	c.httpClient.Timeout = 10 * time.Millisecond
	if _, err := c.Do(mustRequest(t, "https://example.com/a")); !isKind(err, KindTimeout) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestDoSanitizesTransportErrors(t *testing.T) {
	c := newRoundTripClientFunc(t, func(*http.Request) (*http.Response, error) {
		return nil, errors.New("raw-secret token tls url")
	})
	if _, err := c.Do(mustRequest(t, "https://example.com/token?secret=yes")); !isKind(err, KindNetwork) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") {
		t.Fatal(err)
	}
}

func TestOpenReturnsStreamingBodyOwnedByCaller(t *testing.T) {
	sourceHeader := http.Header{"X-Value": {"one"}}
	body := &countingBody{data: []byte("stream")}
	c := newRoundTripClient(t, &http.Response{StatusCode: http.StatusPartialContent, ContentLength: 6, Header: sourceHeader, Body: body})
	response, err := c.Open(mustRequest(t, "https://example.com/media"))
	if err != nil || response == nil || response.StatusCode != http.StatusPartialContent || response.ContentLength != 6 || response.Body == nil {
		t.Fatal(response, err)
	}
	if body.reads != 0 {
		t.Fatalf("Open read %d bytes before returning", body.reads)
	}
	sourceHeader.Set("X-Value", "changed")
	if response.Header.Get("X-Value") != "one" {
		t.Fatal(response.Header)
	}
	read, err := io.ReadAll(response.Body)
	if err != nil || string(read) != "stream" {
		t.Fatal(string(read), err)
	}
	if err := response.Body.Close(); err != nil || !body.closed {
		t.Fatal(err, body.closed)
	}
}

func TestOpenRejectsInvalidURLsAndClosesTransportErrorBodies(t *testing.T) {
	calls := 0
	c := newRoundTripClientFunc(t, func(*http.Request) (*http.Response, error) {
		calls++
		return httpResponse(http.StatusOK, "unexpected", nil), nil
	})
	for _, request := range []*http.Request{nil, {URL: nil}, mustRequest(t, "http://example.com/media"), mustRequest(t, "https://unapproved.example/media")} {
		if _, err := c.Open(request); err == nil {
			t.Fatal(request)
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	body := &recordingBody{Reader: strings.NewReader("ignored")}
	c = newRoundTripClientFunc(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, errors.New("raw-secret token tls url")
	})
	if _, err := c.Open(mustRequest(t, "https://example.com/media?secret=yes")); !isKind(err, KindNetwork) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") || !body.closed {
		t.Fatal(err, body.closed)
	}
}

func TestOpenPreservesRequestContextCancellation(t *testing.T) {
	started := make(chan struct{})
	c := newRoundTripClientFunc(t, func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := mustRequest(t, "https://example.com/media").WithContext(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := c.Open(request)
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("transport was not called")
	}
	cancel()
	select {
	case err := <-done:
		if !isKind(err, KindCanceled) || !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Open did not observe cancellation")
	}
}

func TestOpenUsesRedirectPolicyAndStripsCrossOriginHeaders(t *testing.T) {
	requests := make([]*http.Request, 0, 2)
	c := newTestClient(t, []string{"https://one.example", "https://two.example"}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}, nil)
	c.httpClient.Transport = roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		requests = append(requests, request)
		if len(requests) == 1 {
			return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://two.example/media"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		return httpResponse(http.StatusOK, "stream", http.Header{"X-Result": {"ok"}}), nil
	})
	request := mustRequest(t, "https://one.example/start")
	request.Header.Set("Authorization", "secret")
	response, err := c.Open(request)
	if err != nil || response == nil || len(requests) != 2 {
		t.Fatal(response, err, len(requests))
	}
	if requests[1].Header.Get("Authorization") != "" || response.Header.Get("X-Result") != "ok" {
		t.Fatal(requests[1].Header, response.Header)
	}
	if body, err := io.ReadAll(response.Body); err != nil || string(body) != "stream" {
		t.Fatal(string(body), err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenDisablesOverallTimeoutButDoRemainsBounded(t *testing.T) {
	c := newRoundTripClientFunc(t, func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})
	c.httpClient.Timeout = 10 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	request := mustRequest(t, "https://example.com/media").WithContext(ctx)
	done := make(chan error, 1)
	go func() {
		_, err := c.Open(request)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("Open applied overall timeout: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !isKind(err, KindCanceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Open did not finish after cancellation")
	}
	start := time.Now()
	if _, err := c.Do(mustRequest(t, "https://example.com/media")); !isKind(err, KindTimeout) {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Do exceeded timeout bound: %s", elapsed)
	}
}

func TestCloseIdleConnectionsDelegatesToTransport(t *testing.T) {
	transport := &idleClosingTransport{}
	c := newTestClient(t, []string{"https://example.com"}, nil, nil)
	c.httpClient.Transport = transport
	c.CloseIdleConnections()
	if !transport.closed {
		t.Fatal("transport was not asked to close idle connections")
	}
}

func TestRedactionClonesAndRemovesSecrets(t *testing.T) {
	originalURL, err := url.Parse("https://user:pass@example.com/path-token?query-secret=yes#fragment-secret")
	if err != nil {
		t.Fatal(err)
	}
	redacted := RedactURL(originalURL)
	if redacted == originalURL || redacted.User != nil || redacted.Path != "" || redacted.RawQuery != "" || redacted.Fragment != "" || originalURL.Path != "/path-token" {
		t.Fatal(redacted)
	}
	originalHeaders := http.Header{"Authorization": {"auth-secret"}, "Cookie": {"cookie-secret"}, "X-Token": {"extra-secret"}, "Accept": {"application/json"}}
	redactedHeaders := RedactHeaders(originalHeaders, "X-Token")
	if redactedHeaders.Get("Authorization") != "" || redactedHeaders.Get("Cookie") != "" || redactedHeaders.Get("X-Token") != "" || redactedHeaders.Get("Accept") == "" || originalHeaders.Get("Authorization") == "" {
		t.Fatal(redactedHeaders)
	}
}

func newTestClient(t *testing.T, origins []string, resolver func(context.Context, string) ([]netip.Addr, error), dialer func(context.Context, string, string) (net.Conn, error)) *Client {
	t.Helper()
	c, err := newClient(Config{AllowedOrigins: origins}, resolver, dialer)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func trustedTestClient(t *testing.T, hostname string, server *httptest.Server, roots *x509.CertPool) *Client {
	t.Helper()
	c := newTestClient(t, []string{"https://" + hostname}, func(context.Context, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("1.1.1.1")}, nil
	}, func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	})
	transport := c.httpClient.Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	return c
}

func trustedTLSServer(t *testing.T, hostname string) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	now := time.Now()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test root"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok"))
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{serverDER, caDER}, PrivateKey: serverKey}}, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	return server, roots
}
func newRoundTripClient(t *testing.T, response *http.Response) *Client {
	t.Helper()
	c := newTestClient(t, []string{"https://example.com"}, nil, nil)
	c.httpClient.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) { return response, nil })
	return c
}
func newRoundTripClientFunc(t *testing.T, call roundTripperFunc) *Client {
	t.Helper()
	c := newTestClient(t, []string{"https://example.com"}, nil, nil)
	c.httpClient.Transport = call
	return c
}
func mustRequest(t *testing.T, value string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, value, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
func httpResponse(status int, body string, header http.Header) *http.Response {
	return &http.Response{StatusCode: status, ContentLength: int64(len(body)), Header: header, Body: io.NopCloser(strings.NewReader(body))}
}
func isKind(err error, kind Kind) bool { value, ok := err.(*Error); return ok && value.Kind == kind }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type recordingBody struct {
	io.Reader
	closeErr error
	closed   bool
}

func (b *recordingBody) Close() error { b.closed = true; return b.closeErr }

type countingBody struct {
	data   []byte
	reads  int
	closed bool
}

func (b *countingBody) Read(p []byte) (int, error) {
	b.reads++
	if len(b.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b.data)
	b.data = b.data[n:]
	return n, nil
}

func (b *countingBody) Close() error { b.closed = true; return nil }

type idleClosingTransport struct {
	closed bool
}

func (idleClosingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("unexpected request")
}

func (t *idleClosingTransport) CloseIdleConnections() { t.closed = true }

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("secret read") }
