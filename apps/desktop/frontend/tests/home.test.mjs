// SPDX-License-Identifier: MPL-2.0

import assert from 'node:assert/strict'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { join } from 'node:path'
import { test } from 'node:test'
import { CancellablePromise } from '@wailsio/runtime'
import { transpileModule, ModuleKind, ScriptTarget } from 'typescript'

async function loadHomeModule() {
  const source = await readFile(new URL('../src/home.ts', import.meta.url), 'utf8')
  const compiled = transpileModule(source, {
    compilerOptions: { module: ModuleKind.ESNext, target: ScriptTarget.ESNext },
    fileName: 'home.ts',
  })
  const directory = await mkdtemp(fileURLToPath(new URL('../node_modules/.home-test-', import.meta.url)))
  const modulePath = join(directory, 'home.mjs')
  await writeFile(modulePath, compiled.outputText)
  return { module: await import(pathToFileURL(modulePath).href), directory }
}

function deferred() {
  let resolve
  let reject
  let cancellations = 0
  const promise = new CancellablePromise((nextResolve, nextReject) => {
    resolve = nextResolve
    reject = nextReject
  }, () => {
    cancellations += 1
  })
  return {
    promise,
    resolve,
    reject,
    get cancellations() {
      return cancellations
    },
  }
}

function fulfilled(value) {
  return new CancellablePromise((resolve) => resolve(value))
}

function history(animeId, episodeId, lastPlayed, overrides = {}) {
  return { animeId, episodeId, position: 1000, duration: 10000, completed: false, updatedAt: lastPlayed, lastPlayed, ...overrides }
}

async function settle() {
  await new Promise((resolve) => setImmediate(resolve))
}

