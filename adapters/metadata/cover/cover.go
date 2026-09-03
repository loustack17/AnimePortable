// SPDX-License-Identifier: MPL-2.0

package cover

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"animeportable/adapters/network/securehttp"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	anilistOrigin      = "https://s4.anilist.co"
	bangumiOrigin      = "https://lain.bgm.tv"
	maxResponseBytes   = 4 << 20
	maxDimension       = 4096
	maxPixels          = 4_194_304
	maxConcurrentLoads = 4
)

var (
	ErrInvalidURL      = errors.New("cover: invalid URL")
	ErrUnavailable     = errors.New("cover: unavailable")
	ErrInvalidResponse = errors.New("cover: invalid response")
	ErrInvalidImage    = errors.New("cover: invalid image")
)

type Result struct {
	Bytes     []byte
	MediaType string
	Width     int
	Height    int
}

type responseClient interface {
	Do(*http.Request) (*securehttp.Response, error)
	CloseIdleConnections()
}

type Loader struct {
	client  responseClient
	permits chan struct{}
}

func New() (*Loader, error) {
	client, err := securehttp.New(securehttp.Config{
		AllowedOrigins:   []string{anilistOrigin, bangumiOrigin},
		MaxResponseBytes: maxResponseBytes,
	})
	if err != nil {
		return nil, ErrUnavailable
	}
	return newWithClient(client), nil
}

func newWithClient(client responseClient) *Loader {
	return &Loader{client: client, permits: make(chan struct{}, maxConcurrentLoads)}
}

func (loader *Loader) Load(ctx context.Context, rawURL string) (Result, error) {
	if ctx == nil {
		return Result{}, ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return Result{}, contextFailure(err)
	}
	requestURL, err := validatedURL(rawURL)
	if err != nil {
		return Result{}, ErrInvalidURL
	}
	if loader == nil || loader.client == nil || loader.permits == nil {
		return Result{}, ErrUnavailable
	}
	select {
	case loader.permits <- struct{}{}:
		defer func() { <-loader.permits }()
	case <-ctx.Done():
		return Result{}, contextFailure(ctx.Err())
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return Result{}, ErrInvalidURL
	}
	request.Header.Set("Accept", "image/jpeg, image/png")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := loader.client.Do(request)
	if err != nil {
		return Result{}, requestFailure(err)
	}
	return validateResponse(response)
}

func (loader *Loader) CloseIdleConnections() {
	if loader == nil || loader.client == nil {
		return
	}
	loader.client.CloseIdleConnections()
}

func validatedURL(rawURL string) (*url.URL, error) {
	if rawURL == "" || !metadatapolicy.IsSafeCoverURL(rawURL) {
		return nil, ErrInvalidURL
	}
	value, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(value.Hostname()), ".")
	if host != "s4.anilist.co" && host != "lain.bgm.tv" {
		return nil, ErrInvalidURL
	}
	return value, nil
}

func validateResponse(response *securehttp.Response) (Result, error) {
	if response == nil || response.StatusCode != http.StatusOK {
		return Result{}, ErrInvalidResponse
	}
	if !identityEncoding(response.Header) {
		return Result{}, ErrInvalidResponse
	}
	values := headerValues(response.Header, "Content-Type")
	if len(values) != 1 {
		return Result{}, ErrInvalidResponse
	}
	mediaType, _, err := mime.ParseMediaType(values[0])
	if err != nil || mediaType != "image/jpeg" && mediaType != "image/png" {
		return Result{}, ErrInvalidResponse
	}
	if len(response.Body) == 0 || len(response.Body) > maxResponseBytes {
		return Result{}, ErrInvalidImage
	}
	if http.DetectContentType(response.Body) != mediaType {
		return Result{}, ErrInvalidImage
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(response.Body))
	if err != nil || format != formatForMediaType(mediaType) || !validDimensions(config.Width, config.Height) {
		return Result{}, ErrInvalidImage
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(response.Body))
	if err != nil || decodedFormat != format || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return Result{}, ErrInvalidImage
	}
	return Result{
		Bytes:     append([]byte(nil), response.Body...),
		MediaType: mediaType,
		Width:     config.Width,
		Height:    config.Height,
	}, nil
}

func identityEncoding(header http.Header) bool {
	values := headerValues(header, "Content-Encoding")
	return len(values) == 0 || len(values) == 1 && strings.EqualFold(strings.TrimSpace(values[0]), "identity")
}

func headerValues(header http.Header, name string) []string {
	var result []string
	for key, values := range header {
		if strings.EqualFold(key, name) {
			result = append(result, values...)
		}
	}
	return result
}

func formatForMediaType(mediaType string) string {
	if mediaType == "image/jpeg" {
		return "jpeg"
	}
	return "png"
}

func validDimensions(width, height int) bool {
	return width <= maxDimension && height <= maxDimension && validPixelCount(width, height)
}

func validPixelCount(width, height int) bool {
	return width > 0 && height > 0 && width <= maxPixels/height
}

func requestFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return contextFailure(err)
	}
	return ErrUnavailable
}

func contextFailure(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	return context.DeadlineExceeded
}
