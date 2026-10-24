<!-- SPDX-License-Identifier: MPL-2.0 -->

# Security and Privacy

Treat all remote data as untrusted, including data from services we normally trust. The Go backend is the security broker. The Windows UI is FLTK. Playback is in-process libmpv (ADR-020, ADR-023). Linux and macOS are deferred.

## Trust boundaries

| # | Boundary | Rule |
| --- | --- | --- |
| 1 | Internet -> Go backend | One controlled HTTPS client. Validate everything. |
| 2 | Go backend -> FLTK UI | Typed actions and typed results only. |
| 3 | Go backend -> loopback playback proxy | Proxy holds source credentials. Bound to loopback. |
| 4 | Loopback proxy -> libmpv | libmpv gets only a random loopback session URL. |
| 5 | libmpv DLL loading | Absolute path, SHA-256 pinned, loaded only when playback starts. |
| 6 | Local persistence (SQLite) | Local file, no secrets stored. |
| 7 | Anime1 resolver -> playback CDN | Cookies and signed URLs in memory only; CDN host under `v.anime1.me`, HTTPS, `video/mp4`. |
| 8 | Backend -> forward proxy | CONNECT only, TLS to the original host; see Forward proxy. |
| 9 | Metadata, AniSkip, GitHub, cover origins | Fixed origin allowlists, bounded bodies, plain text or validated images only. |
| 10 | Backend -> files in the data folder | Database, failure log and debug timeline; fixed vocabulary, no secrets. |
| 11 | Portable folder and environment | Pinned DLL, data folder location file, `ANIMEPORTABLE_*` variables. |

Loop 38 checked every row against the sections below (docs/loops/LOOP38.md, 38a).

Hard rules:

- The Internet never talks to the UI directly.
- Remote Anime1 media URLs never reach libmpv directly.
- The UI never receives playback credentials.
- The UI never sends raw libmpv commands.
- The UI never supplies arbitrary network URLs for playback.

## Privacy baseline

No account, telemetry, analytics, ads, ad SDK, automatic crash upload, cloud sync, user tracking ID or elevated privilege. Following, history, progress, metadata cache and settings stay local. Do not run as administrator.

## Sensitive playback data

Sensitive: stream URL, cookies, referer, source-specific user agent, temporary tokens, signed URL queries, proxy session token.

- Keep in memory only. Never SQLite, UI, logs or crash reports.
- Never in command-line arguments when avoidable. No secrets are passed to the player.
- Minimize lifetime and copies.
- Do not claim secure erasure from RAM. The strategy is short lifetime, no persistence, no logging, narrow ownership.

## Secure playback proxy

Flow: `libmpv -> localhost random session URL -> Go proxy -> validated remote media`.

- Bind to loopback only. Never `0.0.0.0`.
- High-entropy random per-session token. No guessable paths such as `/episode/7`.
- Short-lived session. Invalidate it immediately when playback or the session ends.
- Never expose source credentials to libmpv.

## libmpv DLL loading (rewritten 2026-10-13, owner decision; replaces the external-MPV IPC rules)

There is no MPV process and no IPC endpoint (named pipe or socket). libmpv runs in the app process.

- The packaged `libmpv-2.dll` sits next to the executable. Its expected SHA-256 is the constant in `internal/runtimepin/libmpv.go`.
- Load by absolute path only, and only when playback starts. Never load by bare name or from the search path.
- Verify the hash before use. Keep a read-only, non-write-shareable file handle through `LoadLibraryEx` and symbol binding. Reject reparse points and non-regular files.
- Limit dependency search to the DLL directory and Windows System32. libmpv's own imports load with `LOAD_LIBRARY_SEARCH_DLL_LOAD_DIR | LOAD_LIBRARY_SEARCH_SYSTEM32`; libraries loaded by name later (by libmpv, FFmpeg or the app) search only the app folder and System32, set process-wide at startup with `SetDefaultDllDirectories` (Loop 38).
- A missing or altered DLL must leave Home usable and show a safe inline playback error.
- An explicit absolute DLL path plus user-supplied SHA-256 (`ANIMEPORTABLE_LIBMPV_OVERRIDE` and `ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256`) may replace the bundled DLL. The same checks apply. Never accept a DLL silently from the search path.
- The player starts with `config=no`. The user's mpv config files and scripts are not loaded. The pinned DLL is built without scripting, so mpv's built-in scripts (for example the one that starts `yt-dlp`) do not exist in it; an override DLL built with scripting would have them.
- Only FFmpeg's MP4 demuxer may open the stream (`demuxer=lavf`, `demuxer-lavf-format=mp4`). A body labelled `video/mp4` that is really a playlist, EDL or HLS manifest fails to open, so libmpv never fetches an address listed inside it past the proxy (Loop 38).
- The UI cannot issue arbitrary libmpv commands. It uses typed player callbacks.
- Playback source detail is shown to the user only as safe, generic errors.

DLL licensing is a settled owner decision (ADR-023). Do not reopen it.

## External network client

