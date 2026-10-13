<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-13. Branch `main`, last commit `e8f0640`.

## Current state

Loops 1 to 29 pass. Loop 30 (anime detail and episode UI) is in progress: it builds and CI is green, but the owner has accepted only the player controls. Video stutter is still open; the latest fix (`e8f0640`) has not been tested by the owner. The docs restructure is finished and waits for owner review before it is committed.

Platform: Windows amd64 only. Native Windows tests run only in GitHub Windows CI. The local cgo build is blocked; do not retry it.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 29 | PASS |
| 30 | IN_PROGRESS. Details: [loops/LOOP30.md](loops/LOOP30.md) |
| 31 to 42 | NOT_STARTED |

## Open items

| Item | Next action |
| --- | --- |
| Docs commit | Owner reviews the diff and approves the commit. |
| Legacy code and directories | After the docs commit, remove them as a separate verified change. `docs/archive/` is kept. |
| Loop 30 stutter fix `e8f0640` (CI run 38444662617 is green) | Deliver the CI build. Owner plays the same clip for 30 to 60 seconds and reports. |
| Loop 30 owner checks | Owner checks the detail page, episode click/Enter/Space, Continue, back/retry, cover and minimum window size. |

## Settled owner decisions (2026-10-13)

- PLAY-009: the player runs with `config=no`. The user's `mpv.conf` is not loaded.
- Loop 37 Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override.
- Ctrl+K stays a target key binding. It is not implemented yet.

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history
