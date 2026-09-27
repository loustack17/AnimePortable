<!-- SPDX-License-Identifier: MPL-2.0 -->

# Verification Matrix

This file maps acceptance categories to the verifier that is allowed to decide PASS.

The matrix is intentionally outcome-focused. Do not require a specific internal implementation path when multiple correct implementations satisfy the contracts.

## 1. Verifier types

- `A` — Automated deterministic: executable pass/fail is authoritative.
- `H` — Human: human observation/approval is required for PASS.
- `Y` — Hybrid: automated evidence is required, then independent AI review checks gaps.
- `YH` — Hybrid + Human final: automated evidence and independent review run first; human makes the final subjective/product decision.

An AI reviewer must be read-only during its review pass. It cannot turn a deterministic FAIL into PASS.

## 2. Human-gate triggers independent of criterion ID

A human gate is mandatory if any loop:

- changes product, architecture, security, acceptance semantics, or ADRs
- weakens/removes/skips a required test or CI gate
- introduces a new trust boundary, privilege, credential path, arbitrary network/file/process capability, or destructive migration
- introduces a new runtime/native/security-sensitive dependency
- reaches non-convergence or budget stop and requires a strategy decision
- requests an exception for a blocked/flaky/failed gate
- performs final MVP/release acceptance

## 3. Architecture criteria

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| ARCH-001..005 | A | dependency/import guard test or equivalent static check passes |
| ARCH-006..011 | A | contract/fake adapter tests pass |
| ARCH-012..014 | Y | persistence/failure-path tests pass; independent review when touched |
| ARCH-015 | Y; YH when architecture meaning/boundary changes | deterministic dependency checks + reviewer confirms no unjustified layer chain; human approval if architecture boundary changes |

## 4. Functional browsing criteria

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| FUNC-001 | YH once UI is exposed | offline/cache integration test proves no startup network dependency; human confirms visible cached flow once UI exists |
| FUNC-002..003 | Y | adapter/application tests + UI/E2E once wired |
| FUNC-004 | YH | DTO/content assertions + running UI human check for presentation/clipping |
| FUNC-005..006 | Y | deterministic ordering/schedule tests + integration |
| FUNC-007..009 | YH once UI is exposed | persistence/application tests + human workflow check for user-facing behavior |

## 5. Playback criteria

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| PLAY-001..002 | Y | platform/path detection tests; platform smoke where available |
| PLAY-003 | YH | deterministic missing-player path + human confirms error is actionable |
| PLAY-004..008 | Y | integration tests; secret redaction/proxy assertions; PID evidence for switching |
| PLAY-009 | YH | automated config-preservation assertion plus real MPV smoke/human confirmation where practical |
| PLAY-010..013 | Y | IPC/progress/restart/autoplay scenario tests |
| PLAY-014..015 | YH | code/UI absence checks + human confirms no overlay/danmaku in actual playback workflow |

## 6. Keyboard and usability criteria

All `UX-001..010` are `YH`.

Automation should cover keyboard events, focusability, DOM semantics, routing/state, and regression checks where possible. Human final verification must use the running application.

### Human UX procedure

For each affected UI loop:

1. launch the built application
2. complete the intended workflow with keyboard only
3. repeat core actions with mouse
4. confirm visible focus at every keyboard step
5. confirm no keyboard trap
6. test loading, empty, and error states that are in scope
7. resize to the supported small-desktop boundary
8. check clipping, overlap, unreadable density, and accidental mobile-first behavior
9. verify labels communicate user intent rather than implementation internals
10. confirm the UI remains calm/minimal and excluded product patterns were not introduced

PASS requires all mandatory observations to succeed. Record concrete failures by screen/action.

## 7. Metadata correctness criteria

`META-001..006` are `Y`.

Minimum evidence:

- provider contract tests
- fixed fixture set with expected matches and expected no-match cases
- ambiguity/conflict/adversarial cases
- cache-provider-outage behavior
- independent review when matcher thresholds/policy change

Do not use live-provider success alone as proof of matcher correctness.

## 8. External network security criteria

`SEC-001..019` are `Y` and are critical.