All outbound HTTP in the shipped app uses one controlled secure client (`adapters/network/securehttp`). Adapters must not create unmanaged clients. (Development tools under `tools/` are not shipped.)

- Connect, read and request timeouts.
- Reusable transport and connection pool.
- Cancellation through `context.Context`.
- Response bodies always closed.
- Response sizes bounded where possible.

## Protocol allowlist

Remote requests are HTTPS only. The only HTTP exception is the app-owned loopback playback proxy. Reject `file:`, `ftp:`, `gopher:`, `data:`, `javascript:`, `smb:`, `nfs:` and any custom scheme.

## TLS

Certificate, hostname and expiry verification are mandatory. `InsecureSkipVerify: true` is forbidden. No certificate pinning unless a reliable upstream pinning model exists.

## SSRF protection

Before connecting, resolve the destination and validate the actual IP used for the connection (this defends against DNS rebinding). Reject loopback, private IPv4, IPv4 link-local, IPv6 loopback, private and local IPv6, IPv6 link-local, multicast, unspecified, and known metadata-service addresses. Never trust only the hostname string.

## Forward proxy (owner-approved 2026-10-22, Loop 37)

The user may route outbound requests through one HTTP forward proxy: none (default), the Windows system proxy (WinINet `ProxyEnable` / `ProxyServer` under HKCU, read only), or a manual `host:port`.

- HTTPS only through `CONNECT`; the request inside the tunnel keeps full TLS verification against the original hostname. Plain-HTTP proxying of requests is not allowed.
- The client sends `CONNECT <hostname>:443` and the proxy resolves the name (owner decision 2026-10-22: where Anime1 is blocked, local DNS is often blocked too, and rule-based proxy tools route by name). Only while a proxy is in use, the SSRF IP check above is replaced by the proxy's resolution; the hostname must still pass the origin allowlist or the resolver's playback-origin policy, HTTPS and full TLS verification. A destination written as an IP literal is still checked against the SSRF rules. Without a proxy the SSRF rules apply unchanged.
- The proxy address itself is exempt from the SSRF block list, because local proxy tools listen on loopback or LAN. Only that one configured address and port is exempt, and it is used only as the proxy hop.
- Origin allowlists and redirect rules are unchanged.
- No proxy credentials, PAC files or SOCKS. A manual address must be a host name or IP plus a port; anything with a scheme, user information, path or spaces is rejected. The proxy address is never logged in diagnostics or failure logs.
- The Windows system proxy is read as `host:port`, `http://host:port` (no path) or a `http=...;https=...` list; a host without a port means port 80, as in WinINet. If the Windows system proxy is unusable (SOCKS, PAC, unreadable), the app connects directly; 測試連線 shows this. When both a PAC script and a static proxy are set, the app uses the static proxy; it does not run PAC scripts.

## Update check (Loop 37)

`檢查更新` is the only update check. It runs only when the user presses it and asks `https://api.github.com` for the releases of `loustack17/AnimePortable` through the secure client (allowlisted origin, bounded body). It never downloads or runs anything; it shows the version and can open the release page in the default browser.

## Opening times (owner-approved 2026-10-22, Loop 37c)

`https://api.aniskip.com` is asked only when an episode starts playing, through the secure client (own allowlist with that one origin, bounded body, proxy setting applied like every other request).

- Sent: only the MyAnimeList id, the episode number and the episode length in whole seconds, in the path and query of one `GET /v2/skip-times/...`. No title, user data, cookies, Anime1 address or identifiers of the user. The usual `User-Agent` only.
- To find the MyAnimeList id the anime's title goes to AniList, an origin already allowed for metadata; only a match `core.MatchMetadata` accepts is used.
- The response is untrusted: JSON only, at most 64 KiB, at most 20 results; only `op`/`mixed-op` results with finite `startTime` ≥ 0, `endTime` > `startTime`, `endTime` no later than the episode length, an opening of at most 4 minutes, and a submission length within 5 seconds of the episode are used. Anything else is ignored, and a malformed response means no button. No text from the response reaches the UI.
- Failures are silent for the user and never delay or stop playback; nothing is logged. Only the MyAnimeList id per anime is kept, in memory for the app run.

## Redirect policy

Do not auto-follow blindly. For each redirect: parse the destination, apply protocol policy, apply host policy, resolve, apply the SSRF policy, then continue. A legal source redirecting to localhost or a private IP is blocked. Do not forward sensitive headers to unrelated origins.

## Domain policy

Use explicit origin allowlists for the Anime1 site and API, the AniList API, the Bangumi API, for the update check only the GitHub API, and for opening times only the AniSkip API (Loop 37c). Playback CDN origins may be dynamic but must satisfy HTTPS, valid TLS, public IP, acceptable redirects and an expected media response. Unknown playback origins fail closed unless resolver policy approves them.

## Remote HTML

Never render raw Anime1 HTML. Use parser-only extraction into typed models. Ignore scripts, iframes, popups, ads, arbitrary anchors and JavaScript redirects. Raw HTML never reaches the UI.

