// SPDX-License-Identifier: MPL-2.0

package anime1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"animeportable/adapters/network/securehttp"
	"animeportable/core"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	catalogURL            = "https://anime1.me/animelist.json"
	episodesOrigin        = "https://anime1.me"
	providerID            = "anime1"
	maxCatalogRows        = 10000
	maxTitleFragmentBytes = 64 << 10
	maxTitleTextBytes     = 4 << 10
	maxTitleTextRunes     = 1024
	maxEpisodePages       = 32
	maxEpisodeCount       = 2000
	maxEpisodeBodyBytes   = 1 << 20
	maxEpisodeDOMNodes    = 20000
	maxEpisodeTitleBytes  = 4 << 10
	maxEpisodeTitleRunes  = 1024
	maxEpisodeLabelBytes  = 128
	maxEpisodeLabelRunes  = 64
	maxEpisodeIDDigits    = 20
	maxCategoryIDDigits   = 10
	maxCategoryClassBytes = 128
)

var (
	errCatalogUnavailable  = errors.New("anime1: catalog unavailable")
	errCatalogMalformed    = errors.New("anime1: malformed catalog")
	errEpisodesUnavailable = errors.New("anime1: episodes unavailable")
	errEpisodesInvalidRef  = errors.New("anime1: invalid episode reference")
	errEpisodesMalformed   = errors.New("anime1: malformed episodes")
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

func (client *Client) Episodes(ctx context.Context, ref core.SourceRef) ([]core.SourceEpisode, error) {
	if ctx == nil {
		return nil, errEpisodesUnavailable
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	categoryID, ok := parseEpisodeCategoryID(ref)
	if !ok {
		return nil, errEpisodesInvalidRef
	}
	if client == nil || client.do == nil {
		return nil, errEpisodesUnavailable
	}
	initialURL, err := url.Parse(episodesOrigin + "/?cat=" + categoryID)
	if err != nil {
		return nil, errEpisodesUnavailable
	}
	currentURL := initialURL
	pageNumber := 1
	var pagination *episodePaginationState
	seenURLs := map[string]struct{}{canonicalEpisodeURL(initialURL): {}}
	seenIDs := make(map[string]struct{})
	newestFirst := make([]core.SourceEpisode, 0)
	archiveCategoryClass := ""

	for {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		body, err := client.fetchEpisodePage(ctx, currentURL)
		if err != nil {
			return nil, err
		}
		page, err := parseEpisodePage(ctx, body, ref, currentURL)
		if err != nil {
			return nil, err
		}
		if archiveCategoryClass == "" {
			archiveCategoryClass = page.categoryClass
		} else if page.categoryClass != archiveCategoryClass {
			return nil, errEpisodesMalformed
		}
		for _, postID := range page.postIDs {
			if _, exists := seenIDs[postID]; exists {
				return nil, errEpisodesMalformed
			}
			seenIDs[postID] = struct{}{}
		}
		for _, episode := range page.episodes {
			if len(newestFirst) >= maxEpisodeCount {
				return nil, errEpisodesMalformed
			}
			newestFirst = append(newestFirst, episode)
		}
		if !page.hasNext {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			result := make([]core.SourceEpisode, len(newestFirst))
			for index := range newestFirst {
				result[len(newestFirst)-1-index] = newestFirst[index]
			}
			return result, nil
		}
		if pageNumber >= maxEpisodePages {
			return nil, errEpisodesMalformed
		}
		nextURL, nextPagination, err := validateEpisodePagination(currentURL, page.nextHref, pageNumber, pagination)
		if err != nil {
			return nil, err
		}
		canonical := canonicalEpisodeURL(nextURL)
		if _, exists := seenURLs[canonical]; exists {
			return nil, errEpisodesMalformed
		}
		seenURLs[canonical] = struct{}{}
		currentURL = nextURL
		pagination = nextPagination
		pageNumber++
	}
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

func (client *Client) fetchEpisodePage(ctx context.Context, pageURL *url.URL) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return nil, errEpisodesUnavailable
	}
	response, err := client.do.Do(request)
	if err != nil {
		return nil, sanitizeEpisodeRequestError(err)
	}
	if response == nil {
		return nil, errEpisodesUnavailable
	}
	if err := response.RequireSuccess(); err != nil {
		return nil, err
	}
	if len(response.Body) > maxEpisodeBodyBytes {
		return nil, errEpisodesMalformed
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if !isHTMLResponse(response.Header.Get("Content-Type")) {
		return nil, errEpisodesMalformed
	}
	return response.Body, nil
}

func parseEpisodeCategoryID(ref core.SourceRef) (string, bool) {
	if ref.Provider != providerID {
		return "", false
	}
	return parseBoundedPositiveDecimal(ref.ID, maxCategoryIDDigits)
}

func parseBoundedPositiveDecimal(value string, maxDigits int) (string, bool) {
	if len(value) == 0 || len(value) > maxDigits || value[0] == '0' {
		return "", false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", false
		}
	}
	return value, true
}

func isHTMLResponse(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (strings.EqualFold(mediaType, "text/html") || strings.EqualFold(mediaType, "application/xhtml+xml"))
}

func sanitizeEpisodeRequestError(err error) error {
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
	return errEpisodesUnavailable
}

type episodePage struct {
	episodes      []core.SourceEpisode
	postIDs       []string
	categoryClass string
	hasNext       bool
	nextHref      string
}

type episodePaginationState struct {
	decodedPrefix string
	escapedPrefix string
	nextPage      int
}

type episodeVisit struct {
	node       *html.Node
	parent     *html.Node
	navigation *html.Node
}

type episodePageDocument struct {
	main            *html.Node
	navigationLinks []string
}

func parseEpisodePage(ctx context.Context, body []byte, ref core.SourceRef, pageURL *url.URL) (episodePage, error) {
	if err := checkContext(ctx); err != nil {
		return episodePage{}, err
	}
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil || document == nil {
		return episodePage{}, errEpisodesMalformed
	}
	parsed, err := inspectEpisodeDocument(ctx, document)
	if err != nil {
		return episodePage{}, err
	}
	if parsed.main == nil {
		return episodePage{}, errEpisodesMalformed
	}
	episodes := make([]core.SourceEpisode, 0)
	postIDs := make([]string, 0)
	categoryClass := ""
	for child := parsed.main.FirstChild; child != nil; child = child.NextSibling {
		if err := checkContext(ctx); err != nil {
			return episodePage{}, err
		}
		if child.Type != html.ElementNode || child.DataAtom != atom.Article {
			continue
		}
		articleCategoryClass, ok := episodeArticleCategoryClass(child, ref)
		if !ok || categoryClass != "" && articleCategoryClass != categoryClass {
			return episodePage{}, errEpisodesMalformed
		}
		categoryClass = articleCategoryClass
		if postID, ok := parsePostID(attributeValue(child, "id")); ok {
			postIDs = append(postIDs, postID)
		}
		episode, valid, err := parseEpisodeArticle(ctx, child, ref, pageURL)
		if err != nil {
			return episodePage{}, err
		}
		if valid {
			episodes = append(episodes, episode)
		}
	}
	if len(episodes) == 0 {
		return episodePage{}, errEpisodesMalformed
	}
	page := episodePage{episodes: episodes, postIDs: postIDs, categoryClass: categoryClass}
	if len(parsed.navigationLinks) > 0 {
		nextURL, err := mergeEpisodePaginationTargets(pageURL, parsed.navigationLinks)
		if err != nil {
			return episodePage{}, err
		}
		page.hasNext = true
		page.nextHref = nextURL.String()
	}
	return page, nil
}

func episodeArticleCategoryClass(article *html.Node, ref core.SourceRef) (string, bool) {
	categoryClass := ""
	for _, class := range strings.Fields(attributeValue(article, "class")) {
		if !strings.HasPrefix(class, "category-") {
			continue
		}
		if categoryClass != "" {
			return "", false
		}
		suffix := strings.TrimPrefix(class, "category-")
		if !validCategoryClassSuffix(suffix) {
			return "", false
		}
		if allASCIIDigits(suffix) {
			categoryID, ok := parseBoundedPositiveDecimal(suffix, maxCategoryIDDigits)
			if !ok || categoryID != ref.ID {
				return "", false
			}
		}
		categoryClass = class
	}
	return categoryClass, categoryClass != ""
}

func validCategoryClassSuffix(value string) bool {
	if value == "" || len(value) > maxCategoryClassBytes {
		return false
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= '0' && character <= '9':
		case character == '-' || character == '_':
		default:
			return false
		}
	}
	return true
}