Minimum evidence must include relevant negative/adversarial tests. A happy-path smoke alone is insufficient.

Any change to trust policy, SSRF rules, redirect policy, TLS verification, allowed origins, or raw-content exposure also triggers a human gate/ADR rule from the runbook.

Critical security criteria cannot be counted as MVP PASS using only a human exception.

## 9. Privacy and secret handling criteria

`PRIV-001..012` are `Y`.

Use static/dynamic assertions for persistence, logs, argv, UI action data, and privilege requirements. Where possible, test formatted error variants and failure paths, not only success paths.

## 10. Local proxy and IPC criteria

`PROXY-001..007` and `IPC-001..007` are `Y`.

Minimum evidence includes:

- negative token/session cases
- lifecycle cleanup
- platform-specific IPC checks
- malformed input
- cancellation/close paths
- leak/redaction assertions

## 11. Resource safety criteria

`RES-001..010` are `Y`.

Resource criteria require repeated-cycle/stress evidence where the criterion is about growth or cleanup. A single successful execution is insufficient for `RES-008..010`.

No monotonic-growth claim may be PASS based only on visual inspection of one run.

## 12. Performance criteria

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| PERF-001..002 | A | instrumentation/test proves cached startup path does not wait on provider |
| PERF-003 | YH | repeatable startup measurement recorded; human confirms perceived responsiveness. `<1s` remains a target, not universal hardware law |
| PERF-004..005 | Y | concurrency/lazy-loading bounds tested or directly inspectable |
| PERF-006 | Y | idle resource observation under defined environment; threshold/environment recorded |
| PERF-007 | A | write-frequency/checkpoint tests or instrumentation |
| PERF-008 | YH | owner-approved reference host/content and matched repeatable FLTK/libmpv versus OneAnime idle and playback samples; all runs and private resident/commit, CPU, GPU, startup and cleanup recorded; owner resolves a miss before PASS |

## 13. Cross-platform criteria

`PLATFORM-001..009` require `Y`, with actual platform evidence where the criterion names a platform. The current Windows phase closes Windows-specific criteria only; Linux and macOS criteria are deferred to their later platform phases and remain required for all-platform `COMPLETE`. For `PLATFORM-008`, inspect final source/build dependencies and release contents, then observe loaded UI libraries/surfaces and the running process tree on each platform as it enters acceptance. An in-process OS WebView is a failure even without a packaged browser or helper process; Windows-only checks cannot prove Linux or macOS. For `PLATFORM-009`, exercise fresh, upgrade, read-only and non-ASCII/spaced portable folders plus explicit old-profile import without deleting the source.

Cross-compilation is evidence for buildability but is not a substitute for runtime IPC/keyboard validation on the named platform when runtime behavior is the criterion.

If the platform is unavailable, use `BLOCKED`, not PASS.

## 14. CI and release criteria

`CI-001..010` are `Y`. CI-004..006 apply to the final FLTK Windows adapter. For `CI-010`, extract and launch the Windows ZIP without an installer or privileged registration; verify the bundled libmpv runtime and its notices. Linux/macOS artifact checks remain deferred to their later phases.

Required status checks must pass on the final commit/artifact. Manual bypass must be explicit and is not equivalent to the underlying technical check passing.

Final release acceptance also requires the human final gate.

## 15. Loop-integrity criteria

`LOOP-001..025` are checked from the loop evidence/status record.

- LOOP-002..007 and LOOP-010..011 should be mechanically checked where feasible from command logs/diffs.
- LOOP-008 requires independent reviewer evidence when triggered.
- LOOP-009 requires human evidence when triggered.
- LOOP-012 is a migration audit rule and must not cause automatic reimplementation.
- LOOP-013..015 and LOOP-017..021 require sandbox/harness/GitHub-scope evidence, not agent self-report alone.
- LOOP-016 requires successful read-only observation of the connected repo's CI/check evidence when CI is in scope.
- LOOP-022 requires Codex instruction-loading evidence (`AGENTS.md` effective for the touched path).
- LOOP-023 requires worktree/write-set evidence for parallel code-changing agents.
- LOOP-024 requires a distinct final reviewer identity/context from the authoring worker.
- LOOP-025 requires no alternate-agent harness dependency in required execution evidence.



