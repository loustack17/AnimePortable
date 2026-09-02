<!-- SPDX-License-Identifier: MPL-2.0 -->

# MVP Acceptance Criteria

Use these criteria as hard gates.

Each criterion should eventually be tracked as Pass / Fail / Blocked with evidence.

---

## A. Architecture

### ARCH-001
Core imports no Wails packages.

### ARCH-002
Core imports no Anime1 implementation package.

### ARCH-003
Core imports no MPV implementation package.

### ARCH-004
Core imports no SQLite driver.

### ARCH-005
Core imports no AniList/Bangumi concrete implementation.

### ARCH-006
Anime1 implements the core AnimeSource port.

### ARCH-007
AniList and Bangumi implement MetadataProvider.

### ARCH-008
MPV implements Player/PlaybackSession.

### ARCH-009
SQLite implements Store.

### ARCH-010
Core tests run without desktop UI.

### ARCH-011
A fake AnimeSource can replace Anime1 in core tests.

### ARCH-012
Local following/history/progress use canonical local IDs, not Anime1 IDs.

### ARCH-013
Provider-specific tokens are not present in core models.

### ARCH-014
Source failure does not prevent cached history/following from loading.

### ARCH-015
Architecture remains minimal; no unnecessary enterprise-layer chain.

---

## B. Functional browsing

### FUNC-001
App opens to usable cached content without waiting for network when cache exists.

### FUNC-002
Anime catalog is available.

### FUNC-003
Search works.

### FUNC-004
Anime detail shows cover, title, native title, synopsis, season/year, studio, episode count when metadata is available.

### FUNC-005
Episode list is correctly ordered.

### FUNC-006
Schedule is available.

### FUNC-007
Following works.

### FUNC-008
History works.

### FUNC-009
Continue Watching works.

---

## C. Playback

### PLAY-001
MPV can be detected automatically when installed in supported locations/PATH.

### PLAY-002
User can manually configure MPV path if auto-detection fails.

### PLAY-003
Missing MPV produces a clear actionable error.

### PLAY-004
App starts MPV playback without exposing Anime1 credentials in frontend.

### PLAY-005
Remote playback credentials are not passed in normal MPV argv.

### PLAY-006
Playback goes through the secure localhost proxy.

### PLAY-007
A viewing session uses one persistent MPV process.

### PLAY-008
Switching EP01 -> EP02 -> EP03 keeps the same MPV PID.

### PLAY-009
MPV user's existing config remains effective.

### PLAY-010
Progress is tracked through IPC.

### PLAY-011
Progress persists across app restart.

### PLAY-012
Resume playback works.

### PLAY-013
Normal end-of-file can autoplay next episode.

### PLAY-014
No in-video application overlay exists.

### PLAY-015
No danmaku exists.

---

## D. Keyboard and usability

### UX-001
Core workflow works without a mouse.

Flow:

`Launch -> Find anime -> Open -> Select episode -> Play`

### UX-002
Arrow-key navigation has visible focus.

### UX-003
Enter performs open/select.

### UX-004
Space plays selected episode where appropriate.

### UX-005
Esc navigates back/escapes transient UI.

### UX-006
`/` focuses search or equivalent search action.

### UX-007
Ctrl/Cmd+K provides quick search/command access.

### UX-008
Mouse click/hover/scroll remain fully functional.

### UX-009
No interaction requires mobile-style positional tapping zones.

### UX-010
UI is calm and minimal; no decorative or social feature creep.

---

## E. Metadata correctness

### META-001
AniList is primary metadata provider.

### META-002
Bangumi can be used as fallback/cross-check.

### META-003
Metadata matcher does not blindly choose first search result.

### META-004
Traditional/Simplified/punctuation/full-width normalization is handled.

### META-005
Low-confidence match results in missing metadata rather than incorrect metadata.

### META-006
Metadata remains usable from cache when provider is unavailable.

---

## F. External network security

### SEC-001
Frontend performs no arbitrary direct Internet fetches.

### SEC-002
Remote requests use HTTPS only except app-owned loopback HTTP.

### SEC-003
TLS verification is enabled.

### SEC-004
`InsecureSkipVerify` is not enabled.

### SEC-005
Unsupported schemes are rejected.

### SEC-006
Loopback destinations are blocked for remote external fetches.

### SEC-007
Private IP destinations are blocked.

### SEC-008
Link-local destinations are blocked.

### SEC-009
IPv6 private/loopback/link-local cases are handled.

### SEC-010
Redirect destinations are revalidated.

### SEC-011
Redirect to localhost/private IP is blocked.