func inspectEpisodeDocument(ctx context.Context, document *html.Node) (episodePageDocument, error) {
	stack := []episodeVisit{{node: document}}
	parsed := episodePageDocument{}
	nodes := 0
	for len(stack) > 0 {
		last := len(stack) - 1
		visit := stack[last]
		stack = stack[:last]
		if err := checkContext(ctx); err != nil {
			return episodePageDocument{}, err
		}
		if visit.node == nil {
			continue
		}
		nodes++
		if nodes > maxEpisodeDOMNodes {
			return episodePageDocument{}, errEpisodesMalformed
		}
		if visit.node.Type == html.ElementNode {
			if attributeValue(visit.node, "id") == "main" {
				if parsed.main != nil {
					return episodePageDocument{}, errEpisodesMalformed
				}
				parsed.main = visit.node
			}
			if isNavigationContainer(visit.node) {
				visit.navigation = visit.node
			}
			if visit.navigation != nil && visit.node.DataAtom == atom.A && visit.parent != nil && hasClass(visit.parent, "nav-previous") {
				parsed.navigationLinks = append(parsed.navigationLinks, attributeValue(visit.node, "href"))
			}
		}
		for child := visit.node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, episodeVisit{
				node:       child,
				parent:     visit.node,
				navigation: visit.navigation,
			})
		}
	}
	return parsed, nil
}

