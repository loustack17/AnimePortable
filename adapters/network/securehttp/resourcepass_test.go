//go:build resourcepass

// SPDX-License-Identifier: MPL-2.0

package securehttp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"animeportable/internal/resourcepass"
)

// Each cycle cancels requests the ways a user's search or episode change does: before the
// answer arrives, in the middle of a body, before the request starts, and abandons a stream
// without cancelling; one request completes. Server and client run in this process, so their
// goroutines are both counted.
func TestResourcePassCancelledRequests(t *testing.T) {
	var server *httptest.Server
	var client *Client
	arrived := make(chan struct{}, 1)
	request := func(ctx context.Context, path string) *http.Request {
		value, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.test"+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	resourcepass.Run(t, "cancelled-requests", 0, func() {
		tlsServer, pool := trustedTLSServer(t, "example.test")
		tlsServer.Config.Handler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/hang":
				select {
				case arrived <- struct{}{}:
				default:
				}
				<-request.Context().Done()
			case "/stream":
				response.WriteHeader(http.StatusOK)
				chunk := make([]byte, 16<<10)
				for range 256 {
					if _, err := response.Write(chunk); err != nil {
						return
					}
					response.(http.Flusher).Flush()
				}
			default:
				_, _ = response.Write([]byte("ok"))
			}
		})
		server = tlsServer
		client = trustedTestClient(t, "example.test", server, pool)
	}, func(int) {
		// Cancelled while waiting for the answer.
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			select {
			case <-arrived:
			case <-time.After(5 * time.Second):
				t.Error("the request never reached the server")
			}
			cancel()
		}()
		if _, err := client.Do(request(ctx, "/hang")); err == nil {
			t.Fatal("a cancelled request succeeded")
		}
		cancel()

		// Cancelled in the middle of the body.
		ctx, cancel = context.WithCancel(context.Background())
		stream, err := client.Open(request(ctx, "/stream"))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.CopyN(io.Discard, stream.Body, 16<<10)
		cancel()
		_ = stream.Body.Close()

		// Abandoned in the middle of the body without cancelling.
		stream, err = client.Open(request(context.Background(), "/stream"))
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.CopyN(io.Discard, stream.Body, 16<<10)
		_ = stream.Body.Close()

		// Cancelled before it starts.
		ctx, cancel = context.WithCancel(context.Background())
		cancel()
		if _, err := client.Do(request(ctx, "/small")); err == nil {
			t.Fatal("a request cancelled before it started succeeded")
		}

		if _, err := client.Do(request(context.Background(), "/small")); err != nil {
			t.Fatal(err)
		}
	}, func() {
		client.CloseIdleConnections()
		server.Close()
	})
}