### SEC-012
DNS/IP validation occurs on the actual connection destination.

### SEC-013
Unknown playback origins fail closed under source policy.

### SEC-014
Remote Anime1 HTML is parsed, never executed/rendered raw.

### SEC-015
Remote descriptions are sanitized/plain text.

### SEC-016
Raw remote HTML is never sent to Svelte.

### SEC-017
Remote image byte size is bounded.

### SEC-018
Remote image dimensions are bounded before expensive decode where feasible.

### SEC-019
Unexpected media/content types fail safely.

---

## G. Privacy and secret handling

### PRIV-001
No account is required.

### PRIV-002
No telemetry is sent by default.

### PRIV-003
No analytics SDK.

### PRIV-004
No advertising SDK.

### PRIV-005
No automatic crash upload.

### PRIV-006
Watch history remains local.

### PRIV-007
Cookies are never stored in SQLite.

### PRIV-008
Temporary stream tokens are never stored in SQLite.

### PRIV-009
Proxy session tokens are never stored persistently.

### PRIV-010
Secrets are not returned to frontend.

### PRIV-011
Logs redact cookies/tokens/authenticated URL data.

### PRIV-012
Application does not require administrator/root privileges.

---

## H. Local proxy security

### PROXY-001
Proxy binds only to loopback.

### PROXY-002
Proxy uses high-entropy per-session tokens.

### PROXY-003
Unknown session token is rejected.

### PROXY-004
Expired session token is rejected.

### PROXY-005
Session is invalidated on playback/session end.

### PROXY-006
Proxy still enforces secure remote target policy.

### PROXY-007
Required Range requests work for media playback.

---

## I. MPV IPC security/lifecycle

### IPC-001
IPC endpoint is random/short-lived.

### IPC-002
Unix IPC lives in a user-private location with restricted permissions where practical.

### IPC-003
Windows named pipe is scoped to current user where practical.

### IPC-004
Frontend never receives raw IPC endpoint.

### IPC-005
Frontend cannot submit raw MPV commands.

### IPC-006
IPC reader terminates after session close.

### IPC-007
IPC endpoint is cleaned up.

---

## J. Resource safety

### RES-001
HTTP response bodies are always closed.

### RES-002
Background goroutines have cancellation paths.

### RES-003
Tickers/timers are stopped.

### RES-004
Sockets are closed.

### RES-005
MPV processes are reaped/cleaned up appropriately.

### RES-006
Proxy sessions expire and are removed.

### RES-007
Memory caches are bounded.

### RES-008
Repeated episode switching does not cause monotonic RAM growth.

### RES-009
Repeated MPV start/stop does not cause monotonic goroutine growth.

### RES-010
Cancelled searches/requests do not leave background work running.

---

## K. Performance and perceived speed

### PERF-001
Cached startup does not block on Anime1.

### PERF-002
Cached startup does not block on AniList/Bangumi.

### PERF-003
Cached content appears immediately enough to feel responsive; target approximately <1 second perceived startup on normal supported hardware, not a hard universal benchmark.

### PERF-004
Metadata refresh concurrency is bounded.

### PERF-005
Cover images are lazy-loaded or otherwise bounded.

### PERF-006
Application is near-idle while MPV is playing and no refresh work is active.

### PERF-007
No high-frequency SQLite writes for playback progress.

---

## L. Cross-platform

### PLATFORM-001
Windows build succeeds.

### PLATFORM-002
Linux build succeeds on defined supported environment.

### PLATFORM-003
macOS build succeeds.

### PLATFORM-004
Windows named-pipe MPV IPC is validated.

### PLATFORM-005
Unix-socket MPV IPC is validated on Linux.

### PLATFORM-006
Unix-socket MPV IPC is validated on macOS.

### PLATFORM-007
Keyboard workflow is validated on each supported platform.

---

## M. CI and release

### CI-001
`go test ./...` passes.

### CI-002
`go vet ./...` passes.

### CI-003
`govulncheck` is run in CI.

### CI-004
Frontend lint passes.

### CI-005
Frontend typecheck passes.

### CI-006
Frontend build passes.

### CI-007
Dependency lockfiles are committed.

### CI-008
`go mod verify` passes.

### CI-009
Platform release artifacts can be produced.

---

# MVP final gate

MVP status may be marked COMPLETE only when:

- no critical acceptance criterion is Fail
- security criteria pass
- architecture criteria pass
- same-session MPV switching passes
- keyboard-only workflow passes
- resource stress tests show no clear leak pattern
- all three desktop platforms have a validated build path
