<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-21. Branch `main`.

## Current state

Loops 1 to 35 pass. Loop 35 (Cache-first startup and refresh) was accepted on 2026-10-21: Home shows stored data with stored episode numbers and makes no network call; one refresh policy in core (at most 4 requests, background uses 3, per-request timeouts, 10-minute throttle, cancelled and waited for on shutdown); a calm offline line on Home when no followed anime can be reached. The owner accepted the offline check without disconnecting. Owner decision: no video downloads or offline playback, ever. Details: [loops/LOOP35.md](loops/LOOP35.md); commit 8bb524b, pushed with the resize hotfix 0371f05 (clicks and layout work at any window size) and Loop 34b 18b6c6a (follow from 歷史紀錄 and the player, [loops/LOOP34B.md](loops/LOOP34B.md)) in ef97b6c; CI Go 1.26.10 for GO-2026-6809 (92868eb); CI 39532702396 green. Next is Loop 36 (Autoplay next episode), which needs owner approval to start. Loop 34 (Keyboard navigation everywhere) was accepted on 2026-10-20: one key policy for the main window (Esc goes back and never closes the app, `/` and Ctrl+K open 搜尋 on every screen, Enter and Space activate, arrows move focus with scrolling), focus kept across reloads and return from the player; in the player Esc leaves playback and `/` or Ctrl+K open 搜尋 (owner decisions). Details: [loops/LOOP34.md](loops/LOOP34.md); commit c2ad61a, CI 39496901879 green. Loop 33 (Schedule) was accepted on 2026-10-20: 時間表 is a week grid from today built from Bangumi's weekly calendar (owner decisions: Bangumi instead of Anime1's season pages, a grid instead of a list), cached first in migration 0006 and read by the new CachedSchedule action. A show opens its anime when stored Bangumi metadata or the exact title and season matches, including Anime1's catalog; any other show opens 搜尋 with its title. New core port ScheduleProvider; the Anime1 schedule was removed. Details: [loops/LOOP33.md](loops/LOOP33.md); commit d7285ab, CI 39382115519 green. Two pre-existing flakes found while closing it are fixed (2026-10-20): the Following test waits a clock tick between episode lists (02218a2), and the playback event relay drops a queued old-owner event that a terminal clears while it waits for the consumer (9ef3c66; the -race test failed about 3 in 100 runs before). Loop 32 (History and Continue Watching) was accepted on 2026-10-19: 歷史紀錄 page (one row per anime with episode, progress and last-played day; a row continues at the saved position; an X icon removes the anime's history and its saved resume positions), local only, no migration. Owner decision the same day: the 追蹤 page shows only the watched episode and position, with no 有新集數 badge. Details: [loops/LOOP32.md](loops/LOOP32.md); commit 606b914, CI 39327866863 green; row-list refactor c7cac18. Pre-release v0.1.0-alpha.2 ([releases/v0.1.0-alpha.2.md](releases/v0.1.0-alpha.2.md)). Loop 31 (Following) was accepted on 2026-10-14: 追蹤 page with latest and watched episode and a 有新集數 badge from local data (shows when Anime1 is unreachable), episode numbers stored by migration 0004, follow and unfollow as a star icon. Commits a1a6dc6 and 6373957 (failure-log flake fix); CI 38660646062 green. Follow-up f7dffd5 (2026-10-15, owner-approved): Home's first section is 近期更新 (followed anime with an episode first seen in the last 7 days, migration 0005), then 繼續觀看; CI 38681263489 green. Pre-release v0.1.0-alpha.1 was published on 2026-10-15 by a tag-only CI release job ([loops/PRERELEASE_0.1.0-alpha.1.md](loops/PRERELEASE_0.1.0-alpha.1.md)). Loop 30 (anime detail and episode UI) was accepted on 2026-10-14: smooth playback (missing `fltk.Lock()` fixed), playback failures shown and logged in `playback-failures.log`, AniList/Bangumi cover and synopsis in Taiwan Traditional Chinese, general title matching, ascending episodes.

Platform: Windows amd64 only. Native tests run locally (about 90 seconds including race) and in GitHub Windows CI.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 35 | PASS. Loop 35 details: [loops/LOOP35.md](loops/LOOP35.md) |
| 36 to 38 | NOT_STARTED (roadmap shortened 2026-10-14) |

## Open items

| Item | Next action |
| --- | --- |
| 追蹤 更新集數 wording; core/backend `NewEpisode` unused by the UI | Owner to decide whether to reword the refresh button and notes, and whether to remove `NewEpisode`. |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; decide on an automatic retry from the recorded stage and status. |

## Settled owner decisions (2026-10-13)

- The player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K opens 搜尋 on every screen, including the player (implemented in Loop 34).

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
