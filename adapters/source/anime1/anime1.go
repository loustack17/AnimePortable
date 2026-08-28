package anime1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	catalogURL            = "https://anime1.me/animelist.json"
	providerID            = "anime1"
	maxCatalogRows        = 10000
	maxTitleFragmentBytes = 64 << 10
	maxTitleTextBytes     = 4 << 10
	maxTitleTextRunes     = 1024
)

var (
	errCatalogUnavailable = errors.New("anime1: catalog unavailable")
	errCatalogMalformed   = errors.New("anime1: malformed catalog")
)

type responseClient interface {
	Do(*http.Request) (*securehttp.Response, error)
}

type Client struct {
	do responseClient
}

type Source = Client

func New(client *securehttp.Client) *Client {
	return newWithDo(client)
}

func newWithDo(do responseClient) *Client {
	return &Client{do: do}
}

func (client *Client) Catalog(ctx context.Context) ([]core.SourceAnime, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if client == nil || client.do == nil {
		return nil, errCatalogUnavailable
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return nil, errCatalogUnavailable
	}
	response, err := client.do.Do(request)
	if err != nil {
		return nil, sanitizeRequestError(err)
	}
	if response == nil {
		return nil, errCatalogUnavailable
	}
	if err := response.RequireSuccess(); err != nil {
		return nil, err
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if !isJSONResponse(response.Header.Get("Content-Type")) {
		return nil, errCatalogMalformed
	}
	return parseCatalog(ctx, response.Body)
}

func (client *Client) Search(ctx context.Context, query string) ([]core.SourceAnime, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return []core.SourceAnime{}, nil
	}
	items, err := client.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	foldedQuery := strings.ToLower(query)
	results := make([]core.SourceAnime, 0)
	for _, item := range items {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(item.Title), foldedQuery) {
			results = append(results, item)
		}
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (*Client) Episodes(context.Context, core.SourceRef) ([]core.SourceEpisode, error) {
	return nil, core.ErrUnsupported
}

func (*Client) Resolve(context.Context, core.EpisodeRef) (core.PlaybackSource, error) {
	return core.PlaybackSource{}, core.ErrUnsupported
}

func (*Client) Schedule(context.Context, core.ScheduleQuery) ([]core.SourceScheduleItem, error) {
	return nil, core.ErrUnsupported
}

func parseCatalog(ctx context.Context, body []byte) ([]core.SourceAnime, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, errCatalogMalformed
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(trimmed, &rows); err != nil || len(rows) > maxCatalogRows {
		return nil, errCatalogMalformed
	}
	items := make([]core.SourceAnime, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		item, ok := parseRow(ctx, row)
		if !ok {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if _, exists := seen[item.Ref.ID]; exists {
			continue
		}
		seen[item.Ref.ID] = struct{}{}
		items = append(items, item)
	}
	if len(items) == 0 {
		return nil, errCatalogMalformed
	}
	return items, nil
}

func parseRow(ctx context.Context, row json.RawMessage) (core.SourceAnime, bool) {
	var fields []json.RawMessage
	if err := json.Unmarshal(row, &fields); err != nil || len(fields) < 2 {
		return core.SourceAnime{}, false
	}
	id, ok := parseID(fields[0])
	if !ok {
		return core.SourceAnime{}, false
	}
	var fragment string
	if json.Unmarshal(fields[1], &fragment) != nil || len(fields[1]) > maxTitleFragmentBytes || len(fragment) > maxTitleFragmentBytes {
		return core.SourceAnime{}, false
	}
	title, err := normalizeTitle(ctx, fragment)
	if err != nil || title == "" {
		return core.SourceAnime{}, false
	}
	return core.SourceAnime{
		Ref:   core.SourceRef{Provider: providerID, ID: id},
		Title: title,
	}, true
}

func parseID(raw json.RawMessage) (string, bool) {
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "", false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", false
		}
	}
	trimmed := strings.TrimLeft(value, "0")
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func normalizeTitle(ctx context.Context, fragment string) (string, error) {
	root := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), root)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0)
	var walk func(*html.Node) error
	walk = func(node *html.Node) error {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			return nil
		}
		if node.Type == html.TextNode {
			parts = append(parts, node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, node := range nodes {
		if err := walk(node); err != nil {
			return "", err
		}
	}
	normalized := strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	if len(normalized) > maxTitleTextBytes || utf8.RuneCountInString(normalized) > maxTitleTextRunes {
		return "", errCatalogMalformed
	}
	return normalized, nil
}

func excludedElement(name string) bool {
	switch strings.ToLower(name) {
	case "script", "style", "template", "iframe", "textarea", "title", "xmp", "noembed", "noframes", "noscript", "plaintext":
		return true
	default:
		return false
	}
}

func isJSONResponse(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return errCatalogUnavailable
	}
	return ctx.Err()
}

func sanitizeRequestError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	var secureError *securehttp.Error
	if errors.As(err, &secureError) {
		return secureError
	}
	return errCatalogUnavailable
}

var _ core.AnimeSource = (*Client)(nil)
