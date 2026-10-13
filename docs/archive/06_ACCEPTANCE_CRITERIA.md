<!-- SPDX-License-Identifier: MPL-2.0 -->

# MVP Acceptance Criteria

These criteria are hard gates. They are requirements, not suggestions and not a prompt for the implementation agent to reinterpret.

## Verification semantics

Every criterion is tracked with one of:

- `NOT_RUN`
- `PASS`
- `FAIL`
- `BLOCKED`
- `NEEDS_HUMAN`
- `NOT_APPLICABLE`

`PASS` is valid only when the required verifier in `docs/08_VERIFICATION_MATRIX.md` produced evidence against the final relevant repository state.

### PASS

Use `PASS` only when:

- every mandatory observable in the criterion is satisfied
- all required deterministic checks pass
- no required regression check fails
- required independent review has no unresolved must-fix finding
- required human gate is `HUMAN_PASS`
- protected acceptance/test/CI semantics were not weakened to obtain the result

### FAIL

Use `FAIL` when reproducible evidence shows one or more required observables are not satisfied.

A failing required deterministic check is FAIL even when an AI reviewer believes the implementation is acceptable.

### BLOCKED

Use `BLOCKED` when the required verification cannot be executed because of an external environment, unavailable platform/tool, or upstream dependency. A substitute check may add evidence but cannot silently turn a blocked required check into PASS.

### NEEDS_HUMAN

Use `NEEDS_HUMAN` when a mandatory human judgment/approval is pending or when available automation cannot decide the criterion reliably.

### NOT_APPLICABLE

Use only when the criterion genuinely does not apply. Record the rationale. It cannot be used to hide unfinished work.

## Evidence requirements

Evidence should include, as applicable:

- criterion ID
- tested commit/working-tree state
- exact command or manual procedure
- exit code/result
- platform/environment
- test name/count or measured observation
- relevant log/artifact
- independent reviewer result
- human reviewer result

Free-form statements such as `looks correct`, `reviewed`, or `tests should pass` are not sufficient evidence.

Any later code/config change that can affect a criterion invalidates its previous PASS evidence until that criterion is reverified.

## Verification authority order

When signals disagree:

1. documented product/architecture/security contract defines intended behavior
2. deterministic executable evidence decides objective behavior where available
3. independent AI review detects gaps but cannot override deterministic failure
4. human judgment decides explicitly subjective/product/exception gates

An implementation agent's self-report is never sufficient for PASS.

## Anti-bypass rule

A criterion cannot pass if the implementation agent obtained green status by weakening the thing that measures success. Relevant test/CI/acceptance changes require the controls in `docs/05_LOOP_ENGINEERING_RUNBOOK.md`.

---
## A. Architecture

### ARCH-001
Core imports no Fyne or Wails packages.

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
App starts MPV playback without exposing Anime1 credentials to the UI.

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
The UI performs no arbitrary direct Internet fetches.

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
Raw remote HTML is never sent to the Fyne UI; the historical Svelte boundary remains protected until removal.

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
Secrets are not returned to the UI.

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
The UI never receives a raw IPC endpoint.

### IPC-005
The UI cannot submit raw MPV commands.

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
Application background tasks remain near-idle while in-process libmpv plays and no refresh work is active.

### PERF-007
No high-frequency SQLite writes for playback progress.

### PERF-008
On an owner-approved Windows reference host with representative local data and matched content/states, the FLTK/libmpv whole product must use less resource than OneAnime in comparable idle, wake, playback and cleanup states under ADR-023. Record every run, private working set and commit, CPU, GPU and responsiveness; a trimmed working-set minimum alone is not PASS. A long-idle rerun requires renewed owner consent. A miss after bounded causal optimization requires an explicit owner decision.

---

## L. Cross-platform

### PLATFORM-001
Windows build succeeds.

### PLATFORM-002
Linux build succeeds on defined supported environment.

### PLATFORM-003
macOS build succeeds.

### PLATFORM-004
Windows in-process libmpv playback, episode switching, stop and cleanup are validated.

### PLATFORM-005
Linux native player lifecycle is validated when that platform phase begins.

### PLATFORM-006
macOS native player lifecycle is validated when that platform phase begins.

