<!-- SPDX-License-Identifier: MPL-2.0 -->

# Roadmap

Loops 1-38, 37b, 37c and 37d are complete; v1.0.0, the first full release, was published on 2026-10-24. Version 2.0 (Loops 39-46, below) is next and needs owner approval to start. Requirements are in [06_ACCEPTANCE.md](06_ACCEPTANCE.md). How a loop runs is in [05_WORKFLOW.md](05_WORKFLOW.md). Old plans are in `archive/04_MVP_IMPLEMENTATION_PLAN.md` and `archive/16_FYNE_MIGRATION_PLAN.md`.

Platform order: the whole Windows product first (all remaining loops), then Linux in 2.0, then macOS in 3.0 (see "Version 2.0").

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
| 35 | Cache-first startup and refresh: Home shows stored data with no network call, one bounded refresh policy in core, calm offline line on Home. |
| 36 | Autoplay next episode: a normal end starts the next episode in the same player, the last one stops cleanly; Home shows a finished newest episode as 已看完; the player's episode list opens on the playing episode. |
| 37 | Settings: a 設定 page (theme 跟隨系統/暗色/明亮, 自動下一集, 自動播放, resume, hardware decoding, proxy with 測試連線, 檢查更新 and GitHub link, data folder location with restart), a remembered speed button in the player, app-styled dialogs. |
| 37b | Language: 語言 繁體中文 / English / 日本語 switches every screen at once and is saved; one string table for all UI text. |
| 37c | Skip the opening: 跳過片頭 (and S) only during the opening, times from AniSkip by MyAnimeList id (AniList, kept in memory); builds open no console window. |
| 37d | Reliable starts and more synopses: the detail page shows the stored episode list at once; Anime1 page timeout and retry; a 20 s start check; Simplified Bangumi search, Anime1 year and episode count as hints, season lists and narrow name rules (193 of the owner's 228 anime matched, 0 wrong). |
| 38 | Hardening and release: security pass over 20 trust boundaries (mpv opens only MP4, DLLs load only from the app folder and System32, bounded resolver JSON and debug timeline), resource pass with a headless harness (no steady growth in seven scenarios; idle private working set 10.3 MiB after 15 minutes minimized), v1.0.0 published as the first full release. |

## Remaining loops

Loop 38 passed on 2026-10-24 ([loops/LOOP38.md](loops/LOOP38.md)); the history below is kept for reference. The owner changed the plan on 2026-10-14:

- Following and History now come before Schedule, because they build on the finished detail and playback work.
- The former Loops 38 to 42 (security, resources, cross-platform, release, final acceptance) are now one Windows loop, Loop 38.

On 2026-10-22 the owner split autoplay into two settings (自動下一集, 自動播放) and moved language to its own Loop 37b, adding Japanese. After owner check 1 of Loop 37 (same day) the owner added the data folder location and player and system settings (chosen from settings that Animeko, Kazumi and mpv.net offer) to Loop 37, reopening the earlier "no player options" decision, and approved an HTTP forward proxy ([03_SECURITY.md](03_SECURITY.md)).

On 2026-10-22, after trying v0.4.0-beta, the owner added a reliability loop before Loop 38: Loop 37d.

Every loop must also keep the requirements in [06_ACCEPTANCE.md](06_ACCEPTANCE.md) true.

### Loop 38 - Hardening and release (Windows MVP)

- Goal: A trustworthy portable Windows release.
- In scope: A security pass over every trust boundary in [03_SECURITY.md](03_SECURITY.md); a resource pass (repeated player open/close and episode switches, cancelled searches, proxy-session churn: no steady growth in goroutines or memory); the portable ZIP with the pinned libmpv DLL and its notice, built by CI and published as a GitHub Release. The tag-only CI release job already exists (pre-release v0.1.0-alpha.1, [05_WORKFLOW.md section 11](05_WORKFLOW.md)); Loop 38 lets it publish the first full release, v1.0.0.
- Out of scope: New features, installers, Linux and macOS.
- Done when: The security and resource passes have no open must-fix finding, and the CI-built ZIP runs after extraction with no installer.
- Owner check: Download the release ZIP, unzip it, run it, and use the app for a normal session.

## Version 2.0 (after Loop 38 and v1.0.0)

Planned with the owner on 2026-10-23. Not started; each loop still needs owner approval to start.

Owner decisions for 2.0:

- The UI stays on FLTK. No toolkit change and no second UI codebase; the modern look comes from custom-drawn FLTK widgets, SVG icons and timer-driven animation.
- Sources are hot-swappable: every source implements the same core source port, so adding or removing one never touches the UI, player or storage.
- Out of scope: HiAnime and similar aggregator sources, YouTube playback through yt-dlp, and any region-lock bypass. The assistant does not research or implement new unlicensed sources; the owner may add sources through the source port.
- Linux is the 2.0 platform. macOS moves to 3.0 (code signing and notarization, two CPU architectures, deprecated OpenGL).
- The 06_ACCEPTANCE.md requirements still apply, including the idle resource target, no WebView and no video downloads.

### Loop 39 - 2.0 design

- Goal: An owner-approved visual design for the player, Home, detail page and the menu-to-player transition.
- In scope: Mockups of player controls (buttons, icons, seek bar with hover time, volume, subtitle and audio menus), Home and detail; a transition spec; one FLTK prototype of the player control bar proving the design can be built.
- Done when: The owner approves the mockups in writing, and the FLTK prototype control bar matches them while the app stays under the idle memory target in 06_ACCEPTANCE.md.

### Loop 40 - Player UI

- Goal: The player looks and behaves like the approved Loop 39 mockups.
- In scope: The control bar (play/pause, previous and next episode, seek bar with hover time, volume, fullscreen), the subtitle and audio menus (empty until Loop 43), SVG icons, and the episode title shown on the video.
- Done when: A side-by-side comparison with the Loop 39 mockups shows no difference the owner rejects; every control works by mouse and by keyboard; controls still auto-hide after 3 seconds; and no dropped frames are added compared with v1.0 on the same episode.

### Loop 41 - Main UI and transitions

- Goal: Home, detail and the list pages match the Loop 39 mockups, and moving between a page and the player fades instead of cutting.
- In scope: The Loop 39 design for Home, detail, 追蹤, 歷史紀錄 and 時間表; opening an episode fades from the page to the player in the same window, showing the cover and a loading indicator while the episode resolves; leaving the player fades back to the page it came from.
- Done when: The owner approves the pages against the mockups; every page-to-player and player-to-page switch fades over 150 to 300 ms with no blank or black frame; and focus and Esc behave as in Loop 34.

### Loop 42 - Hot-swappable sources and local files

- Goal: More than one source, selectable at runtime, plus local video files.
- In scope: Source selection; a local-files source (folders and files, episode order from file names); an out-of-process source format (a separate executable in `sources/` that answers catalog, search, episodes, stream and subtitles as JSON), with every reply treated as untrusted and streams still going through the playback proxy. Anime1 stays built in.
- Done when: Sources can be added and removed without rebuilding the app, a broken source cannot crash or hang the app, and local files play with progress and history.

### Loop 43 - Subtitles

- Goal: Chinese, English and Japanese subtitles where the video allows it.
- In scope: Subtitle and audio track menus, external subtitle files (SRT, ASS, VTT), two subtitles at once (for language study), language preference, per-anime remembered choice, timing offset. Hard-coded subtitles (as on Anime1) cannot be switched off; the UI says so.
- Done when: A local video with external subtitle files shows any one or two tracks, and the choices survive a restart.

### Loop 44 - Online subtitle search

- Goal: Find subtitles for a local video without leaving the app.
- In scope: OpenSubtitles and Jimaku with the user's own API keys entered in Settings (no bundled keys), results ranked by anime and episode, download into the portable data folder, timing offset for releases that do not match.
- Done when: A search returns usable subtitles for a known episode, keys are never logged, and the app works with no keys.

### Loop 45 - Linux

- Goal: A supported Linux build.
- In scope: The FLTK and libmpv build on Linux, keyboard, portable data, a portable package (AppImage or tar.gz with libmpv), Linux CI.
- Done when: The owner's Linux check passes on a real machine. Windows evidence never counts for Linux.

### Loop 46 - 2.0 hardening and release

- Goal: Release 2.0.0 for Windows and Linux.
- In scope: A security pass over the new trust boundaries in 03_SECURITY.md terms (source executables, subtitle files, subtitle APIs and their keys); the Loop 38 resource pass repeated with source switching and subtitle loading; CI-built Windows ZIP and Linux package published as a GitHub Release.
- Done when: The security and resource passes have no open must-fix finding, and both packages run after extraction on a clean machine.

## Version 3.0

macOS, then automatic subtitle timing (aligning a subtitle file to the video without a manual offset). Each platform needs its own native build, libmpv, keyboard, portable-data and owner checks before it is called supported.
