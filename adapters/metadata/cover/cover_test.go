// SPDX-License-Identifier: MPL-2.0

package cover

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/network/securehttp"
)

func TestLoadAcceptsJPEGAndPNGWithExactRequest(t *testing.T) {
	for _, test := range []struct {
		name      string
		mediaType string
		body      []byte
		width     int
		height    int
	}{
		{"jpeg", "image/jpeg", encodedJPEG(t, 3, 2), 3, 2},
		{"png", "image/png", encodedPNG(t, 2, 3), 2, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeClient{response: &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {test.mediaType}}, Body: test.body}}
			loader := newWithClient(client)
			result, err := loader.Load(context.Background(), "https://s4.anilist.co/file?token=secret")
			if err != nil {
				t.Fatal(err)
			}
			if result.MediaType != test.mediaType || result.Width != test.width || result.Height != test.height || !bytes.Equal(result.Bytes, test.body) {
				t.Fatal(result.MediaType, result.Width, result.Height, len(result.Bytes))
			}
			if client.request == nil || client.request.Method != http.MethodGet || client.request.Header.Get("Accept") != "image/jpeg, image/png" || client.request.Header.Get("Accept-Encoding") != "identity" || client.request.Body != nil {
				t.Fatal(client.request)
			}
			test.body[0] ^= 0xff
			if result.Bytes[0] == test.body[0] {
				t.Fatal("result aliases response bytes")
			}
		})
	}
}

func TestLoadAcceptsExactBodyLimitAndRejectsOtherSizes(t *testing.T) {
	body := encodedJPEG(t, 1, 1)
	exact := append(append([]byte(nil), body...), make([]byte, maxResponseBytes-len(body))...)
	for _, test := range []struct {
		name string
		body []byte
		want error
	}{
		{"exact", exact, nil},
		{"over", append(append([]byte(nil), exact...), 0), ErrInvalidImage},
		{"empty", nil, ErrInvalidImage},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := newWithClient(&fakeClient{response: imageResponse("image/jpeg", test.body)})
			result, err := loader.Load(context.Background(), "https://lain.bgm.tv/pic/cover.jpg")
			if !errors.Is(err, test.want) || test.want != nil && !zeroResult(result) {
				t.Fatal(len(result.Bytes), err)
			}
		})
	}
}

func TestLoadRejectsInvalidResponseMetadata(t *testing.T) {
	pngBody := encodedPNG(t, 2, 2)
	jpegBody := encodedJPEG(t, 2, 2)
	tests := []struct {
		name   string
		status int
		header http.Header
		body   []byte
		want   error
	}{
		{"redirect status", http.StatusFound, http.Header{"Content-Type": {"image/png"}}, pngBody, ErrInvalidResponse},
		{"other success status", http.StatusCreated, http.Header{"Content-Type": {"image/png"}}, pngBody, ErrInvalidResponse},
		{"empty success status", http.StatusNoContent, http.Header{"Content-Type": {"image/png"}}, pngBody, ErrInvalidResponse},
		{"error status", http.StatusNotFound, http.Header{"Content-Type": {"image/png"}}, pngBody, ErrInvalidResponse},
		{"missing content type", http.StatusOK, nil, pngBody, ErrInvalidResponse},
		{"duplicate content type", http.StatusOK, http.Header{"Content-Type": {"image/png", "image/png"}}, pngBody, ErrInvalidResponse},
		{"combined content type", http.StatusOK, http.Header{"Content-Type": {"image/png, image/jpeg"}}, pngBody, ErrInvalidResponse},
		{"malformed content type", http.StatusOK, http.Header{"Content-Type": {"image/png; bad"}}, pngBody, ErrInvalidResponse},
		{"unsupported content type", http.StatusOK, http.Header{"Content-Type": {"image/gif"}}, pngBody, ErrInvalidResponse},
		{"mime mismatch png", http.StatusOK, http.Header{"Content-Type": {"image/jpeg"}}, pngBody, ErrInvalidImage},
		{"mime mismatch jpeg", http.StatusOK, http.Header{"Content-Type": {"image/png"}}, jpegBody, ErrInvalidImage},
		{"gzip", http.StatusOK, http.Header{"Content-Type": {"image/png"}, "Content-Encoding": {"gzip"}}, pngBody, ErrInvalidResponse},
		{"multiple encodings", http.StatusOK, http.Header{"Content-Type": {"image/png"}, "Content-Encoding": {"identity", "identity"}}, pngBody, ErrInvalidResponse},
		{"combined encodings", http.StatusOK, http.Header{"Content-Type": {"image/png"}, "Content-Encoding": {"identity, gzip"}}, pngBody, ErrInvalidResponse},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loader := newWithClient(&fakeClient{response: &securehttp.Response{StatusCode: test.status, Header: test.header, Body: test.body}})
			result, err := loader.Load(context.Background(), "https://s4.anilist.co/image")
			if !errors.Is(err, test.want) || !zeroResult(result) {
				t.Fatal(result, err)
			}
		})
	}
	for _, encoding := range []string{"", "identity", " IDENTITY "} {
		header := http.Header{"Content-Type": {"image/png"}}
		if encoding != "" {
			header["Content-Encoding"] = []string{encoding}
		}
		loader := newWithClient(&fakeClient{response: &securehttp.Response{StatusCode: http.StatusOK, Header: header, Body: pngBody}})
		if _, err := loader.Load(context.Background(), "https://s4.anilist.co/image"); err != nil {
			t.Fatal(encoding, err)
		}
	}
}

