<!-- SPDX-License-Identifier: MPL-2.0 -->

# Product Contract

## 1. Product definition

Build a lightweight desktop anime client whose primary job is to make it fast and convenient to:

- find an anime
- understand what it is
- see recent and scheduled releases
- follow shows
- choose an episode
- start playback
- resume progress later

Playback itself is delegated to MPV.

The application should disappear conceptually once playback starts. It must not put its own UI over the video.

## 2. Product philosophy

The product experience is:

`Find quickly -> Choose quickly -> Play -> Get out of the way`

The user should feel that the application is a clean library and controller, not a feature-heavy media platform.

## 3. Core values

### Lightweight

- Avoid Electron.
- Use the OS WebView through Wails.
- Avoid unnecessary background polling.
- Avoid large in-memory caches.
- Avoid unnecessary runtime dependencies.
- Do not embed a video engine in the application.
- Do not bundle MPV in the MVP.
- Keep frontend dependencies minimal.

### Fast

- Cache-first startup.
- Render cached local data immediately.
- Refresh remote data in the background.
- Lazy-load covers.
- Limit metadata request concurrency.
- Reuse HTTP clients and connection pools.
- Do not block startup on Anime1, AniList, or Bangumi.

### Intuitive

Core user flow:

`Launch -> Find anime -> Open anime -> Choose episode -> Play`

The flow must work entirely with keyboard and entirely with mouse.

### Minimal visual design

Prefer:

- whitespace
- typography
- hierarchy
- clear focus states
- restrained borders
- minimal shadow
- minimal animation
- calm information density

Avoid:

- glassmorphism
- decorative gradients
- excessive animation
- infinite feeds
- social UI
- mobile-bottom-navigation copied onto desktop

### Easy to use

The user should not need to understand MPV internals.

Buttons say `Play`, not `Open in MPV`.

Power users keep their own MPV configuration.

## 4. Playback philosophy

MPV is the only supported MVP player.

The app does not implement:

- video rendering
- playback controls
- fullscreen
- seeking UI
- volume UI
- playback-rate UI
- media-key handling
- subtitle rendering controls

MPV owns those responsibilities.

The user's existing:

- `mpv.conf`
- `input.conf`
- scripts
- shaders
- keyboard mappings

must remain effective.

## 5. Persistent MPV session

Do not spawn a new player for every episode.

A viewing session should use one MPV process:

`EP01 -> loadfile replace -> EP02 -> loadfile replace -> EP03`

The MPV PID should remain unchanged while switching episodes during the same viewing session.

## 6. Episode switching

While an episode is playing, the user may return to the app, select another episode, and the existing MPV process should immediately switch to it.

The application must:

1. persist current progress
2. resolve the target episode
3. prepare a secure playback session
4. send MPV `loadfile ... replace`
5. keep the same MPV process

## 7. Autoplay next episode

Enabled by default.

When MPV reports the current episode ended normally:

1. identify the next episode
2. resolve it
3. play it in the same MPV process

If no next episode exists, stop cleanly.

## 8. Playback progress

The app owns canonical progress data.

MPV watch-later data may exist, but it is not the source of truth.

Persist:

- anime
- episode
- position
- duration
- completed state
- last played timestamp

Do not continuously write SQLite every frame.

Persist at sensible checkpoints such as:

- periodic interval
- pause
- episode switch
- end of file
- player exit
- application shutdown

A practical internal completion policy such as >= 90% may be used for MVP.

## 9. Navigation

Top-level navigation is fixed for MVP:

- Home
- Schedule
- Following
- History
- Search
- Settings

Do not add more top-level sections without an ADR.

## 10. Home

Home contains only high-value information:

- Continue Watching
- Recently Updated
- Following
- Today

Do not add:

- trending feeds
- recommendation algorithms
- rankings
- news
- comments
- social modules

## 11. Anime detail

Show only useful work-respecting metadata:

- cover
- title
- native title
- short synopsis
- season
- year
- studio
- episode count
- continue/play action
- episode list

The product should respect the work by presenting correct metadata, but should not become an encyclopedia UI.

## 12. Schedule

Schedule is an MVP feature.

Prefer a readable chronological list, for example:

- Today · Friday
  - 18:00 Anime A EP07
  - 21:00 Anime B EP04
- Tomorrow · Saturday
  - ...

Avoid a dense calendar grid unless later testing proves it better.

## 13. Following

MVP following supports:

- follow
- unfollow
- latest available episode
- watched episode
- new episode indicator

No push notification system is required in MVP.

## 14. History

History supports:

- anime
- episode
- last played time
- progress
- continue
- remove from history

No analytics or behavioural scoring.

## 15. Keyboard-first desktop UX

Minimum bindings:

- Arrow keys: navigate
- Enter: open/select
- Space: play selected episode
- Esc: back
- `/`: search
- Ctrl/Cmd+K: quick search / command entry

The exact final key map may evolve, but the core workflow must be possible without a mouse.

## 16. Permanent exclusions

Never implement:

- danmaku
- in-video app overlays
- social chat
- comments
- ads
- ad SDKs
- watch parties
- recommendation algorithm
- automatic telemetry
- mandatory account
- remote JavaScript execution from content sources

## 17. Not in MVP

- VLC
- IINA
- mobile
- downloads
- cloud sync
- plugins
- bundled MPV
- multi-source UI
- store distribution
- notification system
- embedded player

These may only be considered after MVP acceptance and an explicit ADR.
