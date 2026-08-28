# Implementation Status

## Current loop

Loop 08 — Anime1 Schedule — complete

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

## In progress

None

## Blocked

None

## Known technical risks

- Wails v3 remains pre-stable at v3.0.0-beta.12
- Windows and macOS release validation remain deferred to their planned phases
- Playback CDN redirect and cookie-origin enforcement remains a required secure-proxy gate before MPV exposure

## Last verified commands

- `go test -count=1 ./...`
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
- `go tool wails3 dev -config ./build/config.yml -port 9245`

Loops 07–08 passed focused and full tests, race detection, vet, live smoke validation, simplify review, and independent code-quality review.

## Next loop

Loop 09 — Anime1 Adapter Acceptance