func TestLoadRejectsMalformedAndUnsupportedImages(t *testing.T) {
	gifBody := encodedGIF(t, 2, 2)
	truncatedJPEG := truncateAfterJPEGConfig(t, encodedJPEG(t, 8, 8))
	for _, test := range []struct {
		name      string
		mediaType string
		body      []byte
	}{
		{"truncated jpeg", "image/jpeg", []byte{0xff, 0xd8, 0xff}},
		{"config-only jpeg", "image/jpeg", truncatedJPEG},
		{"truncated png", "image/png", []byte("\x89PNG\r\n\x1a\n")},
		{"config-only png", "image/png", pngConfig(2, 2)},
		{"unsupported gif", "image/gif", gifBody},
		{"gif declared jpeg", "image/jpeg", gifBody},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := newWithClient(&fakeClient{response: imageResponse(test.mediaType, test.body)})
			result, err := loader.Load(context.Background(), "https://lain.bgm.tv/image")
			if err == nil || !zeroResult(result) {
				t.Fatal(result, err)
			}
		})
	}
}

func TestLoadChecksDimensionAndPixelLimits(t *testing.T) {
	for _, test := range []struct {
		name   string
		width  int
		height int
		want   bool
	}{
		{"exact dimension and pixels", 4096, 1024, true},
		{"dimension over", 4097, 1, false},
		{"zero width", 0, 1, false},
		{"zero height", 1, 0, false},
		{"pixels over", 2049, 2048, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validDimensions(test.width, test.height); got != test.want {
				t.Fatalf("validDimensions(%d, %d) = %v, want %v", test.width, test.height, got, test.want)
			}
		})
	}
}

func TestPixelLimitExactPlusOneAndOverflow(t *testing.T) {
	if !validPixelCount(maxPixels, 1) {
		t.Fatal("exact pixel limit rejected")
	}
	if validPixelCount(maxPixels+1, 1) {
		t.Fatal("pixel limit plus one accepted")
	}
	maxInt := int(^uint(0) >> 1)
	if validPixelCount(maxInt, maxInt) {
		t.Fatal("overflowing product accepted")
	}
}

func TestLoadPrevalidatesURLBeforeClient(t *testing.T) {
	client := &fakeClient{response: imageResponse("image/png", encodedPNG(t, 1, 1))}
	loader := newWithClient(client)
	invalid := []string{
		"",
		"not a url",
		"http://s4.anilist.co/image",
		"ftp://s4.anilist.co/image",
		"https://other.example/image",
		"https://s4.anilist.co.evil.example/image",
		"https://user:secret@s4.anilist.co/image",
		"https://s4.anilist.co/image#secret",
		"https://s4.anilist.co:443/image",
		"https://s4.anilist.co:444/image",
		"https://s4.anilist.co/image\nsecret",
		"https://s4.anilist.co/" + strings.Repeat("x", 8<<10),
	}
	for _, rawURL := range invalid {
		result, err := loader.Load(context.Background(), rawURL)
		if !errors.Is(err, ErrInvalidURL) || !zeroResult(result) {
			t.Fatal(rawURL, result, err)
		}
	}
	if client.calls != 0 {
		t.Fatal(client.calls)
	}
	for _, rawURL := range []string{"https://s4.anilist.co/image", "https://lain.bgm.tv/image", "https://S4.ANILIST.CO./image"} {
		if _, err := loader.Load(context.Background(), rawURL); err != nil {
			t.Fatal(rawURL, err)
		}
	}
}

