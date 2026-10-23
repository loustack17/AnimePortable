<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 37d: Reliable starts and more synopses

Status: PASS (owner accepted 2026-10-23: "all owner-check steps pass"; TypeSafe owner-report HUMAN_PASS 0.97). Commit de3c9ed; GitHub Windows CI 39917415783 green. Owner request 2026-10-22, before Loop 38. Plan approved by the owner 2026-10-22 as written, including migration 0011 and the bounded upstream retry if the evidence supports it. Named 37d because Loop 37c (Skip the opening) is already PASS.

## Goal

1. Episodes start quickly, and a start that cannot succeed says so instead of hanging.
2. More anime show a synopsis, without any wrong one (ADR-009: no metadata beats wrong metadata).

## Owner report (2026-10-22, v0.4.0-beta)

- Many anime show no synopsis.
- Of about 10 episodes opened, 3-4 take noticeably long to start and about 1 fails until the owner retries by hand.

## Evidence (re-verified 2026-10-22)

Owner data was copied (db, -wal, -shm) to a scratch folder and read there; the owner's folder was not changed.

Metadata:

- 228 anime, 19 with metadata (all Bangumi). Of the 30 the owner opened (history, progress or following), 11 have none: 殺手旅店, 殺手旅店 第二季, 被放逐的作弊賦予魔術師…（long subtitle）, Battle Spirits [Re] 絕界之空, 藥師少女的獨語 第三季, 關於我轉生變成史萊姆這檔事 第四季, 世界最強後衛～迷宮國的新人探索者～, Dr.STONE 新石紀 SCIENCE FUTURE, 出租女友 第五季, 異獸魔都 第二季, 判處勇者刑.
- Probe of these with today's matcher (session of 2026-10-22, to be recorded as fixtures in this loop): AniList finds nothing for Chinese titles; Bangumi returns candidates but none is accepted. Causes: Taiwan vs mainland names (藥師少女的獨語 vs 藥屋少女的呢喃, 出租女友 vs 租借女友, 判處勇者刑 0 results), small wording differences (後衛～ vs 的後衛 ～, Battle Spirits vs 戰鬥之魂), a long subtitle (title contains the Bangumi name, score 52, NeedsCrossCheck), an extra Taiwan name in a Latin title (Dr.STONE 新石紀, 52), and a season missing from search results (史萊姆 第四季: only the base title comes back and is rightly rejected).
- Anime1's catalog (`animelist.json`) gives per row: id, title, episode range, year, airing season (冬/春/夏/秋), fansub. Checked live: 判處勇者刑 `1-12 | 2026 | 冬`, 殺手旅店 第二季 `連載中(16) | 2026 | 秋`. The adapter parses only id and title (`adapters/source/anime1/anime1.go` `parseRow`), so `metadataQueryFor` (`core/metadata_refresh.go`) never passes a year or episode count, although `metadataHintScore` (`core/metadata_match.go`) already scores year ±12/14 and episode count +8/−10. With both, a 52 contains-match agreeing on year and count scores 72 and is accepted.
- No memory of misses: a title without a match is searched again every time it is opened.

Playback start:

- From this machine, 15 random episodes of the owner's anime with the real Anime1 resolver, then a direct 2 MB ranged GET: resolve 0.57-0.98 s, CDN first byte 0.10-0.55 s, 2 MB in 0.14-1.35 s, no failures. The network is fast; the delay is inside the app.
- No `playback-failures.log` exists in the owner's data folder although the owner saw failures. The log only gets a line when Play returns an error or libmpv reports a failed load (`apps/desktop/backend/failurelog.go`; a proxy rejection is only remembered by `UpstreamRejected`). A start that hangs, or one that mpv recovers from slowly, writes nothing.
- Code facts: `securehttp` stream `Open` has a 10 s header timeout but no read-stall timeout; the resolver retries the page GET once only on its 5 s timeout, the API POST never (`adapters/source/anime1/resolver.go`, `fetchResolver`); the proxy returns 502 on any upstream open error with no retry and 503 at once when the per-session stream cap (4) or the global cap (16) is full, neither of which is logged (`adapters/playback/proxy/proxy.go`, `serveSession`); mpv runs with `demuxer-max-bytes 16MiB`, `cache-secs 5` and its default `network-timeout` (`apps/desktop/fltkengine/engine_windows.go`, `InitOptions`).

## Scope

