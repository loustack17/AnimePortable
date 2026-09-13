// SPDX-License-Identifier: MPL-2.0

import type { CancellablePromise } from '@wailsio/runtime'
import type * as service from '../bindings/animeportable/apps/desktop/backend/service.js'
import type { Anime, Following, History, PlayRequest } from '../bindings/animeportable/apps/desktop/backend/models.js'

export type HomeBinding = Pick<typeof service, 'Library' | 'History' | 'Following' | 'Play'>
export type LoadName = 'library' | 'history' | 'following'
export type LoadStatus = 'loading' | 'ready' | 'error'
export type PlayStatus = 'idle' | 'playing' | 'success' | 'error'

export type HomeState = {
  library: Anime[]
  history: History[]
  following: Following[]
  loadStatus: Record<LoadName, LoadStatus>
  loadErrors: Record<LoadName, string>
  playStatus: PlayStatus
  playEpisodeId: string
  playMessage: string
}

export const homeLimit = 6
export const serviceErrorMessage = '桌面服務目前無法使用，請稍後重試。'

const loadErrorMessages: Record<LoadName, string> = {
  library: '無法載入作品資料，請重試。',
  history: '無法載入觀看紀錄，請重試。',
  following: '無法載入追蹤清單，請重試。',
}

export function initialHomeState(): HomeState {
  return {
    library: [],
    history: [],
    following: [],
    loadStatus: { library: 'loading', history: 'loading', following: 'loading' },
    loadErrors: { library: '', history: '', following: '' },
    playStatus: 'idle',
    playEpisodeId: '',
    playMessage: '',
  }
}

type Timestamp = { base: number; fraction: string }

type LoadValue = {
  library: Anime[]
  history: History[]
  following: Following[]
}

function timestamp(value: string): Timestamp | undefined {
  const match = /^(.*?)(?:\.(\d+))((?:Z|[+-]\d\d:?\d\d))$/.exec(value)
  const baseValue = match ? `${match[1]}${match[3]}` : value
  const base = Date.parse(baseValue)
  if (Number.isNaN(base)) return undefined
  return { base, fraction: match?.[2] ?? '' }
}

function compareFraction(left: string, right: string): number {
  const length = Math.max(left.length, right.length)
  const a = left.padEnd(length, '0')
  const b = right.padEnd(length, '0')
  return a === b ? 0 : a > b ? 1 : -1
}

function compareNewest(left: History, right: History): number {
  const leftTime = timestamp(left.lastPlayed)
  const rightTime = timestamp(right.lastPlayed)
  if (leftTime && rightTime) {
    if (leftTime.base !== rightTime.base) return rightTime.base - leftTime.base
    const fraction = compareFraction(rightTime.fraction, leftTime.fraction)
    if (fraction !== 0) return fraction
  } else if (leftTime) {
    return -1
  } else if (rightTime) {
    return 1
  } else {
    const lexical = right.lastPlayed.localeCompare(left.lastPlayed)
    if (lexical !== 0) return lexical
  }
  const episode = right.episodeId.localeCompare(left.episodeId)
  return episode !== 0 ? episode : right.animeId.localeCompare(left.animeId)
}

function isResumable(item: History): boolean {
  return !item.completed && item.position > 0 && (item.duration <= 0 || item.position < item.duration)
}

export function latestHistory(items: History[]): History[] {
  const newestByAnime = new Map<string, History>()
  for (const item of items) {
    const previous = newestByAnime.get(item.animeId)
    if (!previous || compareNewest(item, previous) < 0) newestByAnime.set(item.animeId, item)
  }
  return [...newestByAnime.values()].filter(isResumable).sort(compareNewest).slice(0, homeLimit)
}

export function titleFor(library: Anime[], animeId: string): string {
  const title = library.find((item) => item.id === animeId)?.title.trim()
  return title || '無法取得作品名稱'
}

