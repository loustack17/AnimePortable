package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

type fakeRemote struct {
	mu       sync.Mutex
	requests []*http.Request
	response func(*http.Request) *securehttp.StreamResponse
	closed   int
}

func (remote *fakeRemote) Open(request *http.Request) (*securehttp.StreamResponse, error) {
	remote.mu.Lock()
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	remote.requests = append(remote.requests, clone)
	remote.mu.Unlock()
	return remote.response(request), nil
}

func (remote *fakeRemote) CloseIdleConnections() {
	remote.mu.Lock()
	remote.closed++
	remote.mu.Unlock()
}

func (remote *fakeRemote) calls() int {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	return len(remote.requests)
}

func newProxyForTest(t *testing.T, remote *fakeRemote, config Config) *Server {
	return newProxyWithDeps(t, remote, config, serverDeps{})
}

func newProxyWithDeps(t *testing.T, remote remoteClient, config Config, deps serverDeps) *Server {
	t.Helper()
	if deps.remoteFactory == nil {
		deps.remoteFactory = func(origin string) (remoteClient, error) {
			if origin != "https://media.example" {
				t.Fatalf("origin = %q", origin)
			}
			return remote, nil
		}
	}
	server, err := newServer(config, deps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server
}

func newSource() core.PlaybackSource {
	return core.NewPlaybackSource("https://media.example/video.mp4?upstream=secret", http.Header{
		"Cookie":        {"source-cookie"},
		"Authorization": {"Bearer source-token"},
	})
}

func streamResponse(status int, headers http.Header, body string) *securehttp.StreamResponse {
	return &securehttp.StreamResponse{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

func tokenFor(value byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{value}, 32))
}

func sessionRequest(t *testing.T, session *Session, method string) *http.Request {
	t.Helper()
	parsed, err := url.Parse(session.URL())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, parsed.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = parsed.Host
	return request
}

func handleRequest(t *testing.T, server *Server, request *http.Request) *recorder {
	t.Helper()
	result := newRecorder()
	done := make(chan struct{})
	go func() {
		server.handle(result, request)
		close(done)
	}()
	select {
	case <-done:
		return result
	case <-time.After(time.Second):
		t.Fatal("proxy handler did not return")
		return nil
	}
}

func appendRandomBlocks(values ...byte) []byte {
	result := make([]byte, 0, len(values)*32)
	for _, value := range values {
		result = append(result, bytes.Repeat([]byte{value}, 32)...)
	}
	return result
}

func TestSessionStreamsOnlyConfiguredSourceHeaders(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}, "Content-Length": {"5"}, "Set-Cookie": {"upstream-secret"}}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, session.URL(), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Cookie", "client-cookie")
	request.Header.Set("Authorization", "Bearer client-token")
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || string(body) != "video" || response.Header.Get("Set-Cookie") != "" {
		t.Fatalf("status=%d body=%q headers=%v", response.StatusCode, body, response.Header)
	}
	if remote.calls() != 1 {
		t.Fatal("upstream was not called exactly once")
	}
	remote.mu.Lock()
	got := remote.requests[0]
	remote.mu.Unlock()
	if got.URL.String() != "https://media.example/video.mp4?upstream=secret" || got.Header.Get("Cookie") != "source-cookie" || got.Header.Get("Authorization") != "Bearer source-token" || got.Header.Get("Accept-Encoding") != "identity" {
		t.Fatalf("unexpected upstream request: %#v", got)
	}
	if strings.Contains(session.String(), "secret") || strings.Contains(session.GoString(), "secret") {
		t.Fatal("session formatting leaked a secret")
	}
	encoded, err := session.MarshalJSON()
	if err != nil || bytes.Contains(encoded, []byte("secret")) || bytes.Contains(encoded, []byte(session.token)) {
		t.Fatalf("session JSON leaked a secret: %q", encoded)
	}
}