### PLATFORM-007
Keyboard workflow is validated on each supported platform.

### PLATFORM-008
The final desktop UI on Windows, Linux, and macOS uses no WebView, browser-backed widget, or HTML runtime. Final source/build dependencies, packaged artifacts, and platform runtime UI libraries show no Wails/WebView integration; process-tree checks show no browser helper. This criterion applies to the migration/final release, not the already audited transitional Wails baseline.

### PLATFORM-009
App-owned settings, SQLite data and cache use the extracted portable folder by default on all supported platforms. A fresh folder, in-place upgrade, read-only folder, non-ASCII/spaced path, and optional import from the old user-config profile have defined, tested behavior. Existing data is never silently moved or deleted.

---

## M. CI and release

### CI-001
`go test ./...` passes.

### CI-002
`go vet ./...` passes.

### CI-003
`govulncheck` is run in CI.

### CI-004
The final Windows FLTK UI source and dependency checks pass.

### CI-005
The final Windows FLTK UI compile and vet checks pass.

### CI-006
The final native desktop UI build passes; historical frontend build remains required while Svelte is shipped as a transitional artifact.

### CI-007
Dependency lockfiles are committed.

### CI-008
`go mod verify` passes.

### CI-009
Platform release artifacts can be produced.

### CI-010
Each desktop release artifact runs from an extracted folder without an installer, privileged installation, or OS-wide registration. For the Windows Loop 28 community package, the ZIP includes the tested, SHA-256-pinned libmpv runtime, its upstream binary/build links, applicable known license references, and an explicit disclosure of source and license details that remain unverified. This is the owner's accepted project release gate, not a certification of third-party license compliance. The stricter corresponding-source archive verifier remains available when complete build-specific material is obtained.

---

# Windows phase gate

The owner selected Windows-only development and acceptance first. Windows Loop 28 closes against FLTK and in-process libmpv, portable state/import, no WebView, keyboard/mouse, security, regression and the owner-accepted Windows ZIP gate. On 2026-09-28 the owner accepted the existing resource and interaction evidence without another comparison run, and selected the community provenance disclosure described in CI-010 for this individual open-source project. PERF-008 remains the resource objective for later formal comparison; this exception does not turn an unmatched measurement into a matched PASS. Windows Loop 28 PASS permits Loop 29 feature work on Windows. `WINDOWS_COMPLETE` requires all applicable Windows feature and final-release criteria. Linux development follows Windows stability; macOS follows Linux stability. Criteria explicitly naming Linux or macOS remain deferred and cannot be marked PASS from Windows evidence. The all-platform `COMPLETE` gate below remains unchanged.

# MVP final gate

MVP status may be marked `COMPLETE` only when:

- every applicable critical product criterion is `PASS`
- no required criterion is `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, or `NOT_RUN`
- all security and architecture criteria are `PASS`
- same-session in-process libmpv switching is `PASS` with lifecycle evidence
- keyboard-only workflow is `PASS` with required human verification
- resource stress criteria are `PASS` with repeated-cycle evidence
- the matched whole-product resource objective in ADR-023 is met; a miss remains `NEEDS_HUMAN` unless the owner explicitly changes the contract
- all three desktop platforms have the required build/runtime evidence defined by the verification matrix
- final desktop artifacts satisfy the no-WebView and portable-run criteria in ADR-020
- app-owned state and old-profile import satisfy PLATFORM-009
- required CI/release criteria are `PASS` on the final state
- applicable `LOOP-001..025` integrity criteria are `PASS`
- all applicable `QUAL-001..012` code-quality criteria are `PASS`
- all applicable `STATE-001..010` durable-state criteria are `PASS`
- the one-time historical baseline has reached `RETRO_BASELINE_PASS`, and applicable `RETRO-001..015` criteria are `PASS`
- the final human MVP/release gate returns `HUMAN_PASS`
- no critical criterion is covered only by an exception or manual bypass

---

## N. Loop integrity and verification process

These are engineering-process gates. They do not replace the product criteria above; they determine whether product PASS evidence is trustworthy.

### LOOP-001 — Contract before mutation

Each v2 loop records target acceptance criteria, verifier types, human-gate requirement, and stop-loss budgets before new code mutation.

### LOOP-002 — Deterministic verification precedence

Where an objective executable check exists, final status is derived from that check rather than implementer self-assessment.

### LOOP-003 — No verifier weakening

Required tests, CI checks, security policy, acceptance meaning, or verifier logic are not weakened, deleted, skipped, or special-cased to obtain PASS.

### LOOP-004 — Failure-driven correction

A failed gate produces structured failure evidence, classification, diagnosis, a bounded corrective change, and re-verification.

### LOOP-005 — Bounded convergence

The loop respects corrective-iteration, same-failure, replan, turn, and available provider budget limits defined in the runbook.

### LOOP-006 — No-progress termination

Two consecutive corrective iterations with no meaningful progress trigger non-convergence stop rather than continued blind retries.

### LOOP-007 — Regression protection

Focused fixes are followed by affected regression checks; a new regression prevents PASS.

### LOOP-008 — Independent review

Loops that meet independent-review triggers receive a fresh-context/read-only review and resolve all must-fix findings before PASS.

### LOOP-009 — Human gate integrity

Criteria or changes requiring human review are not marked PASS until the documented human procedure is executed and returns `HUMAN_PASS`.

### LOOP-010 — Final-state evidence

Final PASS evidence applies to the final implementation after correction, simplify, and review; relevant changes after verification trigger re-verification.

### LOOP-011 — Clean final verification

Important final acceptance is verified in a clean or otherwise reproducible environment so stale state cannot create a false PASS.

### LOOP-012 — Historical migration safety

Loops 01-22 are retrospectively audited for evidence without reimplementation unless an actual failing criterion is found.

### LOOP-013 — Sandbox containment

Autonomous agent execution occurs inside an isolated sandbox/workspace boundary; the active repository is the only persistent local project area writable by default.

### LOOP-014 — Host filesystem protection

No persistent write is made to the user's host filesystem outside the active repository. Sandbox-owned ephemeral temp/cache state is permitted and discarded after use.

### LOOP-015 — Repository-scoped GitHub access

GitHub access is restricted to the explicitly connected repository. The agent does not read/write unrelated private repositories or organization resources as part of the loop.

### LOOP-016 — CI observability without broad write authority

The agent can read the connected repository's required checks, Actions status, and diagnostic logs/artifacts needed for verification without requiring host-system access or broad GitHub administration permission.

### LOOP-017 — Least-privilege credentials

GitHub credentials are repository-scoped and least-privilege; CI diagnosis uses read access where sufficient, and host SSH keys/global credential stores are not imported into the sandbox.

### LOOP-018 — Controlled network egress

Sandbox network access is deny-by-default or equivalently constrained to documented destinations required by GitHub observation, dependency restore, or explicit live integration tests. Repository/source contents and credentials are not uploaded to arbitrary services.

### LOOP-019 — Protected remote administration

Direct default-branch writes, workflow administration/reruns, releases/tags, repository settings, secrets, variables, webhooks, branch protection, deploy keys, environments, and equivalent GitHub administration actions require an explicit human-approved scope and are not autonomous feature-loop operations.

### LOOP-020 — Boundary violation fail-safe

Any detected write/access outside the allowed local repo, sandbox-ephemeral state, or connected GitHub boundary stops mutation and results in `NEEDS_HUMAN`; the agent must not respond by granting itself broader permissions.

### LOOP-021 — Codex execution profile

Protocol v2.4.1 implementation uses Codex App or Codex CLI under the documented sandbox/approval profile. Full Access/danger-full-access is not used on the host to bypass blocked work.

### LOOP-022 — AGENTS instruction integrity

Applicable repository `AGENTS.md` instructions are present in the Codex execution context before mutation, and no nested instruction file silently weakens the project safety/quality baseline.

### LOOP-023 — Multi-agent write isolation

Parallel code-changing agents use isolated Codex worktrees or otherwise disjoint write sets; concurrent overlapping mutation of the same files/worktree is not accepted as a valid loop execution.

### LOOP-024 — Reviewer independence

The final code-quality reviewer is fresh-context/read-only for its review pass and did not author the final changed code it approves. Specialized security/concurrency reviewers follow the same principle where required.

### LOOP-025 — No alternate-harness dependency

The implementation/verification process does not depend on OpenCode Go or another coding-agent harness. Reintroducing an alternate harness requires explicit human approval plus verification that its sandbox, instruction loading, multi-agent isolation, and evidence semantics meet or exceed this protocol.

---

## O. Code quality, maintainability, and structure

### QUAL-001 — Pragmatic SOLID

Changed production code follows the project-specific SOLID interpretation in `docs/02_ARCHITECTURE.md` without adding ceremonial abstractions.

### QUAL-002 — Single responsibility and cohesion

Changed files, types, packages, and major functions have coherent responsibilities; unrelated responsibilities are not accumulated merely because the existing location is convenient.

### QUAL-003 — Dependency direction

Core/application policy does not gain concrete Fyne, Wails, provider, MPV, SQLite, network, or platform dependencies.

### QUAL-004 — Interface quality

Existing/new interfaces remain narrow, consumer-relevant, and justified by a real boundary; no implementation forces unrelated consumers to depend on extra methods.

### QUAL-005 — Substitutability

Port/adapter implementations preserve shared contract behavior including typed errors, cancellation, lifecycle, security, and failure semantics.

### QUAL-006 — Readability

Names, control flow, error handling, and side effects are understandable from the code without unnecessary indirection or misleading abstractions.

### QUAL-007 — File/package placement

New or moved production code lives in the narrowest existing architectural package that owns the responsibility. New top-level/shared packages are justified and reviewed.

### QUAL-008 — No dumping grounds or god structures

The loop does not introduce/grow generic `utils`/`helpers`/`common`/`misc` dumping grounds, god objects/files, circular ownership, or mixed unrelated responsibilities.

### QUAL-009 — No unnecessary duplication/dead scaffolding

The final diff contains no avoidable duplicated domain/security policy, dead code, temporary debug path, obsolete compatibility branch, or stale scaffolding introduced by the loop.

### QUAL-010 — Documentation/comment accuracy

Non-obvious invariants and security/architecture reasoning are documented where needed; comments and docs changed by the loop are accurate and do not narrate obvious syntax.

### QUAL-011 — Minimal public surface

New exported/public APIs, bindings, and shared helpers are no broader than required by the current feature and acceptance criteria.

### QUAL-012 — Independent code-quality review

A fresh-context independent reviewer reports no unresolved `MUST_FIX` finding for architecture, maintainability, readability, responsibility, or file placement on the final diff.
## P. Durable agent state and session recovery

### STATE-001 — Bounded handoff exists
When an active loop may be paused/handed off, `docs/AGENT_HANDOFF.md` exists and is parseable/current enough to resume after Git reconciliation.

### STATE-002 — Handoff is size bounded
`docs/AGENT_HANDOFF.md` targets <= 4 KiB and MUST remain <= 8 KiB and <= 160 lines.

### STATE-003 — Recovery-critical fields
The handoff records protocol/instruction version, active loop/state, branch/observed HEAD, goal/acceptance IDs, current progress, unresolved failures, ordered next actions, relevant paths, verification state, blockers/gates, and material budget counters.

### STATE-004 — No transcript or secret dumping
The handoff contains no chain-of-thought, full logs/diffs/source copies, credentials/tokens/cookies/authenticated URLs, or duplicated engineering-document sections.

### STATE-005 — Git reconciliation on new root session
A new root session reconciles handoff branch/HEAD/dirty paths against current Git state before mutation; current Git/verifier truth overrides stale handoff text.

### STATE-006 — Progressive source loading
A continuing session reads the handoff and only the referenced/relevant source-of-truth sections first; full-repository/full-doc rediscovery is reserved for invalid/stale/conflicting state, new loops, or material boundary uncertainty.

### STATE-007 — Material checkpointing
The root updates the handoff at recoverability boundaries defined in `docs/10_DURABLE_AGENT_STATE.md`, including before intentional pause/handoff and at loop exit states.

### STATE-008 — Root-only global memory ownership
Workers/subagents do not concurrently mutate the global handoff. The root aggregates material worker results after integration/rejection.

### STATE-009 — Durable decisions promoted out of handoff
Long-lived architectural/security/product decisions are captured in ADR/source-of-truth docs/tests rather than surviving only in the transient handoff.

### STATE-010 — Resume after instruction changes is safe
When `AGENTS.md`/protocol materially changes after a Codex session began, continuation uses a fresh/reconciled session rather than assuming an old resumed session has current instructions.



## Q. Execution environment and verifier fallback

### ENV-001 — Sandbox failures are distinguished from repository failures

A verifier that did not start because the sandbox failed or lacked a required capability is not recorded as `TEST_FAILURE`/implementation failure without capable-environment evidence.

### ENV-002 — Stable sandbox failure circuit breaker

After a stable sandbox/runtime failure fingerprint is confirmed by at most one targeted health/reproduction check, equivalent retries stop until material environment evidence changes.

### ENV-003 — No privilege bypass for acceptance

Full Access, `danger-full-access`, sandbox-bypass flags, or equivalent host privilege expansion are not used to obtain required acceptance evidence.

### ENV-004 — Approved isolated verifier fallback

When the local sandbox cannot execute a deterministic verifier, authoritative fallback evidence comes only from a GitHub-hosted runner or another human-approved isolated CI environment capable of reproducing the criterion.

### ENV-005 — Host diagnostics are non-authoritative

Unsandboxed host execution may be used to diagnose a sandbox/runtime limitation but cannot by itself satisfy a required deterministic acceptance criterion.

### ENV-006 — Fallback CI preserves verifier meaning

Fallback CI verifies the relevant final commit/state without weakening assertions, thresholds, fixtures, failure behavior, required repetitions, or other acceptance semantics; dependencies are reproducible and pinned/locked where practical.

### ENV-007 — Workflow security and authorization

Changes to `.github/workflows/**` / `.github/actions/**` occur only within an explicitly human-approved local scope, receive independent verifier/security review, use least-privilege workflow permissions, and do not imply separate authorization for commit/push/PR/settings/secrets/workflow-control actions.

### ENV-008 — Platform-specific evidence is not substituted

Cross-platform or Linux-hosted CI is not used to claim PASS for criteria that explicitly require native Windows behavior, MPV/native-process integration, or human UX/visual/interaction judgment.

## R. Retrospective baseline audit integrity

### RETRO-001
The retrospective audit starts only after Loop 23 is PASS and blocks normal progression to Loop 24 until baseline completion or explicit human exception.

### RETRO-002
Historical Loops 01–22 are verified against current implementation/current contracts without blanket implementation replay.

### RETRO-003
Each RA batch maps relevant current acceptance criteria to current code/tests/evidence before mutation.

### RETRO-004
Missing legacy evidence is classified as missing verification; it does not by itself trigger code rewrite or FAIL.

### RETRO-005
Historical process-only v2.4 requirements that cannot be reconstructed are marked `NOT_APPLICABLE_PRE_V2_4`, not retroactive failures.

### RETRO-006
Audit scope is risk-bounded by RA-01..RA-06 and does not load/review unrelated future-loop work.

### RETRO-007
High-risk batches receive targeted failure-path/adversarial verification beyond existing happy-path evidence where applicable.

### RETRO-008
Parallel audit agents are used primarily for read-heavy exploration/review; concurrent writers require explicit disjoint write sets and isolation.

### RETRO-009
Every confirmed defect receives a stable RA finding ID, criterion/evidence mapping, severity, and lifecycle status.

### RETRO-010
Code mutation occurs only for confirmed `MUST_FIX` findings; speculative cleanup/refactoring is not part of retrospective repair.

### RETRO-011
Each repair is root-cause driven and receives focused verification, affected regression verification, and required independent review on the integrated final state.

### RETRO-012
Fresh-context read-only review covers correctness/test gaps and architecture/code quality; security-sensitive batches receive security review when triggered.

### RETRO-013
`docs/RETRO_AUDIT_STATUS.md` and `docs/AGENT_HANDOFF.md` are checkpointed after each batch and before budget/context/usage stop.

### RETRO-014
`LEGACY_VERIFIED` is used only when the current implementation for that historical batch passes all mapped applicable criteria; it does not claim historical process compliance.

### RETRO-015
`RETRO_BASELINE_PASS` requires RA-01..RA-06 `LEGACY_VERIFIED`, zero open `MUST_FIX`, required full regression/reviews/human gates PASS, and final verified HEAD recorded.
