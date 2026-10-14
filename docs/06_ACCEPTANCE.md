<!-- SPDX-License-Identifier: MPL-2.0 -->

# Requirements

What the finished app must do and must never do. Every loop keeps these true; a loop's own "done when" is in [04_ROADMAP.md](04_ROADMAP.md). Automated tests check what they can; the owner checks UX. Details behind the security items are in [03_SECURITY.md](03_SECURITY.md).

## Architecture

- Core imports no FLTK, Anime1, AniList, Bangumi, libmpv or SQLite code; adapters implement core ports (enforced by `tests/architecture`).
- Core tests run without a UI, with a fake source.
- Following, history and progress use local anime IDs, never Anime1 IDs; no provider tokens in core models.
- A source failure never stops cached history and following from loading.
- No layers or interfaces without a present need.

## Features

- Opens to usable cached content without waiting for the network when a cache exists.
- Catalog, search, anime detail, schedule, following, history and Continue Watching work.
- Detail shows cover, title, synopsis, season and year, studio and episode count when metadata exists; episodes are listed in ascending number order.

## Playback

- The bundled libmpv DLL is loaded by absolute path only after its SHA-256 matches the pinned hash; a documented environment override exists. A missing or altered DLL keeps Home usable with a clear error.
- Playback goes through the loopback proxy; Anime1 credentials never reach the UI, player options, arguments or logs.
- One player instance per session; switching episodes reuses it.
- libmpv runs with `config=no`: the user's `mpv.conf` is never read or written.
- Progress comes from player events, is saved without high-frequency writes, survives restart, and resume works.
- A normal end of file can autoplay the next episode.
- The player has only the auto-hiding control overlay (play/pause, seek, volume, episode list, fullscreen). No danmaku or comments.

## Keyboard and UI

- Launch, find, open, select episode and play work without a mouse: arrow keys with visible focus, Enter opens or selects, Space plays, Esc goes back, `/` searches, Ctrl+K gives quick search.
- Mouse click, hover and scroll keep working; no tap zones.
- Calm, minimal UI; no social features.

## Metadata

- Bangumi is the primary metadata source. AniList is used to confirm a match, or when Bangumi has none (ADR-009).
- Chinese text is shown only in Traditional Chinese with Taiwan wording. A synopsis that exists only in Japanese is shown as it is.
- The app never takes the first search result blindly. If a match is doubtful, it shows no metadata rather than wrong metadata.
- Saved metadata is still shown when a provider is down.

## Security

- The UI never fetches from the Internet directly. Remote requests use HTTPS with TLS verification; unsupported schemes fail.
- Remote fetches cannot reach loopback, private or link-local addresses (IPv4 and IPv6), checked on the actual connection and after every redirect. Unknown playback origins fail closed.
- Remote HTML is parsed, never executed or shown raw; descriptions are plain text. Remote images are bounded in bytes and dimensions; unexpected content types fail safely.
- The proxy binds to loopback only, uses high-entropy per-session tokens, rejects unknown or expired tokens, ends sessions when playback ends, applies the same remote policy, and supports range requests.
- The player receives only loopback proxy URLs. The UI can call only typed actions: no raw player commands, arbitrary URLs, native handles, tokens or internal endpoints. Player events and native resources are released on close.

## Privacy

- No account, telemetry, analytics, ads or automatic crash upload. History stays on the device.
- Cookies, stream tokens and proxy tokens are never stored in SQLite or persistently, never returned to the UI, and redacted from logs.
- No administrator rights needed.

## Resources and performance

- Every response body, socket, timer and ticker is closed or stopped when no longer needed.
- Background goroutines and cancelled requests stop. Proxy sessions expire. Caches have a size limit.
- Starting and stopping the player, or switching episodes, many times does not make memory or goroutine counts keep growing.
- Startup with a cache does not wait for Anime1, AniList or Bangumi; cached content appears in about 1 second.
- Metadata lookups run with a limited number in parallel, and covers load only when shown.
- Background work is close to idle while a video plays.
- The whole app uses fewer resources than OneAnime in comparable states on the owner's machine (ADR-023).

## Platform and release

- No WebView, browser widget or HTML runtime anywhere.
- App data lives in the portable folder (`<folder>/data`); existing data is never silently moved or deleted.
- CI runs tests, race, vet, `govulncheck` and `go mod verify`, and builds the Windows app.
- The release is a ZIP that runs after extraction with no installer, including the pinned libmpv DLL and its disclosure notice (ADR-023).

## Code quality

- Code follows the structure rules in [02_ARCHITECTURE.md](02_ARCHITECTURE.md) and `AGENTS.md`.
- An independent review reports no unresolved must-fix finding on the final change.
