# MVP Implementation Plan

This document defines the implementation order.

Do not reorder major phases without a new ADR.

The plan intentionally proves the highest-risk assumptions before spending time on UI polish.

---

## Phase 0 — Repository and Contracts

### Goal

Create the project skeleton and freeze architecture/product/security rules.

### Deliverables

- repository initialized
- Go module
- Wails v3 desktop shell bootstrap
- Svelte/TypeScript frontend bootstrap
- docs copied into repository
- CI skeleton
- minimal folder boundaries
- architecture dependency tests or lint checks where practical

### Validation

- core can compile without Wails imports
- frontend can build
- desktop shell can start
- docs are committed

### Do not

- implement features
- add decorative UI
- add extra ports
- add extra providers

---

## Phase 1 — Core Models and Ports

### Goal

Create the minimal stable core vocabulary.

### Deliverables

Domain models for:

- Anime
- Episode
- ScheduleItem
- AnimeMetadata
- PlaybackProgress
- PlaybackEvent
- local IDs
- provider/source refs

Required ports:

- AnimeSource
- MetadataProvider
- Player / PlaybackSession
- Store

Application façade / core app object may coordinate use cases directly.

### Validation

Use fake adapters in tests.

Tests should demonstrate:

- core search can use a fake source
- core playback can use a fake player
- core library can use a fake store
- no concrete adapter imports

### Do not

- introduce DDD-heavy layers
- create repository interface per table
- add Anime1 fields to domain

---

## Phase 2 — Adapter Contract Test Framework

### Goal

Create reusable contract tests before real external adapters.

### Deliverables

Contract suites for:

- AnimeSource
- MetadataProvider
- Player
- Store where useful

### Validation

Fake adapters pass.

### Do not

- couple contract tests to Anime1-specific HTML

---

## Phase 3 — Secure HTTP Foundation

### Goal

Build the outbound-network security boundary before relying on remote services.

### Deliverables

- shared HTTP client/transport
- timeout policy
- redirect validation
- protocol validation
- DNS/IP validation
- SSRF prevention
- TLS default verification
- bounded response helpers
- sanitized error model
- centralized redaction helpers

### Required tests

Reject:

- localhost
- private IPv4
- link-local
- IPv6 loopback/link-local
- unsupported schemes
- redirect to localhost
- redirect to private address
- malformed URL
- oversized response where limit applies

### Validation

All network adapters must be able to depend on this layer.

---

## Phase 4 — Anime1 Catalog and Search Adapter

### Goal

Retrieve and normalize Anime1 catalog data.

### Deliverables

- Anime1 client
- catalog parser
- search behavior
- provider IDs contained only in adapter mapping structures
- fixtures

### Validation

- contract tests pass
- parser tests use fixtures
- cancellation works
- malformed source fails safely
- no raw HTML reaches core/UI

---

## Phase 5 — Anime1 Episode Adapter

### Goal

Retrieve episode lists.

### Deliverables

- episode parser
- episode ordering
- source episode refs
- fixtures

### Validation

- deterministic episode ordering
- invalid/missing fields handled safely
- no Anime1 token stored in domain tables

---

## Phase 6 — Anime1 Playback Resolver

### Goal

Resolve an episode into a backend-only playback source.

### Deliverables

- token extraction
- required POST/authorization flow
- temporary stream model
- header/cookie handling in memory only
- no secret logging

### Validation

A CLI/test harness can resolve an episode.

Do not yet expose secrets to MPV.

---

## Phase 7 — Anime1 Schedule

### Goal

Provide schedule data through the AnimeSource adapter.

### Deliverables

- schedule parser
- normalized schedule model
- cacheable result

### Validation

- fixtures
- correct day/time mapping where source provides it
- source failure returns typed error, not crash

---

## Phase 8 — Anime1 Adapter Acceptance

### Goal

Prove the complete source adapter.

### Validation

Anime1 adapter passes AnimeSource contract suite:

- catalog
- search
- episodes
- resolve
- schedule
- cancellation
- malformed data handling
- no provider secret leakage

---

## Phase 9 — Secure Playback Proxy

### Goal

Prevent remote media credentials from reaching MPV argv or frontend.

### Deliverables

- loopback-only server
- random port
- cryptographically random per-session token
- session registry
- remote request forwarding
- source header injection
- Range forwarding where required
- session expiration
- deterministic cleanup

### Validation

- cannot bind external interface
- unknown token rejected
- expired token rejected
- credentials absent from logs
- media can be read through proxy
- remote target still passes outbound network policy

