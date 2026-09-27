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
	playQueue := newActionQueue()
	stop := func(destination int) {
		generation := viewGeneration.Add(1)
		playQueue.Enqueue(func() {
			_ = service.StopPlayback(context.Background())
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
			})
		})
	}
	control := func(command string, value int) {
		engineMu.Lock()
		current := active
		engineMu.Unlock()
		if current != nil {
			go func() { _ = current.Control(ctx, command, value) }()
		}
	}
	player = fltkplayer.NewWindow(fltkplayer.Callbacks{
		PlayPause: func() { control("pause", 0) },
		Seek:      func(seconds int) { control("seek", seconds) },
		Stop:      func() { stop(0) },
		Volume:    func(percent int) { control("volume", percent) },
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
			request := selection()
			request.EpisodeID = episodes[index].ID
			request.StartAt = 0
			playQueue.Enqueue(func() {
				if generation != viewGeneration.Load() {
					return
				}
				if err := service.Play(ctx, request); err != nil {
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
							player.SetState(state)
							fltk.MessageBox("播放失敗", "無法播放此集，請重試。")
						}
					})
					return
				}
				setSelection(request)
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
				state := player.State()
				state.Playing = true
				state.Paused = paused
				state.Position = position.Seconds()
				state.Duration = duration.Seconds()
				state.Resolution = resolution
				player.SetState(state)
			})
			engineMu.Lock()
			active = created
			engineMu.Unlock()
			fltk.Awake(func() { player.SetRenderHook(func(width, height int) { _ = created.Render(width, height) }) })
			return created, created, nil
		}), nil
	})
	defer func() { _ = service.Close(); player.Close() }()
	play := func(request backend.PlayRequest) {
		generation := viewGeneration.Add(1)
		setSelection(request)
		episodes = nil
		player.SetEpisodes(nil, 0)
		player.SetState(fltkplayer.State{Volume: 100, Focus: 1})
		player.Window().Show()
		player.SetFocus(1)
		home.Hide()
		playQueue.Enqueue(func() {
			if generation != viewGeneration.Load() {
				return
			}
			items, err := service.Episodes(ctx, request.AnimeID)
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
			if err := service.Play(ctx, request); err != nil {
				fltk.Awake(func() {
					if generation != viewGeneration.Load() {
						return
					}
					notifyHome("無法播放此集，請重試。")
					player.Window().Hide()
					home.Show()
				})
			} else {
				setSelection(request)
			}
		})
	}
	var shutdown sync.Once
	closeApp := func() {
		shutdown.Do(func() {
			viewGeneration.Add(1)
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
