<!-- SPDX-License-Identifier: MPL-2.0 -->

# Acceptance Criteria

Look up a criterion, who may decide PASS, and the minimum evidence. These criteria are hard gates. Product facts are in [01_PRODUCT.md](01_PRODUCT.md). Loop mapping is in [04_ROADMAP.md](04_ROADMAP.md). The loop process is in [05_WORKFLOW.md](05_WORKFLOW.md). Do not require one internal design when several correct designs satisfy a criterion.

Marks "(rewritten 2026-10-13, owner decision)" mean: in-process libmpv replaces external MPV, on-video controls are required, and the rules are tool-neutral.

## 1. Statuses, verifiers and evidence

Each criterion has one status: `NOT_RUN`, `PASS`, `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, `NOT_APPLICABLE`.

| Status | Meaning |
| --- | --- |
| `PASS` | Every mandatory observable holds, all required deterministic checks pass on the final relevant repository state, no required regression fails, no must-fix review finding is open, any required owner check returned `HUMAN_PASS`, and no test, CI or criterion was weakened to get the result. |
| `FAIL` | Reproducible evidence shows a required observable is not met. A failing deterministic check is `FAIL` even if a reviewer thinks the code is fine. |
| `BLOCKED` | The required verifier cannot run because of an external environment, platform, tool or upstream dependency. A substitute check adds evidence but does not make a blocked check `PASS`. |
| `NEEDS_HUMAN` | A mandatory human judgment or approval is pending, or automation cannot decide reliably. |
| `NOT_APPLICABLE` | The criterion genuinely does not apply. Record the reason. It cannot hide unfinished work. |

Verifier types say who may decide PASS:

| Code | Name | Who decides PASS |
| --- | --- | --- |
| `A` | Automated | An executable pass or fail result. |
| `Y` | Automated + review | Automated evidence first, then an independent read-only review. |
| `YH` | Automated + review + owner check | Automated evidence and review first, then the owner makes the final product decision. |
| `H` | Owner check | Human observation and approval alone. |

- "Automated + review (A/Y)" means Automated for the mechanical part plus review for the rest. "Review (Y)" means review only.
- A reviewer is read-only. A reviewer cannot turn a deterministic FAIL into PASS.
- Native Windows tests (cgo, FLTK, libmpv) run locally before push and in GitHub Windows CI for the pushed commit.
- If a platform or runner is unavailable, the criterion is `BLOCKED`, not PASS.
- Cross-compiling proves only that the code builds, not that it runs.
- "Negative tests" means failure-path and adversarial cases, not only the happy path.

Evidence for a status names: criterion ID, commit or working-tree state, exact command or manual procedure, exit code or result, platform, test name and count or measurement, log or artifact, independent reviewer result, and owner result. Statements like "looks correct" are not evidence. A later change that can affect a criterion invalidates its earlier `PASS` until re-verified.

When signals disagree, use this order: (1) the documented product, architecture and security contract; (2) deterministic executable evidence; (3) independent AI review, which finds gaps but cannot override a deterministic failure; (4) human judgment for subjective, product and exception gates. An implementation agent's self-report is never enough for `PASS`.

A criterion cannot pass if green status was obtained by weakening the thing that measures success. Changes to tests, CI or criteria follow the controls in [05_WORKFLOW.md](05_WORKFLOW.md).

## 2. When an owner check or extra review is required

An owner check is required when a loop:

- changes product, architecture, security, acceptance meaning, or an ADR;
- weakens, removes or skips a required test or CI gate;
- adds a trust boundary, privilege, credential path, arbitrary network, file or process capability, or a destructive migration;
- adds a runtime, native or security-sensitive dependency;
- hits non-convergence or a budget stop and needs a strategy decision;
- asks for an exception for a blocked, flaky or failed gate;
- is the final release acceptance.

Every production-code loop needs an independent fresh-context read-only code-quality review. A specialized review is also required when the loop:

- changes security-sensitive code;
- changes concurrency, process or resource lifecycle;
- changes a persistence migration or identity mapping;
- changes a public desktop action or API contract;
- changes a required test or verifier;
- needed more than one corrective iteration.

## 3. Criteria by category

### A. Architecture

| ID | Requirement | Verifier |
| --- | --- | --- |
| ARCH-001 | Core imports no UI toolkit package (FLTK) and no Wails or Fyne package. | Automated (A) |
| ARCH-002 | Core imports no Anime1 implementation package. | Automated (A) |
| ARCH-003 | Core imports no libmpv implementation package. | Automated (A) |
| ARCH-004 | Core imports no SQLite driver. | Automated (A) |
| ARCH-005 | Core imports no AniList or Bangumi concrete implementation. | Automated (A) |
| ARCH-006 | Anime1 implements the core AnimeSource port. | Automated (A) |
| ARCH-007 | AniList and Bangumi implement MetadataProvider. | Automated (A) |
| ARCH-008 | libmpv implements the Player and PlaybackSession ports. | Automated (A) |
| ARCH-009 | SQLite implements Store. | Automated (A) |
| ARCH-010 | Core tests run without a desktop UI. | Automated (A) |
| ARCH-011 | A fake AnimeSource can replace Anime1 in core tests. | Automated (A) |
| ARCH-012 | Local following, history and progress use canonical local IDs, not Anime1 IDs. | Automated + review (Y) |
| ARCH-013 | Provider-specific tokens are not present in core models. | Automated + review (Y) |
| ARCH-014 | Source failure does not prevent cached history and following from loading. | Automated + review (Y) |
| ARCH-015 | Architecture stays minimal, with no unnecessary enterprise-layer chain. | Automated + review (Y); plus owner check (YH) if a boundary changes |

Evidence for PASS:

- ARCH-001..005: a dependency or import guard test, or an equivalent static check, passes.
- ARCH-006..011: contract tests and fake-adapter tests pass.
- ARCH-012..014: persistence and failure-path tests pass. A reviewer checks them when the code is touched.
- ARCH-015: dependency checks pass, and a reviewer confirms there is no unjustified layer chain. The owner approves any boundary change.

### B. Functional browsing

| ID | Requirement | Verifier |
| --- | --- | --- |
| FUNC-001 | The app opens to usable cached content without waiting for the network when a cache exists. | Automated + review + owner check (YH) once the UI exists |
| FUNC-002 | The anime catalog is available. | Automated + review (Y) |
| FUNC-003 | Search works. | Automated + review (Y) |
| FUNC-004 | Anime detail shows cover, title, native title, synopsis, season and year, studio and episode count when metadata is available. | Automated + review + owner check (YH) |
| FUNC-005 | The episode list is correctly ordered. | Automated + review (Y) |
| FUNC-006 | The schedule is available. | Automated + review (Y) |
| FUNC-007 | Following works. | Automated + review + owner check (YH) once the UI exists |
| FUNC-008 | History works. | Automated + review + owner check (YH) once the UI exists |
| FUNC-009 | Continue Watching works. | Automated + review + owner check (YH) once the UI exists |

Evidence for PASS:

- FUNC-001: an offline or cache integration test shows no startup network dependency. The owner confirms the cached flow in the UI.
- FUNC-002..003: adapter and application tests pass, plus a UI end-to-end test once the UI is wired.
- FUNC-004: data and content assertions pass. The owner checks presentation and clipping in the running UI.
- FUNC-005..006: deterministic ordering and schedule tests plus an integration test pass.
- FUNC-007..009: persistence and application tests pass. The owner checks the workflow.

### C. Playback

PLAY-001..003, 007..010 and 014 were rewritten on 2026-10-13 by owner decision: the player is in-process libmpv, not an external MPV process.

| ID | Requirement | Verifier |
| --- | --- | --- |
| PLAY-001 | The bundled libmpv DLL is found next to the executable and its SHA-256 matches the pinned hash before it is loaded, using an absolute path. | Automated + review (Y) |
| PLAY-002 | An explicit, documented override of DLL path and expected SHA-256 is supported. | Automated + review (Y) |
| PLAY-003 | A missing or altered DLL keeps Home usable and shows a clear playback error. | Automated + review + owner check (YH) |
| PLAY-004 | The app starts playback without exposing Anime1 credentials to the UI. | Automated + review (Y) |
| PLAY-005 | Remote playback credentials are not passed in player options, arguments or logs. | Automated + review (Y) |
| PLAY-006 | Playback goes through the secure loopback proxy. | Automated + review (Y) |
| PLAY-007 | A viewing session uses one player instance. | Automated + review (Y) |
| PLAY-008 | Switching EP01, EP02, EP03 reuses the same player instance and creates no new player or render context per episode. | Automated + review (Y) |
| PLAY-009 | The in-process libmpv instance is created with `config=no`, so a user's `mpv.conf` is not loaded, and the app never reads, writes or overwrites a user mpv configuration. The app sets only its own fixed options. (confirmed by owner 2026-10-13) | Automated + review (Y) |
| PLAY-010 | Playback progress is tracked from libmpv events and properties. | Automated + review (Y) |
| PLAY-011 | Progress persists across app restart. | Automated + review (Y) |
| PLAY-012 | Resume playback works. | Automated + review (Y) |
| PLAY-013 | A normal end of file can autoplay the next episode. | Automated + review (Y) |
| PLAY-014 | The player shows the minimal auto-hiding control overlay defined in [01_PRODUCT.md](01_PRODUCT.md) (play/pause, seek bar, volume, episode list, fullscreen). It reappears on mouse or keyboard input. No other app overlays exist. | Automated + review + owner check (YH) |
| PLAY-015 | No danmaku or comments exist. | Automated + review + owner check (YH) |

Evidence for PASS:

- PLAY-001..002: the DLL is located by absolute path and its SHA-256 is verified before load. PLAY-002 also covers the documented DLL-path and hash override.
- PLAY-003: a missing or altered DLL gives a safe, actionable error and Home stays usable. The owner confirms the message is clear.
- PLAY-004..008: integration tests assert that no secrets reach the player (checked in player options and logs), playback goes through the loopback proxy, there is one player per viewing session, an episode switch does not recreate the player, and close and cleanup are deterministic.
- PLAY-009: a test asserts the player is created with `config=no` and only the app's fixed options. No code path reads or writes a user mpv configuration.
- PLAY-010..013: scenario tests for progress, restart, resume and autoplay-next pass, using the in-process player events.
- PLAY-014: on-video controls (play/pause, seek bar, volume, episode list, fullscreen) exist, auto-hide after a few idle seconds and reappear on mouse or keyboard input. The owner confirms this in real playback.
- PLAY-015: an absence check shows no danmaku, comments, social features or ads, and the owner confirms.

### D. Keyboard and usability

| ID | Requirement | Verifier |
| --- | --- | --- |
| UX-001 | The core workflow works without a mouse: Launch, find anime, open, select episode, play. | Automated + review + owner check (YH) |
| UX-002 | Arrow-key navigation has visible focus. | Automated + review + owner check (YH) |
| UX-003 | Enter performs open or select. | Automated + review + owner check (YH) |
| UX-004 | Space plays the selected episode where appropriate. | Automated + review + owner check (YH) |
| UX-005 | Esc navigates back or dismisses transient UI. | Automated + review + owner check (YH) |
| UX-006 | `/` focuses search or triggers an equivalent search action. | Automated + review + owner check (YH) |
| UX-007 | Ctrl/Cmd+K gives quick search or command access. | Automated + review + owner check (YH) |
| UX-008 | Mouse click, hover and scroll stay fully functional. | Automated + review + owner check (YH) |
| UX-009 | No interaction requires mobile-style positional tapping zones. | Automated + review + owner check (YH) |
| UX-010 | The UI is calm and minimal, with no decorative or social feature creep. | Automated + review + owner check (YH) |

Evidence for PASS:

- UX-001..010: automation covers keyboard events, focusability, routing, state and regressions where possible. The owner uses the running app (section 4).

### E. Metadata correctness

| ID | Requirement | Verifier |
| --- | --- | --- |
| META-001 | Bangumi is the primary metadata provider (ADR-009). | Automated + review (Y) |
| META-002 | AniList can be used as fallback or cross-check. | Automated + review (Y) |
| META-003 | The matcher does not blindly choose the first search result. | Automated + review (Y) |
| META-004 | Traditional and Simplified Chinese, punctuation and full-width normalization are handled. | Automated + review (Y) |
| META-005 | A low-confidence match yields missing metadata rather than incorrect metadata. | Automated + review (Y) |
| META-006 | Metadata stays usable from cache when the provider is unavailable. | Automated + review (Y) |

Evidence for PASS:

- META-001..006: provider contract tests pass. A fixed fixture set has expected matches, expected no-match cases, and ambiguity, conflict and adversarial cases. Cache behavior is tested when a provider is down. A reviewer checks any change to matcher thresholds or policy. Live-provider success alone is not proof.

### F. External network security

All SEC criteria are critical.

| ID | Requirement | Verifier |
| --- | --- | --- |
| SEC-001 | The UI performs no arbitrary direct Internet fetches. | Automated + review (Y), critical |
| SEC-002 | Remote requests use HTTPS only, except app-owned loopback HTTP. | Automated + review (Y), critical |
| SEC-003 | TLS verification is enabled. | Automated + review (Y), critical |
| SEC-004 | `InsecureSkipVerify` is not enabled. | Automated + review (Y), critical |
| SEC-005 | Unsupported schemes are rejected. | Automated + review (Y), critical |
| SEC-006 | Loopback destinations are blocked for remote external fetches. | Automated + review (Y), critical |
| SEC-007 | Private IP destinations are blocked. | Automated + review (Y), critical |
| SEC-008 | Link-local destinations are blocked. | Automated + review (Y), critical |
| SEC-009 | IPv6 private, loopback and link-local cases are handled. | Automated + review (Y), critical |
| SEC-010 | Redirect destinations are revalidated. | Automated + review (Y), critical |
| SEC-011 | A redirect to localhost or a private IP is blocked. | Automated + review (Y), critical |
| SEC-012 | DNS and IP validation happen on the actual connection destination. | Automated + review (Y), critical |
| SEC-013 | Unknown playback origins fail closed under source policy. | Automated + review (Y), critical |
| SEC-014 | Remote Anime1 HTML is parsed, never executed or rendered raw. | Automated + review (Y), critical |
| SEC-015 | Remote descriptions are sanitized plain text. | Automated + review (Y), critical |
| SEC-016 | Raw remote HTML is never sent to the native UI. | Automated + review (Y), critical |
| SEC-017 | Remote image byte size is bounded. | Automated + review (Y), critical |
| SEC-018 | Remote image dimensions are bounded before expensive decode where feasible. | Automated + review (Y), critical |
| SEC-019 | Unexpected media or content types fail safely. | Automated + review (Y), critical |

Evidence for PASS:

- SEC-001..019: negative and adversarial tests exist for the rule; a happy-path smoke test is not enough. Changes to trust policy, SSRF rules, redirect policy, TLS verification, allowed origins or raw-content exposure also need owner approval and normally an ADR. A human exception alone cannot make a critical criterion PASS.

### G. Privacy and secret handling

| ID | Requirement | Verifier |
| --- | --- | --- |
| PRIV-001 | No account is required. | Automated + review (Y) |
| PRIV-002 | No telemetry is sent by default. | Automated + review (Y) |
| PRIV-003 | No analytics SDK. | Automated + review (Y) |
| PRIV-004 | No advertising SDK. | Automated + review (Y) |
| PRIV-005 | No automatic crash upload. | Automated + review (Y) |
| PRIV-006 | Watch history stays local. | Automated + review (Y) |
| PRIV-007 | Cookies are never stored in SQLite. | Automated + review (Y) |
| PRIV-008 | Temporary stream tokens are never stored in SQLite. | Automated + review (Y) |
| PRIV-009 | Proxy session tokens are never stored persistently. | Automated + review (Y) |
| PRIV-010 | Secrets are not returned to the UI. | Automated + review (Y) |
| PRIV-011 | Logs redact cookies, tokens and authenticated URL data. | Automated + review (Y) |
| PRIV-012 | The app does not require administrator or root privileges. | Automated + review (Y) |

Evidence for PASS:

- PRIV-001..012: static and dynamic assertions cover persistence, logs, player arguments and options, and UI action data and privileges. Formatted-error variants and failure paths are tested.

### H. Local proxy security

| ID | Requirement | Verifier |
| --- | --- | --- |
| PROXY-001 | The proxy binds only to loopback. | Automated + review (Y) |
| PROXY-002 | The proxy uses high-entropy per-session tokens. | Automated + review (Y) |
| PROXY-003 | An unknown session token is rejected. | Automated + review (Y) |
| PROXY-004 | An expired session token is rejected. | Automated + review (Y) |
| PROXY-005 | The session is invalidated when playback or the session ends. | Automated + review (Y) |
| PROXY-006 | The proxy still enforces the secure remote target policy. | Automated + review (Y) |
| PROXY-007 | Range requests required for media playback work. | Automated + review (Y) |

Evidence for PASS:

- PROXY-001..007: tests cover negative token and session cases, lifecycle cleanup, malformed input, cancellation and close paths, and leak and redaction.

### I. Player boundary security and lifecycle

IPC-001..007 were rewritten on 2026-10-13 by owner decision. They keep their IDs but now describe the in-process libmpv player boundary. There is no external player process, JSON IPC, named pipe or socket.

| ID | Requirement | Verifier |
| --- | --- | --- |
| IPC-001 | The player receives only loopback proxy URLs, never remote or arbitrary URLs. | Automated + review (Y) |
| IPC-002 | No secret (cookie, token, authenticated URL) appears in player options or logs. | Automated + review (Y) |
| IPC-003 | The UI cannot send raw player commands or property writes; it can call only typed desktop actions. | Automated + review (Y) |
| IPC-004 | The UI cannot submit arbitrary URLs for playback; the app resolves and proxies them. | Automated + review (Y) |
| IPC-005 | The UI never receives native player handles, proxy tokens or internal endpoints. | Automated + review (Y) |
| IPC-006 | Player event handling stops after the session closes. | Automated + review (Y) |
| IPC-007 | Native player resources (player handle, render context, event goroutines) are released on close. | Automated + review (Y) |

Evidence for PASS:

- IPC-001..007: checks show the UI reaches no raw endpoint or raw player command; control calls go only through the core Player port; the event reader and any internal handle end and are cleaned up at session close; malformed input and close paths are tested.

### J. Resource safety

| ID | Requirement | Verifier |
| --- | --- | --- |
| RES-001 | HTTP response bodies are always closed. | Automated + review (Y) |
| RES-002 | Background goroutines have cancellation paths. | Automated + review (Y) |
| RES-003 | Tickers and timers are stopped. | Automated + review (Y) |
| RES-004 | Sockets are closed. | Automated + review (Y) |
| RES-005 | Native player resources are released and cleaned up on close. | Automated + review (Y) |
| RES-006 | Proxy sessions expire and are removed. | Automated + review (Y) |
| RES-007 | Memory caches are bounded. | Automated + review (Y) |
| RES-008 | Repeated episode switching does not cause monotonic RAM growth. | Automated + review (Y) |
| RES-009 | Repeated player start and stop does not cause monotonic goroutine growth. | Automated + review (Y) |
| RES-010 | Cancelled searches and requests leave no background work running. | Automated + review (Y) |

Evidence for PASS:

- RES-001..010: repeated-cycle or stress evidence shows growth and cleanup behavior. One run is not enough for RES-008..010, and no monotonic-growth claim passes on one visual run.

### K. Performance and perceived speed

| ID | Requirement | Verifier |
| --- | --- | --- |
| PERF-001 | Cached startup does not block on Anime1. | Automated (A) |
| PERF-002 | Cached startup does not block on AniList or Bangumi. | Automated (A) |
| PERF-003 | Cached content appears fast enough to feel responsive; the target is about 1 second perceived startup on normal supported hardware, not a hard universal benchmark. | Automated + review + owner check (YH) |
| PERF-004 | Metadata refresh concurrency is bounded. | Automated + review (Y) |
| PERF-005 | Cover images are lazy-loaded or otherwise bounded. | Automated + review (Y) |
| PERF-006 | Background tasks stay near idle while in-process libmpv plays and no refresh work is active. | Automated + review (Y) |
| PERF-007 | Playback progress causes no high-frequency SQLite writes. | Automated (A) |
| PERF-008 | On an owner-approved Windows reference host with representative local data and matched content and states, the whole product (FLTK plus libmpv) uses fewer resources than OneAnime in comparable idle, wake, playback and cleanup states under ADR-023. Record every run, private working set, commit, CPU, GPU and responsiveness. A trimmed working-set minimum alone is not `PASS`. A long-idle rerun needs renewed owner consent. A miss after bounded causal optimization needs an explicit owner decision. | Automated + review + owner check (YH) |

Evidence for PASS:

- PERF-001..002: instrumentation or a test shows the cached startup path does not wait on a provider.
- PERF-003: a repeatable startup measurement is recorded. The owner confirms perceived responsiveness. The 1 second figure is a target, not a hardware guarantee.
- PERF-004..005: concurrency and lazy-loading bounds are tested or directly inspectable.
- PERF-006: an idle resource observation is recorded with its threshold and environment.
- PERF-007: write-frequency and checkpoint tests or instrumentation pass.
- PERF-008: the owner approves the reference host and content. Repeatable idle and playback samples of this app and OneAnime are recorded: private resident and commit memory, CPU, GPU, startup and cleanup. The owner resolves any miss before PASS.

### L. Cross-platform

Only Windows criteria close now. Linux and macOS are paused and needed only for an all-platform `COMPLETE`.

| ID | Requirement | Verifier |
| --- | --- | --- |
| PLATFORM-001 | The Windows build succeeds. | Automated + review (Y) |
| PLATFORM-002 | The Linux build succeeds on the defined supported environment. | Automated + review (Y) |
| PLATFORM-003 | The macOS build succeeds. | Automated + review (Y) |
| PLATFORM-004 | Windows in-process libmpv playback, episode switching, stop and cleanup are validated. | Automated + review (Y) |
| PLATFORM-005 | The Linux native player lifecycle is validated when that platform phase begins. | Automated + review (Y) |
| PLATFORM-006 | The macOS native player lifecycle is validated when that platform phase begins. | Automated + review (Y) |
| PLATFORM-007 | The keyboard workflow is validated on each supported platform. | Automated + review (Y) |
| PLATFORM-008 | The final desktop UI on Windows, Linux and macOS uses no WebView, browser-backed widget or HTML runtime. Source and build dependencies, packaged artifacts and runtime UI libraries show no Wails or WebView integration, and process-tree checks show no browser helper. | Automated + review (Y) |
| PLATFORM-009 | App-owned settings, SQLite data and cache use the extracted portable folder (`<folder>/data`) by default on all supported platforms. A fresh folder, in-place upgrade, read-only folder, non-ASCII or spaced path, and optional import from the old user-config profile have defined, tested behavior. Existing data is never silently moved or deleted. | Automated + review (Y) |

Evidence for PASS:

- PLATFORM-001..009: real evidence exists on the named platform.
- PLATFORM-008: inspect source, build dependencies and release contents, then observe loaded UI libraries and the running process tree. An in-process OS WebView is a failure.
- PLATFORM-009: test fresh, upgrade, read-only, and non-ASCII or spaced portable folders, plus explicit old-profile import that does not delete the source.

### M. CI and release

| ID | Requirement | Verifier |
| --- | --- | --- |
| CI-001 | `go test ./...` passes. | Automated + review (Y) |
| CI-002 | `go vet ./...` passes. | Automated + review (Y) |
| CI-003 | `govulncheck` runs in CI. | Automated + review (Y) |
| CI-004 | The final Windows FLTK UI source and dependency checks pass. | Automated + review (Y) |
| CI-005 | The final Windows FLTK UI compile and vet checks pass. | Automated + review (Y) |
| CI-006 | The final native desktop UI build passes. | Automated + review (Y) |
| CI-007 | Dependency lockfiles are committed. | Automated + review (Y) |
| CI-008 | `go mod verify` passes. | Automated + review (Y) |
| CI-009 | Platform release artifacts can be produced. | Automated + review (Y) |
| CI-010 | Each desktop release artifact runs from an extracted folder without an installer, privileged installation or OS-wide registration. The Windows ZIP includes the tested, SHA-256-pinned libmpv DLL, its upstream binary and build links, known license references, and an explicit disclosure of source and license details that remain unverified. This is the owner's accepted release gate (2026-09-28, ADR-023), not a certification of third-party license compliance. | Automated + review (Y) |

Evidence for PASS:

- CI-001..010: required status checks pass on the final commit and artifact; a manual bypass is not equivalent. CI-004..006 apply to the Windows FLTK adapter.
- CI-010: extract and launch the Windows ZIP with no installer or privileged registration, and verify the bundled libmpv DLL against its pinned SHA-256 and the disclosure notice (ADR-023). Final release acceptance also needs the owner check.

### N. Loop integrity and verification process

These engineering-process gates decide whether product `PASS` evidence is trustworthy. They are checked from the loop record, command logs, diffs and CI evidence, not from agent self-report. Tool-specific wording was made tool-neutral on 2026-10-13 by owner decision (LOOP-021, LOOP-022, LOOP-025).

| ID | Requirement | Verifier |
| --- | --- | --- |
| LOOP-001 | Contract before mutation: each loop records target criteria, verifier types, owner-check requirement and stop-loss budgets before new code changes. | Automated + review (A/Y) |
| LOOP-002 | Deterministic precedence: where an executable check exists, final status comes from that check, not from implementer self-assessment. | Automated + review (A/Y) |
| LOOP-003 | No verifier weakening: required tests, CI checks, security policy, criterion meaning or verifier logic are not weakened, deleted, skipped or special-cased to obtain `PASS`. | Automated + review (A/Y) |
| LOOP-004 | Failure-driven correction: a failed gate produces failure evidence, classification, diagnosis, a bounded corrective change and re-verification. | Automated + review (A/Y) |
| LOOP-005 | Bounded convergence: the loop respects the corrective-iteration, same-failure, replan, turn and provider budget limits in the workflow. | Automated + review (A/Y) |
| LOOP-006 | No-progress termination: two consecutive corrective iterations with no meaningful progress stop the loop as non-convergent. | Automated + review (A/Y) |
| LOOP-007 | Regression protection: focused fixes are followed by affected regression checks, and a new regression prevents `PASS`. | Automated + review (A/Y) |
| LOOP-008 | Independent review: loops that meet review triggers get a fresh-context read-only review, and all must-fix findings are resolved before `PASS`. | Automated + review (Y) |
| LOOP-009 | Owner check integrity: a criterion needing human review is not `PASS` until the documented procedure returns `HUMAN_PASS`. | Automated + review + owner check (YH) |
| LOOP-010 | Final-state evidence: `PASS` evidence applies to the final implementation after correction, simplify and review. | Automated + review (A/Y) |
| LOOP-011 | Clean final verification: important final acceptance is verified in a clean or reproducible environment. | Automated + review (A/Y) |
| LOOP-012 | Historical migration safety: Loops 01-22 were audited for evidence without reimplementation unless a real failing criterion was found. | Automated + review (Y) |
| LOOP-013 | Sandbox containment: autonomous agent execution happens inside an isolated sandbox or workspace boundary; the active repository is the only persistent local project area writable by default. | Automated + review (A/Y) |
| LOOP-014 | Host filesystem protection: no persistent write is made outside the active repository. Sandbox-owned ephemeral temp and cache state is allowed and discarded after use. | Automated + review (A/Y) |
| LOOP-015 | Repository-scoped GitHub access: access is limited to the connected repository. | Automated + review (A/Y) |
| LOOP-016 | CI observability: the agent can read the repository's required checks, Actions status and diagnostic logs without host access or broad GitHub administration permission. | Automated (A) |
| LOOP-017 | Least-privilege credentials: GitHub credentials are repository-scoped; CI diagnosis uses read access where sufficient; host SSH keys and global credential stores are not imported into the sandbox. | Automated + review (Y) |
| LOOP-018 | Controlled network egress: sandbox network access is deny-by-default or limited to documented destinations (GitHub observation, dependency restore, explicit live integration tests, and `https://api.typesafe.ai` for development judgments with the key read from Bitwarden Secrets Manager into the child process only (updated 2026-10-13, owner decision)). Source contents and credentials are not uploaded to arbitrary services. | Automated + review (A/Y) |
| LOOP-019 | Protected remote administration: default-branch writes, workflow administration and reruns, releases, tags, repository settings, secrets, variables, webhooks, branch protection, deploy keys and environments need explicit human-approved scope. | Automated + review + owner check (YH) |
| LOOP-020 | Boundary violation fail-safe: any detected write or access outside the allowed repository, sandbox ephemeral state or connected GitHub boundary stops mutation and results in `NEEDS_HUMAN`; the agent does not grant itself broader permissions. | Automated (A); owner check (YH) on violation |
| LOOP-021 | Agent execution runs under an approved sandboxed profile; privilege expansion (full host access) is not used to get around blocked work. | Automated + review (A/Y) |
| LOOP-022 | The applicable repository instructions (`AGENTS.md`) are loaded in the agent's context before mutation, and no nested instruction file silently weakens the project safety and quality baseline. | Automated + review (Y) |
| LOOP-023 | Parallel code-changing agents use isolated worktrees or disjoint write sets; overlapping concurrent edits to the same files are not valid loop execution. | Automated + review (A/Y) |
| LOOP-024 | Reviewer independence: the final code-quality reviewer is fresh-context, read-only, and did not author the code it approves. Security and concurrency reviewers follow the same rule where required. | Automated + review (Y) |
| LOOP-025 | The process has no hidden dependency on any specific AI tool or harness. Adding a tool-specific requirement needs explicit human approval and proof that it meets this protocol's sandbox, instruction-loading, isolation and evidence rules. | Automated + review (A/Y) |

