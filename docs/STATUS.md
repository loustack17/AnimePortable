<!-- SPDX-License-Identifier: MPL-2.0 -->

# Status

Updated 2026-10-26. Branch `main`.

## Current state

v1.0.0, the first full release, was published on 2026-10-24 ([releases/v1.0.0.md](releases/v1.0.0.md)). Loop 38R, a behaviour-preserving cleanup before version 2.0, passed on 2026-10-26: duplicated and dead code removed in every package (production Go 24,774 lines, down 572), three known bugs handled (O1-O3), and an owner check of a build with slices S1-S8 passed ([loops/LOOP38R.md](loops/LOOP38R.md)). Version 2.0 (Loops 39-46 in [04_ROADMAP.md](04_ROADMAP.md)) needs owner approval to start.

Platform: Windows amd64 only. The full native suite, including the window and real-libmpv tests, runs only locally; GitHub Windows CI runs the rest on every push.

## Loops

| Loops | Status |
| --- | --- |
| 1 to 38, 37b, 37c, 37d, 38R | PASS. Recent: [loops/LOOP38R.md](loops/LOOP38R.md), [loops/LOOP38.md](loops/LOOP38.md); history in [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md) |
| 39-46 (version 2.0) | NOT_STARTED; needs owner approval. [04_ROADMAP.md](04_ROADMAP.md) |

## Open items

| Item | Next action |
| --- | --- |
| Loop 38R owner decisions O4-O8 | Owner to decide: remove the legacy AppData import (O4), unused UI text keys (O5) and test-only port fields (O6); split `main()` (O7); move shared limits into core (O8). Details in [loops/LOOP38R.md](loops/LOOP38R.md). |
| 追蹤 更新集數 wording; core/backend `NewEpisode` unused by the UI | Owner to decide whether to reword the refresh button and notes, and whether to remove `NewEpisode`. |
| Playback failures (suspected intermittent Anime1 HTTP 502) | Read `playback-failures.log` after the next failure; it now names forward-proxy failures. Decide on an automatic retry from the recorded stage and status. |
| Update check and pre-releases | 檢查更新 offers the newest release including pre-releases, so a later beta would show 有新版本 to users on 1.0.0. Owner to decide. |
| 03_SECURITY close order | The doc lists invalidating the proxy session before destroying the player; the code destroys the player first. Owner to decide which the doc states. |
| Versions after 1.0.0 | Fix-only releases raise PATCH (v1.0.1, owner 2026-10-27); the rule for feature releases is not decided. |
| Resource harness, real-libmpv scenario | Its private-bytes check (first five vs last five samples, 8 MiB) fails on this PC for v1.0.0 and the current code alike, because the first samples sit lower than the plateau. Owner to decide whether to adjust the check's warm-up. |
| Synopsis misses left by Loop 37d | Dr.STONE 新石紀 SCIENCE FUTURE (Anime1 merges three parts), Battle Spirits [Re] 絕界之空 (絶/絕), 紫羅蘭永恆花園 外傳 (not returned by the Simplified search). Owner to decide whether to pursue. |

## Settled owner decisions

- The player runs with `config=no`. The user's `mpv.conf` is not loaded (2026-10-13).
- Settings has no MPV path setting. The app uses the bundled libmpv DLL, with a documented override (2026-10-13).
- Ctrl+K opens 搜尋 on every screen, including the player (2026-10-13).
- Light testing for this small project: run the focused check once and the full suite at most once; no repeated or stress runs (2026-10-26).

## Links

- [AGENT_HANDOFF.md](AGENT_HANDOFF.md): recovery file for a new session
- [archive/IMPLEMENTATION_STATUS.md](archive/IMPLEMENTATION_STATUS.md): full history, including Loops 30-38
