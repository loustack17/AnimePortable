<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 30: anime detail and episode UI (Phase 24)

Status: IN_PROGRESS. Not PASS. Run `git log -1` for the current HEAD.

## 1. Scope and acceptance IDs

Scope (ADR-023, Windows only):

- Detail screen: cover, title, native title, short synopsis, season, year, studio, episode count. Fields appear only when metadata exists. Missing metadata or a missing cover is shown neutrally, not as an error.
- Episodes: shown in source order, in bounded pages (page size 8). Episode rows show visible current/progress state (watched, progress, completed).
- Click on an episode row plays that exact episode immediately. There is no separate Play button for a selected episode.
- Enter and Space on a focused episode row play that exact episode. Arrow keys start from the focused row.
- Continue appears only when there is valid, resumable progress for the same series. Completed, missing, zero-position or other-series history does not show Continue. Continue uses the stored position in milliseconds.
- Opening the detail screen never starts playback.
- Back, Escape, switching series and closing cancel in-flight requests. Late responses are rejected.
- Load failures show a safe error with retry. A playback failure leaves the detail screen usable.
- Cover: loaded lazily through the existing backend `GetCover` action (never by direct URL). The source aspect ratio is preserved (letterboxed into a bounded 240x320 canvas). A missing cover uses the existing not-found result and shows a neutral placeholder. A real load failure shows a message and Retry. Image memory and native object lifetime are bounded.
- Player controls (owner decision 2026-10-13): on-video controls auto-hide after 3 s of idle playback. Any key press or mouse movement shows them. Pause, loading and state changes show them. They stay visible while scrubbing, dragging volume, or with a menu or episode list open.
- Not in scope: Schedule, Following, new settings, new dependencies, provider/core/schema changes, workflow or release changes.

Acceptance IDs: FUNC-004, FUNC-005, UX-001 to UX-005, UX-008, UX-010, the relevant ARCH, SEC, PRIV and resource criteria, the loop-integrity criteria (LOOP-001 to LOOP-025 as applicable), and QUAL-001 to QUAL-012. Loop 29 behavior is preserved.

Gates: pure Go tests, vet, backend/architecture/contract regression, formatting and diff check run locally. Native Windows tests, build and race run locally and in GitHub Windows CI. Owner acceptance is a separate gate.

## 2. Done

| Item | Evidence |
| --- | --- |
| Detail fields, lazy secure cover, ordered paged episodes, exact-ID Play/Continue, cancellation and stale-result protection, focus and scroll handling | Features landed in commits 26e7fae..ebe7316 and were verified at 1158671026c429f1acc254e7376ea0e050803ffb; CI 38357613533 all steps green (native Go, 20-repeat lifecycle, Anime1, race, vet, security, build, startup, artifact) |
| Cover aspect ratio and missing-cover neutral state | Included in the commit above; independent review found no open MUST_FIX |
| Go 1.26.9 pin (fixes 9 reachable standard-library vulnerabilities) and Mesa 26.2.4-2 refresh | Owner-approved CI changes; pushed before CI 38357613533 |
| Episode click plays immediately; redundant Play button removed; Continue hidden without same-series resumable history | Commit dbb191f21603373b8527da9a56b4c8811593e863; CI 38361340591 green |
| Enter plays the focused episode; Space plays; idle control hiding (3 s) | Commit bca387a; CI 38435868704 green |
| Hidden controls stay hidden while keyboard focus is retained; owner confirmed controls hide and show correctly | Commit 3022551; CI 38438012680 green; owner PASS on overlay hide/show |
| Default-off playback diagnostic | Commit 07851b1; fixed in 3d58b0d; CI 38441971359 all checks green |

## 3. Video stutter: root cause fixed

Symptom: video was choppy while audio was normal, in every FLTK/libmpv build since Loop 28.

Root cause: the production `main` never called `fltk.Lock()`. FLTK only wakes its event loop for `Fl::awake` calls from other threads after the main thread has called `Fl::lock` once. libmpv signals "new frame ready" from its own thread through `Fl::awake`, so each frame waited 100 to 340 ms for an unrelated event (timer or mouse). libmpv then timed out after 200 ms ("mpv_render_context_render() not being called or stuck") and dropped frames. All native tests call `fltk.Lock()`, which is why tests were smooth and the app was not.

How it was found: an opt-in local benchmark (since removed) was smooth, the real app was not, and a temporary update-to-render latency histogram (since removed) showed the delay was between the mpv callback and the FLTK awake handler, not in rendering.

Fixes kept:

- `apps/desktop/main_fltk_windows.go`: call `fltk.Lock()` at startup.
- mpv properties (position, duration, pause, resolution) are observed through mpv events instead of synchronous reads on the render thread, which libmpv forbids.

