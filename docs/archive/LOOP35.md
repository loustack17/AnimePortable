<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 35: Cache-first startup and refresh

Status: PASS (owner accepted 2026-10-21). Commit 8bb524b, pushed to main in merge ef97b6c with the resize hotfix and Loop 34b; GitHub Windows CI 39532702396 green on 92868eb.

## Goal

The app starts from cached data and does not need the Internet.

## Current behavior (inventory 2026-10-21)

Startup path: `main` builds the player and an unstarted `backend.Service` (no I/O), then `fltkhome.NewWindow` shows the window. A goroutine opens SQLite (migrations included) and reads the theme; then Home reads Library, History and Following, all from SQLite. No network call runs before Home shows.

Baseline, measured on a copy of the test-build database (6 繼續觀看 rows), evidence in `artifacts/evidence/loop35/baseline-offline-startup.txt`:

- Exe start to visible window: 189-316 ms (5 runs; PowerShell `Start-Process`, then poll `MainWindowHandle`).
- Window build to Home showing stored rows, with every network action held until cancelled: 58-106 ms (5 runs). Command: `ANIMEPORTABLE_STARTUP_DB=<db> go test -run TestNativeOfflineStartupTiming -count=5 -v ./apps/desktop/fltkhome`.

So the 1-second target already holds for the first paint. The gaps are in what Home shows offline and in how refresh runs:

1. 繼續觀看 shows 集數：未知 offline. Home ignores the stored episode number in `History.EpisodeNumber` and asks Anime1 for each row's episode list (`resolveHomeEpisodes`, up to 6 parallel requests, 3 s each) on every Home load.
2. The Following refresh calls Anime1 for every followed anime one after another (15 s each), so a held network keeps it busy for 15 s per anime. Home's start triggers it, and it fetches the same episode lists as gap 1.
3. Refresh timeouts and the 10-minute throttle are written separately in each screen (`followingRefreshInterval` and `scheduleRefreshInterval`; 3 s, 15 s and 45 s timeouts). There is no shared limit on parallel requests.
4. Refresh work is cancelled only by closing the app; nothing waits for it to stop, and no test checks that its goroutines end.
5. Home shows nothing when a refresh fails. 追蹤 and 時間表 already say they show the last saved data.
6. Every backend action takes one of 16 operation slots and fails at once when none is free. An unbounded fan-out of refresh requests could make Home's own reads fail.

## Scope

In scope:

- One refresh policy in core (`core/refresh.go`): at most 4 refresh requests at a time across the app, of which automatic (background) refresh uses at most 3, so a lookup the user waits on (an opened detail screen, 重新整理 on 時間表) never waits behind it. The Following refresh always runs in the background share, even from 更新集數, because nobody waits on any single list; a timeout per request; the 10-minute throttle per kind of data; and cancellation of all running refresh work, waiting for it to stop. Screens use it; they keep no limits or throttles of their own.
- Home shows the stored episode number at once and makes no network call of its own (gaps 1 and 2).
- The Following refresh runs through the policy: parallel up to the limit, cancelled and waited for on shutdown.
- The 時間表 refresh and the detail screen's AniList/Bangumi lookup run through the same policy. Leaving the detail screen still cancels its lookup.
- When the Following refresh cannot reach Anime1 for any followed anime (a network error or a request timeout, reported by the backend as `ErrOffline`), Home shows a calm one-line status: 目前無法連線，顯示上次儲存的資料。 A source that answers with an error, such as a removed show, is not called offline. No dialog, no automatic retry; the next due refresh or 更新集數 tries again, and a successful one clears the line.

Out of scope, never (owner decision 2026-10-21): offline playback and downloading any video. Playback stays online only; offline, the app shows only the stored lists (titles, episode numbers, progress), and playing an episode needs the network as before.

Also out of scope: new data sources, new backend actions, Home's 今日播出 from the stored schedule, a stored episode list for the detail screen (playback needs the network anyway), cover image caching, new dependencies.

## Tests

Core unit tests (`core/refresh_test.go`), each with an expected result:

- Limit: 10 jobs that wait on a channel are started with limit 4. The highest number running at the same time is 4, and all 10 finish.
- Timeout: a job reads its context deadline. The deadline is the policy timeout from when the job started.
- Close: jobs that wait for cancellation are running, and more are waiting for a slot. Close returns only after every job returned; waiting jobs never start; `runtime.NumGoroutine` is back to its starting value within 1 second. No new dependency is needed.
- Throttle: with a fake clock, a kind is allowed once, refused again within 10 minutes, and allowed after 10 minutes. A forced refresh is always allowed.

UI tests (real FLTK windows and a fake service, like the existing `*_windows_test.go`):

