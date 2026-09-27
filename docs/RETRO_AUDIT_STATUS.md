<!-- SPDX-License-Identifier: MPL-2.0 -->

# Retrospective Audit Status

Protocol: **v2.4.1**

This file is the resumable working state for the one-time Loops 01–22 retrospective baseline audit. Keep it concise. Detailed evidence belongs in tests, CI, Git, or referenced artifacts.

## Overall

```yaml
status: RETRO_BASELINE_PASS
verified_head: 3e15651e3d7761122f0587f0ed476b224b79f1de
verified_worktree: "RA-06 repaired at pushed commit 3e15651e; exact-SHA CI 35943800633 SUCCESS and native human PASS; protocol/status edits and resource probes remain local and uncommitted."
active_batch: null
next_action: "No further standalone RA batches. Loop 24 starts the Fyne migration contract in docs/16_FYNE_MIGRATION_PLAN.md; changed RA boundaries become migration acceptance. Wails removal remains mandatory."
open_must_fix: 0
human_gate: "HUMAN_PASS: user observed repaired native Home, arrow-key visible focus and Enter activation on 2026-09-23; isolated test profile removed afterward."
```

## Batch status

| Batch | Loops | Result | Findings open | Last evidence |
| --- | --- | --- | ---: | --- |
| RA-01 Foundation | 01–04 | LEGACY_VERIFIED | 0 | Focused tests/race/vet PASS; independent quality/security reviews no MUST_FIX |
| RA-02 Anime source | 05–09 | LEGACY_VERIFIED | 0 | Focused adapter/contract/race/vet and full Go regression PASS; independent correctness/security reviews no MUST_FIX |
| RA-03 Playback | 10–13 | LEGACY_VERIFIED | 0 | CLOSED at 028b0221; exact-SHA CI 34874294714/linux 104077534611 SUCCESS |
| RA-04 Persistence/state | 14–16 | LEGACY_VERIFIED | 0 | Focused test/race/vet PASS; exact-HEAD CI 34874294714 SUCCESS; independent correctness/security reviews no MUST_FIX |
| RA-05 Metadata | 17–20 | LEGACY_VERIFIED | 0 | Local repair c4bf12b; final-state full Go test/race/vet PASS; independent correctness/security reviews no MUST_FIX |
| RA-06 Desktop boundary | 21–22 | LEGACY_VERIFIED | 0 | F001/F002 closed at 3e15651e; exact-SHA CI 35943800633 SUCCESS; native human PASS; reviews no MUST_FIX |

Allowed batch results:

- `NOT_STARTED`
- `IN_PROGRESS`
- `LEGACY_VERIFIED`
- `REPAIR_REQUIRED`
- `NEEDS_HUMAN`
- `BLOCKED`
- `ABORTED_BUDGET`
- `ABORTED_NON_CONVERGENCE`

## RA-01 completed contract

```yaml
batch: RA-01
historical_loops: [1, 2, 3, 4]
result: LEGACY_VERIFIED
scope:
  - "Phase 0: module/repository boundaries, architecture import guard and hosted build evidence"
  - "Phase 1: core models, source/store/metadata/player ports and App facade"
  - "Phase 2: contract suites, fakes and adversarial validator assertions"
  - "Phase 3: secure HTTP policy, request/response lifecycle and redaction"
criteria: [ARCH-001..005, ARCH-010..015, SEC-002..012, PRIV-010..011, RES-001..004, RES-010, QUAL-001..012]
existing_evidence:
  - "Exact HEAD 2160e5de CI 34790321962/linux 103813204911 SUCCESS: whole-repo regression, race, vet, module, vulnerability, build and clean-worktree evidence reused."
verification_to_run: []
reviewers:
  - "retro_reviewer: current core/contracts/architecture PASS; no MUST_FIX."
  - "retro_security_reviewer: current securehttp/playback source boundary PASS; no MUST_FIX."
human_gate: null
```