Evidence for PASS:

- LOOP-001: the loop contract exists in the record before the first code change.
- LOOP-002..007, 010..011: command logs and diffs show, mechanically where feasible, deterministic precedence, no verifier weakening, failure-driven fixes, bounded budgets, a no-progress stop, a regression run, final-state evidence and a clean final verification.
- LOOP-008..009: independent reviewer evidence is recorded when review is triggered. Owner-check evidence is recorded when an owner check is triggered.
- LOOP-012: Loops 1 to 22 were not reimplemented only because their evidence is in an old format. They reopen only on a confirmed failed requirement.
- LOOP-013: execution configuration shows the repository is the only persistent writable root. A reviewer confirms no privileged control channel was granted.
- LOOP-014: filesystem policy, audit or command trace shows no persistent write outside the repository. Temporary and cache paths are disposable.
- LOOP-015: the remote target is the single named `owner/repo`. Traces show no other repository was targeted.
- LOOP-016: when CI is in scope, check, run and log evidence can be read without write access (`gh pr checks`, `gh run list`, `gh run view --log-failed`, or an API equivalent).
- LOOP-017: the credential source and scope are recorded without the secret. No host credential was imported. Write permission was granted only when needed.
- LOOP-018: a network policy or allowlist is recorded. An unexpected destination fails or needs approval.
- LOOP-019: no protected remote administration happened. If it was assigned, owner approval and independent security review were recorded first.
- LOOP-020: no boundary-violation event is recorded. Any event blocks PASS and goes to the owner.
- LOOP-021: the execution record shows a restricted, repository-scoped writable environment and no full-access or sandbox-bypass mode.
- LOOP-022: the applicable `AGENTS.md` scope is identified. A reviewer confirms no nested instruction weakens safety or quality rules.
- LOOP-023: a worktree or write-set record shows parallel code writers did not overlap files.
- LOOP-024: the final reviewer ran as a separate fresh-context read-only pass and did not author the approved code.
- LOOP-025: required commands and evidence can be reproduced with the repository's documented commands alone. No named tool or vendor is a hidden dependency.