func parseEpisodeArticle(ctx context.Context, article *html.Node, ref core.SourceRef, pageURL *url.URL) (core.SourceEpisode, bool, error) {
	postID, ok, overLimit := parsePostIDResult(attributeValue(article, "id"))
	if overLimit {
		return core.SourceEpisode{}, false, errEpisodesMalformed
	}
	if !ok {
		return core.SourceEpisode{}, false, nil
	}
	anchors, err := findEntryTitleAnchors(ctx, article)
	if err != nil {
		return core.SourceEpisode{}, false, err
	}
	if len(anchors) != 1 {
		return core.SourceEpisode{}, false, nil
	}
	href := attributeValue(anchors[0], "href")
	if !validEpisodeLink(pageURL, href, postID) {
		return core.SourceEpisode{}, false, nil
	}
	title, err := normalizeEpisodeText(ctx, anchors[0])
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, errEpisodesMalformed) {
			return core.SourceEpisode{}, false, err
		}
		return core.SourceEpisode{}, false, nil
	}
	number, ok, overLimit := episodeNumberResult(title)
	if overLimit {
		return core.SourceEpisode{}, false, errEpisodesMalformed
	}
	if !ok {
		return core.SourceEpisode{}, false, nil
	}
	return core.SourceEpisode{
		Ref:    core.EpisodeRef{Anime: ref, ID: postID},
		Number: number,
		Title:  "",
	}, true, nil
}

func parsePostID(value string) (string, bool) {
	postID, ok, _ := parsePostIDResult(value)
	return postID, ok
}

func parsePostIDResult(value string) (string, bool, bool) {
	if !strings.HasPrefix(value, "post-") {
		return "", false, false
	}
	digits := strings.TrimPrefix(value, "post-")
	if len(digits) > maxEpisodeIDDigits && allASCIIDigits(digits) {
		return "", false, true
	}
	postID, ok := parseBoundedPositiveDecimal(digits, maxEpisodeIDDigits)
	return postID, ok, false
}