### Part A: playback start

A1. Measure first. A debug-only start timeline, off unless the environment variable `ANIMEPORTABLE_START_TIMELINE=1` is set at launch, writes `playback-timeline.log` next to the database. One line per event, milliseconds since the click: resolve page, resolve API, each proxy upstream open (byte offset of the requested range, status or fixed error category, time to headers), each proxy 503, mpv `file-loaded`, first frame. Fixed vocabulary like the failure log: no URLs, tokens, cookies, titles or ids; mpv `log-file` stays off. With it, measure 25+ starts of the owner's anime (a fixed list, recorded in evidence) and put the per-stage times in `artifacts/evidence/loop37d/`. The slow starts are explained by stage before any fix.

A2. Never hang silently. A start watchdog: if no first frame arrives within 20 s of the click, the attempt is ended and reported like any failure (existing message 此集暫時無法播放…, one failure-log line with new stage `start_timeout`). The status of a proxy upstream rejection seen during the start (for example 502) is added to that line.

A3. Smallest causal fixes, chosen from the A1 evidence. Candidates, each only if the timeline shows its cause:
- one bounded retry of a proxy upstream open that failed before any byte reached mpv (network error or 5xx; not 4xx, not cancel) — the open proposal in the handoff;
- a short wait (at most 2 s) for a free stream slot instead of an immediate 503;
- mpv `network-timeout` and FFmpeg reconnect options in `stream-lavf-o` (kept together with the existing empty `http_proxy`);
- a read-stall timeout on the upstream body in the proxy, so a dead connection ends and mpv reconnects;
- one retry of the resolver API POST on timeout or network error.
Fixes not supported by the evidence are not made.

### Part B: metadata

B1. Offline evaluation set first. Record Bangumi and AniList responses (search and subject) for the 11 misses, the 19 current correct matches, and negative cases (the 史萊姆 第四季 base title, other seasons of the same series) as test fixtures. A test in the metadata area replays them through the real adapters' parsing and `core.MatchMetadata` with no live calls and prints hits, wrong matches and misses. Each fixture title has an expected answer (a Bangumi subject id or none) checked by hand and listed in the evidence for the owner to see. Baseline before any change is recorded.

B2. Use what Anime1 already gives: parse year, airing season and episode count (`1-12` → 12; `連載中(16)` → unknown count) in the Anime1 adapter, store them (additive migration `0011`, numbered after the newest on origin/main at the time; existing rows default to 0/empty and fill on the next catalog refresh), and pass year and episode count into the metadata query. Then rerun B1.

B3. Then, only for misses still left and each kept only if B1 shows no new wrong match:
- a season browse: with Anime1's year and airing season, fetch that season's Bangumi anime list (Bangumi API v0 browse by year and month; the exact call is confirmed against the official docs first) and match within that small set, including Bangumi's Chinese aliases;
- normalization for comparison only: punctuation, ～, spacing, 之/の (removing 的 only if the evaluation shows it is safe);
- a unique candidate whose full name is a prefix of a long title is accepted when year and episode count agree.

No per-title tables and no Taiwan/mainland word list unless the evaluation shows the above cannot reach the target.

B4. Misses are remembered in memory for the app run (keyed by anime id), so opening the same anime again does not search again; nothing is stored, so a newer version tries again. A provider error is not a miss.

## Findings during the loop

- B4 is already true within an app run: the detail screen keeps `metadataLookedUp` per anime (`apps/desktop/fltkhome/detail_view_windows.go`), so an anime without a match is looked up once per run, not on every open. The repeat happens only after a restart, which also lets a newer matcher try again. No second cache is added.
- The airing season is stored but not put into the match query: only AniList candidates carry a season, and `metadataHintScore` counts a season on one side only against the candidate, so it would push every Bangumi candidate down.
- Anime stored before migration 0011 have no catalog info. The first lookup of such an anime reads its catalog row once (`Service.fillCatalogInfo`).
- The evaluation set records what each provider's `Search` returned (candidates as the core sees them), not raw HTTP bodies; the adapters' parsing keeps its own fixture tests. The recorder is evidence tooling in `artifacts/evidence/loop37d/recorder/` and runs the real `RefreshMetadata` on a scratch copy of the owner's database.

### Episode lists (owner correction 2026-10-23)