func TestRangeValidationAndForwarding(t *testing.T) {
	remote := &fakeRemote{response: func(request *http.Request) *securehttp.StreamResponse {
		if request.Header.Get("Range") == "bytes=3-5" {
			return streamResponse(http.StatusPartialContent, http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 3-5/10"}, "Content-Length": {"3"}}, "def")
		}
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodGet, session.URL(), nil)
	request.Header.Set("Range", "bytes=3-5")
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusPartialContent || string(body) != "def" || response.Header.Get("Content-Range") != "bytes 3-5/10" {
		t.Fatalf("status=%d body=%q range=%q", response.StatusCode, body, response.Header.Get("Content-Range"))
	}
	bad, _ := http.NewRequest(http.MethodGet, session.URL(), nil)
	bad.Header.Set("Range", "bytes=0-1,3-4")
	badResponse, err := (&http.Client{}).Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	_ = badResponse.Body.Close()
	if badResponse.StatusCode != http.StatusRequestedRangeNotSatisfiable || remote.calls() != 1 {
		t.Fatalf("status=%d calls=%d", badResponse.StatusCode, remote.calls())
	}
}

func TestUnknownHostAndClosedSessionDoNotReachUpstream(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(session.URL())
	wrongHost := &http.Request{Method: http.MethodGet, URL: parsed, Host: "127.0.0.1:1", Header: make(http.Header)}
	recorder := newRecorder()
	server.handle(recorder, wrongHost)
	if recorder.status != http.StatusNotFound || remote.calls() != 0 {
		t.Fatalf("host rejection status=%d calls=%d", recorder.status, remote.calls())
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	closedRequest, _ := http.NewRequest(http.MethodGet, session.URL(), nil)
	closed := newRecorder()
	server.handle(closed, closedRequest)
	if closed.status != http.StatusNotFound || remote.calls() != 0 {
		t.Fatalf("closed status=%d calls=%d", closed.status, remote.calls())
	}
}

func TestConfigLimitsAndSessionLimit(t *testing.T) {
	for _, config := range []Config{
		{SessionTTL: -time.Second},
		{SessionTTL: maxSessionTTL + time.Second},
		{MaxSessions: maxSessions + 1},
		{MaxConcurrentStreams: maxConcurrentStreams + 1},
		{MaxStreamsPerSession: maxStreamsPerSession + 1},
		{MaxConcurrentStreams: 1, MaxStreamsPerSession: 2},
	} {
		if _, err := New(config); !errorsIs(err, errInvalidConfig) {
			t.Fatalf("config %#v accepted: %v", config, err)
		}
	}
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "")
	}}
	server := newProxyForTest(t, remote, Config{MaxSessions: 1})
	if _, err := server.NewSession(newSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := server.NewSession(newSource()); !errorsIs(err, errClosed) {
		t.Fatalf("second session error = %v", err)
	}
}

func TestServerListenerUsesIPv4LoopbackAndEphemeralPort(t *testing.T) {
	var gotNetwork string
	var gotAddress string
	listenerFactory := func(network, address string) (net.Listener, error) {
		gotNetwork = network
		gotAddress = address
		return net.Listen(network, address)
	}
	server, err := newServer(Config{}, serverDeps{
		listen: listenerFactory,
		remoteFactory: func(string) (remoteClient, error) {
			return &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
				return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "")
			}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if gotNetwork != "tcp" || gotAddress != "127.0.0.1:0" {
		t.Fatalf("listen(%q, %q)", gotNetwork, gotAddress)
	}
	address, ok := server.listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address type = %T", server.listener.Addr())
	}
	if address.IP.String() != "127.0.0.1" || address.Port == 0 {
		t.Fatalf("listener address = %v", address)
	}
	if server.host != address.String() {
		t.Fatalf("server host = %q, listener = %q", server.host, address.String())
	}
}

func TestSessionTokensAreURLSafeUniqueAndRandomFailuresAreClosed(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "")
	}}
	server := newProxyWithDeps(t, remote, Config{}, serverDeps{random: bytes.NewReader(appendRandomBlocks(1, 2))})
	first, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	second, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	if first.token != tokenFor(1) || second.token != tokenFor(2) || first.token == second.token {
		t.Fatalf("tokens = %q, %q", first.token, second.token)
	}
	if len(first.token) != 43 || strings.ContainsAny(first.token, "+/=") {
		t.Fatalf("token is not raw URL-safe base64: %q", first.token)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first.token)
	if err != nil || len(decoded) != 32 {
		t.Fatalf("token decode: %v, length=%d", err, len(decoded))
	}

	randomErr := errors.New("random failure")
	failingRemote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "")
	}}
	failing := newProxyWithDeps(t, failingRemote, Config{}, serverDeps{random: errorReader{err: randomErr}})
	if _, err := failing.NewSession(newSource()); !errorsIs(err, errClosed) {
		t.Fatalf("random error = %v", err)
	}
	if failingRemote.closed != 1 {
		t.Fatalf("random failure closed clients = %d", failingRemote.closed)
	}

	collisionRemote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "")
	}}
	collision := newProxyWithDeps(t, collisionRemote, Config{}, serverDeps{random: bytes.NewReader(appendRandomBlocks(3, 3, 3, 3, 3))})
	if _, err := collision.NewSession(newSource()); err != nil {
		t.Fatal(err)
	}
	if _, err := collision.NewSession(newSource()); !errorsIs(err, errClosed) {
		t.Fatalf("token collision = %v", err)
	}
	if collisionRemote.closed != 1 {
		t.Fatalf("collision closed clients = %d", collisionRemote.closed)
	}
}

