<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop Workflow

This file defines how any AI agent or human works on this repository. It is tool-neutral. It names roles, not products.

A loop is one coherent phase or vertical slice. A loop is done only when the external gates say so. An agent saying "done" is not a gate.

Related files: [Acceptance criteria and verification](06_ACCEPTANCE.md), [TypeSafe use](08_TYPESAFE.md), [Roadmap](04_ROADMAP.md), [Security](03_SECURITY.md), [Architecture](02_ARCHITECTURE.md). Older process history is in `archive/`.

## 1. Session start

Do these before changing anything.

1. Read the root `AGENTS.md`.
2. Read `docs/AGENT_HANDOFF.md`.
3. Run `git status --short --branch`, `git log -5 --oneline` and `git diff --stat`.
4. Compare branch, HEAD and dirty paths with the handoff. If they differ, Git wins; inspect the real diff and refresh the handoff.
5. Read only the docs, headings, source files and tests the handoff references and the next action needs.
6. Never reset or discard working-tree changes only because the handoff does not mention them.

Read more (relevant sections first, whole files only if needed) when any of these is true:

- A new loop starts, or `AGENTS.md` or this workflow changed.
- The handoff is missing, malformed, over its size cap, or stale.
- Branch, HEAD or working tree contradict the handoff.
- Architecture, security or acceptance intent is unclear.
- A failure appears outside the handoff's known scope.

Start a new session after `AGENTS.md` or this workflow changes. Do not trust old chat history as project state.

## 2. Roles

| Role | May | May not |
| --- | --- | --- |
| Root agent | Own the loop contract, decomposition, integration, budgets, final verification, handoff and human packet. Edit code. | Delegate the final decision. Let workers edit the handoff. |
| Explorer | Search and read the repository, report paths and facts. | Edit anything. |
| Librarian | Read official external documentation and report the relevant facts with sources. | Edit anything. Decide design. |
| Oracle | Review a diff, plan or failure independently; debug by reading and running read-only checks. | Edit anything during a review. Approve code it wrote or repaired. |
| Fixer | Implement a bounded task with a stated write set, add tests, run allowed checks, report files changed and results. | Touch files outside its write set. Edit the global handoff or protected verification assets. Spawn agents. |
| Designer | Do the same as a fixer for UI and accessibility work, and review UI against the human UX procedure. | Same limits as a fixer. |

Rules:

- One root agent owns the contract and the result.
- Parallel code-changing workers get disjoint write sets and, when possible, separate worktrees or workspaces. Never run overlapping writers in one working tree.
- A worktree is isolation, not acceptance. Verify the combined result after integration; separate green worktrees do not make the loop pass.
- Reviewers use fresh context and are read-only. A reviewer that repaired a finding cannot be the final reviewer of that state.
- Delegate only independent, bounded work that saves usage (read-heavy search, tests, triage, review). Do not add agents to get more votes on the same question.
- No nested agents. Only the root agent starts agents.
- Workers return: task and result, files changed, checks run and results, open findings, assumptions and risks.
- A deterministic failure is never overridden by a reviewer. A required human gate is never replaced by an agent.

## 3. Loop contract

The contract lives in `docs/loops/LOOPNN.md` (contract section), where NN is the loop number. Write it there before changing code. A criterion with no verification method or human gate is not ready for autonomous work; clarify it first.

```yaml
loop: <number>
scope: <one coherent slice>
baseline_commit: <sha or working-tree description>
acceptance_ids: [FUNC-001, QUAL-001]
verification:
  commands: [<exact commands or CI job names>]
  environment: <local | github-windows-ci>
  review_required: true | false      # always true for production code (code-quality review)
  human_gate: true | false
protected_assets: [<criteria, specs, tests, CI files this loop must not weaken>]
budgets:
  corrective_iterations: 5
  repairs_per_fingerprint: 2
  replans: 1
  unchanged_flaky_reruns: 2
  agent_turns: 30
  remaining_usage: <value | unavailable>
```