## 15.1 Agent execution-security evidence

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| LOOP-013 | A/Y | sandbox configuration or harness evidence shows repo-scoped persistent writable root; reviewer confirms no host-control socket/privileged escape path was granted |
| LOOP-014 | A/Y | filesystem policy/audit/command trace shows no persistent host write outside repo; sandbox temp/cache is explicitly ephemeral |
| LOOP-015 | A/Y | configured GitHub target is one explicit `owner/repo`; command/API traces do not target unrelated repositories |
| LOOP-016 | A | required check/run/log evidence can be retrieved from the connected repo using read capability (`gh pr checks`, `gh run ...`, API/connector equivalent) |
| LOOP-017 | Y | credential source/scope is recorded without secret value; no host SSH/global credential import; write permission exists only when needed |
| LOOP-018 | A/Y | network proxy/allowlist configuration or equivalent environment policy is recorded; unexpected destination attempts fail or require approval |
| LOOP-019 | YH | no protected GitHub administrative action occurred; if one is assigned, human approval and independent security review are recorded before execution |
| LOOP-020 | A/YH on violation | no boundary-violation event; any detected event automatically prevents PASS and routes to human review |
| LOOP-021 | A/Y | Codex App/CLI reports a sandboxed workspace profile; no host Full Access/danger bypass was used |
| LOOP-022 | Y | applicable `AGENTS.md` scope is identified and reviewer confirms no nested override weakens safety/quality rules |
| LOOP-023 | A/Y | Codex worktree/thread/write-set record shows parallel code writers did not overlap mutable files/worktree |
| LOOP-024 | Y | final reviewer is a separate fresh-context read-only review pass and did not author the approved final code |
| LOOP-025 | A/Y | required loop commands/evidence are reproducible with Codex App/CLI alone; OpenCode/other harness is not a hidden dependency |

CI visibility and host isolation are compatible requirements. The repository may be bind-mounted read-write into the sandbox while GitHub Actions/check data is fetched through repository-scoped API/CLI/connector access. Host filesystem write access is not required to diagnose CI.

## 16. Failure fingerprints and correction evidence

For every corrective iteration record:

```yaml
iteration: 2
criterion: FUNC-001
classification: TEST_FAILURE
fingerprint:
  check: TestHomeUsesCacheWithoutNetwork
  error: unexpected outbound request
  component: home/service
hypothesis: home hydration invokes remote refresh before cached render
new_evidence: <what was inspected/measured>
change: <minimal correction>
focused_result: PASS | FAIL
regression_result: PASS | FAIL | NOT_RUN
```

If the same fingerprint remains after two repair attempts, do not continue blind repair; follow the runbook's non-convergence path.

## 17. Human review record

Use this format:

```yaml
human_gate:
  required: true
  criteria: [UX-001, UX-002, UX-010]
  environment:
    os: <OS/version>
    app_build: <commit/artifact>
    display: <resolution/scaling when relevant>
  procedure:
    - <step>
  observations:
    - id: UX-001
      result: PASS | FAIL | BLOCKED
      evidence: <specific observation>
  overall: HUMAN_PASS | HUMAN_FAIL | HUMAN_BLOCKED | HUMAN_EXCEPTION_APPROVED
  reviewer: human-owner
  date: <date>
  notes: <optional>
```

The human should not be asked to approve until deterministic gates and known must-fix AI review findings are already resolved, unless the purpose of the human gate is to resolve ambiguity/non-convergence.

## 18. Code-quality verification

