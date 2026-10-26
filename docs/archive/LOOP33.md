<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 33: Schedule

Status: PASS (owner accepted 2026-10-20). Commit d7285ab, GitHub Windows CI 39382115519 green.

## Goal

The user can read the airing schedule.

## Scope

In scope:

- The 時間表 page (the existing navigation entry, now built) as a week grid (owner decision 2026-10-20, replacing the list): seven columns starting today (highlighted, 「週二 / 今天 10/20」), one card per show with the title on up to two lines and 「開啟作品」 or 「搜尋片源」 under it. Arrow keys move between cards (Left and Right to the nearest day with shows, Up and Down within a day; Up from the first card reaches 重新整理, Left from the first day the navigation); Enter or a click acts on the card.
- Source (owner decision 2026-10-20, after owner check 1): Bangumi's weekly calendar (`https://api.bgm.tv/calendar`), not Anime1. Bangumi lists each show with its airing weekday; the page lays the week onto the next 7 days. Titles are converted to Taiwan Traditional Chinese with the embedded OpenCC dictionaries, as for metadata.
- Cache first: entering the page shows the stored schedule at once, then fetches Bangumi in the background (at most every 10 minutes, or on 重新整理) and redraws when the fetch lands. A failed fetch keeps the stored rows and says so; with no stored rows it asks for 重新整理.
- Open (owner decision 2026-10-20): a show the app knows opens its detail screen; Back and Escape return to 時間表 with focus on that card. A show is known when an anime's stored Bangumi metadata has its subject, or its title and season equal exactly one anime's (seasons are kept apart: 第二季 never opens season 1). Each refresh also matches Anime1's full catalog this way and stores those anime, so they open even if never searched. Any other show opens 搜尋 with its title (season suffix dropped, for example 「藥屋少女的呢喃」) and runs the search. A known show shows the local title.
- Storage (owner decision 2026-10-20): migration 0006 adds `schedule_entries` (the weekly calendar, at most 512 entries); each successful fetch replaces it. A new read-only backend action `CachedSchedule` reads it without the network and is in the main_test allowlist. `Schedule` now takes no range.
- A new core port `ScheduleProvider` (implemented by the Bangumi adapter). The Anime1 schedule (Loop 8: `AnimeSource.Schedule`, its parser and contract tests) is removed, since nothing uses it.
- The list pages share one page-resize helper with the grid; 追蹤 and 歷史紀錄 are unchanged.

