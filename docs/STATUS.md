<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-14. Branch `main`.

## Current state

Loops 1 to 30 pass. Loop 30 (anime detail and episode UI) was accepted on 2026-10-14: smooth playback (missing `fltk.Lock()` fixed), playback failures shown and logged in `playback-failures.log`, AniList/Bangumi cover and synopsis in Taiwan Traditional Chinese, general title matching, ascending episodes. Next is Loop 31 (Schedule UI), which needs owner approval to start.

Platform: Windows amd64 only. Native tests run locally (about 90 seconds including race) and in GitHub Windows CI.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 30 | PASS. Loop 30 details: [loops/LOOP30.md](loops/LOOP30.md) |
| 31 to 42 | NOT_STARTED |

## Open items

| Item | Next action |
| --- | --- |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; decide on an automatic retry from the recorded stage and status. |

## Settled owner decisions (2026-10-13)

- PLAY-009: the player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
