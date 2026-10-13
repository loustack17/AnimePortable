<!-- SPDX-License-Identifier: MPL-2.0 -->

# Roadmap

Loops 1-29 are complete. Loops 30-42 remain. Criteria are in [06_ACCEPTANCE.md](06_ACCEPTANCE.md). Loop rules and budgets are in [05_WORKFLOW.md](05_WORKFLOW.md). Old plans are in `archive/04_MVP_IMPLEMENTATION_PLAN.md` and `archive/16_FYNE_MIGRATION_PLAN.md`.

Platform order: the whole Windows product first (Loops 30-39, 41, 42 on Windows), then Linux once Windows is stable, then macOS once Linux is stable. Each platform needs its own native runtime, keyboard and mouse, portability, resource and human acceptance before it is called supported. Linux and macOS builds are diagnostics only until then.

Every loop uses the FLTK Windows UI (FLTK is a native desktop UI toolkit) and in-process libmpv (the mpv video player library, loaded inside the app). Every UI loop must keep: all content reachable by mouse and keyboard with visible focus, cached data shown first, no raw remote HTML in the UI, and no WebView.

## Completed loops (all PASS)

Loops 11 and 12 describe the original external-player design, which Loop 28 replaced with in-process libmpv. "M0" to "M4" are the five steps of the native UI migration (Loops 24-28).

| Loop | Scope |
| --- | --- |
| 1 | Repository and contracts. |
| 2 | Core models and ports (interfaces the core owns). |
| 3 | Adapter contract tests. |
| 4 | Secure HTTP foundation. |
| 5 | Anime1 catalog and search. |
| 6 | Anime1 episodes. |
| 7 | Anime1 playback resolver. |
| 8 | Anime1 schedule. |
| 9 | Anime1 adapter acceptance. |
| 10 | Secure loopback playback proxy. |
| 11 | Player process lifecycle (external player, replaced in Loop 28). |
| 12 | Player control channel (external player, replaced in Loop 28). |
| 13 | Same-session episode switching. |
| 14 | SQLite store. |
| 15 | Progress and history. |
| 16 | Following. |
| 17 | AniList metadata. |
| 18 | Bangumi metadata. |
| 19 | Metadata normalization and matching. |
| 20 | Remote metadata content security. |
| 21 | Desktop binding layer (Wails; removed later). |
| 22 | UI shell (Svelte; removed later). |
| 23 | Home screen and loop-protocol completion. |
| 24 | M0: native UI contract, parity inventory, resource reference. |
| 25 | M1: framework-neutral desktop lifecycle, shell and Home. |
| 26 | M2: desktop actions and data ported to framework-neutral Go. |
| 27 | M3: Windows portable packaging; Wails, Svelte and NSIS (an installer tool) removed. |
| 28 | M4: Windows FLTK and libmpv integrated gate (ADR-023, CI-010). |
| 29 | Search UI. |

## Remaining loops

Loop 30 is in progress. Loops 31-42 are not started. Every loop uses the same fields. "Criteria" lists acceptance criteria that must pass, each with a short label; full wording is in [06_ACCEPTANCE.md](06_ACCEPTANCE.md). "Owner check" is the simple manual check the owner performs.

### Loop 30 - Anime detail and episode UI

- Goal: The user can open an anime, read its details and play any episode from the episode list.
- In scope: Cover, title, native title, synopsis, season and year, studio and episode count. A Play/Continue button. An episode list where clicking an episode plays it at once, with no separate Play button. Enter opens a detail page. Enter or Space on a focused episode row plays it. Continue appears only when that series has valid resumable progress. The current episode and watch progress are visible without clutter.
- Out of scope: Schedule, following and history screens; settings.
- Done when: The detail page shows the metadata above, episodes play by click and by keyboard, and Continue appears only after part of an episode was watched.
- Criteria: FUNC-004 (detail content), FUNC-005 (episode order), PLAY-012 (resume works), UX-002 (visible focus), UX-003 (Enter opens or selects), UX-004 (Space plays), SEC-014..SEC-018 (remote HTML and images handled safely), PLAY-004 (no credentials to the UI), PLAY-006 (playback through loopback proxy).
- Owner check: Open an anime, see its details, play an episode by click and by keyboard, and see Continue only after watching part of an episode.

