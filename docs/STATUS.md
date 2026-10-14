<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-13. Branch `main`. Docs restructure committed as `f790dfc`; legacy cleanup committed after it (not yet pushed).

## Current state

Loops 1 to 29 pass. Loop 30 (anime detail and episode UI) is in progress: it builds and CI is green, but the owner has accepted only the player controls. Video stutter is still open; the latest fix (`e8f0640`) has not been tested by the owner. The docs restructure and the legacy cleanup are committed locally. They need owner approval to push, then a green GitHub Windows CI run.

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
| Push docs restructure and legacy cleanup | Owner approves the push; then confirm GitHub Windows CI is green. Details: [loops/LEGACY_CLEANUP.md](loops/LEGACY_CLEANUP.md) |
| Loop 30 stutter fix `e8f0640` (CI run 38444662617 is green) | Deliver the CI build. Owner plays the same clip for 30 to 60 seconds and reports. |
| Loop 30 owner checks | Owner checks the detail page, episode click/Enter/Space, Continue, back/retry, cover and minimum window size. |

## Settled owner decisions (2026-10-13)

- PLAY-009: the player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