func TestValidateSourceRejectsUnsafeURLsAndHeaders(t *testing.T) {
	valid := core.NewPlaybackSource("HTTPS://media.example/video.mp4?upstream=secret", http.Header{"Cookie": {"source-cookie"}})
	remote, origin, headers, err := validateSource(valid)
	if err != nil {
		t.Fatal(err)
	}
	if remote.String() != "https://media.example/video.mp4?upstream=secret" || origin != "https://media.example" || headers.Get("Cookie") != "source-cookie" {
		t.Fatalf("validated source = %v, origin=%q, headers=%v", remote, origin, headers)
	}
	cases := []struct {
		name    string
		url     string
		headers http.Header
	}{
		{name: "http scheme", url: "http://media.example/video"},
		{name: "missing host", url: "https:///video"},
		{name: "userinfo", url: "https://user:pass@media.example/video"},
		{name: "fragment", url: "https://media.example/video#fragment"},
		{name: "malformed url", url: "https://media.example/%zz"},
		{name: "noncanonical name", url: "https://media.example/video", headers: http.Header{"cookie": {"value"}}},
		{name: "invalid name", url: "https://media.example/video", headers: http.Header{"Bad Name": {"value"}}},
		{name: "unsupported header", url: "https://media.example/video", headers: http.Header{"X-Secret": {"value"}}},
		{name: "hop header", url: "https://media.example/video", headers: http.Header{"Connection": {"keep-alive"}}},
		{name: "host header", url: "https://media.example/video", headers: http.Header{"Host": {"media.example"}}},
		{name: "range header", url: "https://media.example/video", headers: http.Header{"Range": {"bytes=0-1"}}},
		{name: "zero values", url: "https://media.example/video", headers: http.Header{"Cookie": nil}},
		{name: "multiple values", url: "https://media.example/video", headers: http.Header{"Cookie": {"one", "two"}}},
		{name: "control value", url: "https://media.example/video", headers: http.Header{"Cookie": {"one\r\ntwo"}}},
		{name: "value too long", url: "https://media.example/video", headers: http.Header{"Cookie": {strings.Repeat("a", maxHeaderValueBytes+1)}}},
		{name: "total too large", url: "https://media.example/video", headers: http.Header{
			"Cookie":        {strings.Repeat("a", 6000)},
			"Authorization": {strings.Repeat("b", 6000)},
			"Referer":       {strings.Repeat("c", 6000)},
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := validateSource(core.NewPlaybackSource(test.url, test.headers))
			if !errorsIs(err, errInvalidSource) {
				t.Fatalf("validateSource(%q, %v) = %v", test.url, test.headers, err)
			}
		})
	}
}

func TestInvalidProxyRequestsDoNotReachUpstream(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	server := newProxyWithDeps(t, remote, Config{}, serverDeps{random: bytes.NewReader(bytes.Repeat([]byte{1}, 32))})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	path := "/media/" + session.token
	unknown := tokenFor(2)
	cases := []struct {
		name    string
		request *http.Request
		status  int
	}{
		{name: "missing url", request: &http.Request{Method: http.MethodGet, Host: server.host}, status: http.StatusNotFound},
		{name: "missing path", request: &http.Request{Method: http.MethodGet, Host: server.host, URL: &url.URL{}, Header: make(http.Header)}, status: http.StatusNotFound},
		{name: "wrong prefix", request: requestForPath(server, "/not-media/"+session.token), status: http.StatusNotFound},
		{name: "short token", request: requestForPath(server, "/media/short"), status: http.StatusNotFound},
		{name: "malformed token", request: requestForPath(server, "/media/"+strings.Repeat("!", 43)), status: http.StatusNotFound},
		{name: "traversal", request: requestForPath(server, path+"/../other"), status: http.StatusNotFound},
		{name: "unknown token", request: requestForPath(server, "/media/"+unknown), status: http.StatusNotFound},
		{name: "wrong host", request: requestForPathWithHost(server, path, "127.0.0.1:1"), status: http.StatusNotFound},
		{name: "query", request: requestForPathWithQuery(server, path, "remote=secret", false), status: http.StatusNotFound},
		{name: "force query", request: requestForPathWithQuery(server, path, "", true), status: http.StatusNotFound},
		{name: "raw path", request: requestForRawPath(server, path), status: http.StatusNotFound},
		{name: "wrong method", request: requestForMethod(server, path, http.MethodPost), status: http.StatusMethodNotAllowed},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := handleRequest(t, server, test.request)
			if result.status != test.status {
				t.Fatalf("status = %d, want %d", result.status, test.status)
			}
		})
	}
	if remote.calls() != 0 {
		t.Fatalf("upstream calls = %d", remote.calls())
	}
}

