<!-- SPDX-License-Identifier: MPL-2.0 -->

# Architecture

## 1. Architecture style

Use a **minimal Ports & Adapters architecture**.

The goal is replaceability and maintainability without enterprise-style ceremony.

The architecture should be understood as three practical layers:

`UI -> Core -> Adapters`

Do not create long chains such as:

`Controller -> Service -> Manager -> Facade -> UseCase -> Repository -> Adapter -> Client`

unless a concrete need appears later.

## 2. Dependency rule

Dependencies point inward.

The core must not import concrete infrastructure.

Forbidden core dependencies include:

- Wails
- Svelte
- Anime1 implementation
- AniList implementation
- Bangumi implementation
- MPV implementation
- SQLite driver
- platform-specific IPC libraries

Adapters implement core-defined ports.

## 3. Required ports

Only these four external-boundary abstractions are mandatory in MVP:

### AnimeSource

Responsible for anime source capabilities.

Conceptual interface:

```go
type AnimeSource interface {
    Catalog(ctx context.Context) ([]SourceAnime, error)
    Search(ctx context.Context, query string) ([]SourceAnime, error)
    Episodes(ctx context.Context, ref SourceRef) ([]SourceEpisode, error)
    Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error)
    Schedule(ctx context.Context, query ScheduleQuery) ([]SourceScheduleItem, error)
}
```

### MetadataProvider

Responsible for external metadata lookup.

```go
type MetadataProvider interface {
    Search(ctx context.Context, query MetadataQuery) ([]MetadataCandidate, error)
    Get(ctx context.Context, ref MetadataRef) (AnimeMetadata, error)
}
```

### Player

Responsible for playback-session lifecycle.

```go
type Player interface {
    Start(ctx context.Context, req PlayRequest) (PlaybackSession, error)
}

type PlaybackSession interface {
    Load(ctx context.Context, req PlayRequest) error
    Events() <-chan PlaybackEvent
    Close() error
}
```

### Store

Responsible for local persistence.

Keep this interface practical and coarse enough to avoid dozens of tiny repository abstractions.

It must support:

- anime records
- source mappings
- metadata
- following
- history
- progress
- settings

## 4. Provider-neutral domain model

Do not place provider-specific fields in domain models.

Wrong:

```go
type Anime struct {
    Anime1ID string
}
```

Correct concept:

```go
type Anime struct {
    ID          AnimeID
    Title       string
    NativeTitle string
    Description string
}
```

Provider mappings are separate:

```go
type SourceRef struct {
    Provider string
    ID       string
}
```

This permits a local anime identity to map to multiple external systems.

Example:

- local anime ID
  - Anime1 source ID
  - AniList metadata ID
  - Bangumi metadata ID

## 5. Local canonical identity

Following, history, and progress must use local canonical IDs.

They must not be keyed directly to Anime1 identifiers.

If Anime1 disappears, the user's local library must remain usable.

## 6. Source-specific data isolation

Anime1-only fields such as `data-apireq` tokens belong inside the Anime1 adapter.

They must never leak into:

- core anime model
- core episode model
- frontend
- SQLite domain tables
- logs

## 7. Source replaceability

MVP ships with Anime1 only, but the architecture must allow another source adapter later.

Future example:

```text
adapters/source/
├── anime1/
├── source_b/
└── source_c/
```

Replacing a source must not require changes to:

- playback core
- metadata matching core
- following/history schema
- UI navigation
- MPV integration

## 8. Source capabilities

Future sources may have different capabilities.

Represent capability differences explicitly, e.g.:

- search
- schedule
- playback
- latest updates

Do not assume every source supports everything.

A missing capability must degrade gracefully.

## 9. Source health and graceful degradation

Remote failure must not equal application failure.

If Anime1 is unavailable, cached features should still work:

- Home from cache
- Following
- History
- metadata
- cached schedule

Only operations that require the source should fail:

- fresh catalog
- fresh search
- playback resolution

If AniList fails, Bangumi and cached metadata may still work.

If Bangumi fails, AniList and cached metadata may still work.

If MPV is missing, browsing still works.

## 10. Metadata architecture

Primary provider:

- AniList

Fallback / cross-check:

- Bangumi

The metadata matcher is not part of either provider implementation.

Providers return candidates.

Core/application policy decides which candidate is acceptable.

Rule:

`No metadata is better than wrong metadata.`

