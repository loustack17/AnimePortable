// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
)

const (
	defaultSessionTTL           = 30 * time.Minute
	defaultMaxSessions          = 64
	defaultMaxConcurrentStreams = 16
	defaultMaxStreamsPerSession = 4
	maxSessionTTL               = 24 * time.Hour
	maxSessions                 = 1024
	maxConcurrentStreams        = 128
	maxStreamsPerSession        = 16
	maxHeaderValueBytes         = 8 << 10
	maxSourceHeaderBytes        = 16 << 10
	maxRangeDigits              = 19
	maxRangeHeaderBytes         = 128
)

var (
	errInvalidConfig = errors.New("playback proxy: invalid configuration")
	errInvalidSource = errors.New("playback proxy: invalid source")
	errClosed        = errors.New("playback proxy: unavailable")
)

type Config struct {
	SessionTTL           time.Duration
	MaxSessions          int
	MaxConcurrentStreams int
	MaxStreamsPerSession int
}

type remoteClient interface {
	Open(*http.Request) (*securehttp.StreamResponse, error)
	CloseIdleConnections()
}

type serverDeps struct {
	listen        func(network, address string) (net.Listener, error)
	random        io.Reader
	now           func() time.Time
	remoteFactory func(origin string) (remoteClient, error)
}

type Server struct {
	mu        sync.Mutex
	closed    bool
	closeDone chan struct{}
	sessions  map[string]*Session
	listener  net.Listener
	host      string
	server    *http.Server
	config    Config
	deps      serverDeps
	streams   chan struct{}
	wg        sync.WaitGroup
}

type Session struct {
	server   *Server
	token    string
	url      string
	remote   *url.URL
	headers  http.Header
	client   remoteClient
	deadline time.Time

	mu        sync.Mutex
	revoked   bool
	cancel    context.CancelFunc
	ctx       context.Context
	timer     *time.Timer
	closeDone chan struct{}
	transfers map[*activeTransfer]struct{}
	streams   chan struct{}
	wg        sync.WaitGroup
}

type activeTransfer struct {
	mu            sync.Mutex
	cancel        context.CancelFunc
	controller    *http.ResponseController
	interruptDone chan struct{}
	body          *trackedBody
	interrupted   bool
	finished      bool
}

type trackedBody struct {
	body io.ReadCloser
	once sync.Once
	err  error
}

func (body *trackedBody) Read(buffer []byte) (int, error) {
	return body.body.Read(buffer)
}

func (body *trackedBody) Close() error {
	body.once.Do(func() { body.err = body.body.Close() })
	return body.err
}

func New(config Config) (*Server, error) {
	return newServer(config, serverDeps{})
}

func newServer(config Config, deps serverDeps) (*Server, error) {
	if err := normalizeConfig(&config); err != nil {
		return nil, err
	}
	if deps.listen == nil {
		deps.listen = net.Listen
	}
	if deps.random == nil {
		deps.random = rand.Reader
	}
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.remoteFactory == nil {
		deps.remoteFactory = func(origin string) (remoteClient, error) {
			return securehttp.New(securehttp.Config{AllowedOrigins: []string{origin}})
		}
	}
	listener, err := deps.listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, errClosed
	}
	proxy := &Server{
		closeDone: make(chan struct{}),
		sessions:  make(map[string]*Session),
		listener:  listener,
		host:      listener.Addr().String(),
		config:    config,
		deps:      deps,
		streams:   make(chan struct{}, config.MaxConcurrentStreams),
	}
	proxy.server = &http.Server{
		Handler:           http.HandlerFunc(proxy.handle),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	proxy.wg.Add(1)
	go func() {
		defer proxy.wg.Done()
		_ = proxy.server.Serve(listener)
	}()
	return proxy, nil
}