func requestForPath(server *Server, path string) *http.Request {
	return requestForPathWithHost(server, path, server.host)
}

func requestForPathWithHost(server *Server, path, host string) *http.Request {
	return &http.Request{Method: http.MethodGet, URL: &url.URL{Path: path}, Host: host, Header: make(http.Header)}
}

func requestForPathWithQuery(server *Server, path, query string, force bool) *http.Request {
	request := requestForPath(server, path)
	request.URL.RawQuery = query
	request.URL.ForceQuery = force
	return request
}

func requestForRawPath(server *Server, path string) *http.Request {
	request := requestForPath(server, path)
	request.URL.RawPath = path
	return request
}

func requestForMethod(server *Server, path, method string) *http.Request {
	request := requestForPath(server, path)
	request.Method = method
	return request
}

func TestSessionExpiryUsesInjectedClockAndCloseIsIdempotent(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	current := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	server := newProxyWithDeps(t, remote, Config{SessionTTL: time.Hour}, serverDeps{
		now:    func() time.Time { return current },
		random: bytes.NewReader(bytes.Repeat([]byte{1}, 32)),
	})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(time.Hour)
	result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet))
	if result.status != http.StatusNotFound || remote.calls() != 0 {
		t.Fatalf("expired session status=%d calls=%d", result.status, remote.calls())
	}
	server.mu.Lock()
	_, present := server.sessions[session.token]
	server.mu.Unlock()
	if present {
		t.Fatal("expired session remained registered")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestValidRangeFormsForwardAndAcceptCompatible206Responses(t *testing.T) {
	cases := []struct {
		name     string
		request  string
		response string
		body     string
	}{
		{name: "closed", request: "bytes=3-5", response: "bytes 3-4/10", body: "de"},
		{name: "open", request: "bytes=3-", response: "bytes 3-9/10", body: "defghij"},
		{name: "suffix", request: "bytes=-3", response: "bytes 7-9/10", body: "hij"},
	}
	remote := &fakeRemote{response: func(request *http.Request) *securehttp.StreamResponse {
		header := http.Header{"Content-Type": {"video/mp4"}}
		body := ""
		switch request.Header.Get("Range") {
		case "bytes=3-5":
			header.Set("Content-Range", "bytes 3-4/10")
			body = "de"
		case "bytes=3-":
			header.Set("Content-Range", "bytes 3-9/10")
			body = "defghij"
		case "bytes=-3":
			header.Set("Content-Range", "bytes 7-9/10")
			body = "hij"
		}
		header.Set("Content-Length", fmt.Sprint(len(body)))
		return streamResponse(http.StatusPartialContent, header, body)
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := sessionRequest(t, session, http.MethodGet)
			request.Header.Set("Range", test.request)
			result := handleRequest(t, server, request)
			if result.status != http.StatusPartialContent || result.body.String() != test.body || result.Header().Get("Content-Range") != test.response {
				t.Fatalf("status=%d body=%q range=%q", result.status, result.body.String(), result.Header().Get("Content-Range"))
			}
		})
	}
	if remote.calls() != len(cases) {
		t.Fatalf("upstream calls = %d, want %d", remote.calls(), len(cases))
	}
	remote.mu.Lock()
	defer remote.mu.Unlock()
	for index, test := range cases {
		if got := remote.requests[index].Header.Get("Range"); got != test.request {
			t.Errorf("request %d range = %q, want %q", index, got, test.request)
		}
	}
}

func TestInvalidRangesDoNotReachUpstream(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusPartialContent, http.Header{"Content-Type": {"video/mp4"}, "Content-Range": {"bytes 0-0/1"}}, "x")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"bytes=0-1,2-3",
		"bytes=" + strings.Repeat("1", maxRangeHeaderBytes),
		"bytes=abc-4",
		"bytes=4-3",
		"bytes=-0",
		"bytes=-",
		"bytes=0-1-2",
		"bytes=9223372036854775808-",
		"bytes=0-9223372036854775808",
	}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			request := sessionRequest(t, session, http.MethodGet)
			request.Header.Set("Range", value)
			result := handleRequest(t, server, request)
			if result.status != http.StatusRequestedRangeNotSatisfiable {
				t.Fatalf("status = %d", result.status)
			}
		})
	}
	multiple := sessionRequest(t, session, http.MethodGet)
	multiple.Header["Range"] = []string{"bytes=0-1", "bytes=2-3"}
	if result := handleRequest(t, server, multiple); result.status != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("multiple Range status = %d", result.status)
	}
	if remote.calls() != 0 {
		t.Fatalf("upstream calls = %d", remote.calls())
	}
}