Historical process-only requirements: NOT_APPLICABLE_PRE_V2_4_1 (user-directed spelling of protocol's NOT_APPLICABLE_PRE_V2_4). Current-state verification does not certify historical process compliance. No new native user gate is planned for foundation code; existing shell/bootstrap evidence is reused.

## RA-02 completed contract

```yaml
batch: RA-02
historical_loops: [5, 6, 7, 8, 9]
result: LEGACY_VERIFIED
scope:
  - "Anime1 catalog and local search"
  - "Episode archive parsing, pagination and ordering"
  - "Episode resolver and transient playback authorization"
  - "Seasonal schedule parsing and calendar precision"
  - "AnimeSource contract and adapter acceptance"
criteria: [ARCH-006, ARCH-011, ARCH-013, ARCH-014, FUNC-002, FUNC-003, FUNC-005, FUNC-006, SEC-013, SEC-014, SEC-016, SEC-019, PRIV-007, PRIV-008, PRIV-010, PRIV-011, RES-001, RES-002, RES-010, QUAL-001..012]
existing_evidence:
  - "Exact HEAD 2160e5de CI 34790321962/linux 103813204911 SUCCESS, reused only where unchanged whole-repository regression evidence applies."
verification_to_run: []
reviewers:
  - "retro_reviewer: no MUST_FIX; provider boundary, contracts, persistence and DTO traces reviewed."
  - "retro_security_reviewer: no MUST_FIX; URL/stream, parsing, redaction, lifecycle and persistence implications reviewed."
human_gate: null
```

Historical process-only requirements: NOT_APPLICABLE_PRE_V2_4_1. This audit certifies current behavior only. No live upstream request was made: deterministic fixtures/contracts prove current provider behavior but do not certify upstream compatibility. No human gate is applicable.

## RA-03 audit contract

```yaml
batch: RA-03
historical_loops: [10, 11, 12, 13]
result: LEGACY_VERIFIED
closeout: CLOSED
scope:
  - "Loop 10 / Phase 9: secure loopback playback proxy"
  - "Loop 11 / Phase 10: MPV detection, launch and process lifecycle"
  - "Loop 12 / Phase 11: typed MPV JSON IPC and private endpoint lifecycle"
  - "Loop 13 / Phase 12: same-process episode switching and capability rotation"
  - "Current evolved playback tracking/resume/persistence and desktop error/cleanup interaction"
criteria: [ARCH-003, ARCH-008, PLAY-001..013, PROXY-001..007, IPC-001..007, SEC-002..013, SEC-019, PRIV-007..012, RES-001..010, PERF-006..007, QUAL-001..012]
existing_evidence:
  - "Exact HEAD CI 34790321962/linux 103813204911 SUCCESS: full Go test/race/vet/module/vulnerability/build and clean-worktree evidence."
  - "Repair commit 028b02210d0ab6e986ccae2b8ac3a77799019343; exact-SHA CI 34874294714/linux 104077534611 SUCCESS with all steps green."
  - "Inherited live MPV 0.41 detection/start/reap, Windows named-pipe property/stop/cleanup, and same-PID three-media/capability-revocation evidence remains relevant because later playback-path changes were tracking/mapping integration or SPDX-only and the current deterministic suite passes."
verification_to_run: []
reviewers:
  - "retro_reviewer: initially no finding; targeted re-review confirmed RA-03-F001 as PLAY-003 MUST_FIX."
  - "retro_security_reviewer: no security/resource MUST_FIX; command, IPC, URL/token/redaction and cleanup boundaries reviewed."
  - "oracle: backend-only repair is incomplete because Home discards Play errors; production mutation would conflict with the explicit no-Loop-23-reopen boundary."
  - "post-repair retro_reviewer: no MUST_FIX; classification, Wails mapping, success/cancellation and scope boundaries PASS."
  - "post-repair retro_security_reviewer: no MUST_FIX; sentinel normalization/redaction and fail-closed frontend mapping PASS; lifecycle paths unchanged."
  - "oracle closeout: PASS_CLOSEOUT; deterministic render-path evidence is sufficient without a redundant human gate."
human_gate: null
```

Historical process-only requirements are `NOT_APPLICABLE_PRE_V2_4_1`; current audit `LOOP-*`, `STATE-*`, `RETRO-*`, and `ENV-*` controls remain applicable. Platform release validation that belongs to later planned platform/release work is not promoted to a historical Loop 10–13 defect. The failed local live-MPV attempt is recorded as native IPC verification unavailable with cause unresolved, not as a repository failure or a sandbox PASS/failure claim.

## RA-01 summary

Current components audited: `go.mod`, `core/{models,source,store,metadata,playback,app}.go`, `tests/{architecture,contract}`, `adapters/network/securehttp/{client,client_test}.go`; bounded trace of the secure streaming consumer in `adapters/playback/proxy/proxy.go`. Current source scope was clean at the verified HEAD. Core remains concrete-adapter-free; canonical IDs and opaque/redacted playback sources preserve provider/secret isolation. Contract suites exercise fake replacement, cancellation, lifecycle, invalid adapter output and format-string redaction. Secure HTTP tests exercise HTTPS/origin policy, actual-destination DNS pinning, private/loopback/IPv6 rejection, redirect revalidation/header stripping, TLS, bounded headers/bodies, cancellation, response-body closure and sanitized errors. Streaming `Open` uses caller/session cancellation and transfer-owned body closure; it is intentionally distinct from bounded buffered `Do` and is not a confirmed defect.

Independent reviews found no MUST_FIX. Advisory only: some early `App` methods rely on non-nil dependencies unlike later defensive methods; source contract membership permits additional actual rows beyond expected fixtures. Neither violates an applicable current acceptance/security/resource criterion or reproduces as a current defect. No production repair, test change, human gate, workflow change, commit or remote mutation was made.

## RA-04 audit contract and closeout

```yaml
batch: RA-04
historical_loops: [14, 15, 16]
result: LEGACY_VERIFIED
closeout: CLOSED
criteria: [ARCH-004, ARCH-009, ARCH-012..014, FUNC-007..009, PLAY-010..012, PRIV-006..011, PERF-007, QUAL-001..012]
verification_to_run: []
human_gate: null
```

Audited current `adapters/persistence/sqlite/{store,path,migrations,playback,following,episode_mappings,source_ingest,settings,validation}.go`, both embedded SQL migrations and their tests; `core/{store,app,playback_tracking}.go`; Store contracts; desktop `backend/{service,actions,ingest,dto}.go`, Home integration tests and the current Home state mapping. Verified canonical IDs, source mappings, migration checksum/schema fingerprint and transactional rollback, reopen and resume, atomic progress/history checkpoints, stale/concurrent writes, following state, malformed rows, cancellation/Close, and DTO/secret isolation. No source or test code changed.

Local sandbox PASS at `028b02210d0ab6e986ccae2b8ac3a77799019343` with repository-local GOCACHE: `go test -count=1 ./core ./adapters/persistence/sqlite ./apps/desktop/backend ./tests/contract`, the same command with `-race`, and `go vet` for those packages. Existing tests include migration rollback/tamper/partial-schema cases, temporary-store reopen, checkpoint atomic rollback and concurrent/stale writes, corrupt stored values, and offline desktop restart. Read-only `gh run view 34874294714 --json headSha,status,conclusion,jobs,url` confirmed exact-HEAD linux job `104077534611` SUCCESS, including full Go test/race/vet and desktop build. Fresh read-only correctness and security/resource reviews found no MUST_FIX. The inherited Loop 23 user-visible disconnected relaunch PASS covers cached Following and Continue Watching; no new RA-04 human gate is needed. Historical process-only evidence remains `NOT_APPLICABLE_PRE_V2_4_1`.

Advisories intentionally unchanged: desktop `Service.Following` returns cached follow/watch data without source availability; this is its tested offline contract and the richer Following UI is future Phase 26. `RemoveHistory` retains progress for resume as current tested behavior. Focused fixtures do not simulate physical SQLite page damage, independent-process concurrent migration, or post-write WAL/SHM permissions; no current failure was reproduced. Metadata cover URLs may contain query strings, but the reviewed persistence paths store no playback URL/token/cookie/header or proxy capability. These observations do not authorize production changes. No repair, commit, push, new CI run or remote mutation was made for RA-04.

## RA-05 audit contract and closeout

```yaml
batch: RA-05
historical_loops: [17, 18, 19, 20]
result: LEGACY_VERIFIED
closeout: CLOSED
criteria: [ARCH-003, ARCH-007, ARCH-013..015, META-001..006, SEC-001..012, SEC-015..019, PRIV-001..005, PRIV-010..011, RES-001..004, RES-010, PERF-002, PERF-004..005, QUAL-001..012]
verification_to_run: []
human_gate: null
```

Audited current `adapters/metadata/{anilist,bangumi,cover,internal}`, `internal/metadata`, `core/{metadata,metadata_match,metadata_normalize,metadata_variants,models}.go`, `tests/contract/metadata.go`, the shared `securehttp` boundary, SQLite metadata validation/readback, and narrow desktop Detail/GetCover/DTO/cache traces. Fixed-origin provider requests, bounded/atomic JSON parsing, malformed and partial responses, cancellation, redaction, provider identity, matching ambiguity and cross-check, bounded plain-text/cover processing, and cache revalidation have deterministic fixtures. Runtime provider orchestration and background refresh are expressly deferred to the application wiring and Phase 29; cached Detail behavior is the current boundary, not an RA-05 defect.

`RA-05-F001` reproduced a distinct same-provider-ID ambiguity that the matcher had accepted as high confidence. A new deterministic fixture failed before the fix. The matcher now treats comparably scored distinct IDs from one provider as a conflict while retaining cross-provider corroboration and duplicate-reference behavior. Focused matching tests, ten shuffled affected-package rounds, final-state full Go tests, full Go race tests, and full vet passed. Fresh read-only correctness and security/resource reviews found no remaining MUST_FIX. The two-file repair was committed locally as `c4bf12b0f5705419715e087a3ca1bfca839de6c6`; it was not pushed. Earlier exact-HEAD CI 34874294714/linux 104077534611 at `028b0221` remains evidence for unchanged metadata components and cannot certify the new matcher commit. No new CI or live upstream request was made; local final-state regression is the closeout verifier. Historical process-only evidence is `NOT_APPLICABLE_PRE_V2_4_1`.

Advisories intentionally unchanged: metadata provider orchestration/cache refresh belongs to future application wiring/Phase 29; live upstream compatibility was not retested, since fixed fixtures/contracts are authoritative for this batch. No human gate is applicable. RA-06 was NOT_STARTED at RA-05 closeout.

## RA-06 audit contract

```yaml
batch: RA-06
historical_loops: [21, 22]
result: LEGACY_VERIFIED
scope: "Current Wails binding, Svelte shell, desktop lifecycle, DTO and keyboard/focus boundary; not future native UI implementation."
criteria: [ARCH-001, ARCH-010, ARCH-014..015, SEC-014..018, PRIV-010..012, RES-002..004, RES-010, UX-002..003, UX-008..010, QUAL-001..012]
verification_to_run:
  - "Completed: frontend shell/Home tests, typecheck and build; exact-SHA hosted Chromium interaction check."
  - "Completed: focused and final full Go tests, race and vet; exact-SHA hosted full regression."
  - "Completed: independent correctness, security/resource and code-quality review."
human_gate: "HUMAN_PASS on repaired native Windows Home, arrow focus and Enter activation."
production_mutation: "Only for a confirmed RA-06 MUST_FIX; no Wails-to-native migration inside the audit."
```

The owner explicitly chose to complete this audit before changing UI architecture. A separate native-UI migration plan is tracked in `.slim/deepwork/native_ui_migration.md`; it is not RA-06 verification evidence. F001/F002 were closed only after local final-state tests, fresh independent review, user-visible native check and exact-SHA [CI 35943800633](https://github.com/loustack17/AnimePortable/actions/runs/35943800633) all passed.

## Findings

RA-04 has no confirmed current findings and required no repair. RA-05 has no open findings.

| ID | Batch | Origin loop | Severity | Type | Criteria | Status | Evidence / next action |
| --- | --- | ---: | --- | --- | --- | --- | --- |
| RA-03-F001 | RA-03 | 11 | MEDIUM / MUST_FIX | FUNCTIONAL_ERROR_PROPAGATION | PLAY-003 | CLOSED | Backend returns only fixed missing/invalid-player sentinels; Home exact-whitelist maps them to safe actionable messages and keeps unknown failures generic. Focused/full verification and independent reviews PASS. |
| RA-05-F001 | RA-05 | 19 | MEDIUM / MUST_FIX | MATCH_IDENTITY_AMBIGUITY | META-003, META-005 | CLOSED | Two distinct AniList IDs with identical available title formerly selected one by lexical ID; focused fixture failed before repair. Local commit c4bf12b fails closed; final-state full test/race/vet and independent reviews PASS. |
| RA-06-F001 | RA-06 | 22 | MEDIUM / MUST_FIX | KEYBOARD_NAVIGATION | UX-002; Product Contract §15 | CLOSED | Browser regression failed before repair on ArrowDown. App moves focus with all four arrows and wraps without activation; native human, independent reviews and exact-SHA CI 35943800633 Chromium interactions PASS at 3e15651e. |
| RA-06-F002 | RA-06 | 21 | MEDIUM / MUST_FIX | WEBVIEW_CSP | Security §20 | CLOSED | Unit regression failed before repair for absent policy. Bundled HTML restricts local scripts/styles/connections and blocks remote script, frame, object, eval and inline execution; native human, independent security review and exact-SHA CI 35943800633 PASS at 3e15651e. |

## RA-02 summary

Current components audited: `core/{source,playback,app}.go`; `adapters/source/anime1/{anime1,resolver,schedule,html_policy}.go`; `adapters/network/securehttp`; `tests/contract/source.go`; relevant core, SQLite and desktop DTO traces. Current provider behavior maps to Loops 05–09: catalog/search, episode archive parsing/pagination, resolver, schedule and adapter acceptance. JSON/HTML inputs are bounded and parsed rather than rendered; fixed HTTPS/origin policy, redirect and URL validation, MIME checks, cancellation, all-or-nothing results, and secret redaction remain covered. Resolver URLs/cookies are transient opaque `PlaybackSource` data; core tests, persistence traces and DTO JSON tests confirm they are not stored or exposed to the frontend.

No confirmed current defect, REPAIR_REQUIRED finding, production/test change, live upstream request or human gate. Independent correctness/quality and security reviews found no MUST_FIX. Advisory only: the previously recorded non-nil early-App-method consistency and permissive source-fixture membership observations remain non-defects; they were intentionally not changed.

## RA-03 summary

Current components audited: `adapters/playback/proxy`; `adapters/player/mpv/{locator,process,ipc,ipc_endpoint*,player}.go`; `core/{playback,app,playback_tracking}.go`; player contracts; narrow SQLite playback checkpoint and desktop service/Home error/lifecycle traces. Proxy, command construction, MPV process and IPC lifecycle, same-PID switching, pause/seek/snapshot events, restart/resume, checkpoint ordering, cleanup, cancellation, failure states, transient URL/header lifetime, frontend/persistence isolation and redaction were inspected.

Focused tests, race detection and vet passed. Fifty race-enabled same-session switch repetitions and ten race-enabled repetitions of the 12-cycle process start/close/reap test passed. Linux and Darwin portable playback packages cross-built, and the full local Go regression passed. Static bounded queues/semaphores plus throttled 15-second/pause/boundary checkpoint tests support non-growing switching resources and no high-frequency SQLite writes. The live MPV attempt found MPV v0.41 on PATH but both native IPC smokes timed out after about five seconds; no orphan remained. The cause is unresolved among environment/IPC/fixture factors, so no equivalent retries or repository finding were created. Applicable inherited native evidence and exact-HEAD hosted evidence are reused as described above.

`RA-03-F001` was repaired within the explicitly authorized cross-boundary scope. The backend now normalizes only `mpv.ErrNotFound` and `mpv.ErrInvalidPath` to their fixed safe sentinels and keeps every other failure generic. Home exact-whitelists those messages for short Traditional Chinese missing/invalid-player guidance across synchronous and asynchronous Wails failures; unknown/non-Error values remain generic. Tests cover direct/wrapped secret-bearing backend errors, both actionable categories, generic failure and successful playback. No proxy, IPC, process, persistence, layout, navigation, search or episode-selection behavior changed.

Final focused backend tests/race/vet, frontend tests/check/build, serialized full Go test/race/vet and independent correctness/security/resource reviews passed. The first full Go test/race attempt was invalidated by running Vite concurrently with Go embed; after build completion the unchanged commands passed serialized. Automated controller mapping, Wails `Error.message` behavior and the unchanged HomeView `role="alert"` render path fully prove this bounded message change, so no human gate remains. Advisory only: the two fixed wire messages are duplicated in backend/frontend; no shared/generated contract was added because that would broaden this repair. The six-file repair was committed as `028b02210d0ab6e986ccae2b8ac3a77799019343` and exact-SHA CI 34874294714/linux 104077534611 completed SUCCESS with every step green; RA-03 is CLOSED / LEGACY_VERIFIED.

Finding status:

- `OPEN`
- `DIAGNOSED`
- `FIX_IN_PROGRESS`
- `VERIFYING`
- `CLOSED`
- `ADVISORY`
- `NEEDS_HUMAN`
- `NEEDS_ADR`

## RA-06 progress

Current boundary inventory found fixed typed Wails service methods, DTO-only frontend exposure, local asset-only Svelte rendering, bounded operation admission and cancellation/shutdown cleanup. Core remains UI-independent. Existing Home controller/browser and desktop tests cover stale/cancelled operations, safe errors, service lifecycle, six destinations, Tab/Enter, skip link, focus and mouse behavior. The existing native Loop 22/23 human evidence predates the new arrow/CSP repair; a fresh user-visible check of the repaired native Home, arrow focus and Enter activation passed on 2026-09-23.

Two independent read-only reviewers confirmed F001/F002. The added CSP unit test failed before the HTML change; the added Chromium arrow-focus assertion failed before the handler change in a host diagnostic. Minimal repairs affect only `frontend/index.html` and `frontend/src/App.svelte`, with corresponding shell/browser regressions. Final local `npm test` (8/8), `npm run check` (zero diagnostics), `npm run build`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, Windows production desktop build and scoped diff whitespace check PASS. Post-repair host Chromium Home script PASS; a short host native Wails launch started with six WebView2 processes under an isolated profile. Host runs are diagnostic only, not project PASS evidence. Fresh independent quality and security/resource review found no code MUST_FIX; a transient status-table count mismatch was corrected.

The prior [CI run 34874294714](https://github.com/loustack17/AnimePortable/actions/runs/34874294714) at `028b0221` was rechecked read-only: Chromium Home interactions, twenty controller rounds, full frontend and Go verification, vulnerability check and desktop build succeeded. `git diff 028b0221..c4bf12b -- apps/desktop` is empty, so that run supports the pre-repair desktop baseline only; it does not certify the new arrow/CSP changes. The user authorized the two-commit `main` push including prior RA-05 commit `c4bf12b0` and focused RA-06 repair `3e15651e`; [exact-commit CI 35943800633](https://github.com/loustack17/AnimePortable/actions/runs/35943800633) completed SUCCESS. The native-UI migration is a separate future loop after retrospective baseline completion.

## Latest verification

| Batch | Command / procedure | Result | Environment / artifact |
| --- | --- | --- | --- |
| RA-01 | `go test -count=1 ./core ./tests/contract ./tests/architecture ./adapters/network/securehttp` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-01 | `go test -race -count=1 ./core ./tests/contract ./tests/architecture ./adapters/network/securehttp` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-01 | `go vet ./core ./tests/contract ./tests/architecture ./adapters/network/securehttp` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-01 | CI 34790321962 / linux 103813204911 | PASS | GitHub-hosted runner, exact HEAD; reused whole-repository regression evidence |
| RA-02 | `go test -count=1 ./adapters/source/anime1 ./tests/contract` | PASS | Local Codex sandbox; repository-local GOCACHE; no live upstream request |
| RA-02 | `go test -race -count=1 ./adapters/source/anime1 ./tests/contract` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-02 | `go vet ./adapters/source/anime1 ./tests/contract ./core` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-02 | `go test -count=1 ./...` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-02 | CI 34790321962 / linux 103813204911 | PASS | GitHub-hosted runner, exact HEAD; reused full race/vet/module/vulnerability/build/clean-worktree evidence |
| RA-03 | `go test -count=1 ./adapters/playback/proxy ./adapters/player/mpv ./core ./tests/contract ./apps/desktop/backend ./adapters/persistence/sqlite` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 | `go test -race -count=1` on the same focused packages | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 | `go vet` on the same focused packages | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 | race `TestPlayerSwitchesEpisodesOnOneProcessAndRotatesCapabilities`, `-count=50` | PASS | Local Codex sandbox |
| RA-03 | race `TestRepeatedStartCloseCyclesRemainReaped`, `-count=10` (120 process cycles) | PASS | Local Codex sandbox |
| RA-03 | CGO-disabled Linux amd64 and Darwin arm64 builds of core/proxy/MPV packages | PASS | Local Codex sandbox; compile evidence only |
| RA-03 | `go test -count=1 ./...` | PASS | Local Codex sandbox |
| RA-03 | `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 ./adapters/player/mpv -run 'TestLive' -v` | NATIVE_IPC_UNAVAILABLE | MPV v0.41 executable runs; two IPC startup timeouts; no orphan process; cause unresolved, not repository FAIL |
| RA-03 | CI 34790321962 / linux 103813204911 | PASS | GitHub-hosted runner, exact HEAD; metadata independently corroborated read-only |
| RA-03 repair | `go test -count=1 ./apps/desktop/backend` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 repair | `go test -race -count=1 ./apps/desktop/backend` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 repair | `go vet ./apps/desktop/backend` | PASS | Local Codex sandbox; repository-local GOCACHE |
| RA-03 repair | frontend `npm test` and `npm run check` | PASS | Local Codex sandbox; 7 tests; zero diagnostics |
| RA-03 repair | frontend `npm run build` | PASS | Local Codex sandbox; production build |
| RA-03 repair | serialized `go test -count=1 ./...` and `go test -race -count=1 ./...` | PASS | Local Codex sandbox; final working-tree state after frontend build |
| RA-03 repair | `go vet ./...` | PASS | Local Codex sandbox; final working-tree state |
| RA-03 closeout | CI 34874294714 / linux 104077534611 | PASS | GitHub-hosted runner at exact repair commit 028b02210d0ab6e986ccae2b8ac3a77799019343; all steps success |
| RA-05 reproducer | `go test -count=1 ./core -run '^TestMatchMetadataFailsClosedForDistinctSameProviderIdentities$'` | FAIL before fix | Accepted AniList ID `1` at score 70 despite indistinguishable ID `2` |
| RA-05 focused | `go test -count=1 ./core -run 'TestMatchMetadata\|TestNormalizeMetadataTitle'` | PASS | Local sandbox, final repair state |
| RA-05 affected regression | `go test -shuffle=on -count=10 ./core ./adapters/metadata/... ./internal/metadata ./adapters/persistence/sqlite ./tests/contract ./apps/desktop/backend` | PASS | Local sandbox, final repair state |
| RA-05 full regression | `go test -count=1 ./...`; `go test -race -count=1 ./...`; `go vet ./...` | PASS | Local sandbox, final repair state before local commit c4bf12b |
| RA-05 inherited CI | CI 34874294714 / linux 104077534611 | PASS | Exact prior HEAD 028b0221; reused for unchanged components only, not final matcher commit |
| RA-06 CSP reproducer | `npm test` | FAIL before fix | New shell test detected missing bundled HTML CSP; no test was weakened |
| RA-06 arrow reproducer | `node tests/browser/home.mjs` | FAIL before fix | Host diagnostic only; ArrowDown did not move focus from Home to Schedule |
| RA-06 final frontend | `npm test`; `npm run check`; `npm run build` | PASS | Local sandbox; 8/8 tests, zero Svelte diagnostics, production bundle |
| RA-06 final Go | `go test -count=1 ./...`; `go test -race -count=1 ./...`; `go vet ./...` | PASS | Local sandbox; repository-local GOCACHE, after final frontend build |
| RA-06 browser | `node tests/browser/home.mjs` | HOST_DIAGNOSTIC_PASS | Browser script with new arrow assertions and bundled CSP; not authoritative acceptance |
| RA-06 native shell | Windows production build plus isolated-profile Wails launch | HOST_DIAGNOSTIC_PASS | Main window process and six new WebView2 processes observed; user-visible response is separately recorded as HUMAN_PASS |
| RA-06 native human | Repaired Windows Home, sidebar arrows/focus and Enter | HUMAN_PASS | User explicitly confirmed all normal while viewing isolated visible test window; all test processes closed and temp profile removed |
| RA-06 review | Fresh quality and security/resource review | PASS_REVIEW | No remaining code MUST_FIX; exact-state CI and native human gates passed |
| RA-06 hosted | CI 35943800633 / linux 107457221421 | PASS | Exact commit 3e15651e; frontend/Chromium, Go test/race/vet, vulnerability check, desktop build, clean worktree |

## Baseline completion

```yaml
retro_baseline_result: RETRO_BASELINE_PASS
final_head: 3e15651e3d7761122f0587f0ed476b224b79f1de
full_regression: "PASS: local final frontend test/check/build and full Go test/race/vet; exact-SHA CI 35943800633 includes frontend, Chromium Home, Go test/race/vet, vulnerability check, desktop build and clean runner worktree."
architecture_quality_review: "PASS: fresh read-only final review found no code MUST_FIX; status/handoff inaccuracies corrected at closeout. Earlier RA-01..RA-06 batch reviews remain recorded above."
security_review: "PASS: fresh read-only final security/resource review found no unresolved MUST_FIX; earlier batch reviews and RA-06 CSP review remain recorded above."
human_result: "HUMAN_PASS: repaired native Windows Home, visible arrow-key focus and Enter activation confirmed by owner on 2026-09-23."
```

`RETRO_BASELINE_PASS` is permitted only under `docs/11_RETROSPECTIVE_BASELINE_AUDIT.md`.