Keep one coherent slice per loop. Do not bundle unrelated work or refactor while adding a feature.

## 4. Loop steps

Each step names the condition that lets the loop move on. Do not jump from implementation to PASS.

1. Contract. Exit: contract written, every criterion has a verifier or human gate, handoff checkpointed.
2. Baseline check. Run the smallest relevant checks before editing. Record existing failures separately from new ones; record protected-asset state. Exit: baseline recorded.
3. Implement. Add tests that encode the intended behavior with the code. Exit: change complete for the slice.
4. Focused check. Run the narrowest test or check for the changed behavior first. Exit: it passes. If it fails, go to step 5.
5. Failure handling. Follow section 5. Exit: failing check passes with the smallest causal fix, or a stop state is reached.
6. Security and resource check. Answer the questions below. Exit: each "yes" has a safeguard and a verification in this loop.
7. Simplify. Review every changed file for clarity without changing behavior, errors, ordering or security policy; no unrelated refactor; rerun focused checks after each accepted change. Exit: focused checks pass again.
8. Regression. Run affected package tests, then static checks, then wider suites. Exit: no required check fails.
9. Independent review. Fresh-context, read-only. Production code always gets a code-quality review (`QUAL-001..012`). Add security, concurrency and lifecycle, performance or platform reviewers when triggered (see [Acceptance](06_ACCEPTANCE.md)). Exit: no unresolved `MUST_FIX` finding. A must-fix finding returns to step 5 and invalidates later evidence.
10. Human gate, when required (section 9). Exit: `HUMAN_PASS`. A fail becomes a new bounded corrective cycle.
11. Final verification. Run all required gates on the final state; check the final diff for protected-asset changes; confirm no required criterion is `FAIL`, `BLOCKED`, `NEEDS_HUMAN` or `NOT_RUN`. Any source or config change after this invalidates the affected evidence. Exit: all gates pass on the final state.
12. Commit and handoff. Update status and evidence only after verification. Evidence (raw TypeSafe answers, failure and human records, CI excerpts) lives in `artifacts/evidence/<loop>/`, which Git ignores; docs link to it. After owner approval, commit the verified logical change before unrelated work. Push only with owner approval; if CI is required, close only after green CI. Checkpoint the handoff. Exit: loop state is one exit state below.

TypeSafe checkpoints (templates in [08_TYPESAFE.md](08_TYPESAFE.md)): `scope-check` before steps 9 and 11; `failure-class` in step 5; `ci-log` for every failed CI run; `review-triage` for every step 9 finding; `owner-report` for every step 10 reply; `doc-quality` for every new or rewritten doc. Answers flagged `REVIEW` are adjudicated against the evidence before acting.

Security and resource questions for step 6:

- New external input, network path, persisted field, logged field, or data exposed to the UI?
- New process, socket, file or library? Who owns it and how is it cleaned up (close body, stop ticker, cancel and join goroutine, free native handle, delete temp file, expire session)?
- Can an attacker control the value? Could it become SSRF, path traversal, command injection, secret leakage, privilege expansion or resource exhaustion?

Any proposal to weaken a security policy, trust boundary or fail-closed behavior needs owner approval and normally an ADR first.

Code-quality gate (step 9) checks against [Architecture](02_ARCHITECTURE.md): clear responsibility per file, type and function; dependencies point inward; narrow ports; code in the narrowest correct package; no generic `utils`, `helpers`, `common` or `misc`; no duplicated policy; no dead or debug scaffolding; no larger public surface than needed. SOLID is applied pragmatically. Do not add layers or interfaces only to look clean.

