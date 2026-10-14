<!-- SPDX-License-Identifier: MPL-2.0 -->

# Decisions

An ADR (architecture decision record) fixes a design choice. Changing an Active decision needs a new ADR using the template at the end. Full text of every ADR is in `archive/`.

## ADR index

| ADR | Title | Status | Decision in one line |
| --- | --- | --- | --- |
| 001 | Go as primary language | Active | Backend and core are written in Go. |
| 002 | Wails v3 + Svelte/TypeScript | Superseded by 020 | Was the desktop shell; removed. |
| 003 | MPV only | Active (rewritten) | mpv is the only player, now as in-process libmpv (ADR-023). |
| 004 | External MPV process | Replaced by 023 | Playback is in-process libmpv, not a separate process. |
| 005 | Persistent player session | Active (rewritten) | One player per viewing session; episodes switch via `loadfile replace`. |
| 006 | Secure local playback proxy | Active | libmpv gets a loopback session URL, never remote credentials. |
| 007 | Minimal Ports & Adapters | Active | Four ports: AnimeSource, MetadataProvider, Player, Store. |
| 008 | Anime1 as adapter | Active | Anime1 is one AnimeSource adapter; local data survives replacement. |
| 009 | Bangumi primary, AniList cross-check | Active | No metadata beats wrong metadata. |
| 010 | SQLite local-first | Active | SQLite holds mappings, following, history, progress, settings, metadata; no credentials. |
| 011 | Cache-first startup | Active | Render cached state at once; refresh in the background. |
| 012 | No danmaku | Active | Danmaku is permanently out of scope. |
| 013 | No direct Internet from the UI | Active | All Internet access goes through Go. |
| 014 | No telemetry by default | Active | No telemetry, analytics, crash upload or tracking ID. |
| 015 | Desktop only | Active (rewritten) | Desktop only; Windows now, Linux and macOS deferred. |
| 016 | Respect user MPV configuration | Replaced by 023 | In-process libmpv runs with `config=no`; user mpv config is not read. |
| 017 | Fail closed | Active | On failed validation of external data, stop. |
| 018 | Local canonical IDs | Active | Following, history, progress use local IDs; provider IDs are mappings. |
| 019 | No over-engineering | Active | No abstraction per helper; coarse packages. |
| 020 | No WebView | Active | No WebView, Wails, Svelte or browser-backed UI. |
| 021 | Fyne for desktop | Superseded by 023 | Fyne was the interim toolkit; removed. |
| 022 | Resource-first UI proposal | Historical | A proposal; resolved by 023. |
| 023 | FLTK and in-process libmpv (Windows) | Active | Windows UI is FLTK; playback is in-process libmpv. |

Not ADRs, archive only: Fyne migration plan (`archive/16_FYNE_MIGRATION_PLAN.md`) and resource prototype plan (`archive/18_RESOURCE_CONTROL_PROTOTYPE_PLAN.md`).

Archive links: ADR-001 to 019 in `archive/07_ADR_BASELINE.md`; ADR-020 in `archive/14_ADR_NO_WEBVIEW.md`; ADR-021 in `archive/15_ADR_FYNE_DESKTOP.md`; ADR-022 in `archive/17_ADR_UI_RESOURCE_DECISION_PROPOSAL.md`; ADR-023 in `archive/19_ADR_FLTK_WINDOWS_DESKTOP.md`.

## Active decisions in detail

### ADR-001 Go

Go suits the main workload: HTTP, HTML and JSON parsing, SQLite, local proxying and concurrency. Native UI and libmpv are reached through narrow bindings in adapters.

### ADR-003, 004, 005 Player (rewritten 2026-10-13, owner decision)

mpv is the only player, loaded as libmpv inside the app process behind the core `Player` port. One player serves a viewing session. Switching episodes loads the new item into the same player. Close is deterministic. The player has on-video controls (see `01_PRODUCT.md`).

### ADR-006 Playback proxy

libmpv receives a loopback session URL, not remote authenticated URLs or headers. This keeps secrets out of player arguments, isolates Anime1 credentials and centralizes validation. Rules in `03_SECURITY.md`.