`QUAL-001..012` are blocking criteria for every loop that changes production code.

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| QUAL-001 | Y | reviewer maps the final diff to the pragmatic SOLID rules and identifies no ceremonial abstraction or clear principle violation |
| QUAL-002 | Y | changed file/type/function responsibility review shows cohesive ownership; mixed responsibilities are resolved or explicitly justified |
| QUAL-003 | A/Y | dependency/import checks plus reviewer inspection show no inward concrete infrastructure dependency |
| QUAL-004 | Y | changed interfaces are necessary, cohesive, and no broader than actual consumers require |
| QUAL-005 | A/Y | relevant contract tests pass and reviewer confirms error/cancellation/lifecycle/security semantics remain substitutable |
| QUAL-006 | Y | fresh-context reviewer can explain control flow, error paths, and side effects without finding misleading indirection; no `MUST_FIX` readability finding |
| QUAL-007 | Y | every new/moved production file has a clear layer/package owner consistent with the repository layout |
| QUAL-008 | A/Y | no new generic dumping-ground package/file, avoidable cycle, or god structure; static dependency checks used where practical |
| QUAL-009 | A/Y | diff/search/reviewer finds no loop-introduced dead/debug scaffolding or duplicated policy; tests still pass after cleanup |
| QUAL-010 | Y | changed comments/docs match final behavior and explain only non-obvious constraints/invariants |
| QUAL-011 | A/Y | exported/binding/API diff is reviewed; unnecessary public surface is removed before PASS |
| QUAL-012 | Y | dedicated final code-quality reviewer returns `PASS_REVIEW` with zero unresolved `MUST_FIX` items |

The quality reviewer must inspect both the diff and enough surrounding code to judge ownership and duplication. Reviewing only isolated changed lines is insufficient for file-placement and responsibility decisions.

No numeric line-count threshold alone creates PASS or FAIL. Size is a signal to inspect cohesion, not a substitute for design judgment.

## 19. Retrospective audit for Loops 01-22

For each historical loop:

```yaml
loop: 12
implementation_replay_required: false
criteria:
  - id: IPC-006
    existing_evidence: <test/review/smoke>
    status: PASS | NEEDS_EVIDENCE | NEEDS_HUMAN | FAIL
missing_verification:
  - <only what is missing>
result: PASS | REOPEN_REQUIRED | HUMAN_CHECK_REQUIRED
```

Rules:

- do not rewrite historical code merely to conform to the new report schema
- run missing checks against current code when still meaningful
- reopen implementation only when a check actually fails or current code no longer satisfies the contract
- treat later code changes that superseded old behavior as current-state verification, not historical reconstruction

## 20. Current migration point: Loop 23

Loop 23 is the first loop that must complete under Protocol v2.4.1.

If implementation is already in progress:

- do not discard existing Loop 23 work
- capture the current state as the migration baseline
- establish the remaining Loop 23 contract now
- include all current Loop 23 changes in final diff/integrity review
- apply bounded diagnosis/correction to all remaining failures
- complete required human UI review before PASS
## 21. Durable-state and session-recovery verification

| Criterion | Primary verifier | PASS evidence | Human gate |
| --- | --- | --- | --- |
| STATE-001 | deterministic + inspection | active handoff file exists and required top-level structure is present | no |
| STATE-002 | deterministic | `wc -c docs/AGENT_HANDOFF.md <= 8192` and `wc -l docs/AGENT_HANDOFF.md <= 160` | no |
| STATE-003 | structured inspection | required recovery fields populated for active state | no |
| STATE-004 | secret/pattern scan + review | no transcript/log dump or sensitive values; long evidence referenced, not copied | security-sensitive ambiguity only |
| STATE-005 | deterministic Git reconciliation | branch/HEAD/status/diff inspected and discrepancies resolved before mutation | conflict that cannot be safely reconciled |
| STATE-006 | trace/review | continuing session used bounded bootstrap or documented why broader reading was necessary | no |
| STATE-007 | status/handoff timestamps/state | checkpoint reflects latest material recoverability boundary / exit state | no |
| STATE-008 | multi-agent trace/review | only root owns global handoff; worker outputs integrated by root | no |
| STATE-009 | architecture/review | durable normative decisions live in ADR/docs/tests rather than handoff-only memory | ADR changes follow normal human gate |
| STATE-010 | session policy review | instruction/protocol change triggers fresh/reconciled session | no |

### Startup reconciliation evidence

Record concise evidence such as:

```text
branch: <branch>
HEAD: <sha>
git status --short --branch: <clean | summarized dirty paths>
handoff observed_head: <sha>
reconciliation: MATCH | REFRESHED | STALE_BLOCKED
next source set: <referenced docs/headings/files>
```