func findEntryTitleAnchors(ctx context.Context, article *html.Node) ([]*html.Node, error) {
	type titleVisit struct {
		node    *html.Node
		inTitle bool
	}
	stack := []titleVisit{{node: article}}
	anchors := make([]*html.Node, 0, 1)
	for len(stack) > 0 {
		last := len(stack) - 1
		visit := stack[last]
		stack = stack[:last]
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		if visit.node == nil {
			continue
		}
		if visit.node.Type == html.ElementNode {
			if excludedElement(visit.node.Data) {
				continue
			}
			if hasClass(visit.node, "entry-title") {
				visit.inTitle = true
			}
			if visit.inTitle && visit.node.DataAtom == atom.A {
				anchors = append(anchors, visit.node)
				if len(anchors) > 1 {
					return anchors, nil
				}
			}
		}
		for child := visit.node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, titleVisit{node: child, inTitle: visit.inTitle})
		}
	}
	return anchors, nil
}

func normalizeEpisodeText(ctx context.Context, root *html.Node) (string, error) {
	parts := make([]string, 0)
	stack := []*html.Node{root}
	for len(stack) > 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if err := checkContext(ctx); err != nil {
			return "", err
		}
		if node == nil {
			continue
		}
		if node.Type == html.ElementNode && excludedElement(node.Data) {
			continue
		}
		if node.Type == html.TextNode {
			parts = append(parts, node.Data)
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	normalized := strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	if len(normalized) > maxEpisodeTitleBytes || utf8.RuneCountInString(normalized) > maxEpisodeTitleRunes {
		return "", errEpisodesMalformed
	}
	return normalized, nil
}

func episodeNumberResult(title string) (string, bool, bool) {
	title = strings.TrimSpace(title)
	if title == "" || !strings.HasSuffix(title, "]") {
		return "", false, false
	}
	start := strings.LastIndexByte(title, '[')
	if start < 0 || start == len(title)-1 {
		return "", false, false
	}
	number := strings.TrimSpace(title[start+1 : len(title)-1])
	if number == "" || strings.ContainsAny(number, "[]") {
		return "", false, false
	}
	if len(number) > maxEpisodeLabelBytes || utf8.RuneCountInString(number) > maxEpisodeLabelRunes {
		return "", false, true
	}
	return number, true, false
}

func validEpisodeLink(pageURL *url.URL, href, postID string) bool {
	if pageURL == nil {
		return false
	}
	reference, err := url.Parse(href)
	if err != nil || reference == nil || reference.User != nil || reference.Fragment != "" || reference.RawQuery != "" || reference.ForceQuery || reference.Opaque != "" || !validReferencePath(reference.EscapedPath()) {
		return false
	}
	target := pageURL.ResolveReference(reference)
	if target == nil || target.Scheme != "https" || target.Host != "anime1.me" || target.Port() != "" || target.User != nil || target.Fragment != "" || target.RawQuery != "" || target.ForceQuery || target.Opaque != "" {
		return false
	}
	expectedPath := "/" + postID
	return target.Path == expectedPath && target.EscapedPath() == expectedPath
}

func validateEpisodePagination(currentURL *url.URL, href string, pageNumber int, state *episodePaginationState) (*url.URL, *episodePaginationState, error) {
	target, ok := normalizeEpisodePaginationTarget(currentURL, href)
	if !ok {
		return nil, nil, errEpisodesMalformed
	}
	decodedPrefix, escapedPrefix, targetPage, ok := parsePaginationPath(target)
	if !ok || targetPage != pageNumber+1 {
		return nil, nil, errEpisodesMalformed
	}
	if state == nil {
		return target, &episodePaginationState{decodedPrefix: decodedPrefix, escapedPrefix: escapedPrefix, nextPage: targetPage + 1}, nil
	}
	if decodedPrefix != state.decodedPrefix || escapedPrefix != state.escapedPrefix || targetPage != state.nextPage {
		return nil, nil, errEpisodesMalformed
	}
	state.nextPage++
	return target, state, nil
}

func mergeEpisodePaginationTargets(currentURL *url.URL, hrefs []string) (*url.URL, error) {
	var merged *url.URL
	for _, href := range hrefs {
		target, ok := normalizeEpisodePaginationTarget(currentURL, href)
		if !ok {
			return nil, errEpisodesMalformed
		}
		if merged == nil {
			merged = target
			continue
		}
		if canonicalEpisodeURL(merged) != canonicalEpisodeURL(target) {
			return nil, errEpisodesMalformed
		}
	}
	return merged, nil
}

func normalizeEpisodePaginationTarget(currentURL *url.URL, href string) (*url.URL, bool) {
	if currentURL == nil {
		return nil, false
	}
	reference, err := url.Parse(href)
	if err != nil || reference == nil || reference.User != nil || reference.Fragment != "" || reference.RawQuery != "" || reference.ForceQuery || reference.Opaque != "" || !validReferencePath(reference.EscapedPath()) {
		return nil, false
	}
	target := currentURL.ResolveReference(reference)
	if target == nil || target.Scheme != "https" || target.Host != "anime1.me" || target.Port() != "" || target.User != nil || target.Fragment != "" || target.RawQuery != "" || target.ForceQuery || target.Opaque != "" {
		return nil, false
	}
	if _, _, _, ok := parsePaginationPath(target); !ok {
		return nil, false
	}
	target.RawPath = ""
	target.RawFragment = ""
	return target, true
}

func parsePaginationPath(target *url.URL) (string, string, int, bool) {
	if target == nil || target.Path == "" || target.EscapedPath() == "" {
		return "", "", 0, false
	}
	decodedParts := strings.Split(target.Path, "/")
	escapedParts := strings.Split(target.EscapedPath(), "/")
	canonicalPath := (&url.URL{Path: target.Path}).EscapedPath()
	if !equalEscapedPathHexCase(target.EscapedPath(), canonicalPath) || len(decodedParts) != len(escapedParts) || len(decodedParts) < 5 || decodedParts[0] != "" || decodedParts[1] != "category" {
		return "", "", 0, false
	}
	end := len(decodedParts)
	if decodedParts[end-1] == "" {
		end--
	}
	if end < 5 || decodedParts[end-2] != "page" {
		return "", "", 0, false
	}
	for _, segment := range decodedParts[1:end] {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `/\\`) {
			return "", "", 0, false
		}
	}
	page, ok := parsePageNumber(decodedParts[end-1])
	if !ok || page > maxEpisodePages {
		return "", "", 0, false
	}
	canonicalParts := strings.Split(canonicalPath, "/")
	decodedPrefix := strings.Join(decodedParts[:end-1], "/") + "/"
	escapedPrefix := strings.Join(canonicalParts[:end-1], "/") + "/"
	return decodedPrefix, escapedPrefix, page, true
}

