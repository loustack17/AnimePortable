package anime1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	resolverOrigin         = "https://v.anime1.me"
	resolverAPIURL         = resolverOrigin + "/api"
	resolverReferer        = episodesOrigin + "/"
	maxResolverBodyBytes   = 1 << 20
	maxResolverDOMNodes    = 20000
	maxResolverTokenBytes  = 16 << 10
	maxResolverSources     = 8
	maxResolverURLBytes    = 8 << 10
	maxResolverCookieBytes = 4 << 10
	maxResolverHeaderBytes = 13 << 10
)

var (
	errResolverUnavailable = errors.New("anime1: resolver unavailable")
	errResolverInvalidRef  = errors.New("anime1: invalid resolver reference")
	errResolverMalformed   = errors.New("anime1: malformed resolver response")
)

func AllowedOrigins() []string {
	return []string{episodesOrigin, resolverOrigin}
}

func (client *Client) Resolve(ctx context.Context, ref core.EpisodeRef) (core.PlaybackSource, error) {
	if ctx == nil {
		return core.PlaybackSource{}, errResolverUnavailable
	}
	if err := ctx.Err(); err != nil {
		return core.PlaybackSource{}, err
	}
	if !validResolverRef(ref) {
		return core.PlaybackSource{}, errResolverInvalidRef
	}
	if client == nil || client.do == nil {
		return core.PlaybackSource{}, errResolverUnavailable
	}
	pageURL := episodesOrigin + "/" + ref.ID
	page, err := client.fetchResolver(ctx, http.MethodGet, pageURL, "", "", "")
	if err != nil {
		return core.PlaybackSource{}, err
	}
	if !isHTMLResponse(page.Header.Get("Content-Type")) {
		return core.PlaybackSource{}, errResolverMalformed
	}
	token, err := parseResolverToken(ctx, page.Body, ref)
	if err != nil {
		return core.PlaybackSource{}, err
	}
	api, err := client.fetchResolver(ctx, http.MethodPost, resolverAPIURL, "d="+token, "application/x-www-form-urlencoded", resolverReferer)
	if err != nil {
		return core.PlaybackSource{}, err
	}
	if !isJSONResponse(api.Header.Get("Content-Type")) {
		return core.PlaybackSource{}, errResolverMalformed
	}
	streamURL, err := parseResolverAPI(ctx, api.Body)
	if err != nil {
		return core.PlaybackSource{}, err
	}
	cookieHeader, err := resolverCookieHeader(api.Header)
	if err != nil {
		return core.PlaybackSource{}, err
	}
	return core.NewPlaybackSource(streamURL, http.Header{"Cookie": {cookieHeader}}), nil
}

func validResolverRef(ref core.EpisodeRef) bool {
	_, animeOK := parseEpisodeCategoryID(ref.Anime)
	_, episodeOK := parseBoundedPositiveDecimal(ref.ID, maxEpisodeIDDigits)
	return animeOK && episodeOK
}