func TestPartialContentCompatibilityRejectsMismatchesAndClosesBodies(t *testing.T) {
	cases := []struct {
		name           string
		status         int
		contentRange   string
		contentType    string
		contentLength  string
		responseLength int64
		wantStatus     int
	}{
		{name: "wrong start", status: http.StatusPartialContent, contentRange: "bytes 2-5/10", contentType: "video/mp4", contentLength: "4", responseLength: 4, wantStatus: http.StatusBadGateway},
		{name: "wrong end", status: http.StatusPartialContent, contentRange: "bytes 3-6/10", contentType: "video/mp4", contentLength: "4", responseLength: 4, wantStatus: http.StatusBadGateway},
		{name: "wrong header length", status: http.StatusPartialContent, contentRange: "bytes 3-5/10", contentType: "video/mp4", contentLength: "2", responseLength: 3, wantStatus: http.StatusBadGateway},
		{name: "wrong response length", status: http.StatusPartialContent, contentRange: "bytes 3-5/10", contentType: "video/mp4", contentLength: "3", responseLength: 2, wantStatus: http.StatusBadGateway},
		{name: "wrong type", status: http.StatusPartialContent, contentRange: "bytes 3-5/10", contentType: "application/octet-stream", contentLength: "3", responseLength: 3, wantStatus: http.StatusBadGateway},
		{name: "wrong status", status: http.StatusOK, contentRange: "bytes 3-5/10", contentType: "video/mp4", contentLength: "3", responseLength: 3, wantStatus: http.StatusBadGateway},
		{name: "malformed range", status: http.StatusPartialContent, contentRange: "bytes 3-5", contentType: "video/mp4", contentLength: "3", responseLength: 3, wantStatus: http.StatusBadGateway},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := newTrackingBody("def")
			remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
				return &securehttp.StreamResponse{
					StatusCode:    test.status,
					Header:        http.Header{"Content-Type": {test.contentType}, "Content-Range": {test.contentRange}, "Content-Length": {test.contentLength}},
					Body:          body,
					ContentLength: test.responseLength,
				}
			}}
			server := newProxyForTest(t, remote, Config{})
			session, err := server.NewSession(newSource())
			if err != nil {
				t.Fatal(err)
			}
			request := sessionRequest(t, session, http.MethodGet)
			request.Header.Set("Range", "bytes=3-5")
			result := handleRequest(t, server, request)
			if result.status != test.wantStatus || result.body.Len() != 0 {
				t.Fatalf("status=%d body=%q", result.status, result.body.String())
			}
			if !body.isClosed() || body.closeCountValue() != 1 {
				t.Fatalf("body closed=%v count=%d", body.isClosed(), body.closeCountValue())
			}
		})
	}
}

func TestHeadClosesUpstreamBodyWithoutWritingIt(t *testing.T) {
	body := newTrackingBody("video")
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return &securehttp.StreamResponse{
			StatusCode:    http.StatusOK,
			Header:        http.Header{"Content-Type": {"video/mp4"}, "Content-Length": {"5"}},
			Body:          body,
			ContentLength: 5,
		}
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	result := handleRequest(t, server, sessionRequest(t, session, http.MethodHead))
	if result.status != http.StatusOK || result.body.Len() != 0 || result.Header().Get("Content-Length") != "5" {
		t.Fatalf("status=%d body=%q content-length=%q", result.status, result.body.String(), result.Header().Get("Content-Length"))
	}
	if !body.isClosed() || body.closeCountValue() != 1 {
		t.Fatalf("body closed=%v count=%d", body.isClosed(), body.closeCountValue())
	}
}

func TestUnsatisfiedRangeReturnsValidated416WithoutErrorBody(t *testing.T) {
	body := newTrackingBody("upstream error")
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return &securehttp.StreamResponse{
			StatusCode:    http.StatusRequestedRangeNotSatisfiable,
			Header:        http.Header{"Content-Range": {"bytes */10"}, "Content-Length": {"14"}, "Content-Type": {"text/plain"}},
			Body:          body,
			ContentLength: 14,
		}
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	request := sessionRequest(t, session, http.MethodGet)
	request.Header.Set("Range", "bytes=100-200")
	result := handleRequest(t, server, request)
	if result.status != http.StatusRequestedRangeNotSatisfiable || result.body.Len() != 0 {
		t.Fatalf("status=%d body=%q", result.status, result.body.String())
	}
	if result.Header().Get("Content-Range") != "bytes */10" || result.Header().Get("Content-Length") != "" {
		t.Fatalf("response headers = %v", result.Header())
	}
	if !body.isClosed() || body.closeCountValue() != 1 {
		t.Fatalf("body closed=%v count=%d", body.isClosed(), body.closeCountValue())
	}
}

