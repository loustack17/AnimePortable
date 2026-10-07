// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"animeportable/apps/desktop/backend"
)

func TestSearchModelFiltersCachedTitlesAndNativeTitles(t *testing.T) {
	model := searchModel{}
	model.setItems([]backend.Anime{
		{ID: "one", Title: "A Story"},
		{ID: "two", Title: "另一個故事", NativeTitle: "青の物語"},
		{ID: "three", Title: "Different"},
	})
	model.setQuery("  STORY ")
	if len(model.results) != 1 || model.results[0].ID != "one" {
		t.Fatalf("title results = %#v", model.results)
	}
	model.setQuery("青の")
	if len(model.results) != 1 || model.results[0].ID != "two" {
		t.Fatalf("native title results = %#v", model.results)
	}
	model.setQuery("missing")
	if len(model.results) != 0 || model.phase != searchPhaseIdle {
		t.Fatalf("empty local results = %#v, phase %d", model.results, model.phase)
	}
}

func TestSearchModelPagesStayBoundedAndSelectionReachesEveryResult(t *testing.T) {
	model := searchModel{}
	items := make([]backend.Anime, searchResultPageSize*2+1)
	for index := range items {
		items[index] = backend.Anime{ID: fmt.Sprintf("id-%02d", index), Title: "Matching"}
	}
	model.setItems(items)
	model.setQuery("matching")
	if got := len(model.visibleResults()); got != searchResultPageSize {
		t.Fatalf("visible result count = %d, want %d", got, searchResultPageSize)
	}
	for range len(items) - 1 {
		model.moveSelection(1)
	}
	selected, ok := model.selectedItem()
	if !ok || selected.ID != "id-16" || model.pageNumber() != 3 {
		t.Fatalf("last selection = %#v, ok %v, page %d", selected, ok, model.pageNumber())
	}
	if got := len(model.visibleResults()); got != 1 {
		t.Fatalf("last page result count = %d, want 1", got)
	}
	model.moveSelection(1)
	selected, _ = model.selectedItem()
	if selected.ID != "id-16" {
		t.Fatalf("selection moved past end: %#v", selected)
	}
}

func TestSearchModelDoesNotSubmitBlankQueryAndMergesRemoteResults(t *testing.T) {
	model := searchModel{}
	model.setItems([]backend.Anime{{ID: "local", Title: "Local"}})
	if _, _, ok := model.beginSearch(); ok {
		t.Fatal("blank query began remote search")
	}
	model.setQuery("remote")
	request, ctx, ok := model.beginSearch()
	if !ok || ctx == nil || model.phase != searchPhaseLoading {
		t.Fatal("nonblank query did not begin remote search")
	}
	if applied := model.finishSearch(request, []backend.Anime{{ID: "remote", Title: "Remote Result"}}, nil); !applied {
		t.Fatal("current remote result was rejected")
	}
	if len(model.results) != 1 || model.results[0].ID != "remote" || !model.loaded {
		t.Fatalf("merged results = %#v, loaded %v", model.results, model.loaded)
	}
}

func TestSearchModelSearchErrorKeepsCachedResultsAndRedactsError(t *testing.T) {
	model := searchModel{}
	model.setItems([]backend.Anime{{ID: "cached", Title: "Find me"}})
	model.setQuery("find")
	request, _, ok := model.beginSearch()
	if !ok {
		t.Fatal("search did not begin")
	}
	secret := errors.New("token=private C:/Users/example")
	if applied := model.finishSearch(request, nil, secret); !applied {
		t.Fatal("current search error was rejected")
	}
	if model.phase != searchPhaseFailed || len(model.results) != 1 || model.results[0].ID != "cached" {
		t.Fatalf("phase/results = %d/%#v", model.phase, model.results)
	}
}

