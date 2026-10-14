<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-13. Branch `main`. Docs restructure committed as `f790dfc`; legacy cleanup committed after it (not yet pushed).

## Current state

Loops 1 to 29 pass. Loop 30 (anime detail and episode UI) is in progress: it builds and CI is green, but the owner has accepted only the player controls. The video stutter is fixed (missing `fltk.Lock()`; the owner confirmed smooth playback on 2026-10-14). Playback-start failures now show a dialog and are recorded in `playback-failures.log`.

Platform: Windows amd64 only. Native tests run locally (about 90 seconds including race) and in GitHub Windows CI.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 29 | PASS |
| 30 | IN_PROGRESS. Details: [loops/LOOP30.md](loops/LOOP30.md) |
| 31 to 42 | NOT_STARTED |

## Open items

| Item | Next action |
| --- | --- |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; decide on an automatic retry from the recorded stage and status. |
| Loop 30 owner checks | Owner checks the detail page, episode click/Enter/Space, Continue, back/retry, cover and minimum window size. |

## Settled owner decisions (2026-10-13)

- PLAY-009: the player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
