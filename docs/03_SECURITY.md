# Security and Privacy Requirements

## 1. Security model

Treat all remote data as untrusted, including data from services we normally trust.

Trust boundaries:

1. Internet -> Go backend
2. Go backend -> frontend
3. Go backend -> local playback proxy
4. local playback proxy -> MPV
5. MPV IPC -> Go backend
6. local persistence

The Go backend is the security broker.

Hard rules:

- Internet never directly talks to Svelte.
- Remote Anime1 media URLs never directly reach MPV.
- Frontend never receives playback credentials.
- Frontend never sends arbitrary MPV commands.
- Frontend never supplies arbitrary network URLs for playback.

## 2. Privacy baseline

MVP has:

- no account
- no telemetry
- no analytics
- no ads
- no advertising SDK
- no automatic crash upload
- no cloud sync
- no user tracking ID
- no elevated privileges

Local data may include:

- following
- history
- playback progress
- metadata cache
- settings

These stay local.

## 3. Sensitive playback data

Sensitive runtime data can include:

- stream URL
- cookies
- referer
- user-agent if source-specific
- temporary token
- signed URL query
- proxy session token
- MPV IPC endpoint

Rules:

- memory-only unless absolutely required
- never SQLite
- never frontend
- never logs
- never crash reports
- never command-line arguments when avoidable
- minimize lifetime
- minimize copies

Do not claim guaranteed secure erasure from RAM. Go GC/runtime may retain copies temporarily. The practical strategy is short lifetime, no persistence, no logging, and narrow ownership.

## 4. Secure playback proxy

Use a local proxy as a credential-isolation boundary.

Flow:

`MPV -> localhost random session URL -> Go proxy -> validated remote media`

Requirements:

- bind only to loopback
- never `0.0.0.0`
- random high-entropy per-session token
- short-lived session
- invalidate immediately on playback/session end
- no guessable `/episode/7` paths
- never expose source credentials to MPV argv

## 5. MPV IPC security

MPV JSON IPC is not considered a secure network protocol.

Requirements:

### Unix

- random socket path
- user-private runtime directory
- directory permissions restricted to current user
- cleanup socket after session

### Windows

- random named pipe
- current-user access where practical
- short-lived endpoint
- cleanup on session end

Frontend never receives the IPC endpoint.

## 6. External network client

All outbound HTTP must use one controlled secure client or transport policy.

Adapters must not instantiate arbitrary unmanaged clients.

Requirements:

- reasonable connect/read/request timeouts
- reusable transport/connection pool
- cancellation through `context.Context`
- response bodies always closed
- bounded response sizes where possible

## 7. Protocol allowlist

Remote external requests:

- HTTPS only

The only normal HTTP exception is the app-owned loopback playback proxy.

Reject:

- `file:`
- `ftp:`
- `gopher:`
- `data:`
- `javascript:`
- `smb:`
- `nfs:`
- arbitrary custom schemes

## 8. TLS

Mandatory:

- certificate verification
- hostname verification
- valid expiry chain

Forbidden:

```go
InsecureSkipVerify: true
```

Do not implement certificate pinning in MVP unless a reliable upstream pinning model exists.

## 9. SSRF protection

Before connecting to a remote target, resolve and validate destination IPs.

Reject at minimum:

- loopback
- private IPv4
- IPv4 link-local
- IPv6 loopback
- IPv6 private/local ranges where applicable
- IPv6 link-local
- multicast
- unspecified addresses
- known metadata-service addresses/ranges

Do not trust only the hostname string.

Defend against DNS rebinding / pinning by validating the actual resolved address used for connection.

## 10. Redirect policy

Do not blindly auto-follow redirects.

For each redirect:

1. parse destination
2. apply protocol policy
3. apply host/domain policy
4. resolve destination
5. apply SSRF/IP policy
6. only then continue

A legal source redirecting to localhost/private IP must be blocked.

Sensitive headers must not be forwarded to unrelated origins.

## 11. Domain policy

Use explicit known-origin policy for:

- Anime1 API/site origins
- AniList API origin
- Bangumi API origin

Playback CDN origins may be more dynamic, but must still satisfy:

- HTTPS
- valid TLS
- public IP
- acceptable redirect chain
- expected media response

Unknown playback origins should fail closed unless explicitly approved by resolver policy.

## 12. Remote HTML policy

Never render Anime1 raw HTML.

Use parser-only extraction into typed models.

Ignore:

- scripts
- iframes
- popup links
- ads
- arbitrary anchors
- JavaScript redirects

Raw HTML must not be passed to the frontend.

## 13. Metadata text policy

AniList/Bangumi descriptions are remote input.

Frontend must not render them using raw HTML injection.

Preferred MVP behavior:

- strip/sanitize markup
- render plain text

Do not use raw remote HTML rendering.

## 14. Remote image policy

Covers are untrusted binary data.

Requirements:

- HTTPS
- allow expected image content types only
- byte-size limit
- timeout
- decoded-dimension limit
- avoid decoding unlimited images simultaneously
- lazy-load images
- reject malformed or suspicious content

Do not allow image URLs to become arbitrary fetch primitives.

## 15. Media validation

Before proxying a playback source to MPV, validate:

- scheme
- TLS
- origin policy
- resolved IP
- redirects
- HTTP status
- expected content type / manifest type
- response sanity

Optional low-cost sanity checks may include:

- duration
- codec
- resolution
- suspiciously tiny content

Example suspicious case:

Expected anime episode ~24 minutes, source reports ~8 seconds.

Such a source should not autoplay without validation.

## 16. Source integrity limitations

If a legitimate upstream itself is compromised and serves malicious or wrong content from:

- correct hostname
- correct TLS
- valid response structure

the client cannot cryptographically prove content authenticity unless the upstream provides signed manifests/hashes.

Therefore the goal is risk reduction and fail-closed validation, not an impossible guarantee of perfect provenance.

## 17. Metadata integrity

Never blindly trust the first search result.

Use confidence scoring and provider cross-checking.

Low confidence:

- show no metadata
- do not attach incorrect metadata

This is both correctness and integrity protection.

## 18. Frontend restrictions

Frontend must not:

- `fetch()` arbitrary Internet resources directly
- receive playback cookies/tokens
- receive signed media URL where avoidable
- send arbitrary MPV commands
- send arbitrary playback URLs
- render raw remote HTML
- navigate WebView to external content

Frontend may invoke typed actions such as:

- `PlayEpisode(id)`
- `Search(query)`
- `Follow(id)`
- `LoadSchedule()`

## 19. External links

MVP may omit external links entirely.

If added later:

- never auto-open
- show/validate destination
- open via OS default browser
- never navigate the application WebView to remote pages
- consider allowlist/confirmation policy

## 20. WebView security

Bundle application JS/CSS locally.

Use a restrictive Content Security Policy.

Do not enable:

- remote JS
- arbitrary iframes
- inline/eval-like execution unless framework tooling absolutely requires a narrowly-scoped exception

The WebView is an application UI, not a browser.

## 21. Logging

Use structured logging.

Central redaction is mandatory.

Never log:

- cookies
- Authorization
- tokens
- proxy session IDs
- authenticated/signed URL query parameters
- full private playback URLs

Prefer:

- provider
- host
- operation
- episode local ID
- sanitized error category

## 22. Crash handling

No automatic crash upload in MVP.

If crash reporting is added later:

- explicit opt-in
- redact paths/usernames
- redact URLs
- redact tokens/cookies
- redact watch history where possible

## 23. Local database privacy

SQLite is local-only.

Store in user-specific application-data directories with normal OS user permissions.

MVP does not require DB encryption.

Do not run as administrator/root.

## 24. Memory/resource safety

Go GC does not prevent logical memory leaks.

Must prevent:

- goroutine leaks
- stuck channels
- unclosed HTTP bodies
- unclosed sockets
- forgotten IPC readers
- abandoned proxy sessions
- unbounded maps/caches
- tickers not stopped
- contexts not cancelled

Long-running work must have a lifecycle.

MPV session close must:

1. cancel session context
2. stop IPC readers
3. persist final progress
4. invalidate proxy session
5. close sockets/streams
6. wait/reap process as appropriate
7. delete IPC endpoint
8. release references

## 25. Cache rules

Prefer SQLite over large in-memory caches.

Any memory cache must have:

- upper bound
- TTL
- eviction

Metadata refresh concurrency must be bounded.

## 26. Supply-chain security

CI should include:

- `go mod verify`
- `go vet`
- `go test`
- `govulncheck`
- frontend lint/typecheck
- frontend dependency audit
- locked dependency files
- dependency update automation where appropriate

Keep dependencies minimal.

## 27. Security hard requirements

The implementation is not accepted unless all of the following hold:

- no telemetry by default
- no account requirement
- no persistent playback credentials
- no secrets in logs
- no secrets in frontend
- no secrets in MPV argv where avoidable
- loopback-only proxy
- random short-lived proxy sessions
- private short-lived MPV IPC
- HTTPS-only remote network
- TLS verification
- redirect revalidation
- SSRF protection
- private/link-local/loopback blocking
- no raw remote HTML
- no remote JavaScript execution
- image limits
- typed frontend commands
- no arbitrary frontend URL playback
- no elevated privilege
- deterministic cleanup
- bounded caches
- fail-closed external validation