func normalizeConfig(config *Config) error {
	if config == nil || config.SessionTTL < 0 || config.SessionTTL > maxSessionTTL || config.MaxSessions < 0 || config.MaxSessions > maxSessions || config.MaxConcurrentStreams < 0 || config.MaxConcurrentStreams > maxConcurrentStreams || config.MaxStreamsPerSession < 0 || config.MaxStreamsPerSession > maxStreamsPerSession {
		return errInvalidConfig
	}
	if config.SessionTTL == 0 {
		config.SessionTTL = defaultSessionTTL
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = defaultMaxSessions
	}
	if config.MaxConcurrentStreams == 0 {
		config.MaxConcurrentStreams = defaultMaxConcurrentStreams
	}
	if config.MaxStreamsPerSession == 0 {
		config.MaxStreamsPerSession = defaultMaxStreamsPerSession
		if config.MaxStreamsPerSession > config.MaxConcurrentStreams {
			config.MaxStreamsPerSession = config.MaxConcurrentStreams
		}
	}
	if config.MaxStreamsPerSession > config.MaxConcurrentStreams {
		return errInvalidConfig
	}
	return nil
}

func (server *Server) NewSession(source core.PlaybackSource) (*Session, error) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.closed {
		return nil, errClosed
	}
	remote, origin, headers, err := validateSource(source)
	if err != nil {
		return nil, err
	}
	if len(server.sessions) >= server.config.MaxSessions {
		return nil, errClosed
	}
	client, err := server.deps.remoteFactory(origin)
	if err != nil || client == nil {
		if client != nil {
			client.CloseIdleConnections()
		}
		return nil, errInvalidSource
	}
	token, err := server.newTokenLocked()
	if err != nil {
		client.CloseIdleConnections()
		return nil, errClosed
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &Session{
		server:    server,
		token:     token,
		url:       "http://" + server.host + "/media/" + token,
		remote:    remote,
		headers:   headers,
		client:    client,
		deadline:  server.deps.now().Add(server.config.SessionTTL),
		cancel:    cancel,
		ctx:       ctx,
		closeDone: make(chan struct{}),
		transfers: make(map[*activeTransfer]struct{}),
		streams:   make(chan struct{}, server.config.MaxStreamsPerSession),
	}
	session.timer = time.AfterFunc(server.config.SessionTTL, func() { _ = session.Close() })
	server.sessions[token] = session
	return session, nil
}

func (server *Server) newTokenLocked() (string, error) {
	for range 4 {
		bytes := make([]byte, 32)
		if _, err := io.ReadFull(server.deps.random, bytes); err != nil {
			return "", err
		}
		token := base64.RawURLEncoding.EncodeToString(bytes)
		if _, exists := server.sessions[token]; !exists {
			return token, nil
		}
	}
	return "", errors.New("token collision")
}

func validateSource(source core.PlaybackSource) (*url.URL, string, http.Header, error) {
	remote, err := url.Parse(source.URL())
	if err != nil || remote == nil || !strings.EqualFold(remote.Scheme, "https") || remote.Host == "" || remote.User != nil || remote.Fragment != "" {
		return nil, "", nil, errInvalidSource
	}
	origin := "https://" + remote.Host
	headers, ok := sourceHeaders(source.Headers())
	if !ok {
		return nil, "", nil, errInvalidSource
	}
	return remote, origin, headers, nil
}

func sourceHeaders(source http.Header) (http.Header, bool) {
	allowed := map[string]struct{}{
		"Cookie": {}, "Authorization": {}, "Referer": {}, "User-Agent": {}, "Origin": {}, "Accept": {},
	}
	result := make(http.Header, len(source))
	totalBytes := 0
	for name, values := range source {
		canonical := http.CanonicalHeaderKey(name)
		if canonical != name || !validHeaderName(name) {
			return nil, false
		}
		if _, ok := allowed[canonical]; !ok || isHopHeader(canonical) || canonical == "Host" || canonical == "Range" {
			return nil, false
		}
		if len(values) != 1 {
			return nil, false
		}
		for _, value := range values {
			if !validHeaderValue(value) {
				return nil, false
			}
			totalBytes += len(canonical) + len(value)
			if totalBytes > maxSourceHeaderBytes {
				return nil, false
			}
			result.Add(canonical, value)
		}
	}
	return result, true
}

func validHeaderName(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return true
}

func validHeaderValue(value string) bool {
	if len(value) > maxHeaderValueBytes || strings.ContainsAny(value, "\r\n") {
		return false
	}
	return true
}