### O. Code quality, maintainability and structure

QUAL-001..012 block every loop that changes production code. The reviewer inspects the diff and enough surrounding code to judge placement and duplication. Line count never decides PASS or FAIL; a large size only prompts a cohesion check. Review findings are `MUST_FIX` (blocks the loop: correctness, architecture, placement, readability, maintainability, clear debt) or `ADVISORY` (optional, no churn).

| ID | Requirement | Verifier |
| --- | --- | --- |
| QUAL-001 | Pragmatic SOLID: changed production code follows the SOLID interpretation in [02_ARCHITECTURE.md](02_ARCHITECTURE.md) without ceremonial abstractions. | Review (Y) |
| QUAL-002 | Single responsibility and cohesion: changed files, types, packages and major functions have coherent responsibilities. | Review (Y) |
| QUAL-003 | Dependency direction: core and application policy gain no concrete UI toolkit, provider, libmpv, SQLite, network or platform dependency. | Automated + review (A/Y) |
| QUAL-004 | Interface quality: interfaces stay narrow, consumer-relevant and justified by a real boundary. | Review (Y) |
| QUAL-005 | Substitutability: port implementations preserve shared contract behavior, including typed errors, cancellation, lifecycle, security and failure semantics. | Automated + review (A/Y) |
| QUAL-006 | Readability: names, control flow, error handling and side effects are clear without needless indirection. | Review (Y) |
| QUAL-007 | File and package placement: new or moved production code lives in the narrowest existing package that owns the responsibility; new top-level or shared packages are justified and reviewed. | Review (Y) |
| QUAL-008 | No dumping grounds or god structures: no new or grown `utils`, `helpers`, `common` or `misc` packages, god objects or files, or circular ownership. | Automated + review (A/Y) |
| QUAL-009 | No avoidable duplication or dead scaffolding: no duplicated domain or security policy, dead code, debug path, obsolete compatibility branch or stale scaffolding. | Automated + review (A/Y) |
| QUAL-010 | Documentation and comment accuracy: non-obvious invariants and security or architecture reasoning are documented, and changed comments and docs are accurate. | Review (Y) |
| QUAL-011 | Minimal public surface: new exported APIs and shared helpers are no broader than the current feature needs. | Automated + review (A/Y) |
| QUAL-012 | Independent code-quality review: a fresh-context reviewer reports no unresolved `MUST_FIX` finding on the final diff. | Review (Y) |