func TestResponseHeadersStripCookiesLocationsAndHopByHopFields(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{
			"Content-Type":        {"video/mp4"},
			"Content-Length":      {"5"},
			"Accept-Ranges":       {"bytes"},
			"Etag":                {"\"etag\""},
			"Last-Modified":       {"Wed, 21 Oct 2015 07:28:00 GMT"},
			"Set-Cookie":          {"secret=1", "other=2"},
			"Location":            {"https://secret.example/"},
			"Connection":          {"close"},
			"Keep-Alive":          {"timeout=5"},
			"Proxy-Authenticate":  {"Basic"},
			"Proxy-Authorization": {"Basic secret"},
			"Te":                  {"trailers"},
			"Trailer":             {"X-Trailer"},
			"Transfer-Encoding":   {"chunked"},
			"Upgrade":             {"websocket"},
		}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet))
	if result.status != http.StatusOK || result.body.String() != "video" {
		t.Fatalf("status=%d body=%q", result.status, result.body.String())
	}
	for _, name := range []string{"Set-Cookie", "Location", "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade", "Content-Range"} {
		if value := result.Header().Get(name); value != "" {
			t.Errorf("header %s leaked %q", name, value)
		}
	}
	for name, want := range map[string]string{
		"Content-Type":   "video/mp4",
		"Content-Length": "5",
		"Accept-Ranges":  "bytes",
		"Etag":           "\"etag\"",
		"Last-Modified":  "Wed, 21 Oct 2015 07:28:00 GMT",
	} {
		if got := result.Header().Get(name); got != want {
			t.Errorf("header %s = %q, want %q", name, got, want)
		}
	}
}

func TestSessionCloseCancelsInflightOpenAndPreventsNewUpstreamRequests(t *testing.T) {
	remote := newBlockingOpenRemote()
	server := newProxyWithDeps(t, remote, Config{}, serverDeps{remoteFactory: func(string) (remoteClient, error) {
		return remote, nil
	}})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	result := newRecorder()
	handlerDone := make(chan struct{})
	go func() {
		server.handle(result, sessionRequest(t, session, http.MethodGet))
		close(handlerDone)
	}()
	select {
	case <-remote.started:
	case <-time.After(time.Second):
		t.Fatal("upstream Open did not start")
	}
	closeDone := make(chan struct{})
	go func() {
		_ = session.Close()
		close(closeDone)
	}()
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Session.Close did not wait for Open to cancel")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after Session.Close")
	}
	if result.status != http.StatusBadGateway || remote.callsValue() != 1 {
		t.Fatalf("status=%d calls=%d", result.status, remote.callsValue())
	}
	if result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet)); result.status != http.StatusNotFound {
		t.Fatalf("closed session status=%d", result.status)
	}
	if remote.callsValue() != 1 {
		t.Fatalf("closed session opened upstream %d times", remote.callsValue())
	}
}

func TestSessionCloseCancelsInflightBodyAndPreventsNewUpstreamRequests(t *testing.T) {
	body := newBlockingBody()
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return &securehttp.StreamResponse{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: body, ContentLength: -1}
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	result := newRecorder()
	handlerDone := make(chan struct{})
	go func() {
		server.handle(result, sessionRequest(t, session, http.MethodGet))
		close(handlerDone)
	}()
	select {
	case <-body.started:
	case <-time.After(time.Second):
		t.Fatal("upstream body Read did not start")
	}
	closeDone := make(chan struct{})
	go func() {
		_ = session.Close()
		close(closeDone)
	}()
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Session.Close did not close the in-flight body")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after body close")
	}
	if !body.isClosedValue() || body.closeCountValue() != 1 || remote.calls() != 1 {
		t.Fatalf("body closed=%v count=%d calls=%d", body.isClosedValue(), body.closeCountValue(), remote.calls())
	}
	if result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet)); result.status != http.StatusNotFound {
		t.Fatalf("closed session status=%d", result.status)
	}
	if remote.calls() != 1 {
		t.Fatalf("closed session opened upstream %d times", remote.calls())
	}
}

