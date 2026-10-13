<!-- SPDX-License-Identifier: MPL-2.0 -->

# Architecture

Minimal Ports & Adapters: `UI -> Core -> Adapters`. Dependencies point inward. The goal is replaceability and maintainability without layer ceremony. Do not build chains such as Controller -> Service -> Manager -> Facade -> UseCase -> Repository.

The Windows UI is FLTK. It calls typed Go application actions. Playback is in-process libmpv behind the core `Player` port (ADR-020, ADR-023).

## Repository layout

| Path | Responsibility |
| --- | --- |
| `core/` | Domain models, ports, matching and playback-tracking policy. No concrete infrastructure. Files: `app.go`, `errors.go`, `metadata.go`, `metadata_match.go`, `metadata_normalize.go`, `metadata_variants.go`, `models.go`, `playback.go`, `playback_tracking.go`, `source.go`, `store.go`. |
| `adapters/source/anime1/` | Anime1 `AnimeSource`. Parser-only HTML extraction (`html_policy.go`), schedule. |
| `adapters/metadata/anilist/`, `adapters/metadata/bangumi/` | `MetadataProvider` implementations. |
| `adapters/metadata/cover/` | Validated cover loader (fixed origins, size and format limits). |
| `adapters/metadata/internal/` | Helpers shared by the two metadata adapters only. |
| `adapters/persistence/sqlite/` | `Store` implementation, embedded migrations in `migrations/`. |
| `adapters/network/securehttp/` | The single controlled outbound HTTP client. |
| `adapters/playback/proxy/` | Loopback playback proxy. |
| `adapters/player/libmpv/` | `Player` implementation over libmpv. |
| `internal/metadata/` | Shared metadata text policy. |
| `internal/runtimepin/` | Pinned libmpv DLL SHA-256 constant. |
| `apps/desktop/` | Desktop entry (`main_fltk_windows.go`), play queue, playback startup. |
| `apps/desktop/backend/` | Typed application actions (`Service`), DTOs, ingest, portable data location. |
| `apps/desktop/fltkengine/` | libmpv engine wiring, DLL loader (`internal/mpvwin`), playback diagnostics. |
| `apps/desktop/fltkhome/` | Home, Search and Detail views and models. |
| `apps/desktop/fltkplayer/` | Player window, on-video controls (`overlay_windows.*`), native input. |
| `tools/portable-package/` | CLI that builds the portable ZIP. |
| `tools/*.ps1` | Build helpers (Go license notices, test execution helper). |
| `tests/architecture/` | Import-rule tests. |
| `tests/contract/` | Shared adapter contract tests and fakes. |
| `docs/` | Project documents. |

Files with `_windows` in the name build on Windows only. Native FLTK tests run only in GitHub Windows CI (see `06_ACCEPTANCE.md`).

## Dependency rule

The core must not import: FLTK, Anime1, AniList or Bangumi implementations, libmpv, a SQLite driver, or platform-specific libraries. Adapters implement core-defined ports. `tests/architecture/imports_test.go` enforces this.

## Ports

Exactly four external-boundary abstractions, defined in `core/`.

```go
// core/source.go
type AnimeSource interface {
	Catalog(ctx context.Context) ([]SourceAnime, error)
	Search(ctx context.Context, query string) ([]SourceAnime, error)
	Episodes(ctx context.Context, ref SourceRef) ([]SourceEpisode, error)
	Resolve(ctx context.Context, ref EpisodeRef) (PlaybackSource, error)
	Schedule(ctx context.Context, query ScheduleQuery) ([]SourceScheduleItem, error)
}

// core/metadata.go
type MetadataProvider interface {
	Search(ctx context.Context, query MetadataQuery) ([]MetadataCandidate, error)
	Get(ctx context.Context, ref MetadataRef) (AnimeMetadata, error)
}

// core/playback.go
type Player interface {
	Start(ctx context.Context, req PlayRequest) (PlaybackSession, error)
}
type PlaybackSession interface {
	Load(ctx context.Context, req PlayRequest) error
	Events() <-chan PlaybackEvent
	Close() error
}
// Optional extension used for progress snapshots:
type PlaybackSnapshotter interface {
	PlaybackSession
	Snapshot(ctx context.Context) (PlaybackSnapshot, error)
}

// core/store.go
type Store interface {
	SaveAnime / Anime / ListAnime
	SaveSourceRef / SourceRefs
	SaveEpisodeMapping / EpisodeMappings
	SaveMetadata / Metadata
	SetFollowing / Following
	AddHistory / History / RemoveHistory
	SaveProgress / SavePlaybackCheckpoint / Progress
	SaveSettings / Settings
}
```

(`Store` is abbreviated: each name is a method taking `ctx` plus typed arguments. See `core/store.go` for exact signatures.)

`PlaybackSource` hides its URL and headers from formatting (`String()` returns `PlaybackSource{redacted}`).

Create a port only for a genuinely external or replaceable dependency. Do not create ports for helpers such as a title normalizer, clock, logger or image decoder.