func isHopHeader(name string) bool {
	switch name {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}

func (session *Session) URL() string {
	if session == nil {
		return ""
	}
	return session.url
}

func (session *Session) String() string { return "proxy.Session{redacted}" }

func (session *Session) GoString() string { return "proxy.Session{redacted}" }

func (session *Session) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Session string `json:"session"`
	}{Session: "redacted"})
}

func (session *Session) Close() error {
	if session == nil {
		return nil
	}
	server := session.server
	if server != nil {
		server.mu.Lock()
		if server.sessions[session.token] == session {
			delete(server.sessions, session.token)
		}
		server.mu.Unlock()
	}
	session.revoke()
	return nil
}

func (session *Session) revoke() {
	session.mu.Lock()
	if session.revoked {
		done := session.closeDone
		session.mu.Unlock()
		<-done
		return
	}
	session.revoked = true
	transfers := make([]*activeTransfer, 0, len(session.transfers))
	client := session.client
	if session.timer != nil {
		session.timer.Stop()
	}
	session.cancel()
	for transfer := range session.transfers {
		transfers = append(transfers, transfer)
	}
	session.mu.Unlock()
	for _, transfer := range transfers {
		transfer.interrupt()
	}
	if client != nil {
		client.CloseIdleConnections()
	}
	session.wg.Wait()
	session.mu.Lock()
	session.remote = nil
	session.headers = nil
	session.client = nil
	session.transfers = nil
	session.mu.Unlock()
	close(session.closeDone)
}

func (session *Session) acquire(now time.Time) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.revoked || !now.Before(session.deadline) {
		return false
	}
	session.wg.Add(1)
	return true
}

func (session *Session) attach(transfer *activeTransfer) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.revoked {
		return false
	}
	session.transfers[transfer] = struct{}{}
	return true
}

func (session *Session) detach(transfer *activeTransfer) {
	session.mu.Lock()
	delete(session.transfers, transfer)
	session.mu.Unlock()
}

func (transfer *activeTransfer) attachBody(body io.ReadCloser) (*trackedBody, bool) {
	tracked := &trackedBody{body: body}
	transfer.mu.Lock()
	if transfer.interrupted || transfer.finished {
		transfer.mu.Unlock()
		_ = tracked.Close()
		return tracked, false
	}
	transfer.body = tracked
	transfer.mu.Unlock()
	return tracked, true
}

func (transfer *activeTransfer) interrupt() {
	transfer.mu.Lock()
	if transfer.interrupted || transfer.finished {
		transfer.mu.Unlock()
		return
	}
	transfer.interrupted = true
	body := transfer.body
	cancel := transfer.cancel
	controller := transfer.controller
	transfer.mu.Unlock()
	cancel()
	_ = controller.SetWriteDeadline(time.Now())
	if body != nil {
		_ = body.Close()
	}
	close(transfer.interruptDone)
}

func (transfer *activeTransfer) finish() {
	transfer.mu.Lock()
	if transfer.finished {
		transfer.mu.Unlock()
		return
	}
	transfer.finished = true
	body := transfer.body
	cancel := transfer.cancel
	controller := transfer.controller
	interrupted := transfer.interrupted
	transfer.body = nil
	transfer.mu.Unlock()
	cancel()
	if body != nil {
		_ = body.Close()
	}
	if interrupted {
		<-transfer.interruptDone
		_ = controller.SetWriteDeadline(time.Time{})
	}
}

func (server *Server) Close() error {
	if server == nil {
		return nil
	}
	server.mu.Lock()
	if server.closed {
		done := server.closeDone
		server.mu.Unlock()
		<-done
		return nil
	}
	server.closed = true
	sessions := make([]*Session, 0, len(server.sessions))
	for _, session := range server.sessions {
		sessions = append(sessions, session)
	}
	server.sessions = make(map[string]*Session)
	server.mu.Unlock()
	_ = server.listener.Close()
	_ = server.server.Close()
	for _, session := range sessions {
		session.revoke()
	}
	server.wg.Wait()
	close(server.closeDone)
	return nil
}