func TestSessionCloseUnblocksBlockedDownstreamWriter(t *testing.T) {
	body := io.NopCloser(strings.NewReader("video"))
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return &securehttp.StreamResponse{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: body, ContentLength: 5}
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	writer := newBlockingWriter()
	handlerDone := make(chan struct{})
	go func() {
		server.handle(writer, sessionRequest(t, session, http.MethodGet))
		close(handlerDone)
	}()
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("downstream Write did not start")
	}
	closeDone := make(chan struct{})
	go func() {
		_ = session.Close()
		close(closeDone)
	}()
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Session.Close did not unblock downstream Write")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish after downstream Write was unblocked")
	}
	if writer.writeErrValue() == nil || remote.calls() != 1 {
		t.Fatalf("write error=%v upstream calls=%d", writer.writeErrValue(), remote.calls())
	}
}

func TestConcurrentServerCloseWaitsForInflightRequestsAndIsIdempotent(t *testing.T) {
	remote := newBlockingOpenRemote()
	server := newProxyWithDeps(t, remote, Config{}, serverDeps{remoteFactory: func(string) (remoteClient, error) {
		return remote, nil
	}})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	result := newRecorder()
	handlerDone := make(chan struct{})
	go func() {
		server.handle(result, sessionRequest(t, session, http.MethodGet))
		close(handlerDone)
	}()
	select {
	case <-remote.started:
	case <-time.After(time.Second):
		t.Fatal("upstream Open did not start")
	}
	firstCloseDone := make(chan struct{})
	go func() {
		_ = server.Close()
		close(firstCloseDone)
	}()
	secondCloseDone := make(chan struct{})
	go func() {
		_ = server.Close()
		close(secondCloseDone)
	}()
	select {
	case <-firstCloseDone:
	case <-time.After(time.Second):
		t.Fatal("Server.Close did not wait for in-flight Open")
	}
	select {
	case <-secondCloseDone:
	case <-time.After(time.Second):
		t.Fatal("concurrent Server.Close did not wait for first close")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("handler did not finish during Server.Close")
	}
	if result.status != http.StatusBadGateway || remote.callsValue() != 1 {
		t.Fatalf("status=%d calls=%d", result.status, remote.callsValue())
	}
	if _, err := server.NewSession(newSource()); !errorsIs(err, errClosed) {
		t.Fatalf("NewSession after close = %v", err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestServerCloseInvalidatesSessionsAndRejectsNewSessions(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := server.NewSession(newSource()); !errorsIs(err, errClosed) {
		t.Fatalf("NewSession after close = %v", err)
	}
	if result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet)); result.status != http.StatusNotFound {
		t.Fatalf("invalidated session status=%d", result.status)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStreamSaturationIsNonblockingAndDoesNotReachUpstream(t *testing.T) {
	cases := []struct {
		name string
		fill func(*Server, *Session)
	}{
		{name: "global", fill: func(server *Server, _ *Session) { server.streams <- struct{}{} }},
		{name: "session", fill: func(_ *Server, session *Session) { session.streams <- struct{}{} }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
				return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
			}}
			server := newProxyForTest(t, remote, Config{MaxConcurrentStreams: 1, MaxStreamsPerSession: 1})
			session, err := server.NewSession(newSource())
			if err != nil {
				t.Fatal(err)
			}
			test.fill(server, session)
			result := handleRequest(t, server, sessionRequest(t, session, http.MethodGet))
			if result.status != http.StatusServiceUnavailable || remote.calls() != 0 {
				t.Fatalf("status=%d calls=%d", result.status, remote.calls())
			}
			if test.name == "global" {
				<-server.streams
			} else {
				<-session.streams
			}
		})
	}
}

func TestSessionFormattingURLsAndErrorsRedactSecrets(t *testing.T) {
	remote := &fakeRemote{response: func(*http.Request) *securehttp.StreamResponse {
		return streamResponse(http.StatusOK, http.Header{"Content-Type": {"video/mp4"}}, "video")
	}}
	server := newProxyForTest(t, remote, Config{})
	session, err := server.NewSession(newSource())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(session.URL(), "upstream=secret") || strings.Contains(session.URL(), "source-cookie") || strings.Contains(session.URL(), "source-token") {
		t.Fatalf("session URL leaked source data: %q", session.URL())
	}
	for name, value := range map[string]string{
		"String":   fmt.Sprintf("%v", session),
		"GoString": fmt.Sprintf("%#v", session),
	} {
		if strings.Contains(value, session.token) || strings.Contains(value, "upstream=secret") || strings.Contains(value, "source-cookie") || strings.Contains(value, "source-token") {
			t.Errorf("%s leaked session data: %q", name, value)
		}
	}
	encoded, err := session.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{session.token, "upstream=secret", "source-cookie", "source-token"} {
		if bytes.Contains(encoded, []byte(secret)) {
			t.Errorf("JSON leaked %q: %q", secret, encoded)
		}
	}
	factoryErr := errors.New("remote failed for upstream=secret cookie=source-cookie")
	failing := newProxyWithDeps(t, remote, Config{}, serverDeps{remoteFactory: func(string) (remoteClient, error) {
		return nil, factoryErr
	}})
	err = nil
	_, err = failing.NewSession(newSource())
	if !errorsIs(err, errInvalidSource) {
		t.Fatalf("remote factory error = %v", err)
	}
	for _, secret := range []string{"upstream=secret", "source-cookie", "source-token"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error leaked %q: %v", secret, err)
		}
	}
}