Criterion statuses: `NOT_RUN`, `PASS`, `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, `NOT_APPLICABLE` (needs a recorded reason). No "probably pass".

Loop exit states (exactly one): `PASS`, `FAIL`, `BLOCKED`, `NEEDS_HUMAN`, `NEEDS_ADR`, `ABORTED_BUDGET`, `ABORTED_NON_CONVERGENCE`. Stop states are correct outcomes when they prevent unsafe or wasteful work; they are not PASS.

If a loop shows the architecture, product or security contract is wrong: stop feature work, keep the conflicting evidence, write a proposed ADR (current decision, new evidence, proposed change, migration, security and resource impact), exit `NEEDS_ADR`, and wait for owner acceptance.

Final loop report contains: result; contract; what changed; files changed; verification evidence per check (ID, verifier type, exact command, environment, exit code, final status); failure and correction history; integrity review (protected assets changed, tests weakened, acceptance meaning changed); security and resource review; independent review result; human gate result; known issues; next loop (named, not started).

## 5. Failure handling

Classify every failed check before the next edit. Use one class:

- `IMPLEMENTATION_FAILURE`: behavior differs from what is expected.
- `REGRESSION_FAILURE`: previously passing behavior now fails.
- `BUILD_OR_TYPE_FAILURE`: compile, type or lint gate fails.
- `SECURITY_FAILURE`: a security criterion or fail-closed rule fails.
- `RESOURCE_FAILURE`: leak, cleanup, race, lifecycle or bound fails.
- `TEST_FAILURE`: a deterministic functional test fails.
- `FLAKY_OR_NONDETERMINISTIC`: the same unchanged state gives different results.
- `ENVIRONMENT_BLOCKED`: no approved capable environment, tool or hardware for the check.
- `SANDBOX_INFRA_FAILURE`: the sandbox cannot start a minimal process; the repository verifier did not run.
- `SANDBOX_CAPABILITY_LIMITATION`: the sandbox runs, but blocks a capability the verifier needs. `SANDBOX_CHILD_PROCESS_LIMITATION` is the accepted label for child-process spawn or fork blocks.
- `UPSTREAM_BLOCKED`: an external service prevents a required live check.
- `SPEC_AMBIGUITY`: the requirement is too vague to decide correct behavior.
- `SPEC_OR_TEST_CONFLICT`: a test conflicts with the documented contract.
- `ARCHITECTURE_CONFLICT`: a correct fix needs an architecture change.
- `VERIFIER_INTEGRITY_FAILURE`: a required check was weakened, bypassed or unexpectedly changed.
- `EXECUTION_BOUNDARY_VIOLATION`: a write or access escaped the allowed boundary (section 7).
- `UNKNOWN_FAILURE`: not yet understood. Investigate; do not edit blindly.

Classification and diagnosis are semantic judgments. Make them through TypeSafe as described in [08_TYPESAFE.md](08_TYPESAFE.md). Keep the exact facts (command, exit code, error text, counters) in deterministic tooling. If TypeSafe is unavailable (no key, `bws` failure, API outage), report `ENVIRONMENT_BLOCKED` for that judgment step and tell the owner; do not silently replace it with free-form judgment.

Correction cycle:

1. Capture the failing check, command, exit status and the smallest relevant error.
2. Compute a fingerprint: check or test ID + failure class + key error or assertion + component or path.
3. State one root-cause hypothesis. Gather new evidence before editing.
4. Make the smallest change that tests the hypothesis.
5. Rerun the narrow failing check, then affected regression checks.
6. Record whether the fingerprint disappeared, changed or stayed the same.

A retry needs new evidence, a changed hypothesis or a changed implementation. Rerunning the same command on the same state is not a corrective iteration, except the flaky check below.

Budgets (finite, per loop):

| Limit | Value |
| --- | --- |
| Corrective iterations | 5 |
| Repairs per fingerprint | 2 |
| Replans (materially different approach) | 1 |
| Unchanged reruns to test flakiness | 2 |
| Agent turns, when the tool can enforce it | 30 |

- Stop as `ABORTED_NON_CONVERGENCE` after 2 consecutive corrective iterations with no progress (same fingerprint, same failing set, no new evidence, workarounds that miss the cause).
- If a fix raises the number or severity of failing required checks, stop stacking fixes. Revert or isolate the last change if safe, restore the last verified state, and choose a new approach within the replan budget. If it cannot be isolated, exit `NEEDS_HUMAN` or `NEEDS_ADR`.
- Transient tool or network failure: retry the same operation at most 2 times, only with credible transient evidence and no repository change. Then classify `ENVIRONMENT_BLOCKED` or `UPSTREAM_BLOCKED`.
- Suspected flaky test: rerun at most 2 times on unchanged state. If results disagree, classify `FLAKY_OR_NONDETERMINISTIC`. Never pick the passing run. A required flaky gate is not PASS until stabilized or covered by an owner-approved exception.
- When remaining usage is exposed as an exact percentage: at 20% or less, start no new broad approach; at 10% or less, stop code changes and preserve evidence and handoff only. If not exposed, record `unavailable`; never guess.
- On any limit reached, stop changing code and exit with an explicit non-PASS state.

## 6. Protected verification assets and anti-weakening

Protected unless changing them is the loop's assigned scope: product contract, architecture, security requirements, acceptance criteria meaning and thresholds, ADR decisions, required CI checks, test runner configuration, regression tests relevant to the loop, verifier scripts.

Rules:

1. A loop may add tests.
2. A loop may update a test when requirements truly changed, but then it cannot self-certify that test. The test or criterion diff needs independent review, and owner approval if meaning or threshold changed.
3. Never delete, skip, quarantine, loosen, mock away or special-case a failing required check to get green.
4. Never swallow an error only to satisfy a test when the documented behavior is to surface or handle it.
5. Never change security from fail-closed to fail-open to pass verification.
6. A required test that looks wrong is `SPEC_OR_TEST_CONFLICT`: keep the failing evidence and escalate.
7. Any unexpected change to a protected asset is `VERIFIER_INTEGRITY_FAILURE` and blocks PASS.
8. Do not edit acceptance criteria in a feature loop to match the implementation.

## 7. Boundaries and approvals

Persistent writes are allowed only in this repository (working tree and its Git metadata). Sandbox-owned temporary and cache directories are allowed because they are disposable.

Not allowed:

- Writes outside the repository: home directory, dotfiles, editor or AI-tool config, SSH, credential stores, keychains, browser profiles, global Git or GitHub CLI config, OS settings, registry, services, scheduled tasks, other repositories, synced folders.
- `sudo`, administrator rights, global package installs, host service installs, or access to host container control sockets.
- Full-access or sandbox-bypass modes, or answering a sandbox denial by widening permissions.
- Reading or copying host credentials into the working environment. Never print secret values in logs, status files, prompts, diffs or reports.
- A new network destination the product or test contract does not already require.

Scoped exception (owner decision 2026-10-13): reading the single secret `TYPESAFE_API_KEY` from Bitwarden Secrets Manager (via `BWS_ACCESS_TOKEN`, into the child process only, never written or printed) and calling `https://api.typesafe.ai` are approved for development judgments, as described in [08_TYPESAFE.md](08_TYPESAFE.md). Nothing else from the credential store may be read.