---

## Phase 10 — MPV Detection and Process Lifecycle

### Goal

Start MPV safely and predictably.

### Deliverables

Platform detection:

- PATH
- common platform paths where appropriate
- user-configured MPV path

Process lifecycle:

- start
- detect failure
- clean exit
- preserve user's MPV config

### Validation

- missing MPV produces user-actionable error
- existing config remains effective
- no source credentials in argv

---

## Phase 11 — MPV JSON IPC

### Goal

Establish reliable typed control of MPV.

### Deliverables

Platform-specific endpoint:

- Unix socket
- Windows named pipe

Commands needed for MVP:

- load file
- observe/get time-pos
- duration
- pause state
- end-file event
- stop

### Validation

- endpoint short-lived
- endpoint private to current user where practical
- reader stops on close
- malformed JSON/events handled without process leak

---

## Phase 12 — Same-Session Episode Switching

### Goal

Prove the key UX requirement.

### Flow

1. start EP01
2. resolve EP02
3. create new proxy session
4. persist EP01 progress
5. MPV `loadfile ... replace`
6. invalidate old proxy session when safe
7. continue with same MPV PID

### Acceptance

Switch:

`EP01 -> EP02 -> EP03`

without spawning a second MPV process.

---

## Phase 13 — SQLite Adapter

### Goal

Create local persistence.

### Tables / concepts

- anime
- episode
- source_ref
- metadata
- metadata_ref
- follow
- playback_progress
- playback_history
- settings
- provider cache if necessary

### Rules

- canonical local IDs
- provider-neutral schema
- no cookies
- no stream tokens
- no proxy tokens

### Validation

- migrations repeat safely
- CRUD tests
- source replacement does not destroy local history

---

## Phase 14 — Playback Progress and History

### Goal

Make progress reliable.

### Deliverables

- observe MPV progress
- checkpoint in memory
- periodic persist
- persist on pause/switch/end/exit
- completed state
- history entry
- resume

### Validation

Scenario:

1. play EP04
2. reach known timestamp
3. stop player
4. stop app
5. restart app
6. continue watching points to EP04 near stored timestamp

---

## Phase 15 — Following

### Goal

Local follow state independent of Anime1 IDs.

### Deliverables

- follow/unfollow
- latest episode comparison
- watched-vs-latest indicator

### Validation

Following survives restart and is tied to local canonical anime identity.

---

## Phase 16 — AniList Metadata Adapter

### Goal

Retrieve high-quality metadata.

### Only request what MVP needs

- title
- native title
- cover
- synopsis
- season
- year
- episode count
- studio

### Validation

- shared secure HTTP layer only
- provider contract tests
- malformed API data fails safely
- no direct frontend fetch

---

## Phase 17 — Bangumi Metadata Adapter

### Goal

Fallback and cross-check provider.

### Uses

- Chinese title matching
- Japanese title matching
- fallback metadata
- validation/cross-reference

### Validation

Same provider contract and network requirements.

---

## Phase 18 — Metadata Normalization and Matching

### Goal

Attach correct metadata without silently choosing the wrong work.

### Inputs

- Anime1 title
- AniList candidates
- Bangumi candidates
- source season/year/episode hints where available

### Normalization

- Traditional/Simplified normalization
- punctuation
- full-width characters
- season suffixes
- episode notation
- native-title comparison

### Policy

- high confidence: accept
- medium confidence: cross-check
- low confidence: no metadata

Exact thresholds can be tuned by tests; do not hardcode a simplistic first-result rule.

### Validation

Build a fixture set of known titles with expected matches and expected no-match cases.

---

## Phase 19 — Remote Metadata Content Security

### Goal

Ensure metadata cannot become an injection or resource-exhaustion vector.

### Deliverables

- synopsis -> sanitized/plain text
- no raw HTML rendering
- image URL policy
- image byte limit
- image dimension limit
- lazy loading strategy

### Validation

- script tags do not execute
- malformed images fail safely
- oversized images rejected
- frontend receives safe typed data only

---

## Phase 20 — Desktop Application Binding Layer

### Goal

Expose typed application actions to Wails without leaking infrastructure details.

### Bindings should look conceptually like

- list home
- search
- get anime detail
- play episode
- follow/unfollow
- get schedule
- get history
- settings

### Forbidden bindings

- raw MPV command
- arbitrary playback URL
- arbitrary HTTP fetch
- arbitrary SQL
- source cookie/token access

