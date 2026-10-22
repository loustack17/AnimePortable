//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"animeportable/apps/desktop/backend"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

type searchViewTestService struct {
	noSettingsActions
	libraryCalls atomic.Int32
	searchCalls  atomic.Int32
	libraryErr   error
	appearance   string
	episodes     func(context.Context, string) ([]backend.Episode, error)
}

type coverSearchViewTestService struct {
	*searchViewTestService
	coverErr error
}

func (service *coverSearchViewTestService) GetCover(context.Context, string) (backend.Cover, error) {
	return backend.Cover{}, service.coverErr
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
	ui := &view{window: window, service: service, ctx: ctx, refresh: core.NewRefresher(ctx, core.RefreshLimit, core.RefreshInterval)}
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
	ui.buildSearchView()
	ui.searchPage.SetLabel("search-page")
	ui.searchScroll.SetLabel("search-results-scroll")
	ui.mainGroup.End()
	window.End()
	ui.mainGroup.Show()
	t.Cleanup(func() {
		ui.stopDetailLoad()
		ui.searchModel.close()
		cancel()
		releaseTestWindow(window)
		fltk.Unlock()
		runtime.UnlockOSThread()
	})
	return ui
}

// pumpSearchEvents runs FLTK events until ready holds. The limit only bounds a failure: with the
// race detector, every package runs at once and the libmpv tests take all cores, so a 2 s limit
// failed while the event was merely late (Loop 37 CI).
func pumpSearchEvents(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
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
	for _, child := range []struct {
		name string
		x    int
		y    int
		w    int
		h    int
	}{
		{name: "episode status", x: ui.previewStatus.X(), y: ui.previewStatus.Y(), w: ui.previewStatus.W(), h: ui.previewStatus.H()},
		{name: "episode list", x: ui.previewEpisodes.X(), y: ui.previewEpisodes.Y(), w: ui.previewEpisodes.W(), h: ui.previewEpisodes.H()},
		{name: "episode page text", x: ui.previewEpisodeText.X(), y: ui.previewEpisodeText.Y(), w: ui.previewEpisodeText.W(), h: ui.previewEpisodeText.H()},
		{name: "previous page", x: ui.previewPrevious.X(), y: ui.previewPrevious.Y(), w: ui.previewPrevious.W(), h: ui.previewPrevious.H()},
		{name: "next page", x: ui.previewNext.X(), y: ui.previewNext.Y(), w: ui.previewNext.W(), h: ui.previewNext.H()},
		{name: "back action", x: ui.previewBack.X(), y: ui.previewBack.Y(), w: ui.previewBack.W(), h: ui.previewBack.H()},
		{name: "continue action", x: ui.previewContinue.X(), y: ui.previewContinue.Y(), w: ui.previewContinue.W(), h: ui.previewContinue.H()},
		{name: "retry action", x: ui.previewRetry.X(), y: ui.previewRetry.Y(), w: ui.previewRetry.W(), h: ui.previewRetry.H()},
		{name: "title", x: ui.previewTitle.X(), y: ui.previewTitle.Y(), w: ui.previewTitle.W(), h: ui.previewTitle.H()},
		{name: "cover", x: ui.previewCover.X(), y: ui.previewCover.Y(), w: ui.previewCover.W(), h: ui.previewCover.H()},
		{name: "metadata", x: ui.previewMeta.X(), y: ui.previewMeta.Y(), w: ui.previewMeta.W(), h: ui.previewMeta.H()},
		{name: "synopsis", x: ui.previewDescription.X(), y: ui.previewDescription.Y(), w: ui.previewDescription.W(), h: ui.previewDescription.H()},
	} {
		if child.x < ui.previewGroup.X() || child.x+child.w > ui.previewGroup.X()+ui.previewGroup.W() || child.y < ui.previewGroup.Y() || child.y+child.h > ui.previewGroup.Y()+ui.previewGroup.H() {
			t.Errorf("%s bounds (%d,%d %dx%d) exceed preview group (%d,%d %dx%d)", child.name, child.x, child.y, child.w, child.h, ui.previewGroup.X(), ui.previewGroup.Y(), ui.previewGroup.W(), ui.previewGroup.H())
		}
	}
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

func TestNativeDetailArrowKeepsLastEpisodeVisibleAtMinimumWindow(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.window.Resize(0, 0, 760, 480)
	ui.resizeSearch()
	ui.searchPreview = true
	ui.detailModel.open("anime")
	episodes := make([]backend.Episode, detailEpisodePageSize)
	for index := range episodes {
		episodes[index] = backend.Episode{ID: fmt.Sprintf("episode-%d", index)}
	}
	ui.detailModel.setEpisodes(episodes)
	ui.previewGroup.Show()
	ui.renderDetailEpisodes()
	ui.previewEpisodeRows[0].TakeFocus()
	for index := 0; index < len(episodes)-1; index++ {
		ui.handleDetailEpisodeKey(index, classifyKey(0xff54, 0))
		pumpSearchEvents(t, func() bool {
			return ui.detailModel.selected == index+1 && ui.previewEpisodeRows[index+1].HasFocus()
		})
	}
	row := ui.previewEpisodeRows[len(ui.previewEpisodeRows)-1]
	rowTop := row.Y()
	rowBottom := rowTop + row.H()
	viewportTop := ui.previewEpisodes.Y()
	viewportBottom := viewportTop + ui.previewEpisodes.H()
	if rowTop < viewportTop || rowBottom > viewportBottom {
		t.Fatalf("focused final episode bounds (%d,%d) are outside viewport (%d,%d)", rowTop, rowBottom, viewportTop, viewportBottom)
	}
	ui.previewEpisodes.ScrollTo(ui.previewEpisodes.XPosition(), 100)
	ui.focusDetailEpisodeRow(len(episodes) - 1)
	rowTop = row.Y()
	rowBottom = rowTop + row.H()
	if rowTop < viewportTop || rowBottom > viewportBottom {
		t.Fatalf("focused final episode after partial scroll bounds (%d,%d) are outside viewport (%d,%d)", rowTop, rowBottom, viewportTop, viewportBottom)
	}
	ui.window.Resize(0, 0, 1000, 618)
	ui.resizeSearch()
	if !row.HasFocus() || row.W() != max(120, ui.previewEpisodes.W()-27) {
		t.Fatalf("default reflow lost row focus or width: focused=%v width=%d viewport=%d", row.HasFocus(), row.W(), ui.previewEpisodes.W())
	}
	rowTop = row.Y()
	rowBottom = rowTop + row.H()
	if rowTop < ui.previewEpisodes.Y() || rowBottom > ui.previewEpisodes.Y()+ui.previewEpisodes.H() {
		t.Fatalf("focused final episode after default reflow is outside viewport: (%d,%d)", rowTop, rowBottom)
	}
	ui.previewBack.TakeFocus()
	ui.window.Resize(0, 0, 760, 480)
	ui.resizeSearch()
	if !ui.previewBack.HasFocus() {
		t.Fatal("detail row reflow stole focus from the Back action")
	}
	for index, resizedRow := range ui.previewEpisodeRows {
		if resizedRow.X() != ui.previewEpisodes.X()+8 || resizedRow.Y() != ui.previewEpisodes.Y()+7+index*37 || resizedRow.W() != max(120, ui.previewEpisodes.W()-27) {
			t.Fatalf("minimum reflow row %d has bounds (%d,%d %dx%d)", index, resizedRow.X(), resizedRow.Y(), resizedRow.W(), resizedRow.H())
		}
	}
}

func TestNativeContinueActionOnlyShowsForResumableEpisode(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.searchPreview = true
	ui.previewGroup.Show()
	ui.onPlay = func(backend.PlayRequest) {}
	ui.detailModel.open("anime")
	ui.detailModel.setEpisodes([]backend.Episode{{ID: "episode-1"}})
	ui.searchModel.preview = &backend.Detail{Anime: backend.Anime{ID: "anime", Title: "作品"}}
	for _, testCase := range []struct {
		name    string
		history []backend.History
		visible bool
	}{
		{name: "unwatched"},
		{name: "other anime", history: []backend.History{{AnimeID: "other", EpisodeID: "episode-1", Position: 5000}}},
		{name: "completed", history: []backend.History{{AnimeID: "anime", EpisodeID: "episode-1", Position: 5000, Completed: true}}},
		{name: "zero position", history: []backend.History{{AnimeID: "anime", EpisodeID: "episode-1"}}},
		{name: "resumable", history: []backend.History{{AnimeID: "anime", EpisodeID: "episode-1", Position: 5000}}, visible: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ui.detailModel.setHistory(testCase.history)
			ui.renderPreview()
			if got := ui.previewContinue.Visible(); got != testCase.visible {
				t.Fatalf("Continue visibility = %v, want %v", got, testCase.visible)
			}
		})
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
	input := "  " + strings.Repeat("作品說明\n", 300) + "  "
	got := truncateSearchDescription(input)
	if utf8.RuneCountInString(got) > maxSynopsisRunes || !strings.HasSuffix(got, "…") || !strings.HasPrefix(got, "作品說明\n作品說明") {
		t.Fatalf("preview excerpt is not bounded and normalized: %d runes, %q", utf8.RuneCountInString(got), got)
	}
	if got := truncateSearchDescription(" a   b \n\n\n  c "); got != "a b\nc" {
		t.Fatalf("paragraphs = %q, want blank lines dropped and breaks kept", got)
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

func TestNativeDetailSpacePlaysExactFocusedEpisodeAndArrowRestoresFocus(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.searchPreview = true
	ui.detailModel.open("anime")
	ui.detailModel.setEpisodes([]backend.Episode{{ID: "episode-0"}, {ID: "episode-1"}, {ID: "episode-2"}, {ID: "episode-3"}})
	ui.detailModel.selectEpisode(2)
	ui.previewGroup.Show()
	ui.renderDetailEpisodes()
	ui.previewEpisodeRows[2].TakeFocus()
	ui.handleDetailEpisodeKey(2, classifyKey(0xff52, 0))
	pumpSearchEvents(t, func() bool { return ui.detailModel.selected == 1 && ui.previewEpisodeRows[1].HasFocus() })
	ui.detailModel.selectEpisode(0)
	ui.renderDetailEpisodes()
	ui.previewEpisodeRows[2].TakeFocus()
	ui.handleDetailEpisodeKey(2, classifyKey(0xff54, 0))
	pumpSearchEvents(t, func() bool { return ui.detailModel.selected == 3 && ui.previewEpisodeRows[3].HasFocus() })
	ui.detailModel.setHistory(nil)
	ui.renderDetailEpisodes()
	if !ui.previewEpisodeRows[3].HasFocus() {
		t.Fatal("asynchronous history refresh lost the focused episode row")
	}
	var request backend.PlayRequest
	played := false
	ui.onPlay = func(value backend.PlayRequest) { request, played = value, true }
	ui.previewEpisodeRows[2].TakeFocus()
	ui.handleDetailEpisodeKey(2, classifyKey(32, 0))
	pumpSearchEvents(t, func() bool { return played })
	if request.AnimeID != "anime" || request.EpisodeID != "episode-2" || request.StartAt != 0 {
		t.Fatalf("focused episode Space request = %#v", request)
	}
}

func TestNativeDetailWithoutPlayCallbackKeepsSelectedRowFocusedAndCancelsLoads(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.searchPreview = true
	ui.detailModel.open("anime")
	ui.detailModel.setEpisodes([]backend.Episode{{ID: "episode-0"}, {ID: "episode-1"}, {ID: "episode-2"}})
	ui.previewGroup.Show()
	ui.renderDetailEpisodes()
	ui.handleDetailEpisodeKey(2, classifyKey(13, 0))
	pumpSearchEvents(t, func() bool { return ui.detailModel.selected == 2 && ui.previewEpisodeRows[2].HasFocus() })
	backCtx, backCancel := context.WithCancel(ui.ctx)
	ui.detailCancel = backCancel
	generation := ui.detailGeneration
	if !ui.detailRequestCurrent(generation, "anime") {
		t.Fatal("current detail request was rejected")
	}
	ui.backToSearchResults()
	if backCtx.Err() == nil || ui.detailRequestCurrent(generation, "anime") {
		t.Fatal("Back kept a detail request active")
	}
	closeCtx, closeCancel := context.WithCancel(ui.ctx)
	ui.detailModel.open("anime")
	ui.detailCancel = closeCancel
	generation = ui.detailGeneration
	ui.stopDetailLoad()
	if closeCtx.Err() == nil || ui.detailRequestCurrent(generation, "anime") {
		t.Fatal("close cancellation kept a detail request current")
	}
}

func TestNativeFailedPlayCallbackKeepsDetailReadyForRetry(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.searchPreview = true
	ui.selected = 4
	ui.searchModel.previewItem = backend.Anime{ID: "anime"}
	ui.detailModel.open("anime")
	ui.detailModel.setEpisodes([]backend.Episode{{ID: "episode"}})
	ui.onPlay = func(backend.PlayRequest) {}
	resourceCtx, cancel := context.WithCancel(ui.ctx)
	ui.detailCancel = cancel
	ui.playSelectedDetailEpisode()
	if resourceCtx.Err() == nil {
		t.Fatal("play did not cancel pending detail resources")
	}
	if ui.detailModel.animeID != "anime" || ui.searchModel.previewItem.ID != "anime" {
		t.Fatal("failed Play callback discarded the selected detail")
	}
	request, ok := ui.detailModel.playRequest()
	if !ok || request.AnimeID != "anime" || request.EpisodeID != "episode" {
		t.Fatalf("detail could not retry Play: %#v, %v", request, ok)
	}
}

func TestNativeDetailMetadataAndUnavailableCoverStayNeutral(t *testing.T) {
	if got := detailMetadataLabel(nil); got != "" {
		t.Fatalf("missing metadata label = %q", got)
	}
	if got := detailMetadataLabel(&backend.Metadata{Season: "春", Year: 2025, Studio: "Studio", EpisodeCount: 12}); got != "春 2025\n製作：Studio\n共 12 集" {
		t.Fatalf("metadata label = %q", got)
	}
	ui := newSearchViewTestFixture(t, nil)
	ui.selected = 4
	ui.searchPreview = true
	ui.detailModel.open("anime")
	ui.searchModel.previewItem = backend.Anime{ID: "anime", Title: "作品"}
	ui.previewCover.SetLabel("目前沒有封面")
	ui.renderPreview()
	if ui.previewCover.Label() != "目前沒有封面" {
		t.Fatalf("cover fallback label = %q", ui.previewCover.Label())
	}
}

func TestNativeCoverDecodeBoundsAndReplacementLifecycle(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 8, 12))
	for y := 0; y < source.Bounds().Dy(); y++ {
		for x := 0; x < source.Bounds().Dx(); x++ {
			source.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 10), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	firstPixels, err := decodeDetailCover(encoded.Bytes(), "image/png")
	if err != nil || len(firstPixels) != detailCoverWidth*detailCoverHeight*4 {
		t.Fatalf("first cover decode = %v, pixel bytes %d", err, len(firstPixels))
	}
	first, err := newDetailCoverImage(firstPixels)
	if err != nil {
		t.Fatal(err)
	}
	ui := newSearchViewTestFixture(t, nil)
	ui.previewCoverImage = first
	secondPixels, err := decodeDetailCover(encoded.Bytes(), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := newDetailCoverImage(secondPixels)
	if err != nil {
		t.Fatal(err)
	}
	ui.releaseDetailCover()
	ui.previewCoverImage = second
	if ui.previewCoverImage.W() != detailCoverWidth || len(secondPixels) != detailCoverWidth*detailCoverHeight*4 {
		t.Fatal("replacement cover is not retained within the decoded image bound")
	}
	if _, err := decodeDetailCover(encoded.Bytes(), "image/jpeg"); err == nil {
		t.Fatal("cover format mismatch was accepted")
	}
	ui.releaseDetailCover()
}

func TestNativeCoverDecodePreservesSquarePortraitAndLandscapeAspect(t *testing.T) {
	for _, dimensions := range []struct {
		name   string
		width  int
		height int
	}{
		{name: "square", width: 12, height: 12},
		{name: "portrait", width: 8, height: 12},
		{name: "landscape", width: 16, height: 9},
	} {
		t.Run(dimensions.name, func(t *testing.T) {
			source := image.NewRGBA(image.Rect(0, 0, dimensions.width, dimensions.height))
			for y := 0; y < dimensions.height; y++ {
				for x := 0; x < dimensions.width; x++ {
					source.Set(x, y, color.RGBA{R: 255, A: 255})
				}
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, source); err != nil {
				t.Fatal(err)
			}
			pixels, err := decodeDetailCover(encoded.Bytes(), "image/png")
			if err != nil {
				t.Fatal(err)
			}
			minX, minY := detailCoverWidth, detailCoverHeight
			maxX, maxY := -1, -1
			for y := 0; y < detailCoverHeight; y++ {
				for x := 0; x < detailCoverWidth; x++ {
					if pixels[(y*detailCoverWidth+x)*4+3] == 0 {
						continue
					}
					minX, maxX = min(minX, x), max(maxX, x)
					minY, maxY = min(minY, y), max(maxY, y)
				}
			}
			if maxX < minX || maxY < minY {
				t.Fatal("decoded cover has no visible source pixels")
			}
			contentWidth, contentHeight := maxX-minX+1, maxY-minY+1
			aspectError := contentWidth*dimensions.height - contentHeight*dimensions.width
			if aspectError < 0 {
				aspectError = -aspectError
			}
			if aspectError > max(dimensions.width, dimensions.height) {
				t.Fatalf("content bounds %dx%d do not preserve source aspect %dx%d", contentWidth, contentHeight, dimensions.width, dimensions.height)
			}
		})
	}
}

func TestNativeAbsentCoverIsNeutralAndCoverFailureCanRetry(t *testing.T) {
	for _, test := range []struct {
		name      string
		err       error
		wantLabel string
		wantRetry bool
	}{
		{name: "absent", err: core.ErrNotFound, wantLabel: "目前沒有封面"},
		{name: "loader failure", err: backend.ErrUnavailable, wantLabel: "封面暫時無法載入", wantRetry: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &coverSearchViewTestService{searchViewTestService: &searchViewTestService{}, coverErr: test.err}
			ui := newSearchViewTestFixture(t, service)
			ui.selected = 4
			ui.searchPreview = true
			ui.searchModel.previewItem = backend.Anime{ID: "anime", Title: "作品"}
			ui.detailModel.open("anime")
			ui.previewGroup.Show()
			ui.previewCover.SetLabel("封面載入中…")
			ui.loadDetailResources(ui.detailGeneration, "anime")
			pumpSearchEvents(t, func() bool {
				return ui.previewCover.Label() == test.wantLabel && ui.detailEpisodesDone
			})
			if ui.detailCoverErr != test.wantRetry || ui.previewRetry.Visible() != test.wantRetry {
				t.Fatalf("cover error=%v retry visible=%v, want %v", ui.detailCoverErr, ui.previewRetry.Visible(), test.wantRetry)
			}
		})
	}
}

func TestNativeCoverRedrawScalesWithinResizedBoxAndKeepsSourceDimensions(t *testing.T) {
	ui := newSearchViewTestFixture(t, nil)
	ui.previewCover.Resize(252, 215, 112, 150)
	ui.previewGroup.Show()
	image, err := newDetailCoverImage(make([]byte, detailCoverWidth*detailCoverHeight*4))
	if err != nil {
		t.Fatal(err)
	}
	ui.previewCoverImage = image
	ui.window.Show()
	ui.previewCover.Redraw()
	fltk.Wait(0.02)
	if image.W() > ui.previewCover.W() || image.H() > ui.previewCover.H() {
		t.Fatalf("display dimensions %dx%d exceed resized cover box %dx%d", image.W(), image.H(), ui.previewCover.W(), ui.previewCover.H())
	}
	ui.previewCover.Resize(252, 215, 160, 200)
	ui.previewCover.Redraw()
	fltk.Wait(0.02)
	if image.W() != 150 || image.H() != 200 {
		t.Fatalf("display dimensions after cover enlargement = %dx%d, want 150x200", image.W(), image.H())
	}
	if image.DataW() != detailCoverWidth || image.DataH() != detailCoverHeight {
		t.Fatalf("source image dimensions changed to %dx%d", image.DataW(), image.DataH())
	}
	ui.releaseDetailCover()
}

func (service *searchViewTestService) Follow(context.Context, string) error        { return nil }
func (service *searchViewTestService) Unfollow(context.Context, string) error      { return nil }
func (service *searchViewTestService) RemoveHistory(context.Context, string) error { return nil }
func (service *searchViewTestService) Schedule(context.Context) ([]backend.ScheduleItem, error) {
	return nil, nil
}
func (service *searchViewTestService) CachedSchedule(context.Context) ([]backend.ScheduleItem, error) {
	return nil, nil
}