A missing tool is `ENVIRONMENT_BLOCKED`. Do not repair the host.

Needs explicit owner approval before it happens:

- Commit, push or any remote write (default target is `main`; use a branch or pull request only if repository protection requires it, the owner asks, or the change is high risk or incompatible).
- Edits to `.github/workflows/**` or `.github/actions/**` (exact scope approved first; independent security review afterwards; local permission does not imply commit, push or rerun).
- Rerunning, cancelling or deleting CI workflows; repository settings, secrets, variables, environments, branch protection, webhooks, deploy keys; releases and tags; force-push.
- A new runtime, native or security-sensitive dependency. Before asking, answer: can the standard library do it; is it maintained; what does it add in size and transitive dependencies; known vulnerabilities or license concerns; can the boundary be isolated.
- A new security or trust boundary or privilege.
- An irreversible or destructive migration or release action.
- Any change to product contract, architecture, security policy, acceptance criteria meaning or ADRs.
- Weakening, removing or skipping a required test or CI check.
- Treating a host diagnostic run as authoritative evidence.
- Exceptions for blocked, flaky or failed gates; strategy decisions after a budget stop; final release acceptance.

Allowed without further approval: read-only observation of this repository's CI (run list, checks, failed-step logs, build artifacts) with `gh` or an equivalent API.