func (client *Client) fetchResolver(ctx context.Context, method, target, body, contentType, referer string) (*securehttp.Response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, errResolverUnavailable
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
	response, err := client.do.Do(request)
	if err != nil {
		return nil, sanitizeResolverError(err)
	}
	if response == nil {
		return nil, errResolverUnavailable
	}
	if err := response.RequireSuccess(); err != nil {
		return nil, err
	}
	if len(response.Body) > maxResolverBodyBytes {
		return nil, errResolverMalformed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return response, nil
}

func sanitizeResolverError(err error) error {
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
	return errResolverUnavailable
}

func parseResolverToken(ctx context.Context, body []byte, ref core.EpisodeRef) (string, error) {
	attributes := map[string][]string{"main": {"id"}, "article": {"id"}, "video": {"data-apireq"}}
	if err := rejectDuplicateRawAttributes(ctx, body, attributes, errResolverMalformed); err != nil {
		return "", err
	}
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil || document == nil {
		return "", errResolverMalformed
	}
	main, err := resolverMain(ctx, document)
	if err != nil {
		return "", err
	}
	article, err := resolverArticle(ctx, main, ref.ID)
	if err != nil {
		return "", errResolverMalformed
	}
	token, count, err := resolverVideoToken(ctx, article)
	if err != nil {
		return "", err
	}
	if count != 1 || !validResolverToken(token) {
		return "", errResolverMalformed
	}
	return token, nil
}

func resolverArticle(ctx context.Context, main *html.Node, postID string) (*html.Node, error) {
	stack := []*html.Node{main}
	var article *html.Node
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			continue
		}
		if node.Type == html.ElementNode && node.DataAtom == atom.Article {
			if article != nil || attributeValue(node, "id") != "post-"+postID {
				return nil, errResolverMalformed
			}
			article = node
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	if article == nil {
		return nil, errResolverMalformed
	}
	return article, nil
}

func resolverMain(ctx context.Context, document *html.Node) (*html.Node, error) {
	stack := []*html.Node{document}
	var main *html.Node
	nodes := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		nodes++
		if nodes > maxResolverDOMNodes {
			return nil, errResolverMalformed
		}
		if node.Type == html.ElementNode && attributeValue(node, "id") == "main" {
			if main != nil {
				return nil, errResolverMalformed
			}
			main = node
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	if main == nil {
		return nil, errResolverMalformed
	}
	return main, nil
}

func resolverVideoToken(ctx context.Context, article *html.Node) (string, int, error) {
	stack := []*html.Node{article}
	token := ""
	count := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			continue
		}
		if node.Type == html.ElementNode && node.DataAtom == atom.Video {
			values := attributeValues(node, "data-apireq")
			if len(values) > 1 {
				return "", 0, errResolverMalformed
			}
			if len(values) == 1 {
				count++
				token = values[0]
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return token, count, nil
}

func attributeValues(node *html.Node, name string) []string {
	values := make([]string, 0, 1)
	if node == nil {
		return values
	}
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			values = append(values, attribute.Val)
		}
	}
	return values
}

func validResolverToken(token string) bool {
	if len(token) == 0 || len(token) > maxResolverTokenBytes || !utf8.ValidString(token) {
		return false
	}
	for index := 0; index < len(token); {
		value := token[index]
		if isUnreserved(value) {
			index++
			continue
		}
		if value != '%' || index+2 >= len(token) || !isUpperHex(token[index+1]) || !isUpperHex(token[index+2]) {
			return false
		}
		index += 3
	}
	decoded, err := url.PathUnescape(token)
	if err != nil || len(decoded) > maxResolverTokenBytes || !utf8.ValidString(decoded) || !jsonObjectWithoutDuplicateKeys([]byte(decoded)) {
		return false
	}
	return true
}

func isUnreserved(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune("-._~", rune(value))
}

func isUpperHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'A' && value <= 'F'
}

type resolverAPIResponse struct {
	Success *bool            `json:"success"`
	Sources []resolverSource `json:"s"`
}

type resolverSource struct {
	Source string `json:"src"`
	Type   string `json:"type"`
}

func parseResolverAPI(ctx context.Context, body []byte) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(body) == 0 || len(body) > maxResolverBodyBytes || !jsonObjectWithoutDuplicateKeys(body) {
		return "", errResolverMalformed
	}
	var response resolverAPIResponse
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&response); err != nil {
		return "", errResolverMalformed
	}
	if response.Success != nil && !*response.Success || len(response.Sources) == 0 || len(response.Sources) > maxResolverSources {
		return "", errResolverMalformed
	}
	for _, source := range response.Sources {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		mediaType, _, err := mime.ParseMediaType(source.Type)
		if err != nil || !strings.EqualFold(mediaType, "video/mp4") {
			continue
		}
		if normalized, ok := normalizeResolverURL(source.Source); ok {
			return normalized, nil
		}
	}
	return "", errResolverMalformed
}

func jsonObjectWithoutDuplicateKeys(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, true); err != nil {
		return false
	}
	_, err := decoder.Token()
	return errors.Is(err, io.EOF)
}

