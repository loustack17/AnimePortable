//go:build windows && amd64 && cgo

package main

import (
	"context"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"animeportable/adapters/player/libmpv"
	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/fltkengine"
	"animeportable/apps/desktop/fltkhome"
	"animeportable/apps/desktop/fltkplayer"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

func main() {
	runtime.LockOSThread()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executable, executableErr := os.Executable()
	configDir, _ := os.UserConfigDir()
	plan, planErr := backend.PlanPortable(executable, configDir)
	if executableErr != nil {
		planErr = executableErr
	}
	var engineMu sync.Mutex
	var active *fltkengine.Engine
	var home *fltk.Window
	var navigateHome func(int)
	var notifyHome func(string)
	var play func(backend.PlayRequest)
	var player *fltkplayer.View
	var service *backend.Service
	var selectedMu sync.Mutex
	var selected backend.PlayRequest
	selection := func() backend.PlayRequest {
		selectedMu.Lock()
		defer selectedMu.Unlock()
		return selected
	}
	setSelection := func(request backend.PlayRequest) {
		selectedMu.Lock()
		selected = request
		selectedMu.Unlock()
	}
	var episodes []backend.Episode
	var viewGeneration atomic.Uint64
	var actionGeneration atomic.Uint64
	var expectedPlaybackGeneration atomic.Uint64
	var awaitingPlayback atomic.Bool
	var playbackContextMu sync.Mutex
	var playbackCancel context.CancelFunc
	newPlaybackContext := func() (context.Context, context.CancelFunc) {
		playbackContextMu.Lock()
		defer playbackContextMu.Unlock()
		if playbackCancel != nil {
			playbackCancel()
		}
		operationCtx, cancelOperation := context.WithCancel(ctx)
		playbackCancel = cancelOperation
		return operationCtx, cancelOperation
	}
	cancelPlayback := func() {
		playbackContextMu.Lock()
		if playbackCancel != nil {
			playbackCancel()
		}
		playbackContextMu.Unlock()
	}
	playQueue := newActionQueue()
	stop := func(destination int) {
		generation := viewGeneration.Add(1)
		cancelPlayback()
		expectedPlaybackGeneration.Store(0)
		awaitingPlayback.Store(false)
		playQueue.Enqueue(func() {
			stopErr := service.StopPlayback(context.Background())
			engineMu.Lock()
			active = nil
			engineMu.Unlock()
			fltk.Awake(func() {
				if generation != viewGeneration.Load() {
					return
				}
				player.SetRenderHook(nil)
				player.Window().Hide()
				player.SetState(fltkplayer.State{Volume: 100, Focus: 1})
				home.Show()
				if navigateHome != nil {
					navigateHome(destination)
				}
				if stopErr != nil {
					fltk.MessageBox("播放進度確認失敗", "無法確認播放進度已儲存，繼續播放時可能回到先前的位置。")
				}
			})
		})
	}
	control := func(command string, value int) {
		engineMu.Lock()
		current := active
		engineMu.Unlock()
		if current != nil {
			playQueue.Enqueue(func() { _ = current.Control(ctx, command, value) })
		}
	}
	stopInPlayer := func() {
		if !awaitingPlayback.Load() {
			control("stop", 0)
			return
		}
		generation := viewGeneration.Add(1)
		cancelPlayback()
		expectedPlaybackGeneration.Store(0)
		engineMu.Lock()
		current := active
		engineMu.Unlock()
		if current != nil {
			go func() { _ = current.Control(ctx, "stop", 0) }()
		}
		state := player.State()
		state.Playing = false
		state.Paused = true
		state.Loading = false
		state.Position = 0
		state.Duration = 0
		for index, episode := range episodes {
			if episode.ID == selection().EpisodeID {
				state.Episode = index
				break
			}
		}
		player.SetState(state)
		playQueue.Enqueue(func() {
			stopErr := service.StopPlayback(context.Background())
			if generation == viewGeneration.Load() {
				awaitingPlayback.Store(false)
			}
			engineMu.Lock()
			active = nil
			engineMu.Unlock()
			fltk.Awake(func() {
				if generation == viewGeneration.Load() {
					player.SetRenderHook(nil)
					if stopErr != nil {
						fltk.MessageBox("播放進度確認失敗", "無法確認播放進度已儲存，繼續播放時可能回到先前的位置。")
					}
				}
			})
		})
	}
	player = fltkplayer.NewWindow(fltkplayer.Callbacks{
		PlayPause: func() {
			state := player.State()
			if state.Loading {
				return
			}
			engineMu.Lock()
			current := active
			engineMu.Unlock()
			if current == nil || !state.Playing {
				request := selection()
				request.StartAt = 0
				if play != nil && request.AnimeID != "" && request.EpisodeID != "" {
					play(request)
				}
				return
			}
			control("pause", 0)
		},
		Seek:   func(seconds int) { control("seek", seconds) },
		SeekTo: func(seconds int) { control("seek_absolute", seconds) },
		Stop:   stopInPlayer,
		Volume: func(percent int) { control("volume", percent) },
		Fullscreen: func() {
			window := player.Window()
			fullscreen := !window.FullscreenActive()
			window.SetFullscreen(fullscreen)
			state := player.State()
			state.Fullscreen = fullscreen
			player.SetState(state)
		},
		SelectEpisode: func(index int) {
			if index < 0 || index >= len(episodes) {
				return
			}
			generation := viewGeneration.Add(1)
			operationCtx, cancelOperation := newPlaybackContext()
			expectedPlaybackGeneration.Store(0)
			awaitingPlayback.Store(true)
			loading := player.State()
			loading.Loading = true
			loading.Position = 0
			loading.Duration = 0
			player.SetState(loading)
			request := selection()
			request.EpisodeID = episodes[index].ID
			request.StartAt = 0
			playQueue.Enqueue(func() {
				defer cancelOperation()
				if generation != viewGeneration.Load() {
					return
				}
				actionGeneration.Store(generation)
				if err := service.Play(operationCtx, request); err != nil {
					if generation == viewGeneration.Load() {
						awaitingPlayback.Store(false)
					}
					fltk.Awake(func() {
						if generation == viewGeneration.Load() {
							previous := 0
							playing := selection()
							for current, episode := range episodes {
								if episode.ID == playing.EpisodeID {
									previous = current
									break
								}
							}
							state := player.State()
							state.Episode = previous
							state.Loading = false
							player.SetState(state)
							fltk.MessageBox("播放失敗", "無法播放此集，請重試。")
						}
					})
					return
				}
				if generation == viewGeneration.Load() {
					awaitingPlayback.Store(false)
					setSelection(request)
				}
			})
		},
		Navigate: stop,
	})
	player.Window().SetCallback(func() { stop(0) })
	service = backend.NewAtWithPlayerFactory(plan.DatabasePath, func() (core.Player, error) {
		return libmpv.NewPlayer(func(factoryCtx context.Context) (libmpv.Bridge, libmpv.Renderer, error) {
			created, err := fltkengine.New(factoryCtx, player.Video())
			if err != nil {
				return nil, nil, err
			}
			created.SetStateHandler(func(position, duration time.Duration, paused bool, resolution int) {
				if awaitingPlayback.Load() {
					return
				}
				state := player.State()
				state.Playing = true
				state.Loading = false
				state.Paused = paused
				state.Position = position.Seconds()
				state.Duration = duration.Seconds()
				state.Resolution = resolution
				player.SetState(state)
			})
			created.SetLoadHandler(func(generation uint64) {
				if actionGeneration.Load() == viewGeneration.Load() {
					expectedPlaybackGeneration.Store(generation)
				}
			})
			created.SetFailureHandler(func(generation uint64) {
				if ctx.Err() != nil {
					return
				}
				engineMu.Lock()
				current := active == created
				engineMu.Unlock()
				if !current || generation != expectedPlaybackGeneration.Load() || !created.FailedGeneration(generation) {
					return
				}
				state := player.State()
				state.Playing = false
				state.Loading = false
				player.SetState(state)
				fltk.MessageBox("播放失敗", "此集暫時無法播放。請重新選取該集或稍後再試。")
			})
			engineMu.Lock()
			active = created
			engineMu.Unlock()
			fltk.Awake(func() { player.SetRenderHook(func(width, height int) { _ = created.Render(width, height) }) })
			return created, created, nil
		}), nil
	})
	defer func() { _ = service.Close(); player.Close() }()
	play = func(request backend.PlayRequest) {
		generation := viewGeneration.Add(1)
		operationCtx, cancelOperation := newPlaybackContext()
		expectedPlaybackGeneration.Store(0)
		awaitingPlayback.Store(true)
		setSelection(request)
		episodes = nil
		player.SetEpisodes(nil, 0)
		player.SetState(fltkplayer.State{Volume: 100, Focus: 1, Loading: true})
		player.Window().Show()
		player.SetFocus(1)
		home.Hide()
		playQueue.Enqueue(func() {
			defer cancelOperation()
			if generation != viewGeneration.Load() {
				return
			}
			actionGeneration.Store(generation)
			items, err := service.Episodes(operationCtx, request.AnimeID)
			if err == nil {
				labels := make([]string, len(items))
				current := 0
				for index, item := range items {
					labels[index] = item.Number
					if item.Title != "" {
						labels[index] += " · " + item.Title
					}
					if item.ID == request.EpisodeID {
						current = index
					}
				}
				fltk.Awake(func() {
					if generation == viewGeneration.Load() {
						episodes = items
						player.SetEpisodes(labels, current)
					}
				})
			}
			if generation != viewGeneration.Load() {
				return
			}
			if err := service.Play(operationCtx, request); err != nil {
				if generation == viewGeneration.Load() {
					awaitingPlayback.Store(false)
				}
				fltk.Awake(func() {
					if generation != viewGeneration.Load() {
						return
					}
					notifyHome("無法播放此集，請重試。")
					player.Window().Hide()
					home.Show()
				})
			} else {
				if generation == viewGeneration.Load() {
					awaitingPlayback.Store(false)
					setSelection(request)
				}
			}
		})
	}
	var shutdown sync.Once
	closeApp := func() {
		shutdown.Do(func() {
			viewGeneration.Add(1)
			cancelPlayback()
			pending := playQueue.Drain()
			cancel()
			go func() {
				<-pending
				_ = service.Close()
				fltk.Awake(func() { player.Window().Hide(); home.Hide() })
			}()
		})
	}
	home, navigateHome, notifyHome = fltkhome.NewWindow(plan, planErr, service, ctx, cancel, play, closeApp)
	home.Show()
	fltk.Run()
	cancel()
}
