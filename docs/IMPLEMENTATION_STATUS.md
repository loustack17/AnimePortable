# Implementation Status

## Current loop

Loop 15 — Progress/History Orchestration — complete

## Completed

- [x] Root Go module and minimal architecture boundaries
- [x] Wails v3.0.0-beta.12 desktop shell
- [x] Svelte and TypeScript frontend shell
- [x] Source-of-truth documents copied exactly
- [x] Linux CI
- [x] Architecture dependency test
- [x] Local test, build, security, and desktop-start validation
- [x] Clean-checkout CI-order validation
- [x] Independent final review
- [x] Provider-neutral core models and canonical IDs
- [x] AnimeSource, MetadataProvider, Player, and Store ports
- [x] Typed application façade with source-independent local library
- [x] Opaque, cloned, and redacted transient playback source
- [x] Fake-based core replacement and security tests
- [x] Reusable AnimeSource, MetadataProvider, Player, and Store contract suites
- [x] Fake adapters covering supported, unsupported, cancellation, lifecycle, and persistence behavior
- [x] Contract validator rejection tests for invalid adapter output
- [x] Independent code-quality and Oracle final review
- [x] Reusable exact-origin HTTPS client and transport policy
- [x] Connection-time DNS/IP validation with literal-address pinning
- [x] Redirect revalidation and cross-origin sensitive-header removal
- [x] TLS verification, timeout, response-size, and response-header limits
- [x] Sanitized typed errors and centralized URL/header redaction
- [x] Deterministic SSRF, redirect, TLS, cancellation, and body-lifecycle tests
- [x] Anime1 `animelist.json` retrieval through the shared secure HTTP client
- [x] Catalog rows normalized to provider-scoped `SourceRef`s
- [x] Local case-insensitive substring search over the loaded catalog
- [x] Bounded HTML title normalization and JSON catalog parsing
- [x] Fixture tests for malformed input, cancellation, and the `AnimeSource` contract
- [x] Live catalog smoke: 1,893 valid entries and a successful query result
- [x] Oracle and simplify review approval
- [x] Anime1 category archives parsed into provider-scoped episode references
- [x] Bounded sequential pagination with canonical same-origin validation
- [x] Deterministic oldest-to-newest episode order across long-running series
- [x] Non-numeric episode labels preserved without exposing playback tokens
- [x] Exact-limit, malformed-page, cancellation, and no-partial-result coverage
- [x] Live episode smoke: 8-episode and 170-episode archives
- [x] Anime1 episode pages resolved through the fixed `v.anime1.me/api` control endpoint
- [x] Resolver tokens, signed stream URLs, and `e`/`h`/`p` cookies kept transient and redacted
- [x] Non-GET/HEAD redirects rejected before sensitive request bodies can be replayed
- [x] Dynamic Anime1 CDN URLs and playback authorization validated with strict bounds
- [x] Live resolver smoke completed without logging or persisting playback secrets
- [x] Provider-neutral day/time schedule precision and unknown-episode representation
- [x] Anime1 seasonal tables parsed with embedded `Asia/Taipei` calendar rules
- [x] Exact season headers, weekday columns, trusted category links, and stable source order
- [x] Schedule parser limits, malformed schema, cancellation, half-open range, and season-boundary tests
- [x] Live schedule smoke against the current Anime1 seasonal table
- [x] Resolver and schedule simplify passes and independent code-quality reviews
- [x] Named Anime1 adapter acceptance gate covering all five source methods
- [x] Malformed-response zero-value and `%v`/`%+v`/`%#v` secret-redaction regression coverage
- [x] Public `securehttp` to Anime1 adapter composition test without production wiring
- [x] Current Anime1 category-slug compatibility with same-page and cross-page integrity checks
- [x] Live end-to-end adapter smoke for catalog, search, episodes, resolver, and schedule
- [x] CI binding generation before frontend verification, full race detection, and clean-worktree gate
- [x] IPv4 loopback-only playback proxy on an ephemeral port
- [x] High-entropy per-session capability URLs with bounded TTL, registry, and stream limits
- [x] Resolver-owned source URLs and credentials isolated from frontend and local player requests
- [x] Shared exact-origin HTTPS, DNS pinning, SSRF, TLS, and redirect policy for streaming requests
- [x] Strict GET/HEAD, single-range, 200/206/416, MP4 MIME, encoding, and response-header validation
- [x] Deterministic session/server revocation with in-flight request, body, and blocked-writer cancellation
- [x] Concurrent close, saturation, expiry, malformed request, redaction, and body-lifecycle race coverage
- [x] Live Anime1 resolver-to-proxy 1 KiB Range smoke without logging or persisting playback secrets
- [x] Secure playback proxy simplify pass and independent security review approval
- [x] User-configured, PATH, and fixed-platform MPV executable detection
- [x] Fail-closed configured-path validation with sanitized actionable errors
- [x] Windows `.exe`, Scoop Junction, Unix regular-file, and executable-bit validation
- [x] Fixed `--idle=yes` launch without shell, source credentials, or MPV config overrides
- [x] Stable PID plus repeatable concurrent `Done`, `Wait`, and idempotent `Close` lifecycle
- [x] Unix TERM-to-KILL and Windows direct-kill cleanup with bounded stop failures and process reaping
- [x] Deterministic helper-process cancellation, exit, escalation, race, and repeated-cycle coverage
- [x] Windows, Linux, and macOS MPV package cross-compilation
- [x] Live local MPV 0.41 detection, start, PID, close, and reap smoke
- [x] MPV lifecycle simplify pass and independent security/concurrency review approval
- [x] Typed MPV JSON IPC commands for loopback proxy loading, playback properties, observation, stop, and quit
- [x] Random short-lived Unix socket and Windows named-pipe endpoints with bounded startup dialing
- [x] Unix private runtime directory, socket permissions, trusted temp fallback, path bounds, and exact cleanup
- [x] Windows current-user protected named-pipe DACL applied before backend connection
- [x] Bounded JSON framing, request-response demultiplexing, malformed-event tolerance, and sanitized errors
- [x] Coalesced progress events with preserved terminal events and cancellable reader/dispatcher lifecycles
- [x] Deterministic timeout, cleanup, redaction, invalid-media, close, and process-reap coverage
- [x] Live local MPV 0.41 named-pipe property, stop, close, and cleanup smoke
- [x] MPV IPC simplify pass and independent security/lifecycle review approval
- [x] Concrete MPV Player and PlaybackSession adapter with one process per viewing session
- [x] Typed application episode switching with resolver secrets confined to the backend
- [x] Per-episode proxy capability rotation with old-session revocation only after commit
- [x] MPV receive-sequence, pre-load barrier, and post-rejection drain against stale event races
- [x] ACK plus validated `file-loaded` commit with same-PID EP01 → EP02 → EP03 switching
- [x] Fail-closed timeout, cancellation, NACK race, partial cleanup, and Load/Close lifecycle handling
- [x] Coherent canonical playback events with bounded non-blocking delivery and terminal priority
- [x] Deterministic sequence, backpressure, redaction, cleanup, concurrency, and high-count race coverage
- [x] Live local MPV three-episode same-PID smoke with capability revocation checks
- [x] Same-session switching simplify, Oracle, and security/lifecycle review approval
- [x] SQLite Store adapter with canonical local IDs and provider-neutral library state
- [x] Embedded, checksum-verified transactional migrations and schema-tamper rejection
- [x] Durable anime, source references, metadata, following, playback progress/history, and settings CRUD
- [x] Store contract, persistence/reopen, migration, lifecycle, input-validation, and path-safety coverage
- [x] Local database path validation, private Unix artifacts, final-path symlink rejection, and SQLite `nofollow`
- [x] Optional synchronous playback snapshots with fail-closed production capability checks
- [x] Durable resume policy with explicit-start precedence and completed/near-complete suppression
- [x] In-memory progress tracking with 15-second and pause/switch/end/stop/failure/exit checkpoints
- [x] Atomic SQLite progress/history checkpoints with stale-write rejection and monotonic completion
- [x] Double-snapshot episode switching with persistence-before-resolve/load failure safety
- [x] EOF-only completion semantics with distinct stopped/failed terminal states
- [x] Bounded, terminal-safe event delivery across backpressure, reload, and episode ownership changes
- [x] Concurrent idempotent close with cancellable final persistence, raw-player cleanup, and owned-run shutdown
- [x] Restart resume, terminal race, corrupt-state, rollback, backpressure, and lifecycle race coverage
- [x] Live MPV IPC and same-process three-media smoke after playback tracking integration
- [x] Progress/history simplify pass plus independent concurrency and Oracle review approval