Do not paste the full diff into the handoff or status file. Reference changed paths and inspect the actual diff directly.

### Token-efficiency rule

The efficiency goal is **not** minimum context at any cost. Required safety/architecture evidence must still be loaded when relevant.

The expected hierarchy is:

1. always-loaded/scoped `AGENTS.md`
2. bounded `docs/AGENT_HANDOFF.md`
3. exact relevant source sections/files/tests/evidence
4. broader canonical docs/repository exploration only on trigger

This prevents repeated whole-repository rediscovery while keeping source-of-truth retrieval available when the task changes.


## 22. Retrospective baseline verification

`RETRO-001..015` are blocking process-integrity criteria for the one-time RA-01..RA-06 audit.

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| RETRO-001 | A/Y | Loop 23 final status is PASS and Loop 24 has not started before audit activation |
| RETRO-002..006 | Y | batch contract/status shows current-state verification, mapped scope, and no blanket replay |
| RETRO-007 | A/Y | targeted failure-path/adversarial tests or documented risk review for applicable high-risk surfaces |
| RETRO-008 | A/Y | agent/thread/worktree evidence shows read-heavy delegation preference and isolated/disjoint writers when used |
| RETRO-009 | A/Y | every confirmed defect has stable RA finding record with criterion/evidence/status |
| RETRO-010 | Y | diff is traceable to confirmed MUST_FIX findings; no speculative broad cleanup |
| RETRO-011 | A/Y | focused reproducer + affected regression + final integrated reviewer evidence for each repair |
| RETRO-012 | Y | fresh-context read-only general review; specialized security review when triggered |
| RETRO-013 | A/Y | durable status/handoff updated after each completed batch and before stop |
| RETRO-014 | Y | each LEGACY_VERIFIED batch has all mapped applicable criteria resolved; historical process claims are not fabricated |
| RETRO-015 | A/YH | RA-01..06 LEGACY_VERIFIED, zero open MUST_FIX, final regression/reviews complete, required human gate PASS, final HEAD recorded |

Historical `LOOP-*` / `STATE-*` criteria that describe pre-v2.4 execution mechanics are not reconstructed. Mark them `NOT_APPLICABLE_PRE_V2_4` where the historical fact cannot be established. The current retrospective audit execution itself must satisfy current `LOOP-*` / `STATE-*` controls.


## 23. Execution-environment and verifier-fallback criteria

`ENV-001..008` are blocking whenever execution-environment substitution or a sandbox capability failure occurs.

| Criteria | Verifier | Minimum PASS evidence |
| --- | --- | --- |
| ENV-001 | A/Y | failure record shows whether the repository verifier actually started; sandbox/bootstrap/capability failure is separated from product/test failure |
| ENV-002 | A/Y | one stable fingerprint + at most one targeted confirmation; no repeated equivalent shell/path/agent retries after classification |
| ENV-003 | A/Y | execution evidence shows no Full Access/danger bypass was used to obtain acceptance |
| ENV-004 | A/Y | required blocked check is executed in GitHub-hosted CI or another human-approved isolated CI environment |
| ENV-005 | Y | any unsandboxed host run is explicitly marked diagnostic/non-authoritative and is not the sole PASS evidence |
| ENV-006 | A/Y | CI run is tied to exact commit/state; verifier semantics/repetition/assertions preserved; dependency lock/pin evidence recorded where applicable |
| ENV-007 | YH | workflow/action diff has prior human-approved scope, independent verifier/security review, least-privilege permissions, and separate remote authorization |
| ENV-008 | YH where criterion is human/platform-specific | CI substitution is used only for criteria the runner can prove; native Windows/MPV/human checks remain unresolved until their required verifier runs |

For GitHub-hosted evidence, record workflow/run/job/check ID, commit SHA, runner label, and conclusion. Repository read access is enough to inspect workflow history/logs; workflow rerun/control or configuration mutation is a separate permission boundary.

A local sandbox `spawn EPERM` or process-bootstrap failure does not become a product failure merely because it repeats. If the same verifier fails on a capable approved runner, classify that runner failure under the normal implementation/test/resource/security taxonomy.

See `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`.