Measured on the real app (40 s, same stream, owner's PC): 5.8 to 23.0 new frames per second (source 23.976), 610 to 0 dropped frames, update-to-render latency under 25 ms. The owner confirmed smooth playback on 2026-10-14.

Reverted experiments: removing the early `ReportSwap` (e8f0640, kept), non-blocking render (`BLOCK_FOR_TARGET_TIME=0`, reverted: default is equally smooth and keeps A/V sync), vsync off, UI-thread stall recorder, diagnostic autoplay, mpv verbose log (removed: it wrote the proxy session token to disk).

## 4. Playback failure visibility

- An owner-seen Continue failure went back to Home with only a small grey label that a late Home reload could clear. Playback-start failures now show a "播放失敗" dialog after Home is shown.
- `playback-failures.log` (next to the database, newest 50 lines, always on, never uploaded) records one line per failed attempt: UTC time, stage (`resolve`, `proxy_upstream`, `player_load`, `other`), HTTP status or `-`, and a fixed error category. No URLs, tokens, titles or IDs. Canceled requests are not recorded.
- Retry behavior is unchanged. Decide on an automatic retry only after the log shows the failure stage (suspected intermittent Anime1 HTTP 502).

## 4b. Next steps

1. Done: fix pushed as 3872166, CI 38523007303 green, CI build delivered to `artifacts/test-build/`.
2. Owner checks 2026-10-14: keyboard, Continue/progress, back/recovery, window and mouse passed. Fixed after the check (owner retest passed): Home Continue Watching hid a series whose newest record was an unwatched episode; episode-row progress text overflowed the row; player menu labels sat below their highlight.
3. Open: the detail page shows no cover or synopsis. Anime1 has neither, and the AniList/Bangumi adapters (Loops 17-18, ADR-009) were never wired into the app. Owner approved wiring them in Loop 30: a core metadata refresh use case, providers in the desktop backend, and a background lookup when the detail page opens without metadata.
4. Close Loop 30 after the owner accepts the detail page.

## 5. Evidence index

| Item | Location |
| --- | --- |
| Owner baseline report (local only) | `artifacts/evidence/loop30/owner-stutter-baseline.json` |
| Derived stats (local only) | `artifacts/evidence/loop30/stutter-derived-local.json` |
| TypeSafe assessment and repair | `artifacts/evidence/loop30/stutter-assessment-state.json`, `stutter-assessment-judgments.json`, `stutter-repair-state.json`, `stutter-repair-judgments.json` |
| 3d58b0d diagnostic build | Removed; its report is `artifacts/evidence/loop30/stutter-3d58b0d-report.json`, proof in `artifacts/evidence/loop30/3d58b0d-test-object.json` |
| Build 1158671 | `artifacts/evidence/loop30/1158671-test-object.json` |
| Build dbb191f | `artifacts/evidence/loop30/dbb191f-test-object.json` |
| Other Loop 30 evidence | `artifacts/evidence/loop30/` |
| CI 38357613533 (1158671) | https://github.com/loustack17/AnimePortable/actions/runs/38357613533 |
| CI 38361340591 (dbb191f) | https://github.com/loustack17/AnimePortable/actions/runs/38361340591 |
| CI 38441971359 (3d58b0d) | https://github.com/loustack17/AnimePortable/actions/runs/38441971359 |
| CI 38444662617 (e8f0640, green) | https://github.com/loustack17/AnimePortable/actions/runs/38444662617 |
| Older long-form notes | `../archive/LOOP30_DETAIL.md`, `../archive/LOOP30_REQUIREMENTS_REVIEW.md`, `../archive/LOOP30_HUMAN_CHECK.md` |

## 6. Owner check (人工驗收)

測試版：`artifacts/test-build/AnimePortable.exe`（commit `3872166`，CI run 38523007303 全部通過；EXE SHA-256 `3324360B…0F8C`；libmpv DLL 與固定雜湊一致）。播放順暢度已於 2026-10-14 通過驗收。

尚未驗收的詳情與選集檢查：

| 項目 | 操作與預期結果 |
| --- | --- |
| 作品詳情 | 從 Search 以 Enter 或滑鼠開啟作品。顯示標題、原文標題、簡介；有資料時顯示封面、季別／年份、製作公司與集數。缺資料或封面失敗有清楚提示，不停在載入中，也不自動播放。 |
| 選集與鍵盤 | 滑鼠點擊集數立即播放該集。方向鍵選集數，Enter 或 Space 播放聚焦的那一集。Tab 換集後，方向鍵與 Space 以該集為起點。焦點清楚可見，集數順序不變。 |
| 繼續觀看與進度 | 播放一集並前進一段時間，返回詳情再按繼續觀看，應回到對應作品、集數與位置。未看過或已完成時不顯示繼續觀看；集數列顯示進度或完成狀態。 |
| 返回與失敗復原 | 開啟作品後立即返回、切換作品或關閉，較晚的資料不能覆蓋目前作品。來源暫時無法播放時，返回詳情後仍可重試或選其他集。 |
| 視窗與滑鼠 | 縮到最小尺寸再放大，按鈕、集數與分頁不超出視窗，方向鍵移到清單末端時焦點仍可見。滑鼠點選、捲動、分頁、返回可用。 |

控制項自動隱藏與重新顯示（滑鼠或鍵盤）已通過驗收，不需重測。

## 7. Correction budget history

Correction budget: 8 documented rounds were used in the original contract (5 rounds, then extensions to 6, 7 and 8); further owner-requested fixes (episode click, Enter/idle controls, hidden overlay, diagnostic, native diagnostic repair, swap-feedback fix) were each run under a scope the owner extended explicitly. The exact total after round 8 is not recorded in the sources.