### Loop 31 - Schedule UI

- Goal: The user can read the airing schedule.
- In scope: A chronological list grouped by day. Cached data shows first and refreshes in the background. Selecting an item opens that anime when the app knows it.
- Out of scope: Notifications and a calendar grid.
- Done when: The schedule screen lists items by day immediately from cache, then updates, and Enter on an item opens its anime.
- Criteria: FUNC-006 (schedule available), PERF-001 (startup not blocked by Anime1), PERF-002 (startup not blocked by AniList or Bangumi), UX-002 (visible focus), UX-003 (Enter opens or selects).
- Owner check: Open Schedule, see items immediately, and open one with Enter.

### Loop 32 - Following UI

- Goal: The user can follow and unfollow anime and see which have new episodes.
- In scope: A followed-anime list showing the latest episode, the watched episode and a new-episode indicator. Follow and unfollow actions.
- Out of scope: Notifications.
- Done when: Following survives an app restart and the indicator matches the real episode state.
- Criteria: FUNC-007 (following works), ARCH-012 (local IDs, not Anime1 IDs), ARCH-014 (source failure keeps cache usable), UX-002 (visible focus).
- Owner check: Follow an anime, restart the app, and see it still followed with the correct episode state.

### Loop 33 - History and Continue Watching UI

- Goal: The user can see watch history and resume where they stopped.
- In scope: A list of anime, episode, progress and last-played time. A continue action. Removing an entry from history.
- Out of scope: Cloud sync.
- Done when: Progress survives restart, Continue resumes at the saved position, and removed entries stay removed.
- Criteria: FUNC-008 (history works), FUNC-009 (Continue Watching works), PLAY-011 (progress survives restart), PLAY-012 (resume works), PRIV-006 (history stays local).
- Owner check: Watch part of an episode, restart, continue from Home or History, then remove an entry.

### Loop 34 - Keyboard navigation across all flows

- Goal: The user can do the whole core workflow with the keyboard only.
- In scope: Arrow keys, Enter, Space, Esc, `/` and Ctrl+K on every completed screen. The workflow launch, find, open, select episode, play must work with no mouse.
- Out of scope: New screens.
- Done when: Every completed screen responds to those keys with visible focus and no keyboard trap, and the mouse still works.
- Criteria: UX-001..UX-010 (keyboard workflow, focus, shortcuts, mouse, calm UI), PLATFORM-007 (keyboard workflow validated; Windows only for now).
- Owner check: Launch the app, find an anime, open it, select an episode and play it using only the keyboard.

### Loop 35 - Cache-first startup and refresh

- Goal: The app starts from cached data and does not need the Internet.
- In scope: The startup path through the App, SQLite and UI. Background refresh of Anime1, schedule and metadata with a bounded number of parallel requests. Cached content shows while refreshing, and the UI updates incrementally. A normal cached startup shows no full-screen loading blocker.
- Out of scope: New data sources.
- Done when: With the network off, the app shows cached content within about 1 second, and refresh work is bounded and cancellable.
- Criteria: FUNC-001 (opens on cache), PERF-001..PERF-005 (startup, refresh and image bounds), META-006 (metadata usable from cache), ARCH-014 (source failure keeps cache usable), RES-007 (bounded caches), RES-010 (cancelled work stops).
- Owner check: Disconnect the network, start the app, and see cached content within about 1 second.

### Loop 36 - Autoplay next episode

- Goal: When an episode ends, the next one starts in the same player.
- In scope: On a normal end of file, resolve and load the next episode in the same libmpv instance. With no next episode, stop cleanly. Persist progress. A failed resolve does not loop or spam retries.
- Out of scope: Playlist editing.
- Done when: The next episode starts without a new window, the last episode stops cleanly, and a failed resolve stops after one reported error.
- Criteria: PLAY-007 (one player per session), PLAY-008 (episode switch reuses player), PLAY-013 (autoplay next), PLAY-010 (progress tracked from player events), PLAY-011 (progress survives restart), RES-008 (no memory growth on repeated switches).
- Owner check: Let an episode finish and see the next one start without a new window.

### Loop 37 - Settings