Evidence for PASS:

- QUAL-001: the reviewer maps the diff to the pragmatic SOLID rules and finds no ceremonial abstraction or clear violation.
- QUAL-002: ownership is cohesive. Mixed responsibilities are resolved or justified.
- QUAL-003: import checks pass. A review confirms no inward dependency on concrete infrastructure.
- QUAL-004: changed interfaces are necessary, cohesive and no wider than their consumers need.
- QUAL-005: contract tests pass. A reviewer confirms error, cancellation, lifecycle and security behavior is substitutable.
- QUAL-006: the reviewer can explain control flow, error paths and side effects. There is no `MUST_FIX` readability finding.
- QUAL-007: every new or moved file has a clear layer or package owner.
- QUAL-008: no new dumping-ground package or file, avoidable cycle or god structure.
- QUAL-009: no dead or debug scaffolding and no duplicated policy. Tests pass after cleanup.
- QUAL-010: comments and docs match final behavior and explain only non-obvious constraints.
- QUAL-011: the exported or API diff is reviewed and unneeded public surface is removed.
- QUAL-012: a final fresh-context reviewer returns `PASS_REVIEW` with zero unresolved `MUST_FIX` findings.

### P. Durable agent state and session recovery

These check the recovery file [AGENT_HANDOFF.md](AGENT_HANDOFF.md). "Inspection" means a person or agent reads the file. "Trace" means the session log. The verifier names below are the plain names used in the loop record, not one of the four codes.