## Metadata text

AniList and Bangumi fields are remote input. Normalize titles, native titles, descriptions, seasons and studios to bounded plain text and render plain text. Reject unsafe or non-canonical cached metadata on write and on read.

## Remote images

Covers are untrusted binary data.

- Fixed AniList and Bangumi HTTPS cover origins.
- The secure client's DNS, SSRF, redirect, TLS and timeout rules apply.
- Final HTTP status 200.
- JPEG or PNG only. Header MIME, content signature and decoder format must agree.
- Identity content encoding only.
- 4 MiB encoded limit.
- 4096 pixels per dimension and 4,194,304 pixels total, checked before full decode.
- At most four concurrent fetch-and-validate operations.
- Load on demand, no memory cache.
- Fixed sanitized errors and zero content on failure.

The UI action returns typed validated bytes and dimensions. It is never a caller-configurable URL fetch.

## Media validation

Before proxying a source to libmpv, validate scheme, TLS, origin policy, resolved IP, redirects, HTTP status, expected content or manifest type and response sanity. Optional low-cost checks: duration, codec, resolution, suspiciously tiny content (for example an 8-second "episode"). Such a source must not autoplay without validation.

Limit: if a legitimate upstream is itself compromised and serves wrong content with correct host, TLS and structure, the client cannot prove authenticity without signed manifests. The goal is risk reduction and fail-closed validation.

## Metadata integrity

Never trust the first search result. Use confidence scoring and provider cross-check. Low confidence shows no metadata.

## UI restrictions

The FLTK UI must not: fetch Internet resources directly, receive playback cookies or tokens, receive signed media URLs where avoidable, send raw libmpv commands or arbitrary playback URLs, render raw remote HTML, or open unvalidated external content. It may call typed Go actions such as `PlayEpisode(id)`, `Search(query)`, `Follow(id)` and `LoadSchedule()`. Widgets get no provider clients, no persistence handles and no credentials.

## External links

The MVP may omit them. If added: never auto-open, show the destination, open in the OS default browser, never load remote pages in the app.

## Logging

Structured logging with central redaction. Never log cookies, Authorization values, tokens, proxy session IDs, signed URL query parameters or full private playback URLs. Prefer provider, host, operation, local episode ID and a sanitized error category.

The app writes two diagnostic files next to the database, both with a fixed vocabulary of stages, statuses and categories (no URLs, titles, ids, error text or proxy address): `playback-failures.log` (newest 50 lines) and, only when `ANIMEPORTABLE_START_TIMELINE=1` is set at launch, `playback-timeline.log` (at most 100 starts of 200 events each per run). `ANIMEPORTABLE_PLAYBACK_DIAGNOSTICS=1` writes playback counters and GPU names next to the exe.

## Crash handling

No automatic crash upload. If added later: explicit opt-in, and redact paths, usernames, URLs, tokens, cookies and watch history.

## Local database

SQLite is local only. The caller supplies an absolute path. In the portable build that path is `<extracted folder>/data`.

- The final database path must be a regular file, never a symlink or other reparse point. On Windows the `Lstat` checks on the path and its folders are the guard; SQLite `nofollow` is also set but only its Unix file layer implements it.
- A same-user replacement race between check and open remains a known local residual risk.
- On Windows, files inherit the ACL of their directory. The database lives in the portable folder and inherits that folder's ACL. If the folder is in a shared or world-readable location, other local users can read the watch history (residual risk).
- No database encryption in the MVP.

## Resource safety

Prevent goroutine leaks, stuck channels, unclosed HTTP bodies and sockets, abandoned proxy sessions, unbounded maps and caches, unstopped tickers and uncancelled contexts. Long-running work needs a lifecycle.

Closing a playback session must, in order:

1. Cancel the session context.
2. Stop event readers.
3. Persist final progress.
4. Invalidate the proxy session.
5. Close sockets and streams.
6. Destroy the libmpv player and release the DLL handle when the last session ends.
7. Release references.

Caches prefer SQLite. Any memory cache needs an upper bound, TTL and eviction. Metadata refresh concurrency is bounded.

## Supply chain

CI runs `go mod verify`, `go vet`, `go test`, `govulncheck`, native build and static checks, and a dependency audit with locked dependency files. Keep dependencies minimal. Third-party notices ship in `THIRD_PARTY_NOTICES.md`.

## Security acceptance

Not accepted unless all hold:

- no telemetry by default, no account requirement
- no persistent playback credentials
- no secrets in logs, the UI or player arguments
- loopback-only proxy with random short-lived sessions
- pinned, hash-verified, absolute-path libmpv load; safe failure when it is missing or altered
- HTTPS only, TLS verification, redirect revalidation
- SSRF protection and private, link-local and loopback blocking
- no raw remote HTML, no remote JavaScript execution
- image limits enforced
- typed UI actions, no arbitrary UI URL playback
- no elevated privilege
- deterministic cleanup
- bounded caches
- fail-closed external validation