type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func newRecorder() *recorder { return &recorder{header: make(http.Header)} }

func (recorder *recorder) Header() http.Header { return recorder.header }

func (recorder *recorder) Write(value []byte) (int, error) {
	if recorder.status == 0 {
		recorder.status = http.StatusOK
	}
	return recorder.body.Write(value)
}

func (recorder *recorder) WriteHeader(status int) {
	if recorder.status == 0 {
		recorder.status = status
	}
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

type trackingBody struct {
	mu         sync.Mutex
	reader     *bytes.Reader
	closed     bool
	closeCount int
}

func newTrackingBody(value string) *trackingBody {
	return &trackingBody{reader: bytes.NewReader([]byte(value))}
}

func (body *trackingBody) Read(value []byte) (int, error) {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.reader.Read(value)
}

func (body *trackingBody) Close() error {
	body.mu.Lock()
	body.closed = true
	body.closeCount++
	body.mu.Unlock()
	return nil
}

func (body *trackingBody) isClosed() bool {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.closed
}

func (body *trackingBody) closeCountValue() int {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.closeCount
}

type blockingBody struct {
	started    chan struct{}
	start      sync.Once
	closed     chan struct{}
	close      sync.Once
	mu         sync.Mutex
	isClosed   bool
	closeCount int
}

func newBlockingBody() *blockingBody {
	return &blockingBody{started: make(chan struct{}), closed: make(chan struct{})}
}

func (body *blockingBody) Read([]byte) (int, error) {
	body.start.Do(func() { close(body.started) })
	<-body.closed
	return 0, io.EOF
}

func (body *blockingBody) Close() error {
	body.mu.Lock()
	body.isClosed = true
	body.closeCount++
	body.mu.Unlock()
	body.close.Do(func() { close(body.closed) })
	return nil
}

func (body *blockingBody) isClosedValue() bool {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.isClosed
}

func (body *blockingBody) closeCountValue() int {
	body.mu.Lock()
	defer body.mu.Unlock()
	return body.closeCount
}

type blockingOpenRemote struct {
	mu      sync.Mutex
	started chan struct{}
	start   sync.Once
	calls   int
	closed  int
}

func newBlockingOpenRemote() *blockingOpenRemote {
	return &blockingOpenRemote{started: make(chan struct{})}
}

func (remote *blockingOpenRemote) Open(request *http.Request) (*securehttp.StreamResponse, error) {
	remote.mu.Lock()
	remote.calls++
	remote.mu.Unlock()
	remote.start.Do(func() { close(remote.started) })
	<-request.Context().Done()
	return nil, request.Context().Err()
}

func (remote *blockingOpenRemote) CloseIdleConnections() {
	remote.mu.Lock()
	remote.closed++
	remote.mu.Unlock()
}

func (remote *blockingOpenRemote) callsValue() int {
	remote.mu.Lock()
	defer remote.mu.Unlock()
	return remote.calls
}

type blockingWriter struct {
	mu       sync.Mutex
	header   http.Header
	status   int
	started  chan struct{}
	start    sync.Once
	unblock  chan struct{}
	close    sync.Once
	writeErr error
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{header: make(http.Header), started: make(chan struct{}), unblock: make(chan struct{})}
}

func (writer *blockingWriter) Header() http.Header { return writer.header }

func (writer *blockingWriter) WriteHeader(status int) {
	writer.mu.Lock()
	if writer.status == 0 {
		writer.status = status
	}
	writer.mu.Unlock()
}

func (writer *blockingWriter) Write([]byte) (int, error) {
	writer.start.Do(func() { close(writer.started) })
	<-writer.unblock
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.writeErr = errors.New("downstream write interrupted")
	return 0, writer.writeErr
}

func (writer *blockingWriter) SetWriteDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		writer.close.Do(func() { close(writer.unblock) })
	}
	return nil
}

func (writer *blockingWriter) writeErrValue() error {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.writeErr
}

func errorsIs(got, want error) bool { return got == want }
