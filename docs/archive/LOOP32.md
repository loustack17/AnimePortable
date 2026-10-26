<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 32: History and Continue Watching

Status: PASS (owner accepted 2026-10-19). Commit 606b914, GitHub Windows CI 39327866863 green. Follow-up c7cac18 shares the 追蹤/歷史紀錄 row list (behavior unchanged). Released in v0.1.0-alpha.2.

## Goal

The user can see watch history and resume where they stopped.

## Scope

In scope:

- The 歷史紀錄 page (the existing navigation entry, now built): one row per anime, its most recently played episode, with title, episode number, progress (position / duration, or 已看完) and last-played time, newest first.
- Continue: clicking a row (or Enter on it) plays that episode at the saved position. A completed episode plays from the start.
- Remove: a remove icon at the right of each row (tooltip 移除紀錄) removes that anime's history. Owner decision 2026-10-19: removal also deletes the anime's saved resume positions, so replaying starts from the beginning and nothing of the entry comes back until the anime is played again.
- Home 繼續觀看 keeps its rules (newest resumable episode per anime) and reloads when entering Home, so a removed anime disappears there too.
- Episode numbers come from stored episode mappings (local), not from Anime1, so the page loads at once and offline.

Built on existing data and policy: `playback_history` and `playback_progress` (checkpoints already survive restart), the backend `History` and `RemoveHistory` actions (no new action; the main_test allowlist stays), core `episodeNumbers`, and the playback request path Home uses. No schema migration.

Out of scope: cloud sync or any export, per-episode removal, a confirmation dialog, changing Home's episode-number lookup, Schedule, new dependencies, workflow or release changes.

## Tests

- SQLite: `RemoveHistory` deletes history and progress for that anime only, survives reopening, is idempotent.
- Core: `ListHistory` adds stored episode numbers; uses only the store; errors and cancellation propagate.
- Backend: History returns episode numbers; progress survives reopening the store; History and RemoveHistory work while the source fails (history never leaves the device); a removed entry stays removed after reopening.
- UI model: one row per anime, newest first, row text for resumable, completed and unknown-number cases; the play request resumes at the saved position, or 0 when completed.
- UI (native): the page lists rows, empty and failure states, a row starts playback with the saved position, remove deletes the row and keeps focus nearby, the page reloads when shown again.
- Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Watch part of an episode, restart, continue from Home or 歷史紀錄, then remove an entry.

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 `A5C4A37A…B736`; libmpv DLL matches the pin; existing `data/` kept, backup in `artifacts/evidence/loop32/data-backup-before-loop32/`).

1. 開啟程式，從首頁或搜尋開啟一部作品，播放一集約一兩分鐘後按 Esc 返回。
2. 關閉程式再重新開啟。首頁「繼續觀看」應有這部作品，顯示集數與上次播放位置。
3. 點左側「歷史紀錄」。應列出這部作品、集數、進度（例如 1:30 / 24:00）與「上次觀看：今天」。
4. 在「歷史紀錄」點這部作品（或用 Tab 選到後按 Enter）。應從剛才的位置繼續播放，而不是從頭。
5. 按 Esc 返回後，再到「歷史紀錄」，按該列右側的 ✕ 圖示。該列消失，焦點留在附近的列。
6. 回首頁：這部作品不再出現在「繼續觀看」。重新開啟程式後，它仍不在歷史紀錄中；從作品頁再播放該集時從頭開始。

## Status

- Release v0.1.0-alpha.2 (2026-10-19): tag on 85fbbc0, release run 39337289767 green; ZIP files checked, libmpv DLL matches the pin, smoke start OK. Owner downloaded, unzipped and ran it: PASS (TypeSafe owner-report HUMAN_PASS, conf 1.00).
- Owner check 2 (2026-10-19): PASS for the 追蹤 page; owner approved commit and push (TypeSafe owner-report HUMAN_PASS, conf 1.00).
- Implementation done. Local: gofmt and vet clean; full native suite and race green (nuwen msvcrt MinGW gcc 15.2); origin/main unchanged since 45e949f, no newer migration.
- Independent review: no MUST_FIX. Fixed: rows from an old or failed load could still be played at a stale or removed position (now blocked while loading; a failed load drops the rows); last-row removal focuses the 歷史紀錄 button; port comment; backend test removes while offline and checks kept progress. Deferred: the 歷史紀錄 list mirrors the 追蹤 list code (render, place, resize, focus restore); sharing it needs edits to Loop 31 files.
- Removing an anime's history also resets its 追蹤 watched state (看到 / 有新集數), because that state is read from history.
- TypeSafe: in_scope 1.88/2; verifier_weakened 0.35 adjudicated NOT_WEAKENED (owner-decided expectation change, coverage kept); evidence in `artifacts/evidence/loop32/`.
- Owner check 1 (2026-10-19): PASS for 歷史紀錄 (TypeSafe owner-report HUMAN_PASS, conf 0.74). The owner asked why 追蹤 shows 有新集數 while Home 近期更新 is empty: by design (Loop 31 rules), 有新集數 means a higher episode than the watched one is stored, while 近期更新 only counts an episode first seen in the last 7 days after the first stored list. Owner request: the 有新集數 badge sat too high; it is now vertically centered on the row, and the status line no longer runs under it. Owner confirmed the centered badge.
- Owner decision 2026-10-19 (replaces the badge): the 追蹤 page shows no new or unwatched hint. No 有新集數 badge, no new-first ordering (rows sort by title), and the status reads "追蹤 N 部。". Each row shows only the episode watched up to and its saved position, for example "看到：第 10 集 · 7:52 / 24:00", "看到：第 1 集 · 已看完" or "尚未觀看". An episode that was only opened shows no position. Home 近期更新 is unchanged. The 追蹤 page now also loads History; a History failure shows the existing load-failure state. Review: no MUST_FIX; fixed a doubled 看到 for an unknown duration and the opened-only case; tests updated so NewEpisode provably no longer reorders rows. Open for the owner: the 更新集數 button and its notes still mention 最新集數 (the refresh feeds Home 近期更新 and the stored episode numbers); core/backend `NewEpisode` is no longer used by the UI. Test EXE SHA-256 `CF73DDA9…E26B`; recheck of the 追蹤 page pending.
