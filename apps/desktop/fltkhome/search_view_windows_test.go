//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"animeportable/apps/desktop/backend"
	fltk "github.com/pwiecz/go-fltk"
)

type searchViewTestService struct {
	libraryCalls atomic.Int32
	searchCalls  atomic.Int32
	libraryErr   error
	appearance   string
	episodes     func(context.Context, string) ([]backend.Episode, error)
}

func (service *searchViewTestService) Start(context.Context) error { return nil }

func (service *searchViewTestService) Settings(context.Context) (backend.Settings, error) {
	if service.appearance != "" {
		return backend.Settings{Appearance: service.appearance}, nil
	}
	return backend.Settings{Appearance: "light"}, nil
}

func (service *searchViewTestService) SaveSettings(context.Context, backend.Settings) error {
	return nil
}

func (service *searchViewTestService) Episodes(ctx context.Context, animeID string) ([]backend.Episode, error) {
	if service.episodes != nil {
		return service.episodes(ctx, animeID)
	}
	return []backend.Episode{{ID: "episode-1"}}, nil
}

func (service *searchViewTestService) Library(context.Context) ([]backend.Anime, error) {
	service.libraryCalls.Add(1)
	return nil, service.libraryErr
}

func (service *searchViewTestService) Search(context.Context, string) ([]backend.Anime, error) {
	service.searchCalls.Add(1)
	return nil, nil
}

func (service *searchViewTestService) Detail(context.Context, string) (backend.Detail, error) {
	return backend.Detail{}, nil
}

func (service *searchViewTestService) History(context.Context) ([]backend.History, error) {
	return nil, nil
}

func (service *searchViewTestService) Following(context.Context) ([]backend.Following, error) {
	return nil, nil
}