| ID | Requirement | Verifier |
| --- | --- | --- |
| STATE-001 | When an active loop may be paused or handed off, `docs/AGENT_HANDOFF.md` exists and is current enough to resume after Git reconciliation. | Automated check + inspection |
| STATE-002 | The handoff targets 4 KiB or less and must stay at 8 KiB and 160 lines or less. | Automated (A) |
| STATE-003 | The handoff records instruction version (`instruction_version`, equal to the version in `AGENTS.md`), active loop and state, branch and observed HEAD, goal and criterion IDs, progress, unresolved failures, ordered next actions, relevant paths, verification state, blockers and gates, and material budget counters. | Inspection |
| STATE-004 | The handoff contains no reasoning transcript, full logs, diffs or source copies, credentials, tokens, cookies, authenticated URLs or duplicated document sections. | Scan + review |
| STATE-005 | A new root session reconciles handoff branch, HEAD and dirty paths against current Git before mutation; current Git and verifier truth override stale handoff text. | Automated (Git) |
| STATE-006 | A continuing session reads the handoff and only the referenced source-of-truth sections first; full rediscovery is for invalid, stale or conflicting state, new loops or material boundary uncertainty. | Trace or review |
| STATE-007 | The root agent updates the handoff at recoverability boundaries defined in [05_WORKFLOW.md](05_WORKFLOW.md), including before an intentional pause and at loop exit. | Status and handoff review |
| STATE-008 | Only the root agent owns the global handoff; workers do not edit it concurrently. | Trace or review |
| STATE-009 | Long-lived architecture, security and product decisions are recorded in decisions, source-of-truth docs or tests, not only in the handoff. | Review |
| STATE-010 | After `AGENTS.md` or the protocol materially changes, work continues in a fresh or reconciled session that has loaded the current repository instructions, not in a session that may hold stale ones. | Policy review |

