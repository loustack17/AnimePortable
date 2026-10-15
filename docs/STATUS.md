<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-14. Branch `main`.

## Current state

Loops 1 to 31 pass. Loop 31 (Following) was accepted on 2026-10-14: 追蹤 page with latest and watched episode and a 有新集數 badge from local data (shows when Anime1 is unreachable), episode numbers stored by migration 0004, follow and unfollow as a star icon. Commits a1a6dc6 and 6373957 (failure-log flake fix); CI 38660646062 green. Next is Loop 32 (History and Continue Watching), which needs owner approval to start. Loop 30 (anime detail and episode UI) was accepted on 2026-10-14: smooth playback (missing `fltk.Lock()` fixed), playback failures shown and logged in `playback-failures.log`, AniList/Bangumi cover and synopsis in Taiwan Traditional Chinese, general title matching, ascending episodes.

Platform: Windows amd64 only. Native tests run locally (about 90 seconds including race) and in GitHub Windows CI.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 31 | PASS. Loop 31 details: [loops/LOOP31.md](loops/LOOP31.md) |
| 32 to 38 | NOT_STARTED (roadmap shortened 2026-10-14) |

## Open items

| Item | Next action |
| --- | --- |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; decide on an automatic retry from the recorded stage and status. |

## Settled owner decisions (2026-10-13)

- The player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