Boundary violation: if any action writes outside the allowed boundary, reaches unrelated credentials, writes to another repository, changes protected remote state, or bypasses network policy, classify `EXECUTION_BOUNDARY_VIOLATION`. Stop changes at once, record the action and the affected path without secret values, do not retry with broader access, mark the loop `NEEDS_HUMAN`, and resume only after the owner restores the boundary and approves any exception.

## 8. Verification environments

Use the first capable environment:

1. Local sandbox or local machine inside the repository boundary.
2. GitHub-hosted Windows runner for this repository. This is the authoritative verifier for native Windows tests.
3. Another isolated CI environment the owner approves.

Known blocker: the local native cgo build fails. Do not retry it. Native Windows tests run only in GitHub Windows CI. Local runs still cover pure Go packages and static checks.

A host run outside the approved boundary is diagnostic only. It can help tell a repository failure from an environment limit, but it never counts as PASS.

CI evidence counts only when all of these hold:

- It is this repository's workflow, on the exact commit being verified.
- The verifier command and criterion meaning are unchanged; no test, assertion, threshold, repetition count or failure behavior was weakened.
- Dependencies are declared and pinned or locked where practical.
- The workflow token has least privilege; no new secret was added only to run the check.
- Workflow, run ID, job, commit SHA, runner label and conclusion are recorded.
- The runner OS can prove the criterion.

A Linux or browser CI check cannot replace a criterion that needs native Windows behavior, real libmpv playback, or human judgment. A real CI failure is real evidence; classify it normally. A passing local diagnostic does not cancel it.

Sandbox failure classes (see section 5) are not repository failures:

- `SANDBOX_INFRA_FAILURE`: cannot start a minimal process.
- `SANDBOX_CAPABILITY_LIMITATION` / `SANDBOX_CHILD_PROCESS_LIMITATION`: ordinary commands run but the verifier's capability (for example child-process spawn) is blocked.

Circuit breaker for a stable environment fingerprint:

1. Capture the exact command, error and environment.
2. Run at most one minimal targeted check, only if it gives new evidence.
3. After the same fingerprint is confirmed, stop equivalent retries across shells, paths, agents or wrappers.
4. Do not change production code for a failure that never reached the repository verifier.
5. Never widen privileges.
6. Checkpoint the handoff and move to the next approved environment.

A retry is valid only after a material change: tool or runtime version, owner-approved configuration change, or new diagnostic evidence.

Record fallback verification as:

```yaml
verification_environment:
  local: {result: BLOCKED, classification: <class>, fingerprint: "<stable error>"}
  fallback: {type: github_windows_ci, workflow: "<path>", run_id: "<id>", job: "<job>", commit: "<sha>", runner: "<label>", result: PASS | FAIL}
  host_diagnostic: {performed: true | false, authoritative: false}
```

## 9. Human gate packet

A human gate is required when [Acceptance](06_ACCEPTANCE.md) marks a criterion `H` or `YH`, when correctness depends on subjective UX, or when section 7 lists an approval. Finish deterministic gates and fix known must-fix review findings first, unless the gate exists to resolve ambiguity or non-convergence.

The packet contains:

- Goal and criteria IDs.
- Exact build or ZIP path, already prepared and launchable. The owner must not hunt for it. Include build identity (commit) and startup steps.
- Numbered steps using only visible actions (click, type, press a key, watch). Never ask for internal IDs, database or AppData edits, fixtures, adapters, IPC or repository internals.
- Expected result for each step, as an observable outcome.
- Deterministic evidence summary, review findings, known issues and residual risk.