Evidence for PASS:

- STATE-001: `docs/AGENT_HANDOFF.md` exists with the required structure.
- STATE-002: `wc -c docs/AGENT_HANDOFF.md` is 8192 or less and `wc -l` is 160 or less.
- STATE-003: the required recovery fields are filled in for the active state.
- STATE-004: the file has no transcript, log dump or sensitive value. Long evidence is referenced, not copied.
- STATE-005: branch, HEAD, status and diff were inspected. Any discrepancy was resolved before changes.
- STATE-006: a continuing session used the bounded start, or recorded why it read more.
- STATE-007: the checkpoint reflects the latest recoverability boundary or exit state.
- STATE-008: only the root agent edited the handoff. Worker output was integrated by the root agent.
- STATE-009: lasting decisions live in ADRs, docs or tests, not only in the handoff.
- STATE-010: a change to instructions or this workflow led to a fresh, reconciled session.

Startup evidence to record:

```text
branch: <branch>
HEAD: <sha>
git status --short --branch: <clean | summarized dirty paths>
handoff observed_head: <sha>
reconciliation: MATCH | REFRESHED | STALE_BLOCKED
next source set: <docs, headings, files>
```

### Q. Execution environment and verifier fallback

ENV-001..008 apply when a different environment is substituted or an environment failure occurs (for example, the local sandbox cannot run a verifier).

