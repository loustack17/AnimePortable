// SPDX-License-Identifier: MPL-2.0

package native

import (
	"context"
	"fmt"

	"animeportable/apps/desktop/backend"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type HomeService interface {
	Library(context.Context) ([]backend.Anime, error)
	History(context.Context) ([]backend.History, error)
	Following(context.Context) ([]backend.Following, error)
	Play(context.Context, backend.PlayRequest) error
}

type loadState uint8

const (
	loading loadState = iota
	ready
	failed
)

const (
	librarySlot = iota
	historySlot
	followingSlot
)

type homeView struct {
	service      HomeService
	canvas       fyne.Canvas
	scroll       *container.Scroll
	menu         fyne.Focusable
	control      fyne.Focusable
	content      fyne.CanvasObject
	ctx          context.Context
	cancel       context.CancelFunc
	loadCancel   [3]context.CancelFunc
	loadID       [3]uint64
	loadStatus   [3]loadState
	library      []backend.Anime
	history      []backend.History
	following    []backend.Following
	continueBox  *fyne.Container
	followBox    *fyne.Container
	firstPlay    *playButton
	playButtons  []*playButton
	playPending  bool
	playMessage  *widget.Label
	historyRetry *widget.Button
	followRetry  *widget.Button
}

func newHomeView(service HomeService, canvas fyne.Canvas, scroll *container.Scroll, menu, control fyne.Focusable) *homeView {
	ctx, cancel := context.WithCancel(context.Background())
	home := &homeView{
		service: service, canvas: canvas, scroll: scroll, menu: menu, control: control,
		ctx: ctx, cancel: cancel, continueBox: container.NewVBox(), followBox: container.NewVBox(),
		playMessage: widget.NewLabel(""),
	}
	home.playMessage.Wrapping = fyne.TextWrapWord
	home.historyRetry = widget.NewButton("重試", func() {
		home.canvas.Focus(home.control)
		home.loadLibrary()
		home.loadHistory()
	})
	home.followRetry = widget.NewButton("重試", func() {
		home.canvas.Focus(home.control)
		home.loadLibrary()
		home.loadFollowing()
	})
	home.content = container.NewPadded(container.NewVBox(
		widget.NewLabelWithStyle("首頁", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewCard("繼續觀看", "從上次停下的位置接著播放。", home.continueBox),
		home.playMessage,
		widget.NewCard("追蹤中", "快速查看你收藏的作品。", home.followBox),
		widget.NewCard("最近更新", "", widget.NewLabel("目前沒有本機更新資料。")),
		widget.NewCard("今日播出", "", widget.NewLabel("目前沒有本機播出資料。")),
	))
	home.refresh()
	home.loadLibrary()
	home.loadHistory()
	home.loadFollowing()
	return home
}

func (home *homeView) close() {
	home.cancel()
	for _, cancel := range home.loadCancel {
		if cancel != nil {
			cancel()
		}
	}
}

func startLoad[T any](home *homeView, slot int, fetch func(context.Context) (T, error), apply func(T, error)) {
	if home.loadCancel[slot] != nil {
		home.loadCancel[slot]()
	}
	home.loadID[slot]++
	id := home.loadID[slot]
	ctx, cancel := context.WithCancel(home.ctx)
	home.loadCancel[slot] = cancel
	home.loadStatus[slot] = loading
	home.refresh()
	go func() {
		result, err := fetch(ctx)
		fyne.Do(func() {
			if home.ctx.Err() != nil || ctx.Err() != nil || home.loadID[slot] != id {
				return
			}
			home.loadCancel[slot] = nil
			cancel()
			apply(result, err)
			home.refresh()
		})
	}()
}

func (home *homeView) loadLibrary() {
	startLoad(home, librarySlot, home.service.Library, func(value []backend.Anime, err error) {
		home.loadStatus[librarySlot] = stateFor(err)
		if err == nil {
			home.library = value
		}
	})
}

func (home *homeView) loadHistory() {
	startLoad(home, historySlot, home.service.History, func(value []backend.History, err error) {
		home.loadStatus[historySlot] = stateFor(err)
		if err == nil {
			home.history = value
		}
	})
}

func (home *homeView) loadFollowing() {
	startLoad(home, followingSlot, home.service.Following, func(value []backend.Following, err error) {
		home.loadStatus[followingSlot] = stateFor(err)
		if err == nil {
			home.following = value
		}
	})
}

func stateFor(err error) loadState {
	if err != nil {
		return failed
	}
	return ready
}

func (home *homeView) refresh() {
	home.refreshHistory()
	home.refreshFollowing()
}

func (home *homeView) refreshHistory() {
	if home.canvas.Focused() == home.historyRetry && home.loadStatus[historySlot] != failed && home.loadStatus[librarySlot] != failed {
		home.canvas.Focus(home.control)
	}
	var focused *playButton
	for _, button := range home.playButtons {
		if home.canvas.Focused() == button {
			focused = button
			home.canvas.Focus(home.control)
			break
		}
	}
	home.playButtons = nil
	home.firstPlay = nil
	rows := make([]fyne.CanvasObject, 0, homeLimit)
	switch {
	case home.loadStatus[historySlot] == loading || home.loadStatus[librarySlot] == loading:
		rows = append(rows, widget.NewLabel("載入中…"))
	case home.loadStatus[historySlot] == failed || home.loadStatus[librarySlot] == failed:
		rows = append(rows, widget.NewLabel("無法載入作品資料，請重試。"), home.historyRetry)
	default:
		items := latestHistory(home.history)
		if len(items) == 0 {
			rows = append(rows, widget.NewLabel("還沒有可繼續觀看的內容。"))
		}
		for _, item := range items {
			entry := item
			title := titleFor(home.library, entry.AnimeID)
			button := newPlayButton(home, entry)
			button.Button.Text = "播放 " + title
			button.Refresh()
			home.playButtons = append(home.playButtons, button)
			if home.firstPlay == nil {
				home.firstPlay = button
			}
			rows = append(rows, container.NewBorder(nil, nil, nil, button,
				container.NewVBox(widget.NewLabel(title), widget.NewLabel("上次播放位置 "+formatPosition(entry.Position))),
			))
		}
	}
	home.continueBox.Objects = rows
	home.continueBox.Refresh()
	if focused != nil {
		for _, button := range home.playButtons {
			if button.item.AnimeID == focused.item.AnimeID && button.item.EpisodeID == focused.item.EpisodeID {
				home.canvas.Focus(button)
				break
			}
		}
	}
}

func (home *homeView) refreshFollowing() {
	if home.canvas.Focused() == home.followRetry && home.loadStatus[followingSlot] != failed && home.loadStatus[librarySlot] != failed {
		home.canvas.Focus(home.control)
	}
	rows := make([]fyne.CanvasObject, 0, homeLimit)
	switch {
	case home.loadStatus[followingSlot] == loading || home.loadStatus[librarySlot] == loading:
		rows = append(rows, widget.NewLabel("載入中…"))
	case home.loadStatus[followingSlot] == failed || home.loadStatus[librarySlot] == failed:
		rows = append(rows, widget.NewLabel("無法載入追蹤清單，請重試。"), home.followRetry)
	default:
		if len(home.following) == 0 {
			rows = append(rows, widget.NewLabel("尚未追蹤任何作品。"))
		}
		for index, item := range home.following {
			if index >= homeLimit {
				break
			}
			rows = append(rows, widget.NewLabel(titleFor(home.library, item.AnimeID)))
		}
	}
	home.followBox.Objects = rows
	home.followBox.Refresh()
}

func (home *homeView) play(item backend.History) {
	if home.playPending || home.ctx.Err() != nil {
		return
	}
	home.playPending = true
	home.playMessage.SetText("播放中…")
	for _, button := range home.playButtons {
		button.Disable()
	}
	request := backend.PlayRequest{AnimeID: item.AnimeID, EpisodeID: item.EpisodeID, StartAt: item.Position}
	go func() {
		err := home.service.Play(home.ctx, request)
		fyne.Do(func() {
			if home.ctx.Err() != nil {
				return
			}
			home.playPending = false
			for _, button := range home.playButtons {
				button.Enable()
			}
			home.playMessage.SetText(playMessage(err))
		})
	}()
}

func playMessage(err error) string {
	if err == nil {
		return "播放已開始。"
	}
	if backend.ClassifyPlayerError(err) == backend.PlayerErrorMissing {
		return "找不到 MPV，請先安裝 MPV；若已安裝，請聯絡程式提供者協助設定路徑。"
	}
	if backend.ClassifyPlayerError(err) == backend.PlayerErrorInvalidPath {
		return "播放器設定無法使用，請聯絡程式提供者協助修正 MPV 路徑。"
	}
	return "無法開始播放，請重試。"
}

func formatPosition(milliseconds int64) string {
	if milliseconds < 0 {
		milliseconds = 0
	}
	seconds := milliseconds / 1000
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}
