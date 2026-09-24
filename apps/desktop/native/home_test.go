// SPDX-License-Identifier: MPL-2.0

package native

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"animeportable/adapters/player/mpv"
	"animeportable/apps/desktop/backend"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

type homeTestService struct {
	playRequests chan backend.PlayRequest
	loads        chan string
	cancelled    chan struct{}
}

func (service *homeTestService) Library(ctx context.Context) ([]backend.Anime, error) {
	if service.loads != nil {
		service.loads <- "library"
	}
	return nil, service.wait(ctx)
}

func (service *homeTestService) History(ctx context.Context) ([]backend.History, error) {
	if service.loads != nil {
		service.loads <- "history"
	}
	return nil, service.wait(ctx)
}

func (service *homeTestService) Following(ctx context.Context) ([]backend.Following, error) {
	if service.loads != nil {
		service.loads <- "following"
	}
	return nil, service.wait(ctx)
}

func (service *homeTestService) Play(ctx context.Context, request backend.PlayRequest) error {
	if service.playRequests != nil {
		service.playRequests <- request
	}
	return service.wait(ctx)
}

func (service *homeTestService) wait(ctx context.Context) error {
	<-ctx.Done()
	if service.cancelled != nil {
		service.cancelled <- struct{}{}
	}
	return ctx.Err()
}

func TestPlayMessagesDoNotExposeRawErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "success", want: "播放已開始。"},
		{name: "missing player", err: fmtError(mpv.ErrNotFound), want: "找不到 MPV，請先安裝 MPV；若已安裝，請聯絡程式提供者協助設定路徑。"},
		{name: "invalid path", err: fmtError(mpv.ErrInvalidPath), want: "播放器設定無法使用，請聯絡程式提供者協助修正 MPV 路徑。"},
		{name: "private failure", err: errors.New("secret path C:/private/player.exe"), want: "無法開始播放，請重試。"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := playMessage(testCase.err)
			if got != testCase.want {
				t.Fatalf("playMessage() = %q, want %q", got, testCase.want)
			}
			if strings.Contains(got, "private") || strings.Contains(got, "secret") {
				t.Fatalf("playMessage exposed raw error: %q", got)
			}
		})
	}
}

func fmtError(err error) error {
	return errors.Join(errors.New("player detail"), err)
}