func TestLoadPreservesContextErrorsAndSanitizesFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{"canceled", context.Canceled, context.Canceled},
		{"deadline", context.DeadlineExceeded, context.DeadlineExceeded},
		{"other", errors.New("token=raw-secret https://s4.anilist.co/private"), ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := newWithClient(&fakeClient{err: test.err})
			result, err := loader.Load(context.Background(), "https://s4.anilist.co/private?token=raw-secret")
			if !errors.Is(err, test.want) || !zeroResult(result) || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "private") {
				t.Fatal(result, err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &fakeClient{response: imageResponse("image/png", encodedPNG(t, 1, 1))}
	result, err := newWithClient(client).Load(ctx, "https://s4.anilist.co/image")
	if !errors.Is(err, context.Canceled) || !zeroResult(result) || client.calls != 0 {
		t.Fatal(result, err, client.calls)
	}
}

func TestLoadLimitsConcurrentWorkAndCancelsPermitWait(t *testing.T) {
	body := encodedPNG(t, 1, 1)
	client := &blockingClient{started: make(chan struct{}, 8), release: make(chan struct{}), response: imageResponse("image/png", body)}
	loader := newWithClient(client)
	results := make(chan error, 5)
	for range maxConcurrentLoads {
		go func() {
			_, err := loader.Load(context.Background(), "https://s4.anilist.co/image")
			results <- err
		}()
	}
	for range maxConcurrentLoads {
		select {
		case <-client.started:
		case <-time.After(time.Second):
			t.Fatal("request did not start")
		}
	}
	waitCtx, cancel := context.WithCancel(context.Background())
	fifth := make(chan error, 1)
	go func() {
		_, err := loader.Load(waitCtx, "https://lain.bgm.tv/image")
		fifth <- err
	}()
	select {
	case <-client.started:
		t.Fatal("fifth request bypassed permit limit")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-fifth:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("permit wait ignored cancellation")
	}
	close(client.release)
	for range maxConcurrentLoads {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := loader.Load(context.Background(), "https://s4.anilist.co/after"); err != nil {
		t.Fatal("permit was not released", err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.maxActive != maxConcurrentLoads {
		t.Fatal(client.maxActive)
	}
}

func TestCloseIdleConnectionsAndNewLifecycle(t *testing.T) {
	client := &fakeClient{}
	loader := newWithClient(client)
	loader.CloseIdleConnections()
	if client.closes != 1 {
		t.Fatal(client.closes)
	}
	created, err := New()
	if err != nil || created == nil {
		t.Fatal(created, err)
	}
	created.CloseIdleConnections()
}

type fakeClient struct {
	response *securehttp.Response
	err      error
	request  *http.Request
	calls    int
	closes   int
}

func (client *fakeClient) Do(request *http.Request) (*securehttp.Response, error) {
	client.calls++
	client.request = request
	return client.response, client.err
}

func (client *fakeClient) CloseIdleConnections() { client.closes++ }

type blockingClient struct {
	mu        sync.Mutex
	active    int
	maxActive int
	started   chan struct{}
	release   chan struct{}
	response  *securehttp.Response
}

func (client *blockingClient) Do(request *http.Request) (*securehttp.Response, error) {
	client.mu.Lock()
	client.active++
	if client.active > client.maxActive {
		client.maxActive = client.active
	}
	client.mu.Unlock()
	client.started <- struct{}{}
	select {
	case <-client.release:
	case <-request.Context().Done():
		client.mu.Lock()
		client.active--
		client.mu.Unlock()
		return nil, request.Context().Err()
	}
	client.mu.Lock()
	client.active--
	client.mu.Unlock()
	return client.response, nil
}

func (*blockingClient) CloseIdleConnections() {}

func imageResponse(mediaType string, body []byte) *securehttp.Response {
	return &securehttp.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {mediaType}}, Body: body}
}

func zeroResult(result Result) bool {
	return result.Bytes == nil && result.MediaType == "" && result.Width == 0 && result.Height == 0
}

func encodedJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height)), nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func encodedPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := png.Encode(&output, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func encodedGIF(t *testing.T, width, height int) []byte {
	t.Helper()
	var output bytes.Buffer
	value := image.NewPaletted(image.Rect(0, 0, width, height), color.Palette{color.Black})
	if err := gif.Encode(&output, value, nil); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func truncateAfterJPEGConfig(t *testing.T, body []byte) []byte {
	t.Helper()
	for length := 1; length < len(body); length++ {
		candidate := body[:length]
		if _, _, err := image.DecodeConfig(bytes.NewReader(candidate)); err != nil {
			continue
		}
		if _, _, err := image.Decode(bytes.NewReader(candidate)); err != nil {
			return candidate
		}
	}
	t.Fatal("JPEG fixture has no config-valid truncated prefix")
	return nil
}

func pngConfig(width, height uint32) []byte {
	result := append([]byte(nil), []byte("\x89PNG\r\n\x1a\n")...)
	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8] = 8
	data[9] = 2
	return appendPNGChunk(result, "IHDR", data)
}

func appendPNGChunk(target []byte, name string, data []byte) []byte {
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, uint32(len(data)))
	target = append(target, length...)
	chunk := append([]byte(name), data...)
	target = append(target, chunk...)
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.ChecksumIEEE(chunk))
	return append(target, checksum...)
}
