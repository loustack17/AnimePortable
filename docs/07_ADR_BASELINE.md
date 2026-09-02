<!-- SPDX-License-Identifier: MPL-2.0 -->

# ADR Baseline

These are the initial architectural decisions.

A future change to one of these decisions requires a new ADR.

---

## ADR-001 — Go as primary language

### Decision

Use Go for the backend/core.

### Reason

The application primarily needs:

- HTTP
- HTML/JSON parsing
- SQLite
- process management
- IPC
- local proxying
- concurrency

These are strong Go use cases.

The project does not need Rust-level embedded multimedia/FFI complexity because playback is delegated to external MPV.

---

## ADR-002 — Wails v3 + Svelte/TypeScript

### Decision

Use Wails v3 for desktop shell and Svelte/TypeScript for UI.

### Reason

- Go integration
- native system WebView
- smaller footprint than Electron-style Chromium bundling
- strong web UI ergonomics
- keyboard/focus semantics are easier than mobile-first Flutter interaction
- desktop-first target

### Risk

Wails v3 is newer than v2.

Mitigation:

- Wails code remains isolated in desktop adapter
- core does not depend on Wails

---

## ADR-003 — MPV only in MVP

### Decision

MPV is the sole MVP player.

### Reason

- lightweight
- reliable in current intended usage
- excellent keyboard UX
- JSON IPC
- playlist/loadfile support
- user configuration
- same-process episode switching
- no need to reimplement playback controls

### Explicit exclusions

- VLC
- IINA
- embedded libmpv

---

## ADR-004 — External MPV process, not embedded player

### Decision

Run MPV as a separate process.

### Reason

Avoid:

- video rendering implementation
- native surface integration
- focus forwarding
- fullscreen complexity
- Wayland/X11/Win32/macOS surface differences
- duplicated shortcut/media-key implementation

---

## ADR-005 — Persistent MPV session

### Decision

One MPV process per viewing session.

Episode changes use MPV JSON IPC and `loadfile ... replace`.

---

## ADR-006 — Secure local playback proxy

### Decision

MPV receives a local loopback session URL rather than remote authenticated Anime1 URL/header credentials.

### Reason

- avoid secrets in argv
- isolate Anime1 credentials
- centralize remote source validation
- create a security boundary between Internet and MPV

---

## ADR-007 — Minimal Ports & Adapters

### Decision

Required external ports:

- AnimeSource
- MetadataProvider
- Player
- Store

Do not use full enterprise Clean Architecture layering.

### Reason

Need replaceability without sacrificing simplicity or maintainability.

---

## ADR-008 — Anime1 as adapter, not core dependency

### Decision

Anime1 is the initial AnimeSource implementation only.

Core/local user data must survive Anime1 replacement.

---

## ADR-009 — AniList primary metadata, Bangumi fallback/cross-check

### Decision

AniList provides primary metadata.

Bangumi improves Chinese/Japanese matching and fallback reliability.

### Rule

No metadata is preferable to wrong metadata.

---

## ADR-010 — SQLite local-first persistence

### Decision

Use SQLite for:

- local canonical anime mapping
- following
- history
- playback progress
- settings
- metadata cache

Do not persist playback credentials.

---

## ADR-011 — Cache-first startup

### Decision

Startup renders SQLite-cached state immediately.

Remote refresh happens in background.

### Reason

Perceived speed matters more than micro-optimizing Go interface dispatch.

---

## ADR-012 — No danmaku

### Decision

Danmaku is permanently out of scope.

### Reason

The product prioritizes immersive, low-distraction playback.

---

## ADR-013 — No direct Internet from frontend

### Decision

All Internet access goes through Go.

### Reason

Centralize:

- SSRF policy
- redirects
- TLS
- logging/redaction
- source validation
- content sanitization

---

## ADR-014 — No telemetry by default

### Decision

No automatic telemetry, analytics, crash upload, or tracking identifier in MVP.

---

## ADR-015 — Desktop only for MVP

### Decision

Support:

- Windows
- Linux
- macOS

Do not include Android/iOS in MVP.

---

## ADR-016 — Respect user MPV configuration

### Decision

The application must not overwrite or replace normal user MPV config.

Only provide the minimum runtime arguments required for:

- IPC
- local playback URL
- title/session behavior if needed

---

## ADR-017 — Fail closed on suspicious external data

### Decision

When source validation, redirects, TLS, IP policy, metadata matching, or media sanity checks fail, stop rather than "try anyway".

---

## ADR-018 — Local canonical IDs

### Decision

Following/history/progress use application-local canonical identity.

Provider IDs are mappings, not primary identity.

---

## ADR-019 — No over-engineering

### Decision

Do not create abstractions for every helper.

Keep package boundaries coarse until real complexity appears.

---

## ADR change template

Future ADRs should include:

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