export function formatPosition(milliseconds: number): string {
  const totalSeconds = Math.max(0, Math.floor(milliseconds / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}

type PendingRequest = {
  id: number
  cancel: () => void | PromiseLike<void>
}

export type HomeController = {
  loadAll(): void
  retry(name: LoadName): void
  play(item: History): void
  dispose(): void
}

export function createHomeController(binding: HomeBinding, onChange: (state: HomeState) => void): HomeController {
  let state = initialHomeState()
  let disposed = false
  let sequence = 0
  const requests = new Map<LoadName, PendingRequest>()
  const loadIds = new Map<LoadName, number>()
  let pendingPlay: PendingRequest | undefined

  function publish(): void {
    onChange(state)
  }

  function cancelRequest(request: PendingRequest | undefined): void {
    if (request) void request.cancel()
  }

  function startLoad<K extends LoadName>(name: K, request: () => CancellablePromise<LoadValue[K]>): void {
    const previous = requests.get(name)
    const id = ++sequence
    loadIds.set(name, id)
    requests.delete(name)
    cancelRequest(previous)
    if (disposed) return

    state = {
      ...state,
      loadStatus: { ...state.loadStatus, [name]: 'loading' },
      loadErrors: { ...state.loadErrors, [name]: '' },
    }
    publish()

    let operation: CancellablePromise<LoadValue[K]>
    try {
      operation = request()
    } catch {
      failLoad(name, id)
      return
    }
    const pending: PendingRequest = { id, cancel: () => operation.cancel() }
    requests.set(name, pending)
    operation.then(
      (value) => completeLoad(name, id, value),
      () => failLoad(name, id),
    )
  }

  function completeLoad<K extends LoadName>(name: K, id: number, value: LoadValue[K]): void {
    const pending = requests.get(name)
    if (disposed || loadIds.get(name) !== id || !pending || pending.id !== id) return
    requests.delete(name)
    state = {
      ...state,
      [name]: value,
      loadStatus: { ...state.loadStatus, [name]: 'ready' },
      loadErrors: { ...state.loadErrors, [name]: '' },
    }
    publish()
  }

  function failLoad(name: LoadName, id: number): void {
    const pending = requests.get(name)
    if (disposed || loadIds.get(name) !== id || (pending && pending.id !== id)) return
    requests.delete(name)
    state = {
      ...state,
      loadStatus: { ...state.loadStatus, [name]: 'error' },
      loadErrors: { ...state.loadErrors, [name]: loadErrorMessages[name] },
    }
    publish()
  }

  function loadAll(): void {
    startLoad('library', binding.Library)
    startLoad('history', binding.History)
    startLoad('following', binding.Following)
  }

  function retry(name: LoadName): void {
    if (name === 'library') startLoad(name, binding.Library)
    if (name === 'history') startLoad(name, binding.History)
    if (name === 'following') startLoad(name, binding.Following)
  }

  function play(item: History): void {
    if (disposed || pendingPlay) return
    const request: PlayRequest = { animeId: item.animeId, episodeId: item.episodeId, startAt: item.position }
    const id = ++sequence
    state = { ...state, playStatus: 'playing', playEpisodeId: item.episodeId, playMessage: '' }
    publish()
    let operation: CancellablePromise<void>
    try {
      operation = binding.Play(request)
    } catch {
      failPlay(id)
      return
    }
    pendingPlay = { id, cancel: () => operation.cancel() }
    operation.then(
      () => completePlay(id),
      () => failPlay(id),
    )
  }

  function completePlay(id: number): void {
    if (disposed || !pendingPlay || pendingPlay.id !== id) return
    pendingPlay = undefined
    state = { ...state, playStatus: 'success', playMessage: '播放已開始。' }
    publish()
  }

  function failPlay(id: number): void {
    if (disposed || (pendingPlay && pendingPlay.id !== id)) return
    pendingPlay = undefined
    state = { ...state, playStatus: 'error', playMessage: '無法開始播放，請重試。' }
    publish()
  }

  function dispose(): void {
    if (disposed) return
    disposed = true
    sequence += 1
    const active = [...requests.values()]
    requests.clear()
    const activePlay = pendingPlay
    pendingPlay = undefined
    for (const request of active) cancelRequest(request)
    cancelRequest(activePlay)
  }

  return { loadAll, retry, play, dispose }
}