Out of scope: notifications, a month view, airing times and episode numbers (Bangumi's calendar gives only the weekday), follow buttons on schedule rows, fuzzy title matching, new dependencies, workflow or release changes.

## Tests

- Core: title matching is unique, ignores spacing and a standalone x, keeps seasons apart, never overrides a stored-metadata match, never guesses between two anime with one title; the search query drops season and arc suffixes.
- Bangumi: weekday mapping (7 is Sunday), Simplified to Traditional titles, empty Chinese name falls back, invalid items and duplicates skipped, more than 512 rejected, malformed or non-JSON bodies rejected, errors do not leak, cancellation.
- SQLite: migration 0006 on old data; replace and read round-trip, order, reopen, metadata join fills the anime, invalid input and corrupt rows fail closed.
- Backend: `Schedule` stores and returns matched items with the local title; catalog anime with the exact title and season are stored and linked, a catalog failure still refreshes; a failed or oversized fetch keeps the cache; `CachedSchedule` works offline after restart without calling the provider.
- UI: week from today with seven columns, cached cards before the fetch, fetch result replaces them, failure keeps them, empty and failure states, equal columns at three window sizes, arrow focus skips empty days and leaves to the navigation, focus kept across a reload, a known show opens the detail screen and Back returns to its card, an unknown show opens 搜尋 with its query and runs the search.
- Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Open 時間表, see items immediately, and open one with Enter.

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 `9ECD9889…FE7A`; grid checked on the owner's data by screenshot at the default and minimum window, 109 shows, `artifacts/evidence/loop33/grid*.png`; libmpv DLL matches the pin; existing `data/` kept, backup in `artifacts/evidence/loop33/data-backup-before-loop33/`).

1. 開啟程式，點左側「時間表」。應看到一週七欄的時間表，第一欄是「今天」，每部作品一張卡片，標題為繁體中文。
2. 按「首頁」，再回到「時間表」；或關閉程式重新開啟後再進入。卡片應立刻出現，之後自動更新。
3. 用方向鍵在卡片間移動（左右換日、上下換作品）。選一張寫著「開啟作品」的卡片按 Enter，應開啟該作品詳情，按鈕為「返回時間表」。
4. 按 Esc 或「返回時間表」。應回到時間表，焦點停在剛才那張卡片。
5. 選一張寫著「搜尋片源」的卡片按 Enter。應切到「搜尋」並已填入作品名稱、開始搜尋。
## Status

- Implementation done (not committed); based on origin/main 8a704d6.
- Owner check 1 (2026-10-20): the first build read Anime1's season page, which does not exist for autumn 2026 (404), so the page showed a retry error. Owner decision: the schedule must come from another source (Anime1 only happens to host the shows); the owner chose Bangumi over AniList and over both combined.
- Live check 2026-10-20: the Bangumi adapter returned 109 shows in Traditional Chinese.
- First review (Anime1 design): no MUST_FIX; its fixes (focus restore on repeated rows, Back focus, status redraw found by race) carry over.
- Local: gofmt and vet clean; full native suite and race green (nuwen msvcrt MinGW gcc 15.2).
- Independent review (Bangumi design): one MUST_FIX, fixed: a Bangumi subject without a Chinese name showed its original name, which is Simplified Chinese for Chinese works; it is now converted unless it is Japanese (test with 凡人修仙传). SHOULD_FIX, fixed: one title that the store would reject (non-canonical text) blocked every refresh, now that item is skipped; focus on an unknown row jumped to the first unknown row after a reload. Nits fixed: tooltip on unknown rows, doc wording. Not changed: the backend keeps its own 512 check (core policy constant); 今天/明天 refresh on the next load, not at midnight; title matching ignores season, but needs an exact, unique title.
- Owner approval 2026-10-20: the new core port `ScheduleProvider` (docs/02_ARCHITECTURE.md now lists five ports) and removing the Anime1 schedule and its tests (Anime1 `TestSchedule*`, the backend malformed-schedule test, the contract schedule validators; all Anime1-schedule only).
- TypeSafe scope-check 2: criteria_covered 1.83/2; in_scope 1.42/2 (conf 0.25, REVIEW) and verifier_weakened 0.78, both from the owner-approved source change and test removal above; adjudicated as approved, not weakened. Evidence in `artifacts/evidence/loop33/`.
- Owner check 2 (2026-10-20): the app showed 「無法在此資料夾開啟資料…」. Cause: the first test build had applied the earlier 0006 (`schedule_items`, never released) to the owner's `data/`; 0006 was then rewritten, so the migration checksum no longer matched. Repair: the empty `schedule_items` table and its migration record were removed and the new 0006 applied; history (8 entries) and all other data kept. Broken copy kept in `artifacts/evidence/loop33/data-before-0006-repair/`.
- Owner check 3 (2026-10-20): the app opened again (repair worked). Owner decisions: show the schedule like Google Calendar (week grid from today), and make 「尚未找到片源」 lead to the app's own sources, reusing search. Live measure: of 109 Bangumi shows, 25 match an Anime1 catalog title exactly; many others are not on Anime1 (Chinese and Western shows) or use other names, and loose matching linked wrong entries (航海王 to a movie), so only exact, season-aware matches link and the rest search.
- Pre-existing flake (not caused by this loop): `core` TestTrackedRelayDropsQueuedOldOwnerEventsBeforeTargetTerminal fails under -race about 1 in 4 runs, also on the untouched base 8a704d6 (2 of 8). Proposed as a separate fix.
- Review (week grid and catalog linking): no MUST_FIX. Fixed: a refresh could be reported as failed after it was stored (UI timeout below Bangumi plus catalog; the catalog step is now best effort and the timeout 45 s); catalog positions are bounds-checked; columns keep a minimum width (90 px) and the grid scrolls sideways in a narrow window, with the right margin clear of the scrollbar; full-width spaces split the search query; the today color follows the model. Accepted: Back from a search result returns to 搜尋, not 時間表; a title with 「第一季」 does not match one without a season.
- Local after fixes: gofmt and vet clean; full native suite and race green.
- Owner check 4 (2026-10-20): PASS for steps 1-5 on the week grid (TypeSafe owner-report HUMAN_PASS, conf 0.99).