Allowed results: `HUMAN_PASS`, `HUMAN_FAIL`, `HUMAN_BLOCKED`, `HUMAN_EXCEPTION_APPROVED`. An exception is not a technical PASS; record scope, reason, owner, date and expiry. Critical security and architecture criteria cannot count as PASS through an exception alone. Record specific observations, not "looks good". The UX procedure and record template are in [Acceptance](06_ACCEPTANCE.md).

## 10. Durable state

`docs/AGENT_HANDOFF.md` is a small recovery index for the active loop. It is not a specification. Memory layers, from highest authority: owner instruction; `AGENTS.md` and numbered docs and ADRs; verifier results and current Git and CI state; `docs/STATUS.md` and retained evidence; the handoff; old chat summaries.

Size: target 4 KiB or less. Hard limit 8 KiB and 160 lines. Check with `wc -c` and `wc -l`. Over the limit is a process failure: remove duplicates and history, move evidence to status, promote decisions to an ADR, keep only current state.

Put in the handoff:

- Instruction version (field `instruction_version`, must equal the version in `AGENTS.md`; a mismatch means the handoff is stale), current loop and loop state, branch and last observed HEAD.
- Loop goal and acceptance IDs.
- Work done and in progress; unresolved failures with fingerprints.
- Ordered next actions; relevant paths; last focused and regression results.
- Pending human gate, ADR or environment blocker; budget counters.
- References to the exact docs, headings and evidence needed next.
- Short `do_not_repeat` notes for disproven approaches.

Keep out: reasoning transcripts, full logs, CI output, diffs or source copies, anything easy to get from `git`, tests or CI, secrets, cookies, tokens or authenticated URLs, speculative ideas, copied doc text. Anything that needs lasting force goes into an ADR, criterion, test, doc or code comment.

Checkpoint (root agent only) when:

- the loop contract becomes stable;
- a material slice is done and its focused check has run;
- a failure is classified and a direction chosen, or a correction changes the diagnosis;
- worker results are integrated;
- an owner decision changes the allowed path;
- the loop enters `BLOCKED`, `NEEDS_HUMAN`, `NEEDS_ADR`, `ABORTED_*` or `PASS`;
- before pausing, handing off, a long operation, or asking the owner for a gate.

Do not checkpoint every command or small edit. Workers never edit the global handoff; the root decides what is material.

At loop PASS: write final status and evidence, promote lasting decisions, replace the handoff with a compact next-loop handoff or `NO_ACTIVE_LOOP`, and drop stale failure notes. A loop cannot reach `PASS` if the handoff is stale, over its cap, or contradicts Git at a session or loop boundary.

Authority on conflict: Git, tests and CI beat the handoff; the handoff never overrides the contract.

## 11. Anti-patterns

Do not:

- rewrite working code for taste, or refactor unrelated code;
- add managers, factories, plugin systems or interfaces without a present need; split into many tiny packages; create `utils`, `helpers`, `common` or `misc`;
- put code in a convenient file when another layer owns the responsibility, or hide mixed responsibilities in a big file because tests pass;
- duplicate security or domain policy outside its owner, or broaden an interface to ease one implementation;
- weaken fail-closed behavior, delete, skip or loosen tests, special-case test inputs, or mock the component under test so far that the real integration is no longer exercised;
- change acceptance criteria to fit the implementation, or hide failing evidence in a report;
- show raw stack traces in the UI, log secrets, or make cached startup wait on the network;
- keep retrying after a stop-loss trigger;
- use a sandbox bypass or widen permissions to make a loop pass;
- optimize without a measured or structurally credible bottleneck and a stated measurement (remote latency, startup blocking, SQLite queries, cover decode, rendering, playback, unbounded work).