---

## Phase 21 — Svelte App Shell

### Goal

Create the visual shell.

### Navigation

- Home
- Schedule
- Following
- History
- Search
- Settings

### Design

- desktop-first
- calm
- minimal
- obvious focus states
- limited animation
- no mobile-first interaction assumptions

---

## Phase 22 — Home UI

### Sections

- Continue Watching
- Recently Updated
- Following
- Today

### Validation

Works from cached SQLite data without network.

---

## Phase 23 — Search UI

### Requirements

- fast input
- keyboard focus
- arrow navigation
- Enter opens
- Esc/back behavior
- clear loading/error state
- cached/local results where practical, refreshed remotely

---

## Phase 24 — Anime Detail and Episode UI

### Show

- cover
- title
- native title
- synopsis
- season/year
- studio
- episode count
- Play/Continue
- episode list

### Requirements

- Space can play selected episode
- Enter can open/select
- current/progress state visible without clutter

---

## Phase 25 — Schedule UI

### Preferred representation

Chronological readable list grouped by day.

### Requirements

- cached immediately
- background refresh
- selecting an item opens relevant anime if known

---

## Phase 26 — Following UI

### Show

- followed anime
- latest episode
- watched episode
- new episode indicator

No notifications in MVP.

---

## Phase 27 — History / Continue Watching UI

### Show

- anime
- episode
- progress
- last played
- continue action

Support remove from history.

---

## Phase 28 — Keyboard-First Navigation

### Goal

Make keyboard support an acceptance feature, not polish.

Minimum:

- arrows
- Enter
- Space
- Esc
- `/`
- Ctrl/Cmd+K

### Acceptance

Complete core workflow with zero mouse input:

`Launch -> Find -> Open -> Select episode -> Play`

---

## Phase 29 — Cache-First Startup and Refresh

### Goal

Perceived startup should not depend on Internet.

### Startup

`App -> SQLite -> UI`

### Background

- Anime1 refresh
- schedule refresh
- metadata refresh

### Requirements

- bounded concurrency
- stale-while-refresh behavior
- UI updates incrementally
- no full-screen blocking loader for normal cached startup

---

## Phase 30 — Autoplay Next

### Goal

Seamless next-episode playback in the same MPV process.

### Trigger

Normal end-file.

### Validation

- next episode exists -> resolve + load in same process
- no next episode -> clean stop
- current progress persisted
- failed resolve does not loop/spam

---

## Phase 31 — Settings

Keep minimal:

### Appearance

- System
- Light
- Dark

### Player

- MPV path
- Autoplay next episode

### Playback

- Resume playback

### Language

- Traditional Chinese
- English

Do not add power-user MPV settings. Those belong in MPV config.

---

## Phase 32 — Security Hardening Pass

Audit every trust boundary.

Verify:

- frontend network isolation
- SSRF
- redirects
- TLS
- secrets
- logging
- WebView CSP
- proxy isolation
- IPC isolation
- metadata sanitization
- source/media validation
- dependency vulnerabilities

No feature work in this phase.

---

## Phase 33 — Memory and Resource Leak Pass

### Stress scenarios

- repeatedly open/close MPV
- switch episodes many times
- refresh metadata repeatedly
- source timeouts
- cancelled searches
- bad network
- proxy session churn

### Verify

- goroutine count returns near baseline
- memory does not grow monotonically
- sockets close
- proxy sessions expire
- IPC endpoints removed
- HTTP bodies closed
- timers stopped

---

## Phase 34 — Cross-Platform Validation

### Windows

- WebView2
- MPV detection
- named pipe IPC
- keyboard
- proxy

### Linux

- current mainstream distro
- Wayland
- X11 if practical
- MPV
- Unix socket
- keyboard
- proxy

### macOS

- Apple Silicon
- Intel build where feasible
- MPV detection
- Unix socket
- keyboard
- proxy

---

## Phase 35 — CI and Release Build

CI:

- Go tests
- Go vet
- govulncheck
- frontend lint
- frontend typecheck
- frontend build
- platform build jobs

MVP release target:

- GitHub Releases

Do not add store distribution.

---

## Phase 36 — Final MVP Acceptance

Run every criterion in `06_ACCEPTANCE_CRITERIA.md`.

Do not call the MVP complete if only happy-path playback works.

All categories must pass:

- functional
- keyboard
- cache/startup
- architecture
- security
- privacy
- resource lifecycle
- cross-platform build
