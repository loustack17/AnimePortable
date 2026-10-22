//go:build windows && amd64 && cgo

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"animeportable/adapters/network/forwardproxy"
	"animeportable/adapters/player/libmpv"
	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/fltkengine"
	"animeportable/apps/desktop/fltkhome"
	"animeportable/apps/desktop/fltkplayer"
	"animeportable/apps/desktop/uitext"
	"animeportable/core"
	fltk "github.com/pwiecz/go-fltk"
)

func main() {
	runtime.LockOSThread()
	// FLTK only wakes the event loop for Fl::awake calls from other threads (libmpv frame
	// notifications, backend results) after the main thread has called Fl::lock once.
	// Without it those callbacks wait for an unrelated event, and video stutters.
	if !fltk.Lock() {
		fmt.Fprintln(os.Stderr, "FLTK threading support is unavailable")
		os.Exit(1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	executable, executableErr := os.Executable()
	// Registered before the service closes (deferred calls run last-in first), so the new
	// process starts only after the database is closed.
	var restartRequested atomic.Bool
	defer func() {
		if restartRequested.Load() {
			restartAfterShutdown(executable, executableErr, startAgain, showRestartFailure)
		}
	}()
	configDir, _ := os.UserConfigDir()
	plan, planErr := backend.PlanPortable(executable, configDir)
	if executableErr != nil {
		planErr = executableErr
	}
	if planErr == nil && plan.Location == backend.LocationCustomMissing {
		// Esc and closing the dialog answer 結束, so quitting never creates or moves data.
		if !fltkhome.AskMissingDataFolder(plan.CustomDir) {
			return
		}
		plan, planErr = plan.UseDefaultLocation()
	}
	var engineMu sync.Mutex
	var active *fltkengine.Engine
	var home *fltk.Window
	var navigateHome func(int)
	var play func(backend.PlayRequest)
	var player *fltkplayer.View
	var service *backend.Service
	// noticeParent is the window a message belongs over: the player while it is shown, else Home.
	// Both live as long as the process.
	noticeParent := func() *fltk.Window {
		if player != nil && player.Window().IsShown() {
			return player.Window()
		}
		return home
	}
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
	follow := &playerFollow{
		ctx:     ctx,
		start:   func(work func()) { go work() },
		awake:   func(update func()) { fltk.Awake(update) },
		playing: func() string { return selection().AnimeID },
		show: func(known, following bool) {
			state := player.State()
			state.FollowKnown, state.Following = known, following
			player.SetState(state)
		},
		fail: func(err error) {
			fltkhome.ShowNotice(noticeParent(), uitext.T().FollowFailedTitle, fltkhome.FollowErrorMessage(err))
		},
	}
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
				player.SetState(fltkplayer.State{Volume: 100, Focus: fltkplayer.FocusProgress})
				home.Show()
				if navigateHome != nil {
					navigateHome(destination)
				}
				if stopErr != nil {
					fltkhome.ShowNotice(noticeParent(), uitext.T().ProgressNotSavedTitle, uitext.T().ProgressNotSaved)
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
	// endPlayback ends the playback session and keeps the player open showing the stopped
	// episode. Awaiting is set first so engine ticks cannot mark the player playing again.
	endPlayback := func() {
		awaitingPlayback.Store(true)
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
						fltkhome.ShowNotice(noticeParent(), uitext.T().ProgressNotSavedTitle, uitext.T().ProgressNotSaved)
					}
				}
			})
		})
	}
	stopInPlayer := func() {
		if !awaitingPlayback.Load() {
			control("stop", 0)
			return
		}
		endPlayback()
	}
	// Playback settings are read once per load, here, so a change on 設定 applies to the next
	// load. A failed read keeps the last values, which start as the defaults.
	var playbackSettingsMu sync.Mutex
	playbackSettings := backend.DefaultSettings()
	// speedUnsaved means the chosen speed could not be stored: the session keeps it over the
	// stored value, and the user is told once.
	speedUnsaved, speedNoticeShown := false, false
	loadPlaybackSettings := func(loadCtx context.Context) {
		settings, err := service.Settings(loadCtx)
		if err != nil {
			return
		}
		playbackSettingsMu.Lock()
		if speedUnsaved {
			settings.PlaybackSpeed = playbackSettings.PlaybackSpeed
		}
		playbackSettings = settings
		playbackSettingsMu.Unlock()
		fltk.Awake(func() { player.SetSpeed(settings.PlaybackSpeed) })
		engineMu.Lock()
		current := active
		engineMu.Unlock()
		if current != nil {
			current.SetPlaybackSpeed(settings.PlaybackSpeed)
		}
	}
	var autoplay *playerAutoplay
	// selectEpisode switches the player to episodes[index]; paused makes it wait at its start
	// position (only autoplay asks for that). failed, when set, replaces the message shown
	// for a failed switch.
	selectEpisode := func(index int, paused bool, failed func()) {
		if index < 0 || index >= len(episodes) {
			return
		}
		generation := viewGeneration.Add(1)
		operationCtx, cancelOperation := newPlaybackContext()
		previousGeneration := expectedPlaybackGeneration.Swap(0)
		awaitingPlayback.Store(true)
		loading := player.State()
		loading.Loading = true
		loading.Position = 0
		loading.Duration = 0
		// A list click already set this; an autoplay switch relies on it to show the new episode.
		loading.Episode = index
		player.SetState(loading)
		request := selection()
		request.EpisodeID = episodes[index].ID
		request.StartAt = 0
		request.Paused = paused
		playQueue.Enqueue(func() {
			defer cancelOperation()
			if generation != viewGeneration.Load() {
				return
			}
			actionGeneration.Store(generation)
			loadPlaybackSettings(operationCtx)
			if err := service.Play(operationCtx, request); err != nil {
				if generation == viewGeneration.Load() {
					// A failed switch leaves the previous file playing; keep hearing its end and
					// failures unless a load already replaced it.
					expectedPlaybackGeneration.CompareAndSwap(0, previousGeneration)
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
						if failed != nil {
							failed()
							return
						}
						fltkhome.ShowNotice(noticeParent(), uitext.T().PlaybackFailedTitle, uitext.T().PlayEpisodeFailed)
					}
				})
				return
			}
			if generation == viewGeneration.Load() {
				// Before awaiting clears: autoplay must never read the previous episode. The
				// selection names the episode, not how it was loaded, so a replay plays.
				request.Paused = false
				setSelection(request)
				awaitingPlayback.Store(false)
			}
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
		SetSpeed: func(percent int) {
			playbackSettingsMu.Lock()
			playbackSettings.PlaybackSpeed = percent
			playbackSettingsMu.Unlock()
			// Queued behind loads, so the engine and the stored value change in the order chosen.
			// The engine is read here, not when the speed was chosen: a load queued earlier may
			// have created it since, at the speed stored before this choice.
			playQueue.Enqueue(func() {
				engineMu.Lock()
				current := active
				engineMu.Unlock()
				if current != nil {
					_ = current.Control(ctx, "speed", percent)
				}
				err := service.SavePlaybackSpeed(ctx, percent)
				playbackSettingsMu.Lock()
				speedUnsaved = err != nil
				// An earlier queued load may have replaced the chosen speed with the stored one.
				playbackSettings.PlaybackSpeed = percent
				fltk.Awake(func() { player.SetSpeed(percent) })
				notify := err != nil && !speedNoticeShown
				speedNoticeShown = speedNoticeShown || notify
				playbackSettingsMu.Unlock()
				if notify {
					fltk.Awake(func() {
						fltkhome.ShowNotice(noticeParent(), uitext.T().SpeedTitle, uitext.T().SpeedSaveFailed)
					})
				}
			})
		},
		SelectEpisode: func(index int) { selectEpisode(index, false, nil) },
		Navigate:      stop,
		Leave:         func() { stop(0) },
		Search:        func() { stop(fltkhome.SearchSection) },
		ToggleFollow: func() {
			if animeID := selection().AnimeID; animeID != "" {
				follow.toggle(animeID, player.State().Following)
			}
		},
	})
	player.SetSpeed(playbackSettings.PlaybackSpeed)
	player.Window().SetCallback(func() { stop(0) })
	// The failure log sits next to the database.
	var failures *backend.FailureLog
	if plan.DatabasePath != "" {
		failures = backend.NewFailureLog(filepath.Dir(plan.DatabasePath))
	}
	// One source for every client that needs the forward proxy; the service keeps it in step
	// with the saved settings.
	proxySource := forwardproxy.New()
	service = backend.NewAtWithPlan(plan, proxySource, failures, func() (core.Player, error) {
		return libmpv.NewPlayer(func(factoryCtx context.Context) (libmpv.Bridge, libmpv.Renderer, error) {
			playbackSettingsMu.Lock()
			hardwareDecoding := playbackSettings.HardwareDecoding == "enabled"
			speed := playbackSettings.PlaybackSpeed
			playbackSettingsMu.Unlock()
			created, err := fltkengine.New(factoryCtx, player.Video(), fltkengine.Options{HardwareDecoding: hardwareDecoding})
			if err != nil {
				return nil, nil, err
			}
			created.SetPlaybackSpeed(speed)
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
				go failures.PlayerFailed() // off the UI thread: the log writes to disk
				fltkhome.ShowNotice(noticeParent(), uitext.T().PlaybackFailedTitle, uitext.T().EpisodeUnavailable)
			})
			created.SetEndHandler(func(generation uint64) {
				if ctx.Err() != nil {
					return
				}
				engineMu.Lock()
				current := active == created
				engineMu.Unlock()
				if !current || generation != expectedPlaybackGeneration.Load() || awaitingPlayback.Load() {
					return
				}
				autoplay.ended(generation)
			})
			engineMu.Lock()
			active = created
			engineMu.Unlock()
			fltk.Awake(func() { player.SetRenderHook(func(width, height int) { _ = created.Render(width, height) }) })
			return created, created, nil
		}, failures.UpstreamRejected, proxySource.Address), nil
	})
	defer func() { _ = service.Close(); player.Close() }()
	follow.service = service
	autoplay = &playerAutoplay{
		ctx:      ctx,
		settings: service,
		start:    func(work func()) { go work() },
		awake:    func(update func()) { fltk.Awake(update) },
		still: func(generation uint64) bool {
			return ctx.Err() == nil && generation == expectedPlaybackGeneration.Load() && !awaitingPlayback.Load()
		},
		view:     viewGeneration.Load,
		episodes: func() []backend.Episode { return episodes },
		playing:  func() string { return selection().EpisodeID },
		advance:  selectEpisode,
		finish:   endPlayback,
		fail: func() {
			fltkhome.ShowNotice(noticeParent(), uitext.T().PlaybackFailedTitle, uitext.T().NextEpisodeFailed)
		},
	}
	play = func(request backend.PlayRequest) {
		generation := viewGeneration.Add(1)
		operationCtx, cancelOperation := newPlaybackContext()
		expectedPlaybackGeneration.Store(0)
		awaitingPlayback.Store(true)
		setSelection(request)
		episodes = nil
		player.SetEpisodes(nil, 0)
		player.SetState(fltkplayer.State{Volume: 100, Focus: fltkplayer.FocusProgress, Loading: true})
		player.Show()
		player.SetFocus(fltkplayer.FocusProgress)
		home.Hide()
		follow.load(request.AnimeID)
		playQueue.Enqueue(func() {
			if generation != viewGeneration.Load() {
				cancelOperation()
				return
			}
			actionGeneration.Store(generation)
			err := startPlayback(operationCtx, cancelOperation, func(playCtx context.Context) error {
				loadPlaybackSettings(playCtx)
				if err := service.Play(playCtx, request); err != nil {
					return err
				}
				if generation == viewGeneration.Load() {
					setSelection(request)
					awaitingPlayback.Store(false)
				}
				return nil
			}, func(episodeCtx context.Context) {
				items, err := service.Episodes(episodeCtx, request.AnimeID)
				if err != nil || episodeCtx.Err() != nil {
					return
				}
				labels := make([]string, len(items))
				current := 0
				for index, item := range items {
					labels[index] = fltkhome.EpisodeLabel(item.Number)
					// The overlay splits labels on newlines, so a title keeps to one line.
					if title := strings.Join(strings.Fields(item.Title), " "); title != "" {
						labels[index] += " · " + title
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
			})
			if err != nil {
				if generation == viewGeneration.Load() {
					awaitingPlayback.Store(false)
				}
				fltk.Awake(func() {
					if generation != viewGeneration.Load() {
						return
					}
					player.Window().Hide()
					home.Show()
					// Shown after Home so a late Home reload cannot clear it.
					fltkhome.ShowNotice(noticeParent(), uitext.T().PlaybackFailedTitle, uitext.T().PlayEpisodeFailed)
				})
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
	home, navigateHome = fltkhome.NewWindow(plan, planErr, service, ctx, cancel, play, closeApp, func() { restartRequested.Store(true) })
	home.Show()
	fltk.Run()
	cancel()
}