func TestSearchModelEditCancelsAndRejectsStaleCompletion(t *testing.T) {
	model := searchModel{}
	model.setQuery("first")
	request, ctx, ok := model.beginSearch()
	if !ok {
		t.Fatal("first request did not begin")
	}
	model.setQuery("second")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("edit did not cancel request: %v", ctx.Err())
	}
	if _, _, ok := model.beginSearch(); ok {
		t.Fatal("second request overlapped a canceled request still unwinding")
	}
	if applied := model.finishSearch(request, []backend.Anime{{ID: "stale", Title: "First result"}}, nil); applied {
		t.Fatal("stale completion changed current state")
	}
	if model.query != "second" || len(model.results) != 0 || model.requestActive {
		t.Fatalf("state after stale completion = query %q, results %#v, active %v", model.query, model.results, model.requestActive)
	}
	if _, _, ok := model.beginSearch(); !ok {
		t.Fatal("current query could not submit after canceled request finished")
	}
}

func TestSearchModelNavigationAndCloseCancelDetailsAndRejectLateResults(t *testing.T) {
	model := searchModel{}
	item := backend.Anime{ID: "work", Title: "Work"}
	request, ctx, ok := model.beginDetail(item)
	if !ok || model.previewItem.ID != item.ID {
		t.Fatal("detail request did not begin for selected result")
	}
	model.cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("navigation did not cancel detail: %v", ctx.Err())
	}
	if model.finishDetail(request, backend.Detail{Anime: item}, nil) {
		t.Fatal("late detail completion changed state after navigation")
	}
	request, ctx, ok = model.beginDetail(item)
	if !ok {
		t.Fatal("detail request did not begin again")
	}
	model.close()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("close did not cancel request: %v", ctx.Err())
	}
	if model.finishDetail(request, backend.Detail{Anime: item}, nil) {
		t.Fatal("late detail completion changed closed state")
	}
	if _, _, ok := model.beginLibrary(); ok {
		t.Fatal("closed model accepted a new request")
	}
}

func TestSearchModelLoadsLibraryOnlyOnceAndIgnoresStaleLoad(t *testing.T) {
	model := searchModel{}
	request, ctx, ok := model.beginLibrary()
	if !ok {
		t.Fatal("initial library load did not begin")
	}
	model.setQuery("changed")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("query edit did not cancel active library request")
	}
	if model.finishLibrary(request, []backend.Anime{{ID: "late", Title: "Late"}}, nil) {
		t.Fatal("stale library completion was applied")
	}
	request, _, ok = model.beginLibrary()
	if !ok {
		t.Fatal("library retry did not begin")
	}
	if !model.finishLibrary(request, []backend.Anime{{ID: "fresh", Title: "Fresh"}}, nil) || !model.loaded {
		t.Fatal("current library completion was not applied")
	}
	if _, _, ok := model.beginLibrary(); ok {
		t.Fatal("library loaded more than once")
	}
}

func TestSearchModelQueuesOnlyLatestExplicitIntentBehindCanceledRequest(t *testing.T) {
	model := searchModel{}
	request, _, ok := model.beginLibrary()
	if !ok {
		t.Fatal("library request did not begin")
	}
	model.setQuery("older query")
	model.queueSearch()
	model.setQuery("latest query")
	if model.pending != 0 {
		t.Fatal("query edit retained an intent for an older query")
	}
	model.queueSearch()
	model.queueDetail(backend.Anime{ID: "selected", Title: "Selected"})
	if model.finishLibrary(request, nil, context.Canceled) {
		t.Fatal("canceled library result changed current state")
	}
	kind, item, ok := model.takePending()
	if !ok || kind != searchRequestDetail || item.ID != "selected" {
		t.Fatalf("pending intent = %d, %#v, %v", kind, item, ok)
	}
}

func TestSearchModelQueuesEnterDuringCurrentLibraryHydration(t *testing.T) {
	model := searchModel{}
	model.setQuery("retained query")
	libraryRequest, _, ok := model.beginLibrary()
	if !ok || libraryRequest.generation != model.generation {
		t.Fatal("current Library hydration did not begin")
	}
	model.queueSearch()
	if model.pending != searchRequestRemote {
		t.Fatalf("Enter during current Library hydration queued %d, want Search", model.pending)
	}
	if !model.finishLibrary(libraryRequest, nil, nil) {
		t.Fatal("current Library hydration did not complete")
	}
	kind, _, ok := model.takePending()
	if !ok || kind != searchRequestRemote {
		t.Fatalf("queued Enter intent = %d, %v", kind, ok)
	}
}

