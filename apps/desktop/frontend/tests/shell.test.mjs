// SPDX-License-Identifier: MPL-2.0

import assert from 'node:assert/strict'
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises'
import { fileURLToPath, pathToFileURL } from 'node:url'
import { join } from 'node:path'
import { test } from 'node:test'
import { compile } from 'svelte/compiler'
import { render } from 'svelte/server'
import { transpileModule, ModuleKind, ScriptTarget } from 'typescript'

test('shell renders six accessible destinations without a backend', async () => {
  const source = await readFile(new URL('../src/App.svelte', import.meta.url), 'utf8')
  const homeSource = await readFile(new URL('../src/HomeView.svelte', import.meta.url), 'utf8')
  const helperSource = await readFile(new URL('../src/home.ts', import.meta.url), 'utf8')
  const compiled = compile(source, { filename: 'App.svelte', generate: 'server' })
  const homeCompiled = compile(homeSource, { filename: 'HomeView.svelte', generate: 'server' })
  assert.deepEqual(compiled.warnings, [])
  assert.deepEqual(homeCompiled.warnings, [])

  const directory = await mkdtemp(fileURLToPath(new URL('../node_modules/.shell-test-', import.meta.url)))
  try {
    const modulePath = join(directory, 'App.mjs')
    const helperCompiled = transpileModule(helperSource, {
      compilerOptions: { module: ModuleKind.ESNext, target: ScriptTarget.ESNext },
      fileName: 'home.ts',
    })
    await writeFile(join(directory, 'home.js'), helperCompiled.outputText)
    await writeFile(join(directory, 'HomeView.mjs'), homeCompiled.js.code)
    await writeFile(modulePath, compiled.js.code.replaceAll('./HomeView.svelte', './HomeView.mjs'))
    const { default: App } = await import(pathToFileURL(modulePath).href)
    const { body } = render(App)
    const nav = body.match(/<nav\b[^>]*>[\s\S]*?<\/nav>/)?.[0]
    assert.ok(nav, 'navigation landmark is rendered')
    assert.equal([...nav.matchAll(/<button\b/g)].length, 6)
    assert.equal([...nav.matchAll(/aria-current="page"/g)].length, 1)
    const labels = ['首頁', '時間表', '追蹤', '歷史紀錄', '搜尋', '設定']
    let previous = -1
    for (const label of labels) {
      const position = nav.indexOf(label)
      assert.ok(position > previous, `${label} follows the required navigation order`)
      previous = position
    }
    assert.match(body, /<main\b[^>]*id="([^"]+)"/)
    const mainID = body.match(/<main\b[^>]*id="([^"]+)"/)[1]
    assert.ok(body.includes(`href="#${mainID}"`), 'skip link points to main')
    assert.match(body, /<main\b[^>]*tabindex="-1"/)
    assert.equal([...body.matchAll(/<h1\b/g)].length, 1)
    for (const heading of ['繼續觀看', '追蹤中', '最近更新', '今日播出']) assert.match(body, new RegExp(`<h2[^>]*>${heading}</h2>`))
    assert.doesNotMatch(body, /<(iframe|video|audio)\b|\son\w+=|https?:\/\//)
  } finally {
    await rm(directory, { recursive: true, force: true })
  }
})