### ADR-007 Ports & Adapters

Ports: `AnimeSource`, `MetadataProvider`, `Player`/`PlaybackSession`, `Store`. No enterprise layering. See `02_ARCHITECTURE.md`.

### ADR-008 Anime1 adapter

Anime1 lives in `adapters/source/anime1`. Core and local user data must survive its replacement.

### ADR-009 Metadata

Bangumi is primary (owner decision 2026-10-14): its record has the Chinese synopsis the app shows, converted to Taiwan Traditional. AniList (English) is a cross-check and the fallback when Bangumi has no match. Score ties go to Bangumi. No metadata is preferable to wrong metadata.

### ADR-010 SQLite

SQLite stores canonical anime mapping, following, history, progress, settings and the metadata cache. Playback credentials are never stored.

### ADR-011 Cache-first startup

Startup renders cached SQLite state at once. Remote refresh runs in the background. Perceived speed matters more than interface-dispatch cost.

### ADR-012 No danmaku

Permanently out of scope. The product favors low-distraction playback.

### ADR-013 No direct Internet from the UI

All Internet access goes through Go, to centralize SSRF policy, redirects, TLS, redaction, source validation and content sanitizing.

### ADR-014 No telemetry

No automatic telemetry, analytics, crash upload or tracking identifier.

### ADR-015 Platform (rewritten 2026-10-13)

Desktop only, no mobile. The active platform is Windows amd64. Linux and macOS return only after the Windows version is stable and with their own native adapters and acceptance evidence.

### ADR-017 Fail closed

When source validation, redirects, TLS, IP policy, metadata matching or media sanity checks fail, stop. Do not try anyway.

### ADR-018 Local canonical IDs

Following, history and progress use local IDs. Provider IDs are mappings, not identity.

### ADR-019 No over-engineering

No abstraction for every helper. Keep packages coarse until real complexity appears.

### ADR-020 No WebView

The shipped app must not use WebView2, WKWebView, WebKitGTK, CEF, Chromium, browser-backed widgets or an HTML UI runtime. Resource measurements cannot reinstate Wails. The UI must not gain direct provider access, raw player commands or credentials. The release is portable, with no installer.

### ADR-023 FLTK and in-process libmpv (Windows)

Decision: the Windows desktop is Go with FLTK, and playback is in-process libmpv through the core `Player`/`PlaybackSession` port. It keeps the Go core, adapters, SQLite, the portable `<folder>/data` contract and all security, cancellation and lifecycle rules. Fyne, Wails, Svelte, NSIS and the external MPV path were removed. Player controls are drawn on the video.

Why: the Fyne build missed the idle resource target (169.50 MiB private working set after 15 minutes minimized, against a goal under 100 MiB). The FLTK and libmpv prototype was lighter.

Package and license decision (settled, owner, 2026-09-28): the portable ZIP bundles the SHA-256-pinned `libmpv-2.dll` that is byte-identical to OneAnime 1.4.7's. The ZIP includes a disclosure of its upstream release, known build inputs and unverified license and source details. The owner accepted prior resource and interaction evidence for Loop 28. Source files stay MPL-2.0. GPL distribution is acceptable if ever required. Agents must not re-research or re-litigate libmpv licensing unless the owner reopens it. The unused strict packaging mode (provenance JSON and corresponding-source ZIP) was removed on 2026-10-13 by owner decision. Related criterion: CI-010.

Open items:

- Linux and macOS are deferred.
- PERF-008 (matched whole-product resource comparison with OneAnime under identical content and counters) is deferred. Do not infer a matched result from unmatched host samples, trim working sets, force GC or raise targets to pass.
- Both apps use the same libmpv DLL, so compare the application and UI overhead outside the shared player baseline.

## ADR template

```markdown
# ADR-NNN — Title

## Status
Proposed / Accepted / Rejected / Superseded

## Context

## Decision

## Alternatives considered

## Security impact

## Performance/resource impact

## Migration impact

## Consequences
```