func TestSearchModelDeduplicatesEnterDuringCurrentRemoteSearch(t *testing.T) {
	model := searchModel{}
	model.setQuery("same query")
	if _, _, ok := model.beginSearch(); !ok {
		t.Fatal("remote search did not begin")
	}
	model.queueSearch()
	if model.pending != 0 {
		t.Fatalf("same-query active remote search queued duplicate intent %d", model.pending)
	}
}

func TestPendingSearchDoesNotReplaceUnfinishedLibraryHydration(t *testing.T) {
	model := searchModel{}
	libraryRequest, _, ok := model.beginLibrary()
	if !ok {
		t.Fatal("initial library request did not begin")
	}
	model.setQuery("remote")
	model.queueSearch()
	if model.finishLibrary(libraryRequest, nil, context.Canceled) {
		t.Fatal("canceled library completion was applied")
	}
	kind, _, ok := model.takePending()
	if !ok || kind != searchRequestRemote {
		t.Fatalf("pending search intent = %d, %v", kind, ok)
	}
	searchRequest, _, ok := model.beginSearch()
	if !ok {
		t.Fatal("queued search did not begin")
	}
	if !model.finishSearch(searchRequest, []backend.Anime{{ID: "remote", Title: "Remote"}}, nil) {
		t.Fatal("remote results did not complete")
	}
	if model.loaded {
		t.Fatal("remote results incorrectly marked local library hydration complete")
	}
	libraryRequest, _, ok = model.beginLibrary()
	if !ok {
		t.Fatal("local library hydration could not start after remote search")
	}
	if !model.finishLibrary(libraryRequest, []backend.Anime{
		{ID: "cached", Title: "Unrelated Cached Title"},
		{ID: "remote", Title: "Remote"},
	}, nil) {
		t.Fatal("current local library hydration was rejected")
	}
	model.setQuery("cached")
	if len(model.results) != 1 || model.results[0].ID != "cached" {
		t.Fatalf("cached results after hydration = %#v", model.results)
	}
}

func TestCanceledDetailAcknowledgementReleasesQueuedSearch(t *testing.T) {
	model := searchModel{}
	item := backend.Anime{ID: "work", Title: "Work"}
	detailRequest, _, ok := model.beginDetail(item)
	if !ok {
		t.Fatal("detail request did not begin")
	}
	model.cancel()
	model.setQuery("latest")
	model.queueSearch()
	if model.finishDetail(detailRequest, backend.Detail{Anime: item}, nil) {
		t.Fatal("canceled detail result was applied")
	}
	kind, _, ok := model.takePending()
	if !ok || kind != searchRequestRemote {
		t.Fatalf("queued search after detail acknowledgement = %d, %v", kind, ok)
	}
	if _, _, ok := model.beginSearch(); !ok {
		t.Fatal("released request gate did not allow the queued search")
	}
}

func TestSelectingCachedResultCancelsRemoteRequestAndQueuesDetail(t *testing.T) {
	model := searchModel{}
	model.setQuery("work")
	searchRequest, ctx, ok := model.beginSearch()
	if !ok {
		t.Fatal("remote search did not begin")
	}
	item := backend.Anime{ID: "selected", Title: "Selected"}
	model.queueDetail(item)
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("selecting a result did not cancel remote search: %v", ctx.Err())
	}
	if model.finishSearch(searchRequest, []backend.Anime{{ID: "late", Title: "Late"}}, nil) {
		t.Fatal("canceled remote results were applied")
	}
	kind, queued, ok := model.takePending()
	if !ok || kind != searchRequestDetail || queued.ID != item.ID {
		t.Fatalf("queued detail = %d, %#v, %v", kind, queued, ok)
	}
}
