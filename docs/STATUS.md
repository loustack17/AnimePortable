<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-19. Branch `main`.

## Current state

Loops 1 to 32 pass. Loop 32 (History and Continue Watching) was accepted on 2026-10-19: 歷史紀錄 page (one row per anime with episode, progress and last-played day; a row continues at the saved position; an X icon removes the anime's history and its saved resume positions), local only, no migration. Owner decision the same day: the 追蹤 page shows only the watched episode and position, with no 有新集數 badge. Details: [loops/LOOP32.md](loops/LOOP32.md); commit 606b914, CI 39327866863 green; row-list refactor c7cac18. Pre-release v0.1.0-alpha.2 ([releases/v0.1.0-alpha.2.md](releases/v0.1.0-alpha.2.md)). Loop 31 (Following) was accepted on 2026-10-14: 追蹤 page with latest and watched episode and a 有新集數 badge from local data (shows when Anime1 is unreachable), episode numbers stored by migration 0004, follow and unfollow as a star icon. Commits a1a6dc6 and 6373957 (failure-log flake fix); CI 38660646062 green. Follow-up f7dffd5 (2026-10-15, owner-approved): Home's first section is 近期更新 (followed anime with an episode first seen in the last 7 days, migration 0005), then 繼續觀看; CI 38681263489 green. Pre-release v0.1.0-alpha.1 was published on 2026-10-15 by a tag-only CI release job ([loops/PRERELEASE_0.1.0-alpha.1.md](loops/PRERELEASE_0.1.0-alpha.1.md)). Next is Loop 33 (Schedule), which needs owner approval to start. Loop 30 (anime detail and episode UI) was accepted on 2026-10-14: smooth playback (missing `fltk.Lock()` fixed), playback failures shown and logged in `playback-failures.log`, AniList/Bangumi cover and synopsis in Taiwan Traditional Chinese, general title matching, ascending episodes.

Platform: Windows amd64 only. Native tests run locally (about 90 seconds including race) and in GitHub Windows CI.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 32 | PASS. Loop 32 details: [loops/LOOP32.md](loops/LOOP32.md) |
| 33 to 38 | NOT_STARTED (roadmap shortened 2026-10-14) |

## Open items

| Item | Next action |
| --- | --- |
| 追蹤 更新集數 wording; core/backend `NewEpisode` unused by the UI | Owner to decide whether to reword the refresh button and notes, and whether to remove `NewEpisode`. |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; decide on an automatic retry from the recorded stage and status. |

## Settled owner decisions (2026-10-13)

- The player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
