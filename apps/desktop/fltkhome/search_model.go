// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"sort"
	"strings"

	"animeportable/apps/desktop/backend"
)

const searchResultPageSize = 8

type searchRequestKind uint8

const (
	searchRequestLibrary searchRequestKind = iota + 1
	searchRequestRemote
	searchRequestDetail
)

type searchRequest struct {
	generation uint64
	kind       searchRequestKind
}

type searchPhase uint8

const (
	searchPhaseIdle searchPhase = iota
	searchPhaseLoading
	searchPhaseFailed
)

type searchModel struct {
	items         []backend.Anime
	results       []backend.Anime
	query         string
	selected      int
	phase         searchPhase
	loaded        bool
	libraryFailed bool
	preview       *backend.Detail
	previewItem   backend.Anime
	remoteQuery   string
	closed        bool
	generation    uint64
	active        searchRequest
	requestActive bool
	cancelRequest context.CancelFunc
	pending       searchRequestKind
	pendingItem   backend.Anime
}

func (model *searchModel) setItems(items []backend.Anime) {
	model.merge(items)
	model.loaded = true
	model.filter()
}

func (model *searchModel) setQuery(query string) {
	if model.query == query || model.closed {
		return
	}
	model.invalidateRequest()
	model.pending = 0
	model.pendingItem = backend.Anime{}
	model.query = query
	model.remoteQuery = ""
	model.selected = 0
	model.preview = nil
	model.previewItem = backend.Anime{}
	model.phase = searchPhaseIdle
	model.filter()
}

func (model *searchModel) beginLibrary() (searchRequest, context.Context, bool) {
	if model.loaded {
		return searchRequest{}, nil, false
	}
	return model.begin(searchRequestLibrary)
}

func (model *searchModel) beginSearch() (searchRequest, context.Context, bool) {
	if strings.TrimSpace(model.query) == "" {
		return searchRequest{}, nil, false
	}
	return model.begin(searchRequestRemote)
}

func (model *searchModel) beginDetail(item backend.Anime) (searchRequest, context.Context, bool) {
	if item.ID == "" {
		return searchRequest{}, nil, false
	}
	request, ctx, ok := model.begin(searchRequestDetail)
	if !ok {
		return searchRequest{}, nil, false
	}
	model.previewItem = item
	model.preview = nil
	return request, ctx, true
}

func (model *searchModel) queueSearch() {
	if !model.closed && model.requestActive && (model.active.kind == searchRequestLibrary || model.active.generation != model.generation) && strings.TrimSpace(model.query) != "" {
		model.pending = searchRequestRemote
		model.pendingItem = backend.Anime{}
	}
}

func (model *searchModel) queueDetail(item backend.Anime) {
	if model.closed || !model.requestActive || item.ID == "" {
		return
	}
	if model.active.generation == model.generation {
		model.invalidateRequest()
	}
	if model.requestActive {
		model.pending = searchRequestDetail
		model.pendingItem = item
		model.previewItem = item
		model.preview = nil
	}
}

func (model *searchModel) takePending() (searchRequestKind, backend.Anime, bool) {
	if model.closed || model.requestActive || model.pending == 0 {
		return 0, backend.Anime{}, false
	}
	kind, item := model.pending, model.pendingItem
	model.pending = 0
	model.pendingItem = backend.Anime{}
	if kind == searchRequestRemote && strings.TrimSpace(model.query) == "" {
		return 0, backend.Anime{}, false
	}
	return kind, item, true
}

func (model *searchModel) begin(kind searchRequestKind) (searchRequest, context.Context, bool) {
	if model.closed || model.requestActive {
		return searchRequest{}, nil, false
	}
	model.generation++
	request := searchRequest{generation: model.generation, kind: kind}
	ctx, cancel := context.WithCancel(context.Background())
	model.active = request
	model.requestActive = true
	model.cancelRequest = cancel
	model.phase = searchPhaseLoading
	return request, ctx, true
}

func (model *searchModel) finishLibrary(request searchRequest, items []backend.Anime, err error) bool {
	if !model.finishRequest(request) {
		return false
	}
	if !model.current(request) {
		return false
	}
	if err != nil {
		model.phase = searchPhaseFailed
		model.libraryFailed = true
		return true
	}
	model.setItems(items)
	model.libraryFailed = false
	model.phase = searchPhaseIdle
	return true
}