## 11. Metadata matching

Matching may use:

- normalized title
- native title
- Traditional/Simplified normalization
- punctuation normalization
- full-width normalization
- season/year
- episode count
- provider cross-check

Never accept `searchResults[0]` blindly.

Low-confidence matches must fail safely.

The core matcher exposes normalized title comparison and an explicit decision:

- accepted high-confidence matches
- medium-confidence matches awaiting provider cross-check
- no-metadata results for low-confidence, ambiguous, or conflicting input

Provider implementations only return candidates; they do not select metadata.

Application-level provider orchestration is deferred until the application wiring phase; the core matcher remains a pure, provider-neutral policy seam.

Metadata display content crosses one shared backend policy before it can be cached:

- provider titles, descriptions, seasons, and studios are normalized to bounded plain text
- SQLite rejects unsafe writes and fails closed when cached rows no longer satisfy that policy
- cover URLs remain backend-only inputs to a fixed-origin loader
- cover bytes are exposed as typed validated content, not as an arbitrary frontend fetch capability

The cover loader performs work only when called and keeps no memory cache. The Wails binding layer must map this seam to a safe DTO without exposing raw provider payloads or a general-purpose URL loader.

## 12. UI boundary

Svelte/Wails must not know about:

- Anime1 implementation details
- MPV IPC details
- SQLite queries
- metadata provider HTTP APIs

UI talks to application-level typed actions such as:

- list anime
- search
- get detail
- play episode
- follow/unfollow
- read schedule
- read history

Do not expose raw MPV commands or arbitrary URLs to the frontend.

## 13. Wails isolation

Wails-specific code must remain in the desktop app adapter.

Core packages must compile and test without Wails.

## 14. CLI architecture test

Maintain a small non-product CLI or test harness that can reuse:

- core
- Anime1 adapter
- metadata adapters
- SQLite adapter
- MPV adapter

Purpose: prove the core is not coupled to Wails.

The CLI does not need polished UX.

## 15. Suggested repository layout

```text
anime-client/
├── core/
│   ├── models.go
│   ├── source.go
│   ├── metadata.go
│   ├── playback.go
│   ├── library.go
│   └── app.go
│
├── adapters/
│   ├── source/
│   │   └── anime1/
│   ├── metadata/
│   │   ├── anilist/
│   │   └── bangumi/
│   ├── player/
│   │   └── mpv/
│   ├── persistence/
│   │   └── sqlite/
│   │       └── migrations/
│   ├── network/
│   │   └── securehttp/
│   └── playback/
│       └── proxy/
│
├── apps/
│   ├── desktop/
│   │   ├── backend/
│   │   ├── frontend/
│   │   └── main.go
│   └── cli/
│
├── testdata/
├── tests/
│   ├── contract/
│   └── integration/
└── docs/
```

Do not over-split `core/` until actual size justifies it.

## 16. Performance rule

Ports are architectural boundaries, not network RPC.

They are in-process Go interface calls.

Do not optimize away useful boundaries out of fear of interface-dispatch overhead.

Real performance risks are:

- network latency
- WebView
- cover image decoding
- unbounded memory cache
- bad SQLite access patterns
- excessive background work
- MPV
- excessive allocations inside hot loops

## 17. Lightweight architecture rule

Create a port only when the dependency is genuinely external or replaceable.

Required:

- Anime source
- metadata provider
- player
- store

Do not create ports for trivial helpers such as:

- title normalizer
- clock
- logger
- image decoder
- random helper utilities

unless a real requirement emerges.

## 18. Contract tests

Each future adapter must pass shared contract tests.

AnimeSource contract examples:

- valid IDs
- correctly ordered episodes
- cancellation works
- source-specific secrets do not leak
- resolver returns a structurally valid playback source
- schedule is normalized

MetadataProvider contract examples:

- malformed responses fail safely
- no raw HTML leaks
- cancellation works
- provider errors are typed/sanitized

Player contract examples:

- session starts
- load replaces current item without spawning a second player
- events stop after close
- cleanup is deterministic

## 19. Architectural acceptance rules

MVP is not accepted unless:

- core imports no Wails
- core imports no Anime1 implementation
- core imports no MPV implementation
- core imports no SQLite driver
- core imports no AniList/Bangumi implementation
- Anime1 can be replaced by a fake source in tests
- UI can be removed while core tests still pass
- source failure does not break cached local library/history