func scanJSONValue(decoder *json.Decoder, requireObject bool) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if requireObject && (!isDelimiter || delimiter != '{') {
		return errResolverMalformed
	}
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errResolverMalformed
			}
			if _, exists := seen[key]; exists {
				return errResolverMalformed
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, false); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errResolverMalformed
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, false); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errResolverMalformed
		}
	default:
		return errResolverMalformed
	}
	return nil
}

func normalizeResolverURL(raw string) (string, bool) {
	if len(raw) == 0 || containsASCIIControl(raw) {
		return "", false
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	} else if !strings.HasPrefix(raw, "https://") {
		return "", false
	}
	if len(raw) > maxResolverURLBytes {
		return "", false
	}
	target, err := url.Parse(raw)
	if err != nil || target == nil || target.Scheme != "https" || target.Opaque != "" || target.User != nil || target.Port() != "" || target.Fragment != "" || target.ForceQuery || target.Path == "" {
		return "", false
	}
	host := target.Hostname()
	if target.Host != host || !validResolverHost(host) || target.EscapedPath() != (&url.URL{Path: target.Path}).EscapedPath() || !strings.HasSuffix(strings.ToLower(target.Path), ".mp4") {
		return "", false
	}
	return target.String(), true
}

func validResolverHost(host string) bool {
	if host == "" || len(host) > 253 || host != strings.ToLower(host) || !isASCII(host) || strings.HasSuffix(host, ".") || host == "v.anime1.me" || !strings.HasSuffix(host, ".v.anime1.me") {
		return false
	}
	prefix := strings.TrimSuffix(host, ".v.anime1.me")
	if prefix == "" {
		return false
	}
	for _, label := range strings.Split(prefix, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, value := range label {
			if value < 'a' || value > 'z' {
				if value < '0' || value > '9' {
					if value != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func isASCII(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] > 0x7f {
			return false
		}
	}
	return true
}

func containsASCIIControl(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return true
		}
	}
	return false
}

func resolverCookieHeader(header http.Header) (string, error) {
	allowed := map[string]string{"e": "", "h": "", "p": ""}
	seen := make(map[string]struct{}, len(allowed))
	for _, raw := range header.Values("Set-Cookie") {
		first := strings.TrimSpace(strings.SplitN(raw, ";", 2)[0])
		pair := strings.SplitN(first, "=", 2)
		if len(pair) != 2 {
			if _, ok := allowed[first]; ok {
				return "", errResolverMalformed
			}
			continue
		}
		name := strings.TrimSpace(pair[0])
		if _, ok := allowed[name]; !ok {
			continue
		}
		if _, exists := seen[name]; exists || strings.HasPrefix(strings.TrimSpace(pair[1]), "\"") {
			return "", errResolverMalformed
		}
		cookie, err := http.ParseSetCookie(raw)
		if err != nil || cookie.Name != name || cookie.Value == "" || len(cookie.Value) > maxResolverCookieBytes || cookie.MaxAge < 0 || len(cookie.Unparsed) != 0 || !validCookieValue(cookie.Value) {
			return "", errResolverMalformed
		}
		if !cookie.Expires.IsZero() && cookie.Expires.Before(time.Now()) {
			return "", errResolverMalformed
		}
		seen[name] = struct{}{}
		allowed[name] = cookie.Value
	}
	if len(seen) != len(allowed) {
		return "", errResolverMalformed
	}
	value := fmt.Sprintf("e=%s; h=%s; p=%s", allowed["e"], allowed["h"], allowed["p"])
	if len(value) > maxResolverHeaderBytes {
		return "", errResolverMalformed
	}
	return value, nil
}

func validCookieValue(value string) bool {
	for index := 0; index < len(value); index++ {
		character := value[index]
		if character != 0x21 && (character < 0x23 || character > 0x2b) && (character < 0x2d || character > 0x3a) && (character < 0x3c || character > 0x5b) && (character < 0x5d || character > 0x7e) {
			return false
		}
	}
	return true
}