func equalEscapedPathHexCase(value, canonical string) bool {
	if len(value) != len(canonical) {
		return false
	}
	for index := 0; index < len(value); {
		if value[index] == '%' || canonical[index] == '%' {
			if value[index] != '%' || canonical[index] != '%' || index+2 >= len(value) || !isHexDigit(value[index+1]) || !isHexDigit(value[index+2]) || !isHexDigit(canonical[index+1]) || !isHexDigit(canonical[index+2]) || !strings.EqualFold(value[index+1:index+3], canonical[index+1:index+3]) {
				return false
			}
			index += 3
			continue
		}
		if value[index] != canonical[index] {
			return false
		}
		index++
	}
	return true
}

func isHexDigit(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func parsePageNumber(value string) (int, bool) {
	if value == "" || value[0] == '0' {
		return 0, false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}
	page, err := strconv.Atoi(value)
	return page, err == nil && page > 0
}

func validReferencePath(path string) bool {
	if path == "" {
		return true
	}
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if part == "" && index != 0 && index != len(parts)-1 {
			return false
		}
		decoded, err := url.PathUnescape(part)
		if err != nil || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, `/\\`) {
			return false
		}
	}
	return true
}

func allASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

func canonicalEpisodeURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.Scheme + "://" + value.Host + value.EscapedPath() + "?" + value.RawQuery
}

func isNavigationContainer(node *html.Node) bool {
	return node != nil && node.DataAtom == atom.Nav && hasClass(node, "navigation")
}

func hasClass(node *html.Node, class string) bool {
	value := attributeValue(node, "class")
	for _, token := range strings.Fields(value) {
		if token == class {
			return true
		}
	}
	return false
}

func attributeValue(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if strings.EqualFold(attribute.Key, name) {
			return attribute.Val
		}
	}
	return ""
}

var _ core.AnimeSource = (*Client)(nil)
