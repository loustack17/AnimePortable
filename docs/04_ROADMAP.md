<!-- SPDX-License-Identifier: MPL-2.0 -->

# Roadmap

Loops 1-34 are complete. Loops 35-38 remain. Requirements are in [06_ACCEPTANCE.md](06_ACCEPTANCE.md). How a loop runs is in [05_WORKFLOW.md](05_WORKFLOW.md). Old plans are in `archive/04_MVP_IMPLEMENTATION_PLAN.md` and `archive/16_FYNE_MIGRATION_PLAN.md`.

Platform order: the whole Windows product first (all remaining loops), then Linux, then macOS (see "After the MVP").

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
| 8 | Anime1 schedule (removed in Loop 33). |
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
| 28 | M4: Windows FLTK and libmpv integrated gate (ADR-023). |
| 29 | Search UI. |
| 30 | Anime detail and episode UI, smooth playback, AniList/Bangumi metadata in Taiwan Traditional Chinese. |
| 31 | Following: 追蹤 page, follow and unfollow, watched episode from local data. |
| 32 | History and Continue Watching: 歷史紀錄 page, continue at the saved position, remove an entry. |
| 33 | Schedule: Bangumi weekly calendar as a week grid, cached first; known shows open, others search. |
| 34 | Keyboard navigation everywhere: one key policy, Esc back, `/` and Ctrl+K search, arrows with visible focus on every screen and in the player. |

## Remaining loops

Four loops remain, run in this order. The owner changed the plan on 2026-10-14:

- Following and History now come before Schedule, because they build on the finished detail and playback work.
- The former Loops 38 to 42 (security, resources, cross-platform, release, final acceptance) are now one Windows loop, Loop 38.

Every loop must also keep the requirements in [06_ACCEPTANCE.md](06_ACCEPTANCE.md) true.

### Loop 35 - Cache-first startup and refresh

- Goal: The app starts from cached data and does not need the Internet.
- In scope: Startup through SQLite and the UI; background refresh of Anime1, schedule and metadata with bounded parallel requests; cached content stays visible while refreshing.
- Out of scope: New data sources.
- Done when: With the network off, cached content shows within about 1 second, and refresh work is bounded and cancellable.
- Owner check: Disconnect the network, start the app, and see cached content within about 1 second.

### Loop 36 - Autoplay next episode

- Goal: When an episode ends, the next one starts in the same player.
- In scope: On a normal end of file, load the next episode in the same libmpv instance; stop cleanly after the last one; keep progress; a failed resolve reports once and does not retry in a loop.
- Out of scope: Playlist editing.
- Done when: The next episode starts without a new window, the last one stops cleanly, and repeated switching does not grow memory.
- Owner check: Let an episode finish and see the next one start without a new window.

### Loop 37 - Settings

- Goal: The user can change a few basic settings and they are kept.
- In scope: Appearance (System, Light, Dark), autoplay next episode, resume playback, language (Traditional Chinese, English).
- Out of scope: Player options and a player-path setting (the DLL override stays a documented environment variable).
- Done when: Each setting changes behavior and survives a restart, stored in the portable data folder, with no account or telemetry.
- Owner check: Change each setting, restart, and see each one kept.

### Loop 38 - Hardening and release (Windows MVP)

- Goal: A trustworthy portable Windows release.
- In scope: A security pass over every trust boundary in [03_SECURITY.md](03_SECURITY.md); a resource pass (repeated player open/close and episode switches, cancelled searches, proxy-session churn: no steady growth in goroutines or memory); the portable ZIP with the pinned libmpv DLL and its notice, built by CI and published as a GitHub Release. The tag-only CI release job already exists (pre-release v0.1.0-alpha.1, [05_WORKFLOW.md section 11](05_WORKFLOW.md)); Loop 38 lets it publish the first full release, v1.0.0.
- Out of scope: New features, installers, Linux and macOS.
- Done when: The security and resource passes have no open must-fix finding, and the CI-built ZIP runs after extraction with no installer.
- Owner check: Download the release ZIP, unzip it, run it, and use the app for a normal session.

## After the MVP

Linux, then macOS. Each platform needs its own native build, libmpv, keyboard, portable-data and owner checks before it is called supported. Windows evidence never counts for another platform.