| ID | Requirement | Verifier |
| --- | --- | --- |
| ENV-001 | A verifier that did not start because the sandbox failed or lacked a capability is not recorded as an implementation failure without evidence from a capable environment. | Automated + review (A/Y) |
| ENV-002 | After a stable sandbox failure fingerprint is confirmed by at most one targeted check, equivalent retries stop until material environment evidence changes. | Automated + review (A/Y) |
| ENV-003 | Full host access or sandbox-bypass flags are not used to obtain required acceptance evidence. | Automated + review (A/Y) |
| ENV-004 | When the local sandbox cannot run a deterministic verifier, authoritative fallback evidence comes only from a GitHub-hosted runner or another human-approved isolated CI environment that can reproduce the criterion. | Automated + review (A/Y) |
| ENV-005 | Unsandboxed host runs may diagnose a limitation but cannot satisfy a required acceptance criterion. | Automated + review (Y) |
| ENV-006 | Fallback CI verifies the relevant final commit without weakening assertions, thresholds, fixtures, failure behavior or required repetitions, and pins or locks dependencies where practical. | Automated + review (A/Y) |
| ENV-007 | Changes to `.github/workflows/**` and `.github/actions/**` happen only within an explicitly human-approved scope, get independent verifier and security review, use least-privilege permissions, and imply no authorization for commit, push, PR, settings, secrets or workflow control. | Automated + review + owner check (YH) |
| ENV-008 | Cross-platform or Linux-hosted CI is not used to claim `PASS` for criteria that require native Windows behavior, native player integration or human UX judgment. | Automated + review + owner check (YH) for human or platform criteria |

Evidence for PASS:

- ENV-001: the failure record shows whether the repository verifier actually started.
- ENV-002: one stable failure fingerprint and at most one targeted confirmation. No equivalent retries after classification.
- ENV-003: no full-access or sandbox-bypass mode was used to obtain acceptance evidence.
- ENV-004: the blocked check ran in GitHub Windows CI or another owner-approved isolated CI.
- ENV-005: any host run outside the sandbox boundary is marked diagnostic and is not the only PASS evidence.
- ENV-006: the CI run is tied to the exact commit. Verifier meaning, assertions and repetitions are preserved. Dependency pin evidence is recorded.
- ENV-007: a workflow diff has prior owner-approved scope, independent security review, least-privilege permissions and separate remote authorization.
- ENV-008: CI stands in only for criteria the runner can prove. Native Windows, real libmpv and human checks stay open until their own verifier runs.
- For CI evidence record: workflow, run ID, job or check ID, commit SHA, runner label and conclusion. Read access to the repository is enough to inspect run history and logs.

### R. Retrospective baseline audit

RETRO-001..015 are complete (RETRO_BASELINE_PASS at 3e15651e). They are kept for traceability and have no active verifier. Details are in `archive/11_RETROSPECTIVE_BASELINE_AUDIT.md` and `archive/RETRO_AUDIT_STATUS.md`.

