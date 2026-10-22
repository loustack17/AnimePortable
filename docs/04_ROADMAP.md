<!-- SPDX-License-Identifier: MPL-2.0 -->

# Roadmap

Loops 1-37 are complete. Loops 37b, 37c and 38 remain. Requirements are in [06_ACCEPTANCE.md](06_ACCEPTANCE.md). How a loop runs is in [05_WORKFLOW.md](05_WORKFLOW.md). Old plans are in `archive/04_MVP_IMPLEMENTATION_PLAN.md` and `archive/16_FYNE_MIGRATION_PLAN.md`.

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
| 35 | Cache-first startup and refresh: Home shows stored data with no network call, one bounded refresh policy in core, calm offline line on Home. |
| 36 | Autoplay next episode: a normal end starts the next episode in the same player, the last one stops cleanly; Home shows a finished newest episode as 已看完; the player's episode list opens on the playing episode. |
| 37 | Settings: a 設定 page (theme 跟隨系統/暗色/明亮, 自動下一集, 自動播放, resume, hardware decoding, proxy with 測試連線, 檢查更新 and GitHub link, data folder location with restart), a remembered speed button in the player, app-styled dialogs. |

## Remaining loops

Three loops remain, run in this order. The owner changed the plan on 2026-10-14:

- Following and History now come before Schedule, because they build on the finished detail and playback work.
- The former Loops 38 to 42 (security, resources, cross-platform, release, final acceptance) are now one Windows loop, Loop 38.

On 2026-10-22 the owner split autoplay into two settings (自動下一集, 自動播放) and moved language to its own Loop 37b, adding Japanese. After owner check 1 of Loop 37 (same day) the owner added the data folder location and player and system settings (chosen from settings that Animeko, Kazumi and mpv.net offer) to Loop 37, reopening the earlier "no player options" decision, and approved an HTTP forward proxy ([03_SECURITY.md](03_SECURITY.md)).

Every loop must also keep the requirements in [06_ACCEPTANCE.md](06_ACCEPTANCE.md) true.

### Loop 37b - Language

- Goal: The user can choose the UI language.
- In scope: A language setting on the 設定 page with Traditional Chinese (default), English and Japanese; every UI string comes from one string table. Anime titles, synopses and other remote content are not translated.
- Out of scope: Simplified Chinese; translating remote content.
- Done when: Every screen shows the chosen language with no Simplified Chinese and no clipped labels, and the choice survives a restart.
- Owner check: Switch to each language, look at every screen, restart, and see the language kept.

### Loop 37c - Skip the opening

- Goal: Like Netflix, a 跳過片頭 button appears only while the opening plays and jumps to its end.
- In scope: Opening times from AniSkip (`api.aniskip.com`, community timestamps looked up by MyAnimeList id, episode number and episode length); the MyAnimeList id from AniList metadata; the button (and S key) shown only inside the opening, gone after it; no button when there is no data. A security section for the new origin (what is sent: only those three numbers).
- Out of scope: Detecting openings from audio, skipping endings, user-submitted timestamps.
- Done when: For an episode with AniSkip data the button appears during the opening and jumps past it; for one without data nothing appears; no request is made unless an episode plays.
- Owner check: Play an episode of a popular series, see 跳過片頭 during the opening only, press it, and see playback continue after the song.

### Loop 38 - Hardening and release (Windows MVP)

- Goal: A trustworthy portable Windows release.
- In scope: A security pass over every trust boundary in [03_SECURITY.md](03_SECURITY.md); a resource pass (repeated player open/close and episode switches, cancelled searches, proxy-session churn: no steady growth in goroutines or memory); the portable ZIP with the pinned libmpv DLL and its notice, built by CI and published as a GitHub Release. The tag-only CI release job already exists (pre-release v0.1.0-alpha.1, [05_WORKFLOW.md section 11](05_WORKFLOW.md)); Loop 38 lets it publish the first full release, v1.0.0.
- Out of scope: New features, installers, Linux and macOS.
- Done when: The security and resource passes have no open must-fix finding, and the CI-built ZIP runs after extraction with no installer.
- Owner check: Download the release ZIP, unzip it, run it, and use the app for a normal session.

## After the MVP

Linux, then macOS. Each platform needs its own native build, libmpv, keyboard, portable-data and owner checks before it is called supported. Windows evidence never counts for another platform.