## Domain rules

- Domain models carry no provider-specific fields. Provider mappings are separate (`SourceRef{Provider, ID}`). One local anime can map to Anime1, AniList and Bangumi IDs.
- Following, history and progress use local canonical IDs (ADR-018), never Anime1 IDs. If Anime1 disappears the local library stays usable.
- Anime1-only fields (such as `data-apireq` tokens) stay inside the Anime1 adapter. They never reach core models, UI, SQLite domain tables or logs.
- Sources may have different capabilities (search, schedule, playback, latest updates). A missing capability degrades gracefully.

## Graceful degradation

Remote failure is not application failure.

- Source down: Home from cache, Following, History, metadata and cached schedule still work. Only fresh catalog, fresh search and playback resolution fail.
- AniList down: Bangumi and cached metadata still work, and the reverse.
- libmpv DLL missing or altered: browsing still works.

## Metadata

AniList is primary. Bangumi is fallback and cross-check. Providers return candidates only. Core policy decides which candidate is acceptable. Never accept `searchResults[0]` blindly. No metadata is better than wrong metadata.

Matching may use normalized and native titles, Traditional/Simplified, punctuation and full-width normalization, season/year, episode count and provider cross-check. The core matcher returns one of: accepted (high confidence), awaiting cross-check (medium confidence), or no metadata (low confidence, ambiguous or conflicting).

Display text crosses one shared policy (`internal/metadata`) before caching: bounded plain text. SQLite rejects unsafe writes and fails closed on unsafe cached rows. Cover URLs stay backend-only. Cover bytes reach the UI as typed validated content, never as a general URL loader. The cover loader keeps no memory cache.

## UI boundary

The FLTK UI must not know Anime1 details, libmpv commands, SQLite queries or metadata provider HTTP APIs. It calls typed actions on `apps/desktop/backend.Service`: list, search, detail, play episode, follow/unfollow, schedule, history. It never receives raw libmpv commands, arbitrary URLs or playback credentials.

FLTK code stays in `apps/desktop/`. Core packages compile and test without FLTK.

## Performance

Ports are in-process Go interface calls. Do not remove a useful boundary for dispatch cost. Real risks are: network latency, UI rendering and decoded cover lifetime, unbounded memory caches, bad SQLite access patterns, excessive background work, libmpv memory, allocations in hot loops.

## Contract tests

Every adapter passes the shared tests in `tests/contract/`.

| Port | Checks |
| --- | --- |
| AnimeSource | valid IDs, ordered episodes, cancellation, no source secrets leak, structurally valid playback source, normalized schedule |
| MetadataProvider | malformed responses fail safely, no raw HTML leaks, cancellation, typed sanitized errors |
| Player | session starts, `Load` replaces the current item without a second player, events stop after close, deterministic cleanup |
| Store | persistence round trips, fail-closed validation |

## Pragmatic SOLID

SOLID is a quality constraint, not a reason to add layers.

| Principle | Rule here |
| --- | --- |
| SRP | A package, file, type or function has one reason to change. Split when the boundary is real, not by line count. |
| OCP | Replace things by adding an adapter or strategy at a real port. Do not grow provider conditionals in core. |
| LSP | Every port implementation honors the same contract, errors, cancellation, lifecycle and security semantics. |
| ISP | Ports are no broader than their consumers need. No interface per helper. |
| DIP | Core depends on core-owned abstractions. Concrete UI, source, metadata, player, persistence and network code stays outward. |

Code quality:

- Names reveal domain intent. No generic `utils`, `helpers`, `common`, `misc`, `manager` dumping grounds.
- Straightforward control flow. Early returns over deep nesting.
- Minimal typed exported API.
- One shared place for each security or policy rule. No duplicates in adapters.
- No dead code, debug scaffolding or speculative abstraction.
- Comments explain non-obvious constraints or why, not syntax.
- Tests describe behavior, not implementation trivia.
- There is no universal file-length limit. A file fails review when it mixes unrelated responsibilities, hides ownership, or sits outside the layer where a maintainer would look for it.

## File placement

Before adding a production file, answer:

1. What single responsibility does it own?
2. Which layer owns that responsibility?
3. Why is this the narrowest existing package that fits?
4. Does it create an inward-dependency violation, cycle, provider leak or dumping ground?
5. Would a maintainer look for this behavior here?

Do not add top-level directories or broad shared packages without a concrete need and independent review. If a feature makes one file or package hold several independent responsibilities, restructure it in the same loop when the fix is local and low-risk. If the fix changes architecture, write an ADR.

## Architecture acceptance

- Core imports no FLTK, Anime1, AniList, Bangumi, libmpv or SQLite driver code.
- Anime1 can be replaced by a fake source in tests.
- The UI can be removed while core tests still pass.
- Source failure does not break the cached library or history.
- Production files have coherent responsibilities and sit in the documented layer.
- No new generic dumping-ground package or file.
- Adapters are substitutable under the shared contracts.