## In progress

None

## Blocked

None

## Known technical risks

- Wails v3 remains pre-stable at v3.0.0-beta.12
- Windows and macOS release validation remain deferred to their planned phases

## Last verified commands

- `go test -count=1 ./...`
- `go test ./adapters/source/anime1 -run '^TestAnime1AdapterAcceptance' -count=1`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `go build -o bin/animeportable.exe .`
- `go mod verify`
- `npm run check`
- `npm run build`
- `npm audit --audit-level=high`
- `npm ci`
- `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
- `go tool wails3 doctor`
- `go tool wails3 build`
- `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 -run '^TestLiveMPVLoadsThreeMediaURLsOnOneProcess$' ./adapters/player/mpv`
- `go tool wails3 dev -config ./build/config.yml -port 9245`
- `go test -count=1 ./adapters/persistence/sqlite`
- `go test -race -count=1 ./adapters/persistence/sqlite`
- `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 ./adapters/player/mpv -run 'TestLive' -v`
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./core ./adapters/... ./tests/...`
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./core ./adapters/... ./tests/...`

Loops 07–15 passed focused and full tests, race detection, vet, live smoke validation where applicable, simplify review, and independent code-quality review.

## Next loop

Loop 16 — Following

- add local follow and unfollow operations
- compare watched progress with the latest known episode
- preserve follow state across restart using canonical local anime identity
