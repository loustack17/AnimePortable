<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop Engineering Runbook

This document defines the bounded implementation, verification, correction, escalation, and human-approval loop used by AI coding agents in this project.

The objective is not to make an agent work indefinitely. The objective is **controlled convergence toward independently verified acceptance criteria**.

A loop is not complete because an implementation agent says it is complete. A loop is complete only when the required external gates say it is complete.

## 1. Protocol version and migration

This is Loop Engineering Protocol v2.3 (Codex-targeted, durable-state).

It is mandatory beginning with **Loop 23**.

Loops 01-22 were executed under the original runbook and MUST NOT be reimplemented solely because this protocol was introduced.

Historical loops are grandfathered for implementation but subject to retrospective evidence audit:

1. identify the acceptance criteria the historical loop claimed to satisfy
2. locate existing test, CI, live-smoke, review, and runtime evidence
3. classify each criterion as `PASS`, `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, or `NOT_RUN`
4. run only missing verification that is still relevant
5. reopen implementation only when verification demonstrates an actual defect or an acceptance criterion is not satisfied

Absence of v2-format evidence is not itself proof that old code is wrong.

If Loop 23 already contains work performed before this protocol was adopted, treat the current repository state as the migration baseline. All existing Loop 23 changes remain in scope and MUST pass the v2 final gates before Loop 23 can be marked complete.

## 2. Session bootstrap and source reading

The project uses bounded durable state to avoid rebuilding context from scratch on every Codex session.

### 2.1 Continuing the same active loop in a new session

Before mutation:

1. use/verify applicable `AGENTS.md`
2. read `docs/AGENT_HANDOFF.md`
3. run `git status --short --branch`, recent Git log, `git diff --stat`, and `git diff --name-only`
4. reconcile branch/HEAD/dirty paths with the handoff
5. read only the handoff-referenced/relevant source-of-truth sections, source files, tests, and evidence required for the next action
6. expand reading if any conflict or uncertainty appears

Do not rescan the entire repository or reread all engineering docs simply because a Codex thread/session changed.

### 2.2 New loop or invalid handoff

Perform broader canonical initialization when:

- a new loop begins
- protocol/instruction version changed
- handoff is missing, stale, conflicting, malformed, or over its cap
- architecture/security/acceptance intent is unclear
- actual Git/test/CI state contradicts the handoff

Canonical sources are:

- `docs/README.md`
- `docs/01_PRODUCT_CONTRACT.md`
- `docs/02_ARCHITECTURE.md`
- `docs/03_SECURITY.md`
- relevant phase in `docs/04_MVP_IMPLEMENTATION_PLAN.md`
- this runbook
- `docs/06_ACCEPTANCE_CRITERIA.md`
- `docs/08_VERIFICATION_MATRIX.md`
- `docs/09_CODEX_EXECUTION_PROFILE.md`
- `docs/10_DURABLE_AGENT_STATE.md`
- active ADRs
- `docs/IMPLEMENTATION_STATUS.md` / retained evidence as needed

The root `AGENTS.md` is the short operational entry point. `docs/AGENT_HANDOFF.md` is a recovery cache/index only and never overrides canonical sources or current verifier/Git truth.

OpenCode Go is not a required or validated execution path for Protocol v2.3.

## 3. Roles and separation of duties

The loop has four distinct roles. A single model may technically perform more than one role only when the required independence cannot be provided, but it must use a fresh context and must not claim stronger independence than actually exists.

### 3.1 Implementer

May:

- inspect source and tests
- modify implementation
- add tests that encode intended behavior
- run allowed development commands
- diagnose failures

Must not decide final PASS by self-assessment.

### 3.2 Deterministic verifier

This is the primary authority whenever an objective check exists.

Examples:

- compiler/build exit code
- unit/integration/E2E tests
- race detector
- linter/type checker
- static dependency-boundary checks
- security policy tests
- process/PID assertions
- measured resource bounds
- CI status

A deterministic failure cannot be overridden by an AI reviewer saying the code looks correct.

### 3.3 Independent AI reviewer

The reviewer should use a fresh context and be read-only while reviewing.

Its job is to find gaps not covered by deterministic checks, including:

- mismatch between implementation and documented intent
- missed edge cases
- suspicious test weakening or evaluator gaming
- architecture drift
- security/resource-lifecycle issues
- qualitative defects that require a rubric

The reviewer may return `PASS_REVIEW` or `FAIL_REVIEW`, but `PASS_REVIEW` never converts a failing deterministic gate into PASS.

### 3.4 Human owner

Human review is authoritative where correctness depends on product judgment, subjective UX, approval of policy/spec changes, or acceptance of exceptional risk.

AI approval is not a substitute for a required human gate.

### 3.5 Codex root orchestrator

For Codex multi-agent runs, one root thread owns the loop contract, work decomposition, integration, budgets, final verification state, and human handoff.

The root may delegate when parallel work materially improves speed or quality, but it must keep the overall write set and verification state coherent.

### 3.6 Codex worker/reviewer isolation

Multi-agent delegation is explicitly permitted for this repository subject to `AGENTS.md` and `docs/09_CODEX_EXECUTION_PROFILE.md`.

Rules:

- code-changing workers receive concrete, bounded subtasks and disjoint write sets
- in Codex App, parallel code-changing workers should use isolated worktrees
- in Codex CLI/subagent mode, do not allow simultaneous overlapping writes to the same working tree/files
- reviewers are fresh-context and read-only for the review pass
- a reviewer that authored or directly repaired the same finding does not count as the independent reviewer for that final state
- root and workers must not duplicate the same unresolved task in parallel
- subagents must not create an unbounded delegation tree; global concurrency and loop budgets still apply
- model selection is explicit: workers inherit the active root model unless the human intentionally configures a different model

## 4. One-loop rule

Each loop performs exactly one coherent phase or vertical slice.

Do not bundle unrelated work.

Valid examples:

- secure redirect handling
- Anime1 episode parser
- MPV IPC lifecycle
- Home UI

Invalid examples:

- implement playback, metadata, and redesign settings
- refactor architecture while adding an unrelated feature

## 5. Loop contract: required before mutation

Before modifying code, create a short loop contract in the working notes/status record.

The contract must contain:

```yaml
loop: 23
scope: Home UI
execution_harness: codex_app | codex_cli
root_model: gpt-5.6-sol | gpt-6-astra | <explicit>
multi_agent: true | false
subagent_model_policy: inherit | <human-approved override>
baseline_commit: <git commit or working-tree identifier>
execution_boundary:
  sandbox_required: true
  repo_root: <absolute sandbox-mounted repo root>
  host_filesystem_outside_repo: deny_write
  sandbox_temp: <ephemeral sandbox-only path>
  github_repository: <owner/repo>
  github_access: read_ci | branch_pr_write
  network_policy: allowlist
