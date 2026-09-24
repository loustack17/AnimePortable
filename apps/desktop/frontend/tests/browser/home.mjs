// SPDX-License-Identifier: MPL-2.0

import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { readFile } from 'node:fs/promises'
import { resolve, extname, sep } from 'node:path'
import { fileURLToPath } from 'node:url'
import { chromium } from 'playwright'

const root = fileURLToPath(new URL('../../dist', import.meta.url))
const server = createServer(async (request, response) => {
  const pathname = new URL(request.url, 'http://localhost').pathname
  if (pathname === '/favicon.ico') {
    response.writeHead(204).end()
    return
  }
  const filename = resolve(root, `.${pathname === '/' ? '/index.html' : pathname}`)
  if (!filename.startsWith(root + sep)) {
    response.writeHead(403).end()
    return
  }
  try {
    const bytes = await readFile(filename)
    response.setHeader('Content-Type', ({ '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css' })[extname(filename)] ?? 'application/octet-stream')
    response.end(bytes)
  } catch {
    response.writeHead(404).end()
  }
})
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
const origin = `http://127.0.0.1:${server.address().port}`
async function within(promise) {
  let timer
  try {
    return await Promise.race([promise, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Fixture request timed out')), 10000) })])
  } finally {
    clearTimeout(timer)
  }
}
let browser
try {
  browser = await chromium.launch({ headless: true })
  const page = await browser.newPage({ viewport: { width: 1000, height: 618 } })
  page.setDefaultTimeout(10000)
  page.setDefaultNavigationTimeout(10000)
  const errors = []
  const requests = []
  const calls = []
  await page.addInitScript(() => { window.chrome = { webview: { postMessage() {} } } })
  page.on('pageerror', error => errors.push(error.message))
  page.on('console', message => { if (['warning', 'error'].includes(message.type())) errors.push(message.text()) })
  page.on('request', request => requests.push(request.url()))
  const anime = (id, title) => ({ id, title, nativeTitle: '', description: '' })
  const history = (animeId, episodeId, lastPlayed, completed = false) => ({ animeId, episodeId, position: 125500, duration: 1440000, completed, updatedAt: lastPlayed, lastPlayed })
  let library = [anime('a', '測試動畫 <script>'), anime('b', '已完成動畫'), anime('c', '第二部動畫')]
  let records = [history('a', 'opaque-latest', '2026-09-12T12:00:00Z'), history('a', 'opaque-older', '2026-09-11T12:00:00Z'), history('b', 'complete', '2026-09-12T11:00:00Z', true), history('b', 'old-incomplete', '2026-09-10T12:00:00Z'), history('c', 'other', '2026-09-12T10:00:00Z')]
  let follows = [{ animeId: 'a', latestAvailable: '', latestWatched: 'opaque-latest', hasAvailable: false, hasWatched: true, newEpisode: false }]
  let failLibrary = false
  let failPlay = false
  let holdHistory = false
  let releaseHistory
  let releasePlay
  let onPlay = () => {}
  let onHistory = () => {}
  let onCancel = () => {}
  let holdService = false
  let releaseService
  let onService = () => {}
  await page.route('**/assets/service-*.js', async route => {
    if (holdService) await new Promise(resolve => { releaseService = resolve; onService() })
    await route.continue()
  })
  await page.route('**/wails/custom.js', route => route.fulfill({ status: 200, contentType: 'text/plain', body: '' }))
  await page.route('**/wails/runtime', async route => {
    const payload = route.request().postDataJSON()
    calls.push(payload)
    if (payload.object === 10) {
      onCancel()
      await route.fulfill({ json: null })
      return
    }
    const method = payload.args.methodID
    let result
    if (method === 4215164161) {
      if (failLibrary) {
        await route.fulfill({ status: 500, json: { kind: 'RuntimeError', message: 'fixture-private-error' } })
        return
      }
      result = library
    } else if (method === 2534497190) {
      if (holdHistory) await new Promise(resolve => { releaseHistory = resolve; onHistory() })
      result = records
    } else if (method === 879874013) result = follows
    else if (method === 3828312544) {
      await new Promise(resolve => { releasePlay = resolve; onPlay() })
      if (failPlay) {
        await route.fulfill({ status: 500, json: { kind: 'RuntimeError', message: 'fixture-private-error' } })
        return
      }
      result = null
    } else throw new Error(`Unexpected runtime request: ${JSON.stringify(payload)}`)
    await route.fulfill({ json: result })
  })
  await page.goto(origin)
  await page.getByRole('button', { name: /播放|繼續/ }).first().waitFor({ timeout: 10000 })
  const continued = page.getByRole('region', { name: '繼續觀看', exact: true })
  const playButtons = continued.getByRole('button')
  assert.equal(await playButtons.count(), 2)
  assert.ok((await continued.innerText()).includes('測試動畫 <script>'))
  assert.ok(!(await continued.innerText()).includes('已完成動畫'))
  assert.equal(await continued.locator('script').count(), 0)
  assert.deepEqual(calls.map(call => call.args.methodID).sort(), [4215164161, 2534497190, 879874013].sort())
  let started = new Promise(resolve => { onPlay = resolve })
  await playButtons.first().focus()
  await page.keyboard.press('Space')
  await within(started)
  assert.ok(await playButtons.evaluateAll(buttons => buttons.every(button => button.disabled)))
  assert.deepEqual(calls.at(-1).args.args, [{ animeId: 'a', episodeId: 'opaque-latest', startAt: 125500 }])
  const pendingCount = calls.length
  await playButtons.last().dispatchEvent('click')
  assert.equal(calls.length, pendingCount)
  releasePlay()
  await page.waitForFunction(() => document.querySelector('[aria-labelledby="continue-title"] button:not(:disabled)'))
  console.log('PLAY SUCCESS', await continued.innerText())
  failPlay = true
  started = new Promise(resolve => { onPlay = resolve })
  await playButtons.first().click()
  await within(started)
  releasePlay()
  await continued.getByRole('alert').waitFor()
  assert.ok(!(await continued.innerText()).includes('fixture-private-error'))
  failPlay = false
  const nav = page.getByRole('navigation', { name: '頁面' })
  const homeButton = nav.getByRole('button', { name: '首頁', exact: true })
  const scheduleButton = nav.getByRole('button', { name: '時間表', exact: true })
  const settingsButton = nav.getByRole('button', { name: '設定', exact: true })
  await homeButton.focus()
  await page.keyboard.press('ArrowDown')
  assert.equal(await scheduleButton.evaluate(button => document.activeElement === button), true, 'ArrowDown moves visible navigation focus')
  assert.equal(await homeButton.getAttribute('aria-current'), 'page', 'focus alone does not activate a destination')
  await page.keyboard.press('ArrowUp')
  assert.equal(await homeButton.evaluate(button => document.activeElement === button), true, 'ArrowUp restores navigation focus')
  await page.keyboard.press('ArrowUp')
  assert.equal(await settingsButton.evaluate(button => document.activeElement === button), true, 'arrow navigation wraps between destinations')
  await page.keyboard.press('Enter')
  assert.equal(await settingsButton.getAttribute('aria-current'), 'page')
  await homeButton.click()
  started = new Promise(resolve => { onPlay = resolve })
  const playCancelled = new Promise(resolve => { onCancel = resolve })
  await playButtons.first().click()
  await within(started)
  await nav.getByRole('button', { name: '設定', exact: true }).click()
  await within(playCancelled)
  releasePlay()
  await nav.getByRole('button', { name: '首頁', exact: true }).click()
  await playButtons.first().waitFor()
  assert.ok(!(await continued.innerText()).includes('播放已開始。'))
  for (const label of ['時間表', '追蹤', '歷史紀錄', '搜尋', '設定', '首頁']) {
    const button = nav.getByRole('button', { name: label, exact: true })
    await button.focus()
    await page.keyboard.press('Enter')
    assert.equal(await page.getByRole('heading', { level: 1 }).innerText(), label)
    assert.equal(await button.getAttribute('aria-current'), 'page')
  }
  await playButtons.first().waitFor()
  await page.getByRole('link', { name: '跳至主要內容' }).focus()
  await page.keyboard.press('Enter')
  assert.equal(await page.evaluate(() => document.activeElement?.id), 'main-content')
  await page.screenshot({ fullPage: true })
  for (const [width, height] of [[800, 600], [500, 309]]) {
    await page.setViewportSize({ width, height })
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth))
    await page.screenshot({ fullPage: true })
  }
  holdHistory = true
  const historyStarted = new Promise(resolve => { onHistory = resolve })
  const cancelled = new Promise(resolve => { onCancel = resolve })
  await page.reload()
  await within(historyStarted)
  await nav.getByRole('button', { name: '設定', exact: true }).click()
  await within(cancelled)
  holdHistory = false
  releaseHistory()
  await nav.getByRole('button', { name: '首頁', exact: true }).click()
  await playButtons.first().waitFor()
  holdService = true
  const serviceStarted = new Promise(resolve => { onService = resolve })
  await page.reload({ waitUntil: 'domcontentloaded' })
  await within(serviceStarted)
  await nav.getByRole('button', { name: '設定', exact: true }).click()
  const beforeImport = calls.length
  const serviceLoaded = page.waitForResponse(response => /\/assets\/service-.*\.js$/.test(response.url()))
  holdService = false
  releaseService()
  await serviceLoaded
  await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  assert.equal(calls.length, beforeImport, 'unmounted Home must not start requests after lazy binding import')
  await nav.getByRole('button', { name: '首頁', exact: true }).click()
  await playButtons.first().waitFor()
  failLibrary = true
  await page.reload()
  await page.getByRole('button', { name: /重試|重新/ }).first().waitFor()
  assert.ok(!(await page.locator('main').innerText()).includes('fixture-private-error'))
  failLibrary = false
  await page.getByRole('button', { name: /重試|重新/ }).first().click()
  await continued.getByRole('heading', { name: '測試動畫 <script>', exact: true }).waitFor()
  library = []; records = []; follows = []
  await page.reload()
  await page.getByText('還沒有可繼續觀看的內容。', { exact: true }).waitFor()
  assert.equal(await playButtons.count(), 0)
  assert.deepEqual(errors.filter(error => !error.includes('Failed to load resource: the server responded with a status of 500')), [])
  assert.ok(requests.every(url => url.startsWith(origin)))
  console.log('PASS: typed fixtures, latest history, plain text, keyboard Play, pending guard, error redaction, six destinations, skip focus, responsive layout, cancellation, retry, empty state and local-only requests')
} finally {
  try {
    await browser?.close()
  } finally {
    server.closeAllConnections()
    await new Promise((resolve, reject) => server.close(error => error ? reject(error) : resolve()))
  }
}