func (model *searchModel) finishSearch(request searchRequest, items []backend.Anime, err error) bool {
	if !model.finishRequest(request) {
		return false
	}
	if !model.current(request) {
		return false
	}
	if err != nil {
		model.phase = searchPhaseFailed
		model.remoteQuery = ""
		return true
	}
	model.merge(items)
	model.filter()
	model.remoteQuery = model.query
	model.phase = searchPhaseIdle
	return true
}

func (model *searchModel) finishDetail(request searchRequest, detail backend.Detail, err error) bool {
	if !model.finishRequest(request) {
		return false
	}
	if !model.current(request) {
		return false
	}
	if err != nil {
		model.phase = searchPhaseFailed
		return true
	}
	model.preview = &detail
	model.phase = searchPhaseIdle
	return true
}

func (model *searchModel) finishRequest(request searchRequest) bool {
	if !model.requestActive || model.active != request {
		return false
	}
	model.requestActive = false
	model.active = searchRequest{}
	if model.cancelRequest != nil {
		model.cancelRequest()
		model.cancelRequest = nil
	}
	return true
}

func (model *searchModel) current(request searchRequest) bool {
	return !model.closed && model.generation == request.generation
}

func (model *searchModel) cancel() {
	model.invalidateRequest()
	model.pending = 0
	model.pendingItem = backend.Anime{}
	model.phase = searchPhaseIdle
}

func (model *searchModel) close() {
	model.closed = true
	model.invalidateRequest()
	model.pending = 0
	model.pendingItem = backend.Anime{}
	model.phase = searchPhaseIdle
}

func (model *searchModel) invalidateRequest() {
	model.generation++
	if model.cancelRequest != nil {
		model.cancelRequest()
		model.cancelRequest = nil
	}
	model.preview = nil
	model.previewItem = backend.Anime{}
	model.phase = searchPhaseIdle
}

func (model *searchModel) moveSelection(delta int) {
	if len(model.results) == 0 {
		model.selected = 0
		return
	}
	model.selected = max(0, min(len(model.results)-1, model.selected+delta))
}

func (model *searchModel) movePage(delta int) {
	if len(model.results) == 0 || delta == 0 {
		return
	}
	page := model.selected / searchResultPageSize
	lastPage := (len(model.results) - 1) / searchResultPageSize
	page = max(0, min(lastPage, page+delta))
	model.selected = min(page*searchResultPageSize, len(model.results)-1)
}

func (model *searchModel) selectedItem() (backend.Anime, bool) {
	if model.selected < 0 || model.selected >= len(model.results) {
		return backend.Anime{}, false
	}
	return model.results[model.selected], true
}

func (model *searchModel) visibleResults() []backend.Anime {
	start := model.pageStart()
	end := min(len(model.results), start+searchResultPageSize)
	if start >= end {
		return nil
	}
	return model.results[start:end]
}

func (model *searchModel) pageStart() int {
	if len(model.results) == 0 {
		return 0
	}
	return model.selected / searchResultPageSize * searchResultPageSize
}

func (model *searchModel) pageCount() int {
	if len(model.results) == 0 {
		return 0
	}
	return (len(model.results) + searchResultPageSize - 1) / searchResultPageSize
}

func (model *searchModel) pageNumber() int {
	if len(model.results) == 0 {
		return 0
	}
	return model.selected/searchResultPageSize + 1
}

func (model *searchModel) merge(items []backend.Anime) {
	byID := make(map[string]backend.Anime, len(model.items)+len(items))
	for _, item := range model.items {
		if item.ID != "" {
			byID[item.ID] = item
		}
	}
	for _, item := range items {
		if item.ID != "" {
			byID[item.ID] = item
		}
	}
	model.items = make([]backend.Anime, 0, len(byID))
	for _, item := range byID {
		model.items = append(model.items, item)
	}
	sort.Slice(model.items, func(i, j int) bool {
		left, right := strings.ToLower(model.items[i].Title), strings.ToLower(model.items[j].Title)
		if left != right {
			return left < right
		}
		return model.items[i].ID < model.items[j].ID
	})
}

func (model *searchModel) filter() {
	query := strings.ToLower(strings.TrimSpace(model.query))
	model.results = model.results[:0]
	for _, item := range model.items {
		if query == "" || strings.Contains(strings.ToLower(item.Title), query) || strings.Contains(strings.ToLower(item.NativeTitle), query) {
			model.results = append(model.results, item)
		}
	}
	model.selected = max(0, min(model.selected, len(model.results)-1))
}