acceptance_criteria:
  - FUNC-001
  - FUNC-009
verification:
  deterministic:
    - <commands/tests>
  independent_review: required | not_required
  human_gate: required | not_required
protected_verification_assets:
  - <criteria/spec/test/CI files whose weakening would invalidate self-certification>
budgets:
  max_corrective_iterations: 5
  max_repairs_per_failure_fingerprint: 2
  max_replans: 1
  max_unchanged_flaky_reruns: 2
  max_agent_turns: 30
  provider_cost_budget: <value | unavailable>
  token_budget: <value | unavailable>
  remaining_usage: <value | unavailable>
```

If a criterion cannot be connected to a verification method or human gate, it is not ready for autonomous completion. Clarify the contract before implementation continues.

## 6. Agent execution sandbox and mutation boundary

All autonomous implementation, verification, review, and correction work MUST run inside an isolated execution boundary. For Codex App/CLI, use the supported profiles in `docs/09_CODEX_EXECUTION_PROFILE.md`; do not silently fall back to Full Access / danger-full-access because a command is inconvenient.

The purpose is to let the agent operate normally on this repository and observe its GitHub CI while preventing accidental or adversarial modification of the user's host system, unrelated repositories, credentials, or services.

### 6.1 Required sandbox model

Preferred configuration:

- mount the active repository into the sandbox as the only persistent writable project workspace
- keep the user's host filesystem outside that repository read-only or inaccessible
- permit normal toolchain/system reads required to execute compilers, runtimes, and OS libraries without making those host paths writable
- use sandbox-owned ephemeral temp/cache locations for compiler caches, package caches, test output, browser profiles, and temporary files
- discard sandbox-owned ephemeral state when the run ends
- do not expose the host Docker/Podman daemon socket or equivalent privileged control socket to the agent
- do not grant administrator/root/sudo privileges

A writable sandbox temp/cache directory is allowed even though it is outside the repository path because it is disposable sandbox state, not persistent modification of the user's system.

For local interactive use, the recommended balance is a **read-write bind mount of the actual repository** into the sandbox. This keeps edits immediately visible to the human in the normal local repo while all unrelated host paths remain protected.

A stronger isolation mode may instead work in an ephemeral sandbox clone and export a verified patch back to the mounted repository. Use that mode when the harness supports safe patch application and live visibility is not required.

### 6.2 Persistent mutation allowlist

Without an explicit human-approved exception, persistent mutation is allowed only in:

1. the active local repository working tree and its repository metadata as required for normal Git operations
2. the explicitly connected GitHub repository named in the loop contract, and only within the remote-operation policy below

Persistent mutation is forbidden in:

- the user's home directory outside the repository
- shell/editor/AI configuration and dotfiles
- `~/.ssh`, SSH agents, credential stores, system keychains, browser profiles, or global Git/GitHub CLI configuration
- OS configuration, registry, services, launch agents, cron/systemd tasks, drivers, firewalls, or package-manager global state
- unrelated local repositories, mounted drives, cloud-sync folders, or user documents
- repository/org secrets, Actions secrets/variables, branch protection, repository settings, webhooks, deploy keys, environments, or organization settings unless the assigned task explicitly requires it and a human gate approves it

Do not use `sudo`, global package installation, or host-level service installation to make a loop pass.

If a required tool is absent from the sandbox, classify the check as `ENVIRONMENT_BLOCKED` or use an approved sandbox image/dependency setup. Do not repair the host system autonomously.

### 6.3 GitHub observation boundary

The agent MUST be able to observe CI for the connected repository when CI evidence is part of verification.

Allowed read operations include:

- repository/branch/commit metadata
- pull requests and review comments
- required checks and commit statuses
- GitHub Actions workflow/run/job status
- failed-step/full Actions logs
- build/test artifacts required to diagnose the current loop

Examples include `gh pr checks`, `gh run list`, and `gh run view --log-failed` when available.

CI observation is not permission to modify CI configuration or rerun/cancel/delete workflows.

### 6.4 GitHub remote mutation boundary

Remote GitHub writes are restricted to the connected repository and should use a feature branch/PR workflow.

Autonomously allowed when the loop contract grants `branch_pr_write`:

- create/update the loop's working branch
- push commits for the current loop
- create/update the current pull request
- add implementation/evidence comments relevant to that PR

Forbidden without an explicit human gate:

- direct push/force-push to the default/protected branch
- force-push over unrelated remote history
- create/delete releases or tags
- merge the final PR
- rerun, cancel, delete, enable, or disable Actions workflows
- change repository/org settings, permissions, environments, secrets, variables, webhooks, deploy keys, or branch protections
- modify GitHub Actions workflow/action definitions (`.github/workflows/**`, `.github/actions/**`) unless that change is the assigned scope; such a change always requires independent security review and a human gate before remote application/merge
- write to any repository other than the explicitly connected repository

### 6.5 Credential policy

Use repository-scoped, least-privilege credentials supplied by the sandbox/harness/connector.

Preferred order:

1. repository-scoped GitHub App/connector credentials
2. fine-grained repository-scoped token
3. another temporary credential with equivalent least privilege

Do not read or copy host SSH private keys, host Git credential stores, browser cookies, or global `gh` credentials into the sandbox.

Default GitHub capability should be read-only. Add write permissions only when the loop needs branch/PR mutation.

CI diagnosis normally needs read access only. Repository write credentials must not imply administration, secrets, or organization access.

Never print credential values in logs, status files, prompts, diffs, or error reports.

### 6.6 Network egress policy

Network access should be deny-by-default with an allowlist appropriate to the loop.

Typical allowed destinations may include:

- GitHub/API endpoints needed for the connected repository and CI observation
- package registries/proxies required to restore declared dependencies
- documented upstream endpoints required by an explicit live integration/smoke test

The agent must not use open-ended network access to upload repository contents, logs containing secrets, local files, or credentials to arbitrary services.

Adding an unfamiliar network destination during a loop requires a recorded reason. If it is not clearly required by the existing product/test contract, stop and request a human gate.

### 6.7 Sandbox escape / boundary violation

Any attempt or accidental action that:

- writes persistently outside the allowed local repository
- accesses unrelated host credentials or sensitive user files
- writes to an unapproved GitHub repository
- changes protected GitHub administration/security state
- obtains host-level privilege or control-socket access
- bypasses the configured network policy

is classified `EXECUTION_BOUNDARY_VIOLATION`.

On detection:

1. stop mutation immediately
2. preserve the command/action and affected path/resource as evidence without exposing secret values
3. do not retry the same action with broader permissions
4. mark the loop `NEEDS_HUMAN`
5. resume only after the boundary is restored and the human explicitly approves any required exception

## 7. Protected verification boundary

The agent must solve the task, not weaken the measurement of the task.

During a feature loop, the following are protected unless changing them is explicitly the loop's assigned scope:

- product contract
- architecture contract
- security requirements
- acceptance-criteria meaning or pass threshold
- ADR decisions
- required CI checks
- test runner configuration that determines pass/fail
- pre-existing regression tests relevant to the loop
- verifier scripts and hidden/adversarial checks

Rules:

1. The implementer may add tests.
2. The implementer may update tests when requirements legitimately changed, but that loop can no longer self-certify those changed checks. The test/criterion diff requires independent review and, when meaning or threshold changed, human approval.
3. Never delete, skip, quarantine, loosen, mock away, or special-case a failing required check merely to obtain green status.
4. Never catch and discard an error solely to satisfy a test if the documented behavior requires surfacing or handling it.
5. Never change production security from fail-closed to fail-open to make verification pass.
6. If a required test appears incorrect, classify `SPEC_OR_TEST_CONFLICT`; preserve the failing evidence and escalate rather than silently rewriting the test.
7. Any unexpected modification to protected verification assets is an integrity failure and prevents PASS.

## 8. Loop state machine

Every loop follows this state machine:

```text
CONTRACT
  -> BASELINE_VERIFY
  -> IMPLEMENT
  -> FOCUSED_VERIFY
       -> PASS -> REGRESSION_VERIFY
       -> FAIL -> CLASSIFY_FAILURE -> DIAGNOSE -> CORRECT -> FOCUSED_VERIFY
       -> BLOCKED -> BLOCKED
  -> SECURITY_RESOURCE_REVIEW
  -> SIMPLIFY
  -> REVERIFY_CHANGED_BEHAVIOR
  -> INDEPENDENT_REVIEW (when required)
       -> FAIL -> DIAGNOSE -> CORRECT -> verification path again
  -> HUMAN_GATE (when required)
       -> FAIL -> feedback becomes a new bounded corrective cycle
  -> FINAL_CLEAN_VERIFY
  -> PASS
```

A state transition must be supported by evidence. Do not jump from `IMPLEMENT` directly to `PASS`.

## 9. Baseline verification

Before changing code:

- run the smallest relevant pre-change checks when practical
- record existing failures separately from failures introduced by the loop
- record the working-tree state
- identify protected verification files and their current diff/hash state

A loop must not claim it introduced or fixed a failure without distinguishing it from baseline state.

## 10. Failure classification

Every failed verification must be classified before another code edit.

Use one of:

- `IMPLEMENTATION_FAILURE` - implementation violates expected behavior
- `REGRESSION_FAILURE` - previously passing behavior now fails
- `BUILD_OR_TYPE_FAILURE` - compile/type/lint gate fails
- `SECURITY_FAILURE` - security criterion or fail-closed invariant fails
- `RESOURCE_FAILURE` - leak, cleanup, race, lifecycle, or bound fails
- `TEST_FAILURE` - deterministic functional test fails
- `FLAKY_OR_NONDETERMINISTIC` - same unchanged state produces inconsistent result
- `ENVIRONMENT_BLOCKED` - missing OS/runtime/tool/hardware prevents required verification
- `UPSTREAM_BLOCKED` - external service prevents a required live check
- `SPEC_AMBIGUITY` - requirement is not precise enough to decide correct behavior
- `SPEC_OR_TEST_CONFLICT` - test/evaluator conflicts with the documented contract
- `ARCHITECTURE_CONFLICT` - compliant implementation requires changing an architectural decision
- `VERIFIER_INTEGRITY_FAILURE` - required check or evaluator was weakened, bypassed, or unexpectedly modified
- `EXECUTION_BOUNDARY_VIOLATION` - persistent write/access escaped the approved repo/GitHub/sandbox boundary
- `UNKNOWN_FAILURE` - failure cannot yet be interpreted

`UNKNOWN_FAILURE` permits investigation, not repeated blind edits.

## 11. Diagnostic correction loop

When a deterministic check fails:

1. capture the failing criterion/check, command, exit status, and smallest relevant error
2. compute a stable failure fingerprint using at least:
   - check/test ID
   - failure class
   - key error/assertion
   - relevant component/path
3. state the current root-cause hypothesis
4. gather new evidence before changing code
5. make the smallest change that tests the hypothesis
6. rerun the narrow failing check first
7. if it passes, run affected regression checks
8. record whether the failure fingerprint disappeared, changed, or remained identical

A retry is valid only when it has new evidence, a changed hypothesis, or a changed implementation that can plausibly affect the failure.

Repeating the same command against the same state is not a corrective iteration except for the bounded flakiness check in Section 13.

## 12. Stop-loss and convergence budgets

These are project defaults for a single loop. They are deliberately finite.

### 12.1 Hard convergence limits

- maximum total corrective iterations: **5**
- maximum repair attempts for the same stable failure fingerprint: **2**
- maximum full replans / materially different implementation approaches: **1**
- maximum unchanged reruns used only to diagnose possible flakiness: **2**
- default maximum agentic tool-use turns when the harness supports it: **30**

If any limit is reached without satisfying the gate, stop mutation and exit with an explicit non-PASS state.

### 12.2 No-progress rule

Stop early as `ABORTED_NON_CONVERGENCE` when **two consecutive corrective iterations** produce no meaningful progress, such as:

- identical failure fingerprint
- identical failing criterion set
- no reduction in regression count
- no new evidence
- repeated workaround attempts that do not address the diagnosed cause

Do not spend the remaining budget merely because it exists.

### 12.3 Regression rule

If a proposed correction increases the number or severity of required failing checks:

1. stop stacking more fixes on top
2. revert or isolate the last causal change when safe
3. re-establish the previous verified state
4. choose a new diagnosis/approach within the remaining replan budget

If the regression cannot be isolated, exit `NEEDS_HUMAN` or `NEEDS_ADR` as appropriate.

### 12.4 Cost/token/usage budget

When the harness exposes cost, token, turn, or quota telemetry, track it in the loop record.

Provider-specific limits are additional stop conditions; they do not replace the hard convergence limits above.

Project reserve policy when an exact **remaining-usage percentage** is available:

- at `<= 20%` remaining: do not start a new broad implementation approach; finish only a bounded verification/fix cycle that is likely to fit, otherwise preserve state and stop
- at `<= 10%` remaining: stop code mutation; preserve evidence, status, and handoff only

If remaining usage is not exposed, record `unavailable`. Never guess a percentage.

Token and monetary budgets may be configured per harness/model because accounting differs across providers. If unavailable, the finite corrective-iteration and turn limits remain mandatory.

Subagent/reviewer fan-out is also bounded: use only roles that add distinct evidence. Do not spawn additional agents merely to obtain more votes for the same conclusion.

## 13. Flaky and transient failures

### Transient tool/network failure

The exact operation may be retried at most twice when there is credible evidence that the failure is transient and no repository state changed.

If it still fails, classify `ENVIRONMENT_BLOCKED` or `UPSTREAM_BLOCKED`.

### Suspected flaky test

On an unchanged repository state:

- rerun at most twice
- if results disagree, classify `FLAKY_OR_NONDETERMINISTIC`
- do not choose the passing run and ignore the failing run
- a required flaky gate is not PASS until stabilized or explicitly moved behind a human-approved exception

## 14. Security gate in every loop

Before completion, ask:

- Did this add a new external input?
- Did this add a new network path?
- Did this add a new persistence field?
- Did this add a new log field?
- Did this expose new data to frontend?
- Did this add a new process/socket/file?
- Does it need cleanup?
- Can an attacker control this value?
- Could it become SSRF, XSS, command injection, path traversal, secret leakage, privilege expansion, or resource exhaustion?

If yes, add safeguards and verification in the same loop.

Any proposal to weaken a security policy, trust boundary, authentication/authorization expectation, network restriction, secret handling rule, or fail-closed behavior requires a human gate and normally an ADR before implementation continues.

## 15. Resource-lifecycle gate

For every created resource, define ownership and cleanup.

Examples:

- HTTP response -> close body
- goroutine -> cancellation and join/termination path
- ticker -> `Stop`
- socket -> `Close`
- MPV process -> wait/reap
- IPC endpoint -> delete/close
- proxy session -> expire/invalidate
- temporary file -> remove

A loop is incomplete if cleanup is implicit or deferred without an accepted criterion.

## 16. Testing and verification order

Use the cheapest trustworthy signal first, then widen only after it passes:

1. targeted unit/contract test
2. affected package/component tests
3. static checks / lint / typecheck / vet
4. integration or E2E test
5. race/resource/security checks when relevant
6. broader regression suite
7. live smoke check when required
8. platform-specific verification when required

At minimum, report exact commands and results.

Do not claim a test passed if it was not actually run.

If the environment prevents a required check:

- state exactly what could not run
- state why
- run the closest safe substitute when useful
- keep the affected criterion `BLOCKED` or `NEEDS_HUMAN`; do not mark it PASS

## 17. Code-quality and structure gate

Every loop that changes production code must pass this gate after focused behavior is working and before final independent acceptance.

Functional correctness is necessary but not sufficient. The changed code and its surrounding ownership boundary must remain maintainable, readable, and structurally coherent.

Review against `docs/02_ARCHITECTURE.md` and `QUAL-*` acceptance criteria:

- each changed file/type/function has a clear responsibility
- dependencies still point inward and concrete infrastructure does not leak into core
- existing ports remain cohesive and substitutable; no interface is widened merely for implementation convenience
- new behavior lives in the expected existing package/directory; new top-level/shared packages require a concrete justification
- no god object/file, generic `utils`/`helpers`/`common` dumping ground, duplicated policy/security logic, or avoidable circular dependency is introduced
- names communicate domain intent
- control flow and error handling are understandable without reconstructing hidden side effects
- exported API surface is no larger than required
- comments/documentation explain non-obvious intent and remain accurate
- dead code, temporary debug paths, stale TODOs, and obsolete scaffolding introduced by the loop are removed before PASS
- cleanup does not become unrelated refactoring or ceremonial abstraction

### 17.1 SOLID interpretation

Use SOLID pragmatically:

- SRP: cohesive reasons to change
- OCP: extend at real seams, especially adapters/policies, instead of provider conditionals in core
- LSP: port implementations preserve contract/error/cancellation/lifecycle/security semantics
- ISP: keep ports narrow and consumer-relevant
- DIP: core owns policy/abstractions and does not depend on infrastructure implementations

Do **not** create extra layers, interfaces, repositories, factories, managers, or use-case classes solely to make the code look more "SOLID" or "Clean Architecture". The project's minimal Ports & Adapters structure is the intended Clean Architecture form.

### 17.2 Quality review authority

For production-code changes, a dedicated fresh-context code-quality review is required even when no security/concurrency trigger exists. In Codex multi-agent mode, use a reviewer subagent that did not author the final changed code.

The reviewer returns findings classified as:

- `MUST_FIX`: correctness, architecture, responsibility, placement, readability, maintainability, or clear technical-debt regression that should block this loop
- `ADVISORY`: optional improvement that does not justify churn

Any unresolved `MUST_FIX` finding prevents PASS. Advisory findings do not authorize speculative refactoring.

## 18. Independent review gate

Independent AI review is required for every production-code loop as a code-quality review, and additional specialized review is required when any of these apply:

- security-sensitive code changed
- concurrency/process/IPC/resource lifecycle changed
- persistence migration or identity mapping changed
- public desktop binding/API contract changed
- a required test or verifier changed
- the implementation required more than one corrective iteration
- the loop is specifically marked hybrid in `docs/08_VERIFICATION_MATRIX.md`

Reviewer rules:

- use a fresh context where practical
- read the contract/spec and actual diff
- remain read-only during the review pass
- inspect evidence, not only the final summary
- actively search for criterion bypasses and test weakening
- return criterion-specific findings
- distinguish must-fix defects from optional suggestions

A must-fix review defect sends the loop back to diagnosis/correction and invalidates later PASS evidence until reverified.

## 19. Human gates

Human review is required before PASS when any of these apply:

1. the verification matrix marks a criterion `Human` or `Hybrid + Human final`
2. correctness depends on visual quality, interaction feel, clarity, accessibility judgment, or other subjective UX
3. product contract, architecture, security policy, acceptance meaning, or ADR must change
4. the agent proposes weakening/removing/skipping a required test or CI check
5. a new security/trust boundary or privilege is introduced
6. an irreversible/destructive migration or release action is proposed
7. a required gate remains flaky, ambiguous, or blocked and an exception is requested
8. non-convergence or budget stop requires a decision about the next approach
9. any request to expand persistent filesystem, GitHub-repository, credential, privilege, or network boundaries beyond the loop contract
10. final MVP/release acceptance

### Human review packet

Do not ask the human to infer what happened from the repository.

Present:

- loop goal and acceptance criteria
- plan/contract
- exact diff or concise changed-file summary
- deterministic verification evidence
- independent-review findings
- known issues and residual risk
- a short reproducible human procedure with explicit PASS/FAIL observations

### Human result

Use only:

- `HUMAN_PASS`
- `HUMAN_FAIL`
- `HUMAN_BLOCKED`
- `HUMAN_EXCEPTION_APPROVED`

A human exception is not a technical PASS. Record scope, reason, owner, date, and whether it expires. Critical security/architecture criteria cannot be counted as MVP PASS while covered only by an exception.

## 20. Human UX verification standard

For UI/UX criteria, human review must exercise the running application, not only read code or inspect a static screenshot.

At minimum when relevant:

- complete the intended workflow using keyboard only
- complete it using mouse
- verify visible focus and no keyboard trap
- verify loading/error/empty states
- resize to the supported small-desktop boundary
- verify text is not clipped/overlapped
- verify actions are understandable without knowledge of internal MPV/provider details
- confirm no unintended network-blocking behavior for cached/offline flows
- confirm the visual result remains minimal/calm and does not introduce excluded product patterns

Record specific observations, not `looks good`.

## 21. Simplify gate

After relevant behavior passes focused verification, review every changed code/configuration file for genuine clarity and maintainability improvements.

The simplify pass must:

- preserve behavior, errors, side effects, ordering, and security policy
- follow repository conventions
- avoid line-count optimization
- avoid unrelated refactors
- accept unchanged code when it is already clear
- rerun focused checks after each accepted simplification

Simplification cannot weaken tests or acceptance behavior.

## 22. Final clean verification

Before PASS:

1. ensure all implementation/review/simplify changes are complete
2. inspect the final diff for protected-verifier changes
3. verify the execution-boundary record shows no unauthorized host/repository/GitHub mutation
4. verify CI evidence, when required, was obtained from the connected repository without broadening permissions
5. run the required final deterministic gates on the final state
6. run required independent review on the final relevant diff
7. complete required human gates
8. verify no required criterion is `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, or `NOT_RUN`
9. update status/evidence only after verification

Any source/config change after final verification invalidates affected evidence and requires re-verification.

For important final acceptance, prefer verification from a clean checkout/worktree or otherwise clean reproducible environment so stale state cannot create a false PASS.

## 23. Criterion status model

Acceptance criteria use these statuses only:

- `NOT_RUN` - required verification has not been executed
- `PASS` - required verification passed with evidence for the current final state
- `FAIL` - reproducible evidence shows the criterion is not satisfied
- `BLOCKED` - required verification cannot currently execute because of environment/upstream dependency
- `NEEDS_HUMAN` - automated evidence cannot decide the criterion or a mandatory human gate is pending
- `NOT_APPLICABLE` - criterion truly does not apply; requires recorded rationale and must not be used to hide missing work

Do not use `probably pass`, `conditional pass`, or `good enough` as criterion states.

## 24. Loop exit states

A loop terminates with exactly one of:

- `PASS`
- `FAIL`
- `BLOCKED`
- `NEEDS_HUMAN`
- `NEEDS_ADR`
- `ABORTED_BUDGET`
- `ABORTED_NON_CONVERGENCE`

`PASS` requires all mandatory criteria and gates to be PASS for the final state.

`ABORTED_*`, `BLOCKED`, and `NEEDS_HUMAN` are successful control outcomes when they correctly prevent unsafe or wasteful continued autonomy. They are not implementation PASS.

## 25. Scope drift and ADR rule

If a loop reveals that the documented architecture or product/security contract is materially wrong:

1. stop feature implementation
2. preserve failing/conflicting evidence
3. write a proposed ADR or contract change
4. explain current decision, new evidence, proposed change, migration impact, security impact, and performance/resource impact
5. exit `NEEDS_ADR`
6. wait for explicit human acceptance before broad redesign

Minor implementation details do not require an ADR.

## 26. Dependency rule

Before adding a dependency, answer:

- Can the standard library solve it safely?
- Is the dependency actively maintained?
- Is it cross-platform if needed?
- Does it materially increase binary/runtime size?
- What transitive dependencies are added?
- Are there known vulnerabilities/licensing concerns?
- Can the boundary be isolated if replacement is needed?

A new runtime/native/security-sensitive dependency requires human approval before PASS.

Avoid npm packages for trivial helpers.

## 27. Performance rule

Do not micro-optimize Go interface dispatch.

Measure or reason about real bottlenecks:

- remote latency
- startup blocking
- SQLite queries
- cover decode
- WebView rendering
- MPV
- unbounded work
- repeated allocations in actual hot paths

Performance work must target measured or structurally credible bottlenecks and must define the measurement used to establish improvement.

## 28. AI anti-patterns prohibited

The agent must not:

- rewrite working modules merely for cleanliness
- introduce generic managers/factories without need
- build future plugin/source systems before MVP needs them
- create dozens of tiny packages
- make every function an interface
- put everything in `utils`
- duplicate security logic in adapters
- silently weaken fail-closed behavior to make tests pass
- replace typed errors with raw stack traces in UI
- put secrets in debug logs
- turn cached startup into network-blocking startup
- add code to a convenient file/package when another layer clearly owns the responsibility
- create or grow `utils`, `helpers`, `common`, `misc`, generic manager/factory/service dumping grounds
- hide mixed responsibilities inside a large file simply because tests pass
- duplicate an existing domain/security policy instead of using its owner
- broaden an interface only to make one implementation easier
- use `--dangerously-bypass-approvals-and-sandbox`, danger-full-access, or equivalent on the host to make the loop pass
- delete or weaken tests to obtain PASS
- special-case known test inputs instead of implementing the contract
- mock the component under test so completely that the real integration is no longer exercised
- edit acceptance criteria during a feature loop to match the implementation
- hide failing evidence from the final report
- continue retrying after a stop-loss trigger

## 29. Required loop sequence

Use this order unless an accepted ADR changes it:

1. Repository/contracts
2. Core models/ports
3. Contract tests
4. Secure HTTP
5. Anime1 catalog/search
6. Anime1 episodes
7. Anime1 resolver
8. Anime1 schedule
9. Anime1 adapter acceptance
10. Secure playback proxy
11. MPV process lifecycle
12. MPV IPC
13. Same-session switching
14. SQLite
15. Progress/history
16. Following
17. AniList
18. Bangumi
19. Metadata matching
20. Metadata content security
21. Wails binding layer
22. Svelte shell
23. Home
24. Search
25. Anime detail
26. Schedule UI
27. Following UI
28. History UI
29. Keyboard navigation
30. Cache-first refresh
31. Autoplay next
32. Settings
33. Security hardening
34. Resource-leak testing
35. Cross-platform validation
36. CI/release
37. Full acceptance

## 30. Loop output and evidence format

Every loop ends with a structured report containing:

### Result

One loop exit state from Section 23.

### Contract

- loop/scope
- baseline state
- acceptance criteria
- verifier types
- budgets

### Implemented

Concise list of actual changes.

### Files changed

Exact paths.

### Verification evidence

For every required check:

- criterion/check ID
- verifier type (`deterministic`, `AI-review`, `human`)
- exact command/procedure
- environment/platform
- result and exit code where applicable
- artifact/log reference where applicable
- final status

### Failure/correction history

For each failed cycle:

- failure fingerprint
- classification
- diagnosis/hypothesis
- corrective change
- outcome
- remaining budgets

### Integrity review

- protected verifier/spec changes: yes/no
- test weakening detected: yes/no
- acceptance semantics changed: yes/no

### Security/resource review

New trust boundaries/resources and their safeguards/cleanup.

### Independent review

Required/not required, reviewer result, must-fix findings.

### Human gate

Required/not required, exact procedure, result, unresolved observations.

### Known issues

Only real remaining issues.

### Next loop

Name the next intended loop, but do not implement it.

## 31. Definition of done for each loop

A loop is done as `PASS` only if:

- the loop contract was defined
- implementation is complete for the assigned scope
- relevant tests were added/updated without weakening the verifier
- required deterministic checks actually ran and passed
- all failures were resolved through bounded diagnostic correction rather than bypassed
- affected regressions pass
- changed code passed simplify review and was reverified
- security reviewed
- resource lifecycle reviewed
- required independent review passed
- required human gates passed
- all applicable `QUAL-*` code-quality criteria passed
- fresh-context code-quality review has no unresolved `MUST_FIX` findings
- Codex execution profile / multi-agent isolation requirements were respected
- protected verification integrity passed
- documentation/status was updated after verification
- no unrelated scope was added
- repository remains buildable at the expected level
- final evidence applies to the final repository state

## 32. External engineering basis

This project-specific protocol is informed by, but not mechanically copied from:

- Anthropic, *Building Effective AI Agents* and Agent SDK loop guidance: agents should use environmental ground truth, stopping conditions, evaluator/optimizer separation, and bounded turns/cost.
  - https://www.anthropic.com/engineering/building-effective-agents
  - https://code.claude.com/docs/en/agent-sdk/agent-loop
- Anthropic, *Harness design for long-running application development*: generator/evaluator separation, sprint contracts, and hard criterion thresholds help counter optimistic self-evaluation.
  - https://www.anthropic.com/engineering/harness-design-long-running-apps
- Microsoft Agent Framework, *Agent Looping*: autonomous loops should always be bounded because completion predicates and evaluators can fail or stall.
  - https://learn.microsoft.com/en-us/agent-framework/agents/looping
- OpenAI evaluation guidance: define measurable success, use deterministic checks where possible, use structured graders where rules fall short, track efficiency/thrashing, and calibrate automated graders against human judgment. OpenAI also explicitly documents reward hacking by coding agents that edit tests or disable checks.
  - https://developers.openai.com/api/docs/guides/evaluation-best-practices
  - https://developers.openai.com/blog/eval-skills
  - https://openai.com/index/how-we-monitor-internal-coding-agents-misalignment/
- NIST SSDF: define criteria for software/security checks and preserve evidence/approvals in the development workflow.
  - https://csrc.nist.gov/projects/ssdf
- OpenSSF OSPS Baseline: automated checks should pass before accepting changes, automated tests belong in CI, and human review is a distinct approval control.
  - https://baseline.openssf.org/
- OWASP secure code review guidance: automated verification and manual review are complementary, especially for security logic and context-specific flaws.
  - https://cheatsheetseries.owasp.org/cheatsheets/Secure_Code_Review_Cheat_Sheet.html
- METR evaluation-integrity research: capable agents may reward-hack by exploiting scorer/test infrastructure, so protected and independent verification boundaries are necessary.
  - https://metr.org/research/
- OpenAI Codex safety/configuration guidance: sandbox and approvals form separate controls; workspace-write constrains mutation; network access and GitHub/tool access should be explicitly scoped; Codex App supports worktrees for parallel agents; `AGENTS.md` is the repository instruction mechanism.
  - https://openai.com/index/running-codex-safely/
  - https://openai.com/index/building-codex-windows-sandbox/
  - https://openai.com/index/introducing-the-codex-app/
  - https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json
  - https://github.com/openai/codex/blob/main/codex-rs/core/src/agents_md.rs

The numerical retry/usage thresholds in this document are **project stop-loss defaults**, not universal industry constants. Change them only deliberately and record why.
## 33. Durable agent-state checkpoint

The active root Codex agent maintains `docs/AGENT_HANDOFF.md` under the policy in `docs/10_DURABLE_AGENT_STATE.md`.

Purpose:

- resume after an intentional stop or lost session
- survive context compaction without depending on compacted chat history
- avoid repeatedly rereading the full repository and every engineering document
- preserve unresolved failures, next actions, and verification state across root-agent replacement

The handoff is intentionally small:

- target `<= 4 KiB`
- hard cap `<= 8 KiB` and `<= 160 lines`
- references evidence instead of embedding logs/diffs/docs
- contains no secrets or chain-of-thought

The root checkpoints at recoverability boundaries, including after stable contract creation, material verified implementation, failure diagnosis/correction, worker integration, human decisions, and before intentional pause/handoff or any loop exit.

Workers/subagents do not concurrently edit the global handoff. They return structured results to the root, which decides what materially belongs in durable state.

On every new root session, reconcile the handoff with current Git/test/CI truth before mutation. If they disagree, current repository/verifier evidence wins and the handoff must be refreshed.

A loop cannot reach `PASS` if required durable-state evidence is stale, over cap, contradictory, or missing at a session/loop handoff boundary.