func newSearchViewTestFixture(t *testing.T, service Service) *view {
	t.Helper()
	runtime.LockOSThread()
	if !fltk.Lock() {
		runtime.UnlockOSThread()
		t.Fatal("FLTK threading initialization failed")
	}
	window := fltk.NewWindow(1000, 618, "Search test")
	ctx, cancel := context.WithCancel(context.Background())
	ui := &view{window: window, service: service, ctx: ctx}
	window.Begin()
	ui.mainGroup = fltk.NewGroup(0, 0, 1000, 618)
	ui.mainGroup.Begin()
	ui.header = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.tagline = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.message = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.sectionTitle = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.contentHint = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.scroll = fltk.NewScroll(0, 0, 1, 1)
	ui.scroll.End()
	ui.loadingText = fltk.NewBox(fltk.NO_BOX, 0, 0, 1, 1, "")
	ui.retryButton = fltk.NewButton(0, 0, 1, 1, "")
	ui.themeToggle = fltk.NewButton(0, 0, 1, 1, "")
	ui.buildSearchView()
	ui.searchPage.SetLabel("search-page")
	ui.searchScroll.SetLabel("search-results-scroll")
	ui.mainGroup.End()
	window.End()
	ui.mainGroup.Show()
	t.Cleanup(func() {
		ui.searchModel.close()
		cancel()
		window.Hide()
		window.Destroy()
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	return ui
}

func pumpSearchEvents(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !ready() && time.Now().Before(deadline) {
		fltk.Wait(0.01)
	}
	if !ready() {
		t.Fatal("FLTK event did not reach the expected Search state")
	}
}

func groupHasLabel(group *fltk.Group, label string) bool {
	for _, child := range group.Children() {
		if child.Label() == label {
			return true
		}
	}
	return false
}

func groupLabelCount(group *fltk.Group, label string) int {
	count := 0
	for _, child := range group.Children() {
		if child.Label() == label {
			count++
		}
	}
	return count
}

func TestNativeSearchViewBoundsRowsParentsFooterAndReflows(t *testing.T) {
	ui := newSearchViewTestFixture(t, &searchViewTestService{})
	items := make([]backend.Anime, searchResultPageSize*2+1)
	for index := range items {
		items[index] = backend.Anime{ID: fmt.Sprintf("id-%02d", index), Title: "Matching"}
	}
	ui.searchModel.setItems(items)
	ui.searchModel.setQuery("matching")
	ui.searchPage.Show()
	ui.renderSearch()
	if got := len(ui.searchRows); got != searchResultPageSize {
		t.Fatalf("native result widgets = %d, want bounded %d", got, searchResultPageSize)
	}
	if got := groupLabelCount(&ui.searchScroll.Group, "Matching"); got != searchResultPageSize {
		t.Fatalf("scroll result labels = %d, want only the visible result page", got)
	}
	if ui.searchPrevious.Parent().Label() != "search-page" || ui.searchNext.Parent().Label() != "search-page" || ui.searchScroll.Parent().Label() != "search-page" {
		t.Fatal("pagination controls or results scroll are not children of the Search page")
	}
	if !groupHasLabel(ui.searchPage, "上一頁") || !groupHasLabel(ui.searchPage, "下一頁") || groupHasLabel(&ui.searchScroll.Group, "上一頁") || groupHasLabel(&ui.searchScroll.Group, "下一頁") {
		t.Fatal("pagination controls are missing from the Search page or nested in the results scroll")
	}
	ui.searchModel.movePage(2)
	ui.renderSearch()
	if len(ui.searchRows) != 1 || ui.searchModel.selected != searchResultPageSize*2 || groupLabelCount(&ui.searchScroll.Group, "Matching") != 1 {
		t.Fatalf("last page rendered %d rows at selection %d", len(ui.searchRows), ui.searchModel.selected)
	}
	ui.window.Resize(0, 0, 760, 480)
	ui.resizeSearch()
	if bottom := ui.searchScroll.Y() + ui.searchScroll.H(); bottom >= ui.searchPrevious.Y() {
		t.Fatalf("results viewport bottom %d overlaps footer at %d", bottom, ui.searchPrevious.Y())
	}
	if ui.searchRows[0].W() != max(300, ui.searchScroll.W()-35) {
		t.Fatalf("result width %d did not follow viewport width %d", ui.searchRows[0].W(), ui.searchScroll.W())
	}
	ui.searchModel.selected = searchResultPageSize * 2
	ui.searchPreview = true
	ui.searchPage.Hide()
	ui.previewGroup.Show()
	ui.backToSearchResults()
	if ui.searchModel.selected != searchResultPageSize*2 || !ui.searchPage.Visible() || ui.previewGroup.Visible() {
		t.Fatal("Back did not restore the selected result page")
	}
}

func TestNativeDeferredSubmitIsDiscardedAfterLeavingSearch(t *testing.T) {
	service := &searchViewTestService{}
	ui := newSearchViewTestFixture(t, service)
	ui.selected = 4
	ui.searchModel.setQuery("query")
	ui.deferredSearchAction(ui.submitSearch, false)()
	ui.showSection(0)
	ui.showSection(4)
	pumpSearchEvents(t, func() bool { return service.libraryCalls.Load() == 1 && ui.searchModel.loaded })
	if got := service.searchCalls.Load(); got != 0 {
		t.Fatalf("deferred Search ran after leaving the page: %d calls", got)
	}
}

func TestNativeStoredSearchButtonActionUsesCurrentPageGeneration(t *testing.T) {
	ui := newSearchViewTestFixture(t, &searchViewTestService{})
	called := false
	activate := ui.deferredSearchAction(func() { called = true }, false)
	ui.showSection(4)
	activate()
	pumpSearchEvents(t, func() bool { return called })
}

func TestNativeDeferredResultActionIsInvalidatedByPreviewRoundTrip(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.searchPage.Show()
	called := false
	ui.deferredSearchAction(func() { called = true }, false)()
	ui.openSearchResult(backend.Anime{ID: "cached", Title: "Cached"})
	ui.backToSearchResults()
	processed := false
	fltk.AddTimeout(0.03, func() { processed = true })
	pumpSearchEvents(t, func() bool { return processed })
	if called {
		t.Fatal("deferred result action ran after preview and Back")
	}
}

func TestNativeRemoteSubmitShowsLoadingAndDisablesDuplicateSubmit(t *testing.T) {
	service := &searchViewTestService{}
	ui := newSearchViewTestFixture(t, service)
	ui.selected = 4
	ui.searchPage.Show()
	ui.searchModel.setItems([]backend.Anime{{ID: "cached", Title: "Cached Match"}})
	ui.searchModel.setQuery("match")
	ui.renderSearch()
	ui.submitSearch()
	if ui.searchModel.phase != searchPhaseLoading || ui.searchStatus.Label() != "正在搜尋…" || ui.searchButton.IsActive() {
		t.Fatalf("submission state = phase %d, status %q, active %v", ui.searchModel.phase, ui.searchStatus.Label(), ui.searchButton.IsActive())
	}
	if len(ui.searchRows) != 1 || ui.searchRows[0].Label() != "Cached Match" {
		t.Fatal("loading state hid cached results")
	}
	pumpSearchEvents(t, func() bool { return ui.searchModel.remoteQuery == "match" })
}

func TestNativePreviewDescriptionUsesBoundedNormalizedExcerpt(t *testing.T) {
	input := "  " + strings.Repeat("作品說明\n", 100) + "  "
	got := truncateSearchDescription(input)
	if utf8.RuneCountInString(got) > 320 || !strings.HasSuffix(got, "…") || strings.Contains(got, "\n") {
		t.Fatalf("preview excerpt is not bounded and normalized: %d runes, %q", utf8.RuneCountInString(got), got)
	}
}

func TestNativeLibraryFailureSettlesUntilExplicitSearchRetry(t *testing.T) {
	service := &searchViewTestService{libraryErr: errors.New("private path")}
	ui := newSearchViewTestFixture(t, service)
	ui.selected = 4
	ui.searchPage.Show()
	ui.searchModel.setQuery("query")
	ui.loadSearchLibrary()
	pumpSearchEvents(t, func() bool { return ui.searchModel.libraryFailed })
	if got := service.libraryCalls.Load(); got != 1 || !ui.searchModel.libraryFailed {
		t.Fatalf("first library attempt = %d, failed state %v", got, ui.searchModel.libraryFailed)
	}
	ui.refreshSearchView()
	if got := service.libraryCalls.Load(); got != 1 {
		t.Fatalf("failed Library auto-retried %d times", got-1)
	}
	ui.submitSearch()
	pumpSearchEvents(t, func() bool { return ui.searchModel.remoteQuery == "query" })
	if got := service.libraryCalls.Load(); got != 2 {
		t.Fatalf("explicit retry made %d Library calls, want 2 total", got)
	}
	if got := service.searchCalls.Load(); got != 1 {
		t.Fatalf("explicit Search retry made %d remote calls, want 1", got)
	}
}