func TestPlayUsesHistoryIdentifiersAndAllowsOnlyOnePendingRequest(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	service := &homeTestService{
		playRequests: make(chan backend.PlayRequest, 2),
		cancelled:    make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	home := &homeView{service: service, ctx: ctx, cancel: cancel, playMessage: widget.NewLabel("")}
	t.Cleanup(home.close)

	first := backend.History{AnimeID: "local-id", EpisodeID: "opaque:episode/id", Position: 125500}
	home.play(first)
	home.play(backend.History{AnimeID: "ignored", EpisodeID: "second", Position: 1})
	request := <-service.playRequests
	if want := (backend.PlayRequest{AnimeID: first.AnimeID, EpisodeID: first.EpisodeID, StartAt: first.Position}); request != want {
		t.Fatalf("Play request = %#v, want %#v", request, want)
	}
	if len(service.playRequests) != 0 || !home.playPending {
		t.Fatal("a second playback request was started while the first was pending")
	}
	if got := home.playMessage.Text; got != "播放中…" {
		t.Fatalf("pending message = %q, want playback status", got)
	}
	home.close()
	select {
	case <-service.cancelled:
	case <-time.After(time.Second):
		t.Fatal("closing Home did not cancel the pending Play request")
	}
}

func TestSupersededHomeReadCannotReplaceNewerResult(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	scroll := container.NewVScroll(widget.NewLabel(""))
	window := test.NewWindow(scroll)
	t.Cleanup(window.Close)
	window.Show()
	ctx, cancel := context.WithCancel(context.Background())
	home := &homeView{
		canvas: window.Canvas(), scroll: scroll, ctx: ctx, cancel: cancel,
		continueBox: container.NewVBox(), followBox: container.NewVBox(),
	}
	t.Cleanup(home.close)
	type readResult struct{ value string }
	started := make(chan chan struct{}, 2)
	finished := make(chan string, 2)
	applyResults := make(chan string, 2)
	fetch := func(value string) func(context.Context) (readResult, error) {
		return func(context.Context) (readResult, error) {
			release := make(chan struct{})
			started <- release
			<-release
			finished <- value
			return readResult{value: value}, nil
		}
	}
	apply := func(result readResult, err error) {
		if err != nil {
			t.Errorf("read error = %v", err)
			return
		}
		applyResults <- result.value
	}
	startLoad(home, historySlot, fetch("stale"), apply)
	firstRelease := <-started
	startLoad(home, historySlot, fetch("current"), apply)
	secondRelease := <-started
	close(secondRelease)
	if got := <-applyResults; got != "current" {
		t.Fatalf("new read applied %q, want current", got)
	}
	close(firstRelease)
	<-finished
	<-finished
	select {
	case got := <-applyResults:
		t.Fatalf("superseded read applied %q", got)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestFailedHistoryRefreshMovesFocusToControlAndReplacesRetryState(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	loads := make(chan string, 8)
	service := &homeTestService{loads: loads, cancelled: make(chan struct{}, 8)}
	scroll := container.NewVScroll(widget.NewLabel(""))
	window := test.NewWindow(scroll)
	t.Cleanup(window.Close)
	window.Show()
	canvas := window.Canvas()
	menu := widget.NewButton("menu", nil)
	control := widget.NewButton("content", nil)
	menu.ExtendBaseWidget(menu)
	control.ExtendBaseWidget(control)
	window.SetContent(container.NewBorder(container.NewPadded(control), nil, nil, nil, scroll))
	home := newHomeView(service, canvas, scroll, menu, control)
	scroll.Content = home.content
	scroll.Refresh()
	t.Cleanup(home.close)
	home.loadStatus = [3]loadState{ready, ready, ready}
	home.library = []backend.Anime{{ID: "anime", Title: "Title"}}
	home.history = []backend.History{{AnimeID: "anime", EpisodeID: "opaque", Position: 1000}}
	home.refresh()
	if home.firstPlay == nil {
		t.Fatal("ready history did not create a Play control")
	}
	canvas.Focus(home.firstPlay)
	home.loadStatus[historySlot] = failed
	home.refresh()
	if canvas.Focused() != control {
		t.Fatalf("focus after replacing Play control = %#v, want content control", canvas.Focused())
	}
	if len(home.continueBox.Objects) != 2 {
		t.Fatalf("failed history rows = %d, want message and retry control", len(home.continueBox.Objects))
	}
	retry, ok := home.continueBox.Objects[1].(*widget.Button)
	if !ok {
		t.Fatalf("retry control = %T, want button", home.continueBox.Objects[1])
	}
	retry.OnTapped()
	if len(home.continueBox.Objects) != 1 {
		t.Fatalf("retry rows = %d, want loading state", len(home.continueBox.Objects))
	}
	if label, ok := home.continueBox.Objects[0].(*widget.Label); !ok || label.Text != "載入中…" {
		t.Fatalf("retry state = %#v, want loading label", home.continueBox.Objects[0])
	}
	seen := map[string]int{}
	for range 5 {
		select {
		case call := <-loads:
			seen[call]++
		case <-time.After(time.Second):
			t.Fatal("retry did not request both library and history")
		}
	}
	if seen["library"] != 2 || seen["history"] != 2 || seen["following"] != 1 {
		t.Fatalf("service calls = %#v, want initial reads plus library/history retry", seen)
	}
}

func TestHomeRefreshPreservesFocusForSamePlayAction(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	service := &homeTestService{playRequests: make(chan backend.PlayRequest, 1)}
	window, home, _ := focusedHomeFixture(t, service)
	defer window.Close()
	defer home.close()

	item := backend.History{AnimeID: "anime", EpisodeID: "opaque:episode", Position: 125500}
	home.loadStatus = [3]loadState{ready, ready, loading}
	home.library = []backend.Anime{{ID: item.AnimeID, Title: "Title"}}
	home.history = []backend.History{item}
	home.refresh()
	previous := home.firstPlay
	if previous == nil {
		t.Fatal("ready history did not create a Play control")
	}
	home.canvas.Focus(previous)

	home.following = []backend.Following{{AnimeID: "followed"}}
	home.loadStatus[followingSlot] = ready
	home.refresh()

	focused, ok := home.canvas.Focused().(*playButton)
	if !ok {
		t.Fatalf("focus after Following response = %T, want Play action", home.canvas.Focused())
	}
	if focused == previous {
		t.Fatal("refresh did not replace the stale widget with the current matching Play action")
	}
	focused.OnTapped()
	select {
	case request := <-service.playRequests:
		if want := (backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: item.Position}); request != want {
			t.Fatalf("focused Play action request = %#v, want %#v", request, want)
		}
	case <-time.After(time.Second):
		t.Fatal("focused Play action did not start playback")
	}
}

func TestHomeRefreshMovesFocusToContentWhenPlayActionDisappears(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	window, home, control := focusedHomeFixture(t, &homeTestService{})
	defer window.Close()
	defer home.close()

	item := backend.History{AnimeID: "anime", EpisodeID: "opaque:episode", Position: 125500}
	home.loadStatus = [3]loadState{ready, ready, ready}
	home.library = []backend.Anime{{ID: item.AnimeID, Title: "Title"}}
	home.history = []backend.History{item}
	home.refresh()
	if home.firstPlay == nil {
		t.Fatal("ready history did not create a Play control")
	}
	home.canvas.Focus(home.firstPlay)

	home.history = nil
	home.refresh()

	if got := home.canvas.Focused(); got != control {
		t.Fatalf("focus after removing the Play action = %#v, want content control", got)
	}
}

func focusedHomeFixture(t *testing.T, service HomeService) (fyne.Window, *homeView, *widget.Button) {
	t.Helper()
	scroll := container.NewVScroll(widget.NewLabel(""))
	window := test.NewWindow(scroll)
	window.Show()
	canvas := window.Canvas()
	menu := widget.NewButton("menu", nil)
	control := widget.NewButton("content", nil)
	menu.ExtendBaseWidget(menu)
	control.ExtendBaseWidget(control)
	window.SetContent(container.NewBorder(container.NewPadded(control), nil, nil, nil, scroll))
	home := newHomeView(service, canvas, scroll, menu, control)
	scroll.Content = home.content
	scroll.Refresh()
	return window, home, control
}

func TestShellKeyboardMovesVisibleFocusAndCancelsHomeLoads(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	cancelled := make(chan struct{}, 3)
	service := &homeTestService{cancelled: cancelled}
	window := NewWindow(application, service)
	t.Cleanup(window.Close)
	window.Show()
	canvas := window.Canvas()

	beforeFocus := test.RenderToMarkup(canvas)
	focused, ok := canvas.Focused().(interface{ TypedKey(*fyne.KeyEvent) })
	if !ok {
		t.Fatalf("initial focus = %T, want keyboard focusable", canvas.Focused())
	}
	focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	navigation, ok := canvas.Focused().(*navigationButton)
	if !ok || navigation.index != 1 {
		t.Fatalf("Down focused %#v, want the second navigation button", canvas.Focused())
	}
	if afterFocus := test.RenderToMarkup(canvas); afterFocus == beforeFocus {
		t.Fatal("keyboard focus did not change the rendered navigation appearance")
	}

	navigation.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	for range 3 {
		select {
		case <-cancelled:
		case <-time.After(time.Second):
			t.Fatal("leaving Home did not cancel every pending read")
		}
	}
}