| ID | Subject |
| --- | --- |
| RETRO-001 | Audit started only after Loop 23 PASS and blocked Loop 24 until baseline completion. |
| RETRO-002 | Loops 01-22 verified against current code and contracts, without blanket replay. |
| RETRO-003 | Each RA batch mapped current criteria to current code, tests and evidence before mutation. |
| RETRO-004 | Missing legacy evidence classified as missing verification, not a defect. |
| RETRO-005 | Unreconstructable pre-v2.4 process requirements marked `NOT_APPLICABLE_PRE_V2_4`. |
| RETRO-006 | Scope bounded by RA-01..RA-06. |
| RETRO-007 | High-risk batches got failure-path and adversarial verification. |
| RETRO-008 | Parallel audit agents were read-heavy; concurrent writers needed disjoint write sets. |
| RETRO-009 | Each confirmed defect had a stable RA finding ID, mapping, severity and status. |
| RETRO-010 | Code changed only for confirmed `MUST_FIX` findings. |
| RETRO-011 | Each repair was root-cause driven with focused and regression verification and review. |
| RETRO-012 | Fresh-context read-only reviews covered correctness, architecture and security where triggered. |
| RETRO-013 | Audit status and handoff checkpointed after each batch. |
| RETRO-014 | `LEGACY_VERIFIED` meant current implementation passed mapped criteria, not historical process compliance. |
| RETRO-015 | `RETRO_BASELINE_PASS` required RA-01..RA-06 `LEGACY_VERIFIED`, zero open `MUST_FIX`, full regression, reviews, human gates and recorded final HEAD. |

## 4. Owner UX check procedure

Use for every affected UI loop. Before asking the owner, prepare:

- the clickable executable or ZIP path and the build identity (commit);
- startup steps;
- what to type or click;
- the expected result of each action.

Steps:

1. Launch the built application.
2. Do the intended workflow with the keyboard only.
3. Repeat the core actions with the mouse.
4. Confirm visible focus at every keyboard step.
5. Confirm there is no keyboard trap.
6. Test the loading, empty and error states that are in scope.
7. Resize to the smallest supported desktop size.
8. Check for clipping, overlap, unreadable density and mobile-first layout.
9. Check that labels say what the user wants to do, not internal terms.
10. Confirm the UI is calm and minimal and has no danmaku, comments, social features or ads.
11. For playback loops: confirm the on-video controls (play/pause, seek bar, volume, episode list, fullscreen) appear on mouse or keyboard input and hide after a few idle seconds.

PASS needs every mandatory observation to succeed. Record failures by screen and action. Cached or offline flows must not block on the network.

Do not ask for approval until deterministic gates pass and known must-fix findings are fixed, unless the gate resolves an ambiguity.

## 5. Record templates

Save both records under `artifacts/evidence/<loop>/` next to the loop's other evidence.

### Failure record

Fill one after each corrective iteration (each time a failed check is diagnosed and fixed).

```yaml
iteration: 2
criterion: FUNC-001
classification: TEST_FAILURE
fingerprint:
  check: TestHomeUsesCacheWithoutNetwork
  error: unexpected outbound request
  component: home/service
hypothesis: home hydration calls remote refresh before cached render
new_evidence: <what was inspected or measured>
change: <minimal correction>
focused_result: PASS | FAIL
regression_result: PASS | FAIL | NOT_RUN
```

If the same fingerprint remains after 2 repairs, stop blind repair and follow the non-convergence path in [05_WORKFLOW.md](05_WORKFLOW.md).

### Human record

Fill it after the owner finishes the section 4 procedure for a loop with an owner check.

```yaml
human_gate:
  required: true
  criteria: [UX-001, UX-002, UX-010]
  environment:
    os: <OS and version>
    app_build: <commit and artifact path>
    display: <resolution and scaling when relevant>
  procedure:
    - <step>
  observations:
    - id: UX-001
      result: PASS | FAIL | BLOCKED
      evidence: <specific observation>
  overall: HUMAN_PASS | HUMAN_FAIL | HUMAN_BLOCKED | HUMAN_EXCEPTION_APPROVED
  reviewer: owner
  date: <date>
  notes: <optional>
```

## 6. Phase and release gates

### Windows phase gate

The owner selected Windows-first development. Linux follows once Windows is stable. macOS follows once Linux is stable. Windows Loop 28 closed against FLTK and in-process libmpv, portable state and import, no WebView, keyboard and mouse, security, regression and the Windows ZIP gate (CI-010). On 2026-09-28 the owner accepted the existing resource and interaction evidence without another comparison run. PERF-008 stays the resource objective for a later formal comparison; this exception does not turn an unmatched measurement into a matched `PASS`.

`WINDOWS_COMPLETE` requires all applicable Windows feature and final-release criteria. Criteria that name Linux or macOS stay deferred and cannot be marked `PASS` from Windows evidence.

### MVP final gate

MVP status may be `COMPLETE` only when all of these hold:

- Every applicable critical product criterion is `PASS`.
- No required criterion is `FAIL`, `BLOCKED`, `NEEDS_HUMAN` or `NOT_RUN`.
- All security and architecture criteria are `PASS`.
- Same-session in-process libmpv switching is `PASS` with lifecycle evidence.
- The keyboard-only workflow is `PASS` with the required owner check.
- Resource stress criteria are `PASS` with repeated-cycle evidence.
- The matched whole-product resource objective in ADR-023 is met. A miss stays `NEEDS_HUMAN` unless the owner changes the contract.
- All three desktop platforms have the build and runtime evidence the criteria above require.
- Final artifacts satisfy the no-WebView and portable-run criteria (PLATFORM-008, CI-010, ADR-020).
- PLATFORM-009 (portable state and old-profile import) is `PASS`.
- Required CI and release criteria are `PASS` on the final state.
- Applicable LOOP, QUAL, STATE, ENV and RETRO criteria are `PASS`.
- The final owner release check returns `HUMAN_PASS`.
- No critical criterion is covered only by an exception or manual bypass.
