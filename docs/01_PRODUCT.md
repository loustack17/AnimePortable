<!-- SPDX-License-Identifier: MPL-2.0 -->

# Product

AnimePortable is a lightweight Windows desktop anime client. It helps the user find an anime, choose an episode, play it, and resume later. Playback runs inside the app through libmpv (the library form of the mpv player).

Flow: `Find quickly -> Choose quickly -> Play -> Get out of the way`.

The app is a clean library and controller. It is not a feature-heavy media platform. The player UI is inspired by OneAnime (https://github.com/Predidit/oneAnime).

## Goals

- Unzip and watch. The portable ZIP contains the executable and the pinned libmpv DLL. No installer, no separate player install, no account.
- Data lives in the extracted folder (`<folder>/data`). Moving the folder moves the user's data.
- Find, open, play and resume must work with keyboard only and with mouse only.
- The whole product should use fewer resources than OneAnime in comparable states (see `06_ACCEPTANCE.md`, Resources and performance).

## Principles

| Principle | Rule |
| --- | --- |
| Lightweight | No Electron, no WebView, no browser-backed UI. FLTK native UI. No unnecessary background polling, large in-memory caches or runtime dependencies. |
| Fast | Cache-first startup: render cached SQLite data at once, refresh remote data in the background. Lazy-load covers. Limit metadata concurrency. Reuse HTTP clients. Never block startup on Anime1, AniList or Bangumi. |
| Intuitive | Buttons say `Play`. Users need no knowledge of mpv. |
| Minimal visuals | Whitespace, typography, hierarchy, clear focus, restrained borders, minimal shadow and animation. No glassmorphism, decorative gradients, infinite feeds, social UI or mobile bottom navigation. |
| Local-first privacy | No telemetry, account or ads. Data stays local. See `03_SECURITY.md`. |
| Replaceable dependencies | Anime1 must not become a permanent dependency of the core. See `02_ARCHITECTURE.md`. |

## Platform

Windows amd64 only. Linux and macOS work is deferred until the Windows version is stable. Mobile is not planned.

## Playback

In-process libmpv sits behind the core `Player` port (ADR-023). libmpv is loaded only when playback starts, from the DLL next to the executable, after its SHA-256 hash is verified (see `03_SECURITY.md`). If the DLL is missing or altered, Home stays usable and playback shows a safe error.

### On-video controls (rewritten 2026-10-13, owner decision)

The player has on-video controls drawn over the video:

- play/pause
- seek bar (draggable; one absolute seek on release)
- volume (with mute)
- episode list
- fullscreen

Behavior:

- Controls show at the start of playback, then hide after a few seconds of idle active playback.
- Mouse movement or keyboard input shows them again.
- Controls stay visible while paused, while loading, and during an open menu or a drag.
- A loading cue shows while an episode loads.

Controls are minimal. The player does not show danmaku, comments, social features or ads.

### Player keyboard bindings

The default focus is the seek bar. Tab and Shift+Tab choose the control target. Arrow keys act on that target without moving focus. Auto-hide, pointer movement and playback updates keep the target. If the target is missing, focus falls back to the seek bar. New playback and re-entry start on the seek bar.

| Input | Behavior |
| --- | --- |
| Space or K | Play/pause. Inside an open episode list, Space selects the row. |
| Left / Right | Seek bar focus: seek -5 / +5 seconds. Volume focus: volume -5 / +5 percent. Other controls: consumed, no action. |
| J / L | Seek -10 / +10 seconds. |
| Up / Down | Volume focus: volume +5 / -5 percent (0 to 100). Open list: move the selection. Other controls: consumed. |
| M | Mute (volume 0). Unmute restores the last nonzero volume (default 100). |
| F | Toggle fullscreen. |
| Esc | Close an open list or menu first; otherwise leave fullscreen. |
| Tab / Shift+Tab / Enter | Keep visible control focus and activation. |
| Media keys | Play/Pause, volume up/down and mute reach the existing typed player callbacks when the player window is focused. |
| Held keys | Seek, volume and list navigation may repeat. Pause, mute, fullscreen, Esc and Enter/Space selection act once per press. |
| Modified keys | Ctrl, Alt and Meta combinations do not trigger playback shortcuts. |

### One player per viewing session

A viewing session uses one libmpv player. Choosing another episode loads it into the same player (`loadfile ... replace`) without recreating the player. Closing the session closes the player deterministically and invalidates its proxy session.

Episode switch order:

1. Persist current progress.
2. Resolve the target episode.
3. Prepare a secure playback session.
4. Load it into the existing player.

### Autoplay next episode

Enabled by default. When the current episode ends normally, the app resolves the next episode and plays it in the same player. If no next episode exists, playback stops cleanly.

### User mpv configuration

The in-process player starts with `config=no`. The user's `mpv.conf`, `input.conf`, scripts and shaders are not read. (Verified in `apps/desktop/fltkengine/engine_windows.go`.)

## Resume and progress

The app owns the canonical progress data. mpv watch-later data is not used as the source of truth.

Stored per anime and episode: position, duration, completed state, last played time.

Write rules:

- Never write SQLite every frame.
- Checkpoint at: periodic interval, pause, episode switch, end of file, player close, application shutdown.
- An episode counts as completed at 90% or more.
- Show `Continue` only when the series has valid resumable saved progress.

## Navigation

The top-level sections are fixed: Home, Search, Schedule, Following, History, Settings. Adding another section needs an ADR.

### Home

Only high-value information:

- Continue Watching
- Recently Updated
- Following
- Today

Not allowed: trending feeds, recommendations, rankings, news, comments, social modules.

### Search

Search the Anime1 catalog. Results open the anime detail screen. `/` focuses search.

### Anime detail

Shows cover, title, native title, short synopsis, season, year, studio, episode count (when metadata is available), a Continue/Play action and the ordered episode list. It is not an encyclopedia.

Clicking an episode, or pressing Enter or Space on a focused episode row, plays it at once. There is no separate "selected episode Play" button. Metadata shown must be correct. No metadata is better than wrong metadata.

### Schedule

A readable chronological list, for example `Today · Friday`, then `18:00 Anime A EP07`, then `Tomorrow · Saturday`. No dense calendar grid unless later testing proves it better.

### Following

Follow, unfollow, latest available episode, watched episode, new-episode indicator. No push notifications.

### History

Anime, episode, last played time, progress, continue, remove from history. No analytics or behavioral scoring.

### Settings

Local preferences only (appearance, toggles, language). Settings never hold credentials or arbitrary commands.

### App keyboard bindings

| Input | Behavior |
| --- | --- |
| Arrow keys | Move focus spatially. |
| Tab / Shift+Tab | Bounded focus order. |
| Enter | Open the focused item, or play the focused episode. |
| Space | Play the focused episode row. |
| Esc | Back. |
| `/` | Focus search. |
| Ctrl+K | Quick search. Target binding; not yet implemented in `apps/desktop/fltkhome`. |

The exact key map may change. The core workflow must work without a mouse. Focus must always be visible.

## Permanent exclusions

Never implement:

- danmaku
- comments or social chat
- watch parties
- ads or ad SDKs
- recommendation algorithms
- automatic telemetry
- mandatory account
- remote JavaScript execution from content sources

## Not in scope now

Considered only after acceptance and an ADR: other players (VLC, IINA), mobile, downloads, cloud sync, plugins, multi-source UI, store distribution, notification system, Linux and macOS builds.