- Offline start: the fake's Episodes and Schedule wait until cancelled. Home shows the stored 近期更新 rows and the 繼續觀看 cards with captions like 集數：05, and Home's own load makes no Episodes call.
- Held network: 10 followed anime, Episodes waits until cancelled. At most 3 Episodes calls run at once (the background share of 4). Closing the window cancels them, and the goroutine count is back to its starting value.
- Failing network: Episodes returns `ErrOffline` at once. Home shows the offline status line and no dialog; 追蹤 does not show Home's line. When a later refresh succeeds, the line is gone.
- Source answers with an error: Episodes returns `ErrUnavailable`. Home shows no offline line.
- Core: background jobs use at most 3 of 4 slots while a user job starts at once; a Run whose context already ended starts no job.
- The existing 追蹤, 時間表 and detail tests still pass.

Measurement: `TestNativeOfflineStartupTiming` (skipped unless `ANIMEPORTABLE_STARTUP_DB` is set) is run again after the change, with the same command as the baseline.

Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Disconnect the network, start the app, and see cached content within about 1 second.

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 `9E1AE85C…A4FA`; libmpv DLL matches the pin; existing `data/` kept).

1. 網路連線時開啟程式一次，看一下首頁的「繼續觀看」（以及有追蹤作品時的「近期更新」），然後關閉程式。
2. 中斷網路（拔掉網路線或關閉 Wi-Fi）。
3. 開啟程式。約 1 秒內應看到首頁的已儲存內容，「繼續觀看」卡片顯示集數（例如「集數：05」），不是「未知」。
4. 不應出現任何錯誤視窗。有追蹤作品時，稍等一下（最多約半分鐘），首頁上方會出現「目前無法連線，顯示上次儲存的資料。」，首頁內容保持不變。
5. 依序切到追蹤、時間表、歷史紀錄：每頁顯示上次記錄的內容，上方不顯示首頁那行字。回到首頁，那行字再次出現。
6. 恢復網路，到追蹤按「更新集數」，等狀態不再顯示「正在更新」後回到首頁：那行字應消失。
7. 關閉程式：視窗應在一兩秒內關閉，不會卡住。

## Status

- 2026-10-21: inventory and baseline done.
- Implementation done (not committed): `core/refresh.go` (Refresher: limit 4, background share 3, per-kind throttle, per-request timeouts, Close waits); Home uses stored episode numbers and makes no network call; 追蹤, 時間表 and detail metadata refresh through the Refresher; backend `ErrOffline` for network errors and timeouts; Home offline line. After the change: 52-101 ms from window build to stored Home with the network held (5 runs), captions show stored numbers. Evidence in `artifacts/evidence/loop35/`.
- Test changes: three tests of the removed live Home episode lookup (`TestNativeHomeEpisodeLookupIsImmediate`, `...RejectsStaleResult`, `...UpdatesWhileSearchIsVisible`) and `TestEpisodeNumberUsesExactIDAndSafeFallback` were removed with the code they tested; replaced by `TestHomeCaptionUsesStoredEpisodeNumberAndSafeFallback` and the new native offline tests. The 追蹤 throttle test no longer reads the removed `followingRefreshAttemptedAt` field; it checks that 更新集數 starts a refresh. Owner accepted these test changes on 2026-10-21.
- Owner approval 2026-10-21: commit locally, then merge with the resize hotfix and the 追蹤 star change and push the three together to main after the full suite and race pass on the merge.
- TypeSafe doc-quality: Tests clarity 1.20 rewritten to 1.40/2 (REVIEW band; adjudicated by reading: names files, inputs and expected results).
- Local: gofmt and vet clean; full native suite and race green (nuwen msvcrt MinGW gcc 15.2), before and after the review fixes.
- TypeSafe scope-check (final): criteria_covered 1.82/2 and in_scope 1.81/2 (conf 0.71); verifier_weakened 0.58 (REVIEW). Adjudicated by reading the diff: the removed tests checked only the removed live lookup and its stale-result guard, which no longer exist; the new tests check the stored number, offline start, the bound, cancellation and the offline line. No assertion on remaining behavior was loosened. Owner to confirm (listed above).
- Owner check (2026-10-21): the owner could not disconnect the network (other work needs it) and accepted the offline part as passed without running it; keep the Home offline line. TypeSafe owner-report HUMAN_EXCEPTION_APPROVED (conf 0.37, REVIEW; adjudicated by reading the reply: an explicit acceptance without running the check). The offline behavior is covered by the native offline tests and the held-network timing measurement only.
- Owner decision 2026-10-21: no offline playback or video downloads, ever (recorded in 01_PRODUCT.md permanent exclusions).
- Independent review: no MUST_FIX. TypeSafe review-triage rated S1 (offline line for non-network failures) and S3 (schedule timeout split between core and backend) MUST_FIX; both fixed with tests. S2 fixed (background share leaves a slot for what the user waits on); NITs N1-N4 fixed; N5 rated invalid. Re-review: no MUST_FIX; its last SHOULD_FIX (更新集數 filled every slot) and two NITs fixed.
- CI (2026-10-21): run 39531980794 on ef97b6c passed tests, race and vet but failed govulncheck on a new standard-library advisory GO-2026-6809 (crypto/x509, fixed in Go 1.26.10; TypeSafe ci-log stage vet_or_security, conf 1.00). Owner approved the CI Go bump to 1.26.10 (92868eb); run 39532702396 green.