The owner's slow and failing case is loading the episode list and synopsis on the detail page, not playback. Reproduced without any window (artifacts/evidence/loop37d/detail/: the backend called as Home, 追蹤 and the detail page call it, with the owner's data):

- Backend episode-list loads: 60+ loads, 21-156 ms, 0 errors, no lock or slot waits, also while 追蹤 refreshes every followed list. The screen's request queue restarts queued detail loads correctly.
- Anime1 episode pages, 540 fetches: 0 errors, no stalls; the first fetch of a list is slow (14 of 30 took 1.0-2.9 s), repeats take about 26 ms (Anime1's cache). The detail page shows nothing until this answer arrives, so a first open waits 1-3 s.
- A stalled anime1.me request (seen once in 30 resolver page requests: 5 s timeout, retry answered in 0.5 s) would leave an episode list waiting up to the client's 10-30 s timeouts and then fail. The resolver already had a 5 s timeout and one retry; the episode list had neither. Fixed: one shared `getPage` (adapters/source/anime1/anime1.go) gives every Anime1 page request 5 s and one retry after a timeout.
- Synopsis: the season browse added three Bangumi pages at about 2.6 s each, one after another (7.9 s); they are now fetched at once (3.6 s).
- Not reproduced: a failing episode list.
- Owner approved one new backend action, `StoredEpisodes` (2026-10-23): the episode list stored last time, from local data only. The detail page shows it at once and replaces it with Anime1's answer; a failed refresh keeps the stored list (its episodes still play) instead of showing 無法載入. Decision in `detailModel.applyStored`/`applyRefresh`. With the owner's data all 30 opened anime have a stored list, read in under 1 ms (artifacts/evidence/loop37d/detail-stored.txt).

### Synopsis matching (B1-B3 results)

Owner approved on 2026-10-23 adding Simplified search and the series-name rule, provided performance and resources do not get worse.

- Bangumi's search does not match Traditional keywords: 判處勇者刑 finds 0, 判处勇者刑 finds 445905; 殺手旅店 and 異獸魔都 likewise. The Bangumi adapter now searches the Chinese title in Simplified (core's existing OpenCC table, exported as `core.FoldToSimplified`; no new dictionary, no extra request). The Japanese native title is searched unchanged.
- Rules, each kept because the evaluation shows hits gained and 0 wrong: Anime1 year and episode count as hints; a one-year gap is neutral (Anime1 can list a show a year after Japan); season browse when search accepts nothing (3 pages fetched at once, about 3.6 s, cached per run, at most 16 seasons in memory); an unclosed ～ starts a subtitle; main titles differing only in 的/之/の match weakly when the subtitles are the same; a series name taken from an earlier season's alias (藥師少女的獨語 → 藥屋少女的呢喃) lets a sequel match weakly, with season and year deciding.
- Evaluation set (core/testdata/metadata_eval.json, recorded 2026-10-23 from the owner's 228 anime, trimmed to candidates that can score; trimming changed no result): 193 hits, 0 wrong, 3 misses, 32 correctly without a match. Before Loop 37d the same anime got 135 matches. All 59 new or changed matches were checked by hand (artifacts/evidence/loop37d/new-accepts.txt). Of the owner's 11 missing, 9 now match; Dr.STONE 新石紀 SCIENCE FUTURE (Anime1 merges three parts) and Battle Spirits [Re] 絕界之空 (絶/絕) still do not. Lost compared with before: 紫羅蘭永恆花園 外傳 (the Simplified search no longer returns it; accepted rather than searching twice).
- Turning off any one rule loses hits and never adds a wrong match (series name −4 must-hit, 的/之/の −1, unclosed ～ −1, season browse −10, year and count hints −15, one-year neutral −1 hit).
- Any anime labelled "no match" that starts matching fails the test, so a new rule cannot add an unchecked match.

### Review (2026-10-23)

Independent read-only review: 1 MUST_FIX, 3 SHOULD_FIX, 3 NIT, all addressed.
- MUST_FIX: after a start had already failed (switch, first play, or libmpv load), the start watch still fired 20 s later, showing a second message and logging a false `start_timeout`. Each failure path now ends the watch.
- The retry of a timed-out Anime1 page now keeps only the client's own timeouts, so a slow but working connection still gets its page (only the first attempt is limited to 5 s).
- Lookups of one season at the same time share a single Bangumi fetch; a failed page cancels its sibling requests.
- A season marker after an unclosed ～ costs the match but never gives a wrong one (test added); `FoldToSimplified` is one exported function; `fillCatalogInfo` documents its cost.
- The A2 wording above was changed to what was built (a recent upstream rejection status, not 503s).

## Out of scope

- The matcher's score thresholds (50, 60, 70) and ambiguity gap stay as they are.
- AniList stays as it is; making AniList find Chinese titles is not attempted.
- No new backend actions, settings, screens or UI text. The owner sees only faster starts, the existing failure message instead of a hang, and more synopses.
- Loop 38 work (security pass, resource pass, v1.0.0 release).

## Done when

- A1: timeline evidence for 25+ starts, each slow start (> 5 s to first frame) attributed to a stage.
- A2: no start can wait more than 20 s without a first frame or a reported failure (tested with a fake upstream that stalls before and after headers, and with a full stream cap).
- A3: on the same fixed episode list after the fixes: median click-to-first-frame ≤ 3 s, at most 1 of 25 above 6 s, 0 failures needing a manual retry. If the evidence shows a cause outside the app (for example the CDN), that is reported to the owner instead of a fix.
- B: on the offline set, 0 wrong matches (before and after); all 19 current matches still correct; at least 8 of the 11 owner misses matched; the result printed by the test and saved in evidence.
- Every fix has a test that fails without it. Full native suite and race, `go vet`, simplify, oracle review with no MUST_FIX, scope check with TypeSafe.

## Tests

Each test below is written only for the fixes actually made in A3 and B3.

- Proxy upstream retry: an upstream open that fails with a network error or 5xx before any byte was sent to mpv is tried once more. It is not retried after bytes were sent, on a 4xx, or when mpv cancelled the request.
- Proxy stream slot: when all slots are busy, a request waits up to 2 s for a free slot and then gets 503.
- Proxy stall: an upstream body that sends nothing for the stall timeout is closed. After every proxy test the goroutine count is back to its starting value.
- Watchdog: no first frame within 20 s of the click gives exactly one `start_timeout` failure line and closes the playback session. A first frame at 19 s gives no failure. A user stop or episode switch before 20 s gives no failure.
- Timeline: with the variable unset, no file is created. With it set, a test passes URLs, tokens and cookies through every event and checks that none of them appears in the file.
- Resolver: the API POST is retried once on a timeout or network error, and not on a 4xx or when cancelled.
- Anime1 parsing: year, airing season and episode range are read from catalog rows; a missing or malformed value becomes unknown (0 or empty) and does not drop the row.
- Migration 0011: applied to a copy of a database at migration 0010, every existing row is kept.
- Metadata: the evaluation test from B1 prints hits, wrong matches and misses and fails on any wrong match. A second open of an anime with no match makes no provider call in the same app run.

## Owner check

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted Loop 37d worktree build 2026-10-23, EXE SHA-256 `C1184A88…B501`, Windows GUI; libmpv DLL matches the pin; existing `data/` kept, backed up to `artifacts/evidence/loop37d/test-build-data-before-0011/`).