func (server *Server) handle(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	token, ok := server.requestToken(request)
	if !ok {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	server.mu.Lock()
	if server.closed {
		server.mu.Unlock()
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	session := server.sessions[token]
	if session == nil {
		server.mu.Unlock()
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if !session.acquire(server.deps.now()) {
		server.mu.Unlock()
		_ = session.Close()
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	server.wg.Add(1)
	server.mu.Unlock()
	defer func() {
		session.wg.Done()
		server.wg.Done()
	}()
	server.serveSession(writer, request, session)
}

func (server *Server) requestToken(request *http.Request) (string, bool) {
	if request.Host != server.host || request.URL == nil || request.URL.RawQuery != "" || request.URL.ForceQuery || request.URL.RawPath != "" || request.URL.Path == "" || !strings.HasPrefix(request.URL.Path, "/media/") {
		return "", false
	}
	token := strings.TrimPrefix(request.URL.Path, "/media/")
	if len(token) != 43 || strings.Contains(token, "/") {
		return "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return token, err == nil && len(decoded) == 32
}

func (server *Server) serveSession(writer http.ResponseWriter, request *http.Request, session *Session) {
	rangeHeader, singleRange := singleHeader(request.Header, "Range")
	rangeSpec, rangeRequested, ok := parseRange(rangeHeader)
	if !singleRange || !ok {
		writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if !tryAcquire(server.streams) {
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	if !tryAcquire(session.streams) {
		release(server.streams)
		writer.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	defer release(session.streams)
	defer release(server.streams)
	ctx, cancel := context.WithCancel(request.Context())
	stop := context.AfterFunc(session.ctx, cancel)
	if session.ctx.Err() != nil {
		cancel()
	}
	transfer := &activeTransfer{
		cancel:        cancel,
		controller:    http.NewResponseController(writer),
		interruptDone: make(chan struct{}),
	}
	if !session.attach(transfer) {
		stop()
		cancel()
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	defer func() {
		stop()
		transfer.finish()
		session.detach(transfer)
	}()
	upstream := session.remoteRequest(ctx, request.Method, rangeRequested, rangeHeader)
	response, err := session.client.Open(upstream)
	if err != nil || response == nil || response.Body == nil {
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	body, attached := transfer.attachBody(response.Body)
	if !attached {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if rangeRequested && response.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		if size, valid := unsatisfiedContentRange(response.Header.Get("Content-Range")); valid {
			writer.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(size, 10))
			writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	if !validStatusAndRange(response, rangeSpec, rangeRequested) || !validVideoType(response.Header.Get("Content-Type")) || !validContentEncoding(response.Header) {
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	if !copyResponseHeaders(writer.Header(), response, rangeRequested) {
		writer.WriteHeader(http.StatusBadGateway)
		return
	}
	writer.WriteHeader(response.StatusCode)
	if request.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(writer, body)
}

func tryAcquire(semaphore chan struct{}) bool {
	select {
	case semaphore <- struct{}{}:
		return true
	default:
		return false
	}
}

func release(semaphore chan struct{}) { <-semaphore }

func singleHeader(header http.Header, name string) (string, bool) {
	values := header.Values(name)
	if len(values) == 0 {
		return "", true
	}
	return values[0], len(values) == 1
}

func (session *Session) remoteRequest(ctx context.Context, method string, ranged bool, rangeHeader string) *http.Request {
	request := &http.Request{
		Method: method,
		URL:    cloneURL(session.remote),
		Header: session.headers.Clone(),
		Host:   "",
	}
	request = request.WithContext(ctx)
	request.Header.Set("Accept-Encoding", "identity")
	if ranged {
		request.Header.Set("Range", rangeHeader)
	}
	return request
}

func cloneURL(value *url.URL) *url.URL {
	clone := *value
	return &clone
}

type byteRange struct {
	start  int64
	end    int64
	suffix int64
	kind   byteRangeKind
}

type byteRangeKind uint8

const (
	rangeNone byteRangeKind = iota
	rangeClosed
	rangeOpen
	rangeSuffix
)

func parseRange(value string) (byteRange, bool, bool) {
	if value == "" {
		return byteRange{}, false, true
	}
	if len(value) > maxRangeHeaderBytes || !strings.HasPrefix(value, "bytes=") || strings.Contains(value, ",") {
		return byteRange{}, true, false
	}
	pair := strings.TrimPrefix(value, "bytes=")
	if strings.Count(pair, "-") != 1 {
		return byteRange{}, true, false
	}
	before, after, _ := strings.Cut(pair, "-")
	if before == "" && after == "" {
		return byteRange{}, true, false
	}
	if before == "" {
		suffix, ok := parseRangeNumber(after)
		return byteRange{suffix: suffix, kind: rangeSuffix}, true, ok && suffix > 0
	}
	start, ok := parseRangeNumber(before)
	if !ok {
		return byteRange{}, true, false
	}
	if after == "" {
		return byteRange{start: start, kind: rangeOpen}, true, true
	}
	end, ok := parseRangeNumber(after)
	return byteRange{start: start, end: end, kind: rangeClosed}, true, ok && start <= end
}

func parseRangeNumber(value string) (int64, bool) {
	if value == "" || len(value) > maxRangeDigits {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	result, err := strconv.ParseInt(value, 10, 64)
	return result, err == nil && result >= 0
}

func validStatusAndRange(response *securehttp.StreamResponse, requested byteRange, hasRange bool) bool {
	if !hasRange {
		return response.StatusCode == http.StatusOK && response.Header.Get("Content-Range") == ""
	}
	if response.StatusCode != http.StatusPartialContent {
		return false
	}
	start, end, size, ok := satisfiedContentRange(response.Header.Get("Content-Range"))
	if !ok || !requested.matches(start, end, size) {
		return false
	}
	expectedLength := end - start + 1
	if response.ContentLength >= 0 && response.ContentLength != expectedLength {
		return false
	}
	if value := response.Header.Get("Content-Length"); value != "" {
		length, err := strconv.ParseInt(value, 10, 64)
		if err != nil || length != expectedLength {
			return false
		}
	}
	return true
}

func (requested byteRange) matches(start, end, size int64) bool {
	switch requested.kind {
	case rangeClosed:
		return start == requested.start && end <= requested.end
	case rangeOpen:
		return start == requested.start && end >= start && end < size
	case rangeSuffix:
		want := requested.suffix
		if want > size {
			want = size
		}
		return end == size-1 && end-start+1 == want
	default:
		return false
	}
}

func satisfiedContentRange(value string) (int64, int64, int64, bool) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, 0, 0, false
	}
	pair, sizeText, found := strings.Cut(strings.TrimPrefix(value, "bytes "), "/")
	if !found || sizeText == "*" {
		return 0, 0, 0, false
	}
	startText, endText, found := strings.Cut(pair, "-")
	if !found {
		return 0, 0, 0, false
	}
	start, a := parseRangeNumber(startText)
	end, b := parseRangeNumber(endText)
	size, c := parseRangeNumber(sizeText)
	return start, end, size, a && b && c && size > 0 && start <= end && end < size
}

func unsatisfiedContentRange(value string) (int64, bool) {
	if !strings.HasPrefix(value, "bytes */") {
		return 0, false
	}
	size, ok := parseRangeNumber(strings.TrimPrefix(value, "bytes */"))
	return size, ok
}

func validVideoType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "video/mp4")
}

func validContentEncoding(header http.Header) bool {
	values := header.Values("Content-Encoding")
	return len(values) == 0 || len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity")
}

func copyResponseHeaders(destination http.Header, response *securehttp.StreamResponse, hasRange bool) bool {
	allowed := map[string]struct{}{
		"Content-Type": {}, "Content-Length": {}, "Content-Range": {}, "Accept-Ranges": {}, "Etag": {}, "Last-Modified": {},
	}
	for name, values := range response.Header {
		canonical := http.CanonicalHeaderKey(name)
		if _, ok := allowed[canonical]; !ok {
			continue
		}
		if len(values) != 1 || !validHeaderValue(values[0]) {
			return false
		}
		if canonical == "Content-Range" && !hasRange {
			continue
		}
		if canonical == "Content-Length" {
			length, err := strconv.ParseInt(values[0], 10, 64)
			if err != nil || length < 0 {
				return false
			}
		}
		destination.Set(canonical, values[0])
	}
	if destination.Get("Content-Length") == "" && response.ContentLength >= 0 {
		destination.Set("Content-Length", strconv.FormatInt(response.ContentLength, 10))
	}
	return true
}