- Goal: The user can change a few basic settings and they are kept.
- In scope: Appearance (System, Light, Dark), autoplay next episode, resume playback, and language (Traditional Chinese, English). No power-user player options.
- Out of scope: Any player-path setting. The DLL override in PLAY-002 is documented, not a UI setting, unless the owner asks.
- Done when: Each setting changes behavior and is still set after restart, with no account or telemetry.
- Criteria: PLATFORM-009 (data in portable folder), UX-010 (calm, minimal UI), PRIV-001..PRIV-005 (no account, telemetry, analytics, ads or crash upload).
- Owner check: Change each setting, restart, and see each one kept.

### Loop 38 - Security hardening

- Goal: Every trust boundary has been audited. This loop adds no features.
- In scope: Native UI network isolation, SSRF (tricking the app into reaching internal addresses), redirects, TLS, secrets, logging, validation of untrusted input, proxy isolation, the player boundary, metadata sanitization, source and media validation, and dependency vulnerabilities.
- Out of scope: New features.
- Done when: A security report exists and has no open must-fix finding.
- Criteria: SEC-001..SEC-019 (network, redirect, TLS and content safety), PRIV-001..PRIV-012 (privacy and secret handling), PROXY-001..PROXY-007 (loopback proxy rules), IPC-001..IPC-007 (UI-to-player boundary), CI-003 (govulncheck runs in CI).
- Owner check: Read the security report and confirm no must-fix finding is open.

### Loop 39 - Memory and resource-leak pass

- Goal: Resources are proven to be released.
- In scope: Repeated player open and close, many episode switches, repeated metadata refresh, source timeouts, cancelled searches, bad network and proxy-session churn. Check that goroutines (Go background tasks) return near their baseline, memory does not grow steadily, sockets close, proxy sessions expire, native player resources are released, HTTP bodies close and timers stop.
- Out of scope: Feature work.
- Done when: Recorded repeated-cycle numbers show no steady growth and all resources released.
- Criteria: RES-001..RES-010 (cleanup and no growth), PERF-006..PERF-008 (idle load, write frequency, comparison with OneAnime), PLATFORM-004 (Windows playback and cleanup validated).
- Owner check: Run the repeated-cycle scenario and read the recorded numbers.

### Loop 40 - Cross-platform validation

- Goal: The app is validated on Windows first, then Linux, then macOS.
- In scope: On Windows, the FLTK runtime, proof that no WebView is used (process and library check), libmpv, keyboard and proxy. Linux (a current mainstream distribution, Wayland, X11 if practical) starts only when Windows is stable. macOS (Apple Silicon, Intel where feasible) starts only when Linux is stable. Document each platform's runtime library needs; portable means no installer, not zero system libraries.
- Out of scope: Claiming Linux or macOS support from Windows evidence.
- Done when: Each platform has its own native evidence before it is called supported.
- Criteria: PLATFORM-001..PLATFORM-009 (builds, player lifecycle, keyboard, no WebView, portable data), ENV-008 (CI cannot stand in for native or human checks).
- Owner check: On each platform in turn, run the portable build and complete the keyboard workflow.

### Loop 41 - Portable CI and release

- Goal: Release artifacts can be reproduced.
- In scope: Go tests, vet, govulncheck, FLTK UI static checks, native UI build, platform build jobs, and the portable ZIP containing the pinned libmpv DLL and its disclosure. GitHub Releases is the release target. Workflow edits need explicit owner-approved scope and independent review.
- Out of scope: Store distribution and installers.
- Done when: CI builds the ZIP from the final commit and the ZIP runs after extraction with no installer.
- Criteria: CI-001..CI-010 (tests, vet, vulnerability scan, builds, lockfiles, portable artifact), ENV-004..ENV-007 (fallback CI rules and workflow-edit approval).
- Owner check: Download the artifact, unzip it, and run it without an installer.

### Loop 42 - Final MVP acceptance

- Goal: Every acceptance criterion has been run.
- In scope: All categories: functional, keyboard, cache and startup, architecture, security, privacy, resource lifecycle, cross-platform build, portable operation, no Wails or WebView dependency, resource envelope and code quality. Happy-path playback alone is not enough.
- Out of scope: New features.
- Done when: Every criterion has its required verifier result and the MVP final gate passes.
- Criteria: all criteria in [06_ACCEPTANCE.md](06_ACCEPTANCE.md), including the MVP final gate.
- Owner check: The final release gate returns `HUMAN_PASS`.