1. 開啟 App，到「追蹤」，依序點開 10 部不同的動畫。每一部的集數清單應在開頁時立刻出現，不用等；過一兩秒後清單可能更新（例如多出集數標題）。
2. 點開 殺手旅店、判處勇者刑、出租女友 第五季、藥師少女的獨語 第三季、世界最強後衛：應顯示簡介與封面，而且是這部作品、這一季的簡介（第一次打開可能要等幾秒）。
3. 點開 關於我轉生變成史萊姆這檔事 第四季：應顯示第四季的簡介，不可以是第一季或其他季的。
4. 點開 Dr.STONE 新石紀 SCIENCE FUTURE：目前預期仍沒有簡介；若有，必須是 SCIENCE FUTURE 的。
5. 從任一動畫頁播放一集：應照常開始播放。
## Release

Pre-release v0.4.1-beta (patch, owner request 2026-10-23: "這算是小修正"): release commit f2c3483 (CI 39917947428 green), tag v0.4.1-beta, release job 39918270994. The published ZIP holds animeportable.exe (Windows GUI, version 0.4.1-beta), libmpv-2.dll (SHA-256 matches the pin), LICENSE, THIRD_PARTY_NOTICES.md and licenses/libmpv.txt; unzipped to a new folder it starts, shows its window and creates its database. Notes: [releases/v0.4.1-beta.md](../releases/v0.4.1-beta.md).