test('latestHistory selects newest records before filtering and orders fractional timestamps', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const items = [
      history('finished', 'old', '2026-09-12T10:00:00Z'),
      history('finished', 'new', '2026-09-12T11:00:00Z', { completed: true }),
      history('fraction', 'low', '2026-09-12T12:00:00.9Z'),
      history('fraction', 'high', '2026-09-12T12:00:00.910Z'),
      history('zero-duration', 'episode', '2026-09-12T09:00:00Z', { duration: 0 }),
      ...Array.from({ length: 7 }, (_, index) => history(`extra-${index}`, `episode-${index}`, `2026-09-11T00:00:0${index}Z`)),
    ]
    const result = module.latestHistory(items)
    assert.equal(result.length, 6)
    assert.equal(result[0].episodeId, 'high')
    assert.ok(!result.some((item) => item.animeId === 'finished'))
    assert.ok(result.some((item) => item.animeId === 'zero-duration'))
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('controller cancels superseded requests and ignores stale late results', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const first = deferred()
    const second = deferred()
    const updates = []
    const binding = {
      Library: () => updates.length < 2 ? first.promise : second.promise,
      History: () => fulfilled([]),
      Following: () => fulfilled([]),
      Play: () => fulfilled(undefined),
    }
    const controller = module.createHomeController(binding, (state) => updates.push(state))
    controller.loadAll()
    controller.retry('library')
    assert.equal(first.cancellations, 1)
    first.resolve([{ id: 'stale', title: 'Stale', nativeTitle: '', description: '' }])
    await settle()
    assert.equal(updates.at(-1).loadStatus.library, 'loading')
    second.resolve([{ id: 'current', title: 'Current', nativeTitle: '', description: '' }])
    await settle()
    assert.equal(updates.at(-1).loadStatus.library, 'ready')
    assert.equal(updates.at(-1).library[0].id, 'current')
    controller.dispose()
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('controller cancellation disposes all pending reads and Play without exposing raw errors', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const library = deferred()
    const historyRead = deferred()
    const following = deferred()
    const play = deferred()
    const updates = []
    const binding = {
      Library: () => library.promise,
      History: () => historyRead.promise,
      Following: () => following.promise,
      Play: () => play.promise,
    }
    const controller = module.createHomeController(binding, (state) => updates.push(state))
    controller.loadAll()
    controller.play(history('anime', 'episode', '2026-09-12T10:00:00Z', { position: 3210 }))
    controller.play(history('anime-two', 'other', '2026-09-12T10:01:00Z'))
    assert.equal(updates.at(-1).playStatus, 'playing')
    controller.dispose()
    assert.equal(library.cancellations, 1)
    assert.equal(historyRead.cancellations, 1)
    assert.equal(following.cancellations, 1)
    assert.equal(play.cancellations, 1)
    library.resolve([{ id: 'late', title: 'Late', nativeTitle: '', description: '' }])
    historyRead.resolve([])
    following.resolve([])
    play.resolve(undefined)
    await settle()
    assert.equal(updates.at(-1).playStatus, 'playing')
    assert.ok(!JSON.stringify(updates).includes('backend secret'))
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('controller converts synchronous binding failures to recoverable generic errors', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const updates = []
    const binding = {
      Library: () => { throw new Error('database secret') },
      History: () => fulfilled([]),
      Following: () => fulfilled([]),
      Play: () => fulfilled(undefined),
    }
    const controller = module.createHomeController(binding, (state) => updates.push(state))
    controller.loadAll()
    assert.equal(updates.at(-1).loadStatus.library, 'error')
    assert.equal(updates.at(-1).loadErrors.library, '無法載入作品資料，請重試。')
    assert.ok(!JSON.stringify(updates).includes('database secret'))
    controller.dispose()
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('play maps safe player errors and preserves success', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const cases = [
      ['mpv: executable not found; install mpv or configure its path', '找不到播放所需的播放器，請聯絡程式提供者協助安裝。'],
      ['mpv: configured executable path is invalid', '播放器設定無法使用，請聯絡程式提供者協助修正。'],
      ['player secret C:/private/mpv.exe', '無法開始播放，請重試。'],
    ]
    for (const [errorMessage, expected] of cases) {
      const updates = []
      const binding = {
        Library: () => fulfilled([]),
        History: () => fulfilled([]),
        Following: () => fulfilled([]),
        Play: () => { throw new Error(errorMessage) },
      }
      const controller = module.createHomeController(binding, (state) => updates.push(state))
      controller.play(history('anime', 'episode', '2026-09-12T10:00:00Z'))
      assert.equal(updates.at(-1).playMessage, expected)
      assert.ok(!updates.at(-1).playMessage.includes('private'))
      controller.dispose()
    }

    const updates = []
    const binding = {
      Library: () => fulfilled([]),
      History: () => fulfilled([]),
      Following: () => fulfilled([]),
      Play: () => fulfilled(undefined),
    }
    const controller = module.createHomeController(binding, (state) => updates.push(state))
    controller.play(history('anime', 'episode', '2026-09-12T10:00:00Z'))
    await settle()
    assert.equal(updates.at(-1).playStatus, 'success')
    assert.equal(updates.at(-1).playMessage, '播放已開始。')
    controller.dispose()
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})

test('play maps rejected player errors and keeps unknown values generic', async () => {
  const { module, directory } = await loadHomeModule()
  try {
    const cases = [
      [new Error('mpv: executable not found; install mpv or configure its path'), '找不到播放所需的播放器，請聯絡程式提供者協助安裝。'],
      [new Error('mpv: configured executable path is invalid'), '播放器設定無法使用，請聯絡程式提供者協助修正。'],
      [new Error('backend secret'), '無法開始播放，請重試。'],
      ['mpv: executable not found; install mpv or configure its path', '無法開始播放，請重試。'],
    ]
    for (const [failure, expected] of cases) {
      const updates = []
      const rejected = deferred()
      const binding = {
        Library: () => fulfilled([]),
        History: () => fulfilled([]),
        Following: () => fulfilled([]),
        Play: () => rejected.promise,
      }
      const controller = module.createHomeController(binding, (state) => updates.push(state))
      controller.play(history('anime', 'episode', '2026-09-12T10:00:00Z'))
      rejected.reject(failure)
      await settle()
      assert.equal(updates.at(-1).playMessage, expected)
      assert.ok(!JSON.stringify(updates).includes('backend secret'))
      controller.dispose()
    }
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})
