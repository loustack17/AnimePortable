//go:build resourcepass

// SPDX-License-Identifier: MPL-2.0

package proxy

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"animeportable/adapters/network/securehttp"
	"animeportable/internal/resourcepass"
)

// churnMediaBytes is far more than loopback socket buffers hold, so a stream that is not read
// keeps its slot until it is closed.
const churnMediaBytes = 64 << 20

// zeroMedia answers every upstream open with zero bytes of a churnMediaBytes video, honouring
// the requested range the way a CDN does. It keeps no per-request state.
type zeroMedia struct{}

func (zeroMedia) Open(request *http.Request) (*securehttp.StreamResponse, error) {
	header := http.Header{"Content-Type": {"video/mp4"}, "Accept-Ranges": {"bytes"}}
	start, end := int64(0), int64(churnMediaBytes-1)
	status := http.StatusOK
	if value := request.Header.Get("Range"); value != "" {
		first, last, _ := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
		start, _ = strconv.ParseInt(first, 10, 64)
		if last != "" {
			end, _ = strconv.ParseInt(last, 10, 64)
		}
		status = http.StatusPartialContent
		header.Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.Itoa(churnMediaBytes))
	}
	length := end - start + 1
	header.Set("Content-Length", strconv.FormatInt(length, 10))
	return &securehttp.StreamResponse{StatusCode: status, Header: header, Body: io.NopCloser(io.LimitReader(zeroReader{}, length)), ContentLength: length}, nil
}

func (zeroMedia) CloseIdleConnections() {}

type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

// Each cycle is one playback session as mpv uses it: a few range requests read in part and
// abandoned and one read to the end. Every 10th cycle sends more requests at once than a
// session may stream (4), every 20th more than all sessions together may (16), and every 5th
// closes the session while a request is still streaming.
func TestResourcePassProxySessionChurn(t *testing.T) {
	var server *Server
	var busy atomic.Int64
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	get := func(session *Session, rangeHeader string) *http.Response {
		request, err := http.NewRequest(http.MethodGet, session.URL(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if rangeHeader != "" {
			request.Header.Set("Range", rangeHeader)
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	readAndClose := func(response *http.Response, limit int64) int64 {
		read, _ := io.Copy(io.Discard, io.LimitReader(response.Body, limit))
		_ = response.Body.Close()
		return read
	}
	// atOnce opens perSession streams on each session at the same time, holds them all open,
	// then closes them, and returns how many the proxy refused as busy.
	atOnce := func(sessions []*Session, perSession int) int64 {
		refusedBefore := busy.Load()
		var wg sync.WaitGroup
		responses := make(chan *http.Response, len(sessions)*perSession)
		for _, session := range sessions {
			for range perSession {
				wg.Go(func() { responses <- get(session, "bytes=0-") })
			}
		}
		wg.Wait()
		close(responses)
		for response := range responses {
			readAndClose(response, 1)
		}
		return busy.Load() - refusedBefore
	}
	newSession := func() *Session {
		session, err := server.NewSession(newSource())
		if err != nil {
			t.Fatal(err)
		}
		return session
	}
	resourcepass.Run(t, "proxy-session-churn", 0, func() {
		var err error
		config := Config{OnStream: func(event StreamEvent) {
			if event.Outcome == StreamBusy {
				busy.Add(1)
			}
		}}
		server, err = newServer(config, serverDeps{remoteFactory: func(string) (remoteClient, error) { return zeroMedia{}, nil }})
		if err != nil {
			t.Fatal(err)
		}
	}, func(i int) {
		session := newSession()
		var wg sync.WaitGroup
		for _, request := range []struct {
			rangeHeader string
			limit       int64
		}{{"bytes=0-", 64 << 10}, {"bytes=1000-2999", 1 << 20}, {"", 32 << 10}} {
			wg.Go(func() { readAndClose(get(session, request.rangeHeader), request.limit) })
		}
		wg.Wait()
		if i%10 == 0 {
			if refused := atOnce([]*Session{session}, 6); refused < 2 {
				t.Fatalf("six streams on one session: %d refused, want at least 2 (cap 4)", refused)
			}
		}
		if i%20 == 0 {
			sessions := []*Session{newSession(), newSession(), newSession(), newSession(), newSession()}
			if refused := atOnce(sessions, 4); refused < 4 {
				t.Fatalf("twenty streams on five sessions: %d refused, want at least 4 (cap 16)", refused)
			}
			for _, other := range sessions {
				_ = other.Close()
			}
		}
		if i%5 == 0 {
			response := get(session, "bytes=0-")
			_ = session.Close()
			if read := readAndClose(response, churnMediaBytes); read >= churnMediaBytes {
				t.Fatal("closing the session did not end its stream")
			}
		}
		_ = session.Close()
	}, func() {
		_ = server.Close()
		client.CloseIdleConnections()
	})
}
