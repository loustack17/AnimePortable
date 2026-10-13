<!-- SPDX-License-Identifier: MPL-2.0 -->

# Retrospective Baseline Audit — Loops 01–22

Protocol version: **v2.4.1**

This document defines the one-time baseline audit required after Loop 23 reaches PASS and before Loop 24 begins.

The audit verifies the **current implementation produced by historical Loops 01–22** against the current product, architecture, security, quality, and behavioral contracts. It does not replay implementation history and does not rewrite working code merely because older loops predate Protocol v2.4.1.

> Numbering note: the runbook's Loops 01–22 correspond to implementation-plan Phases 0–21. Loop 23 is Phase 22 — Home UI.

## 1. Purpose

The retrospective audit exists to detect:

- latent bugs not covered by the old verification process
- architecture or dependency-direction drift
- security boundary mistakes
- resource-lifecycle, cancellation, concurrency, and cleanup defects
- missing regression coverage for already-supported behavior
- maintainability/readability/file-placement problems that materially violate `QUAL-*`
- stale assumptions that later code made unsafe or incorrect

The audit must not become an unlimited cleanup/refactor project.

## 2. External engineering basis

The protocol is intentionally aligned with current public guidance:

- OWASP Secure Code Review distinguishes **baseline review** from diff-based review and lists legacy-system onboarding / major releases as baseline-review use cases.
- NIST SSDF requires defined security checks, testing, documented results, regression tests for known vulnerabilities, and root-cause remediation.
- OpenSSF OSPS Baseline separates automated test/status checks from human approval and expects CI test evidence.
- Google Engineering Practices expects reviewers to inspect design, functionality, complexity, tests, readability, maintainability, and code health without blocking progress for perfection.
- Current Codex documentation recommends parallel subagents primarily for bounded/read-heavy exploration, tests, triage, and review; parallel write-heavy work needs tighter coordination.

Reference URLs are listed in Section 17.

## 3. Activation point

The audit starts only after:

1. Loop 23 — Home has reached Protocol v2.4.1 `PASS`.
2. `docs/IMPLEMENTATION_STATUS.md` records the Loop 23 final evidence.
3. `docs/AGENT_HANDOFF.md` has been checkpointed.
4. No unresolved Loop 23 human gate remains.

Then:

```text
Loop 23 PASS
  -> RETRO BASELINE AUDIT (RA-01 .. RA-06)
  -> RETRO_BASELINE_PASS
  -> Loop 24
```

Do not start Loop 24 while the audit result is incomplete, `REPAIR_REQUIRED`, `NEEDS_HUMAN`, or `BLOCKED`, unless the human owner explicitly records an exception. An exception does not convert failed technical criteria into PASS.

## 4. Core rule: verify, do not replay

For historical Loops 01–22:

- inspect current code and current behavior
- reuse valid existing evidence when it still proves the current state
- rerun missing or stale verification
- add tests when a real coverage gap prevents confidence
- reopen implementation only for a confirmed defect or blocking criterion failure

The following are **not defects by themselves**:

- old loops lack v2.4+ report formatting
- old loops lack a fresh-context reviewer record
- historical token/turn counts are unavailable
- old execution did not use today's multi-agent/worktree process
- an old loop has no `AGENT_HANDOFF.md` entry

Historical process-only evidence that cannot be reconstructed is `NOT_APPLICABLE_PRE_V2_4`, not FAIL.

## 5. Audit batches

Audit in bounded batches so context stays relevant.

| Batch | Historical loops | Primary scope | Risk |
| --- | --- | --- | --- |
| RA-01 Foundation | 01–04 | repository/contracts, core ports/models, contract tests, secure HTTP | High |
| RA-02 Anime source | 05–09 | catalog/search, episodes, resolver, schedule, source acceptance | High |
| RA-03 Playback | 10–13 | proxy, MPV lifecycle, IPC, same-session switching | Critical |
| RA-04 Persistence/state | 14–16 | SQLite, progress/history, following | Critical |
| RA-05 Metadata | 17–20 | AniList, Bangumi, matching, content security | High |
| RA-06 Desktop boundary | 21–22 | Wails bindings, Svelte shell | High |

Order is RA-01 -> RA-06 by default. The root may audit a Critical batch earlier when current evidence shows a concrete high-risk reason, but record the reason and preserve batch completeness.

## 6. Audit state machine

Each batch uses:

```text
INVENTORY
 -> CRITERIA_MAP
 -> EXISTING_EVIDENCE_REVIEW
 -> DETERMINISTIC_VERIFY
 -> GAP_ANALYSIS
 -> ADVERSARIAL_REVIEW
 -> FINDINGS_TRIAGE
 -> [REPAIR_REQUIRED -> DIAGNOSE -> CORRECT -> FOCUSED_VERIFY -> REGRESSION_VERIFY]
 -> INDEPENDENT_REVIEW
 -> HUMAN_GATE (only when required)
 -> BATCH_FINAL_VERIFY
 -> LEGACY_VERIFIED
```

A batch may exit as:

- `LEGACY_VERIFIED`
- `REPAIR_REQUIRED`
- `NEEDS_HUMAN`
- `BLOCKED`
- `ABORTED_BUDGET`
- `ABORTED_NON_CONVERGENCE`

`LEGACY_VERIFIED` means the **current implementation** passes applicable current criteria. It does not claim the historical execution process complied with the current protocol.

## 7. Criteria mapping

For each batch:

1. map historical loop deliverables to current `ARCH-*`, functional, security, privacy, resource, performance, platform, CI, and `QUAL-*` criteria that actually apply
2. map current tests/integration checks/smokes/CI evidence to those criteria
3. identify evidence gaps
4. identify criteria that require current human/runtime verification
5. mark historical-only process criteria `NOT_APPLICABLE_PRE_V2_4`

Do not broaden the audit into unrelated future-loop criteria.

## 8. Risk-based verification

Use the smallest evidence set that actually proves the criterion, then deepen where risk warrants it.

High-risk components require more than happy-path tests. As applicable inspect/test:

- malformed/untrusted external input
- cancellation/timeouts
- concurrency/race behavior
- cleanup/close/reap semantics
- repeated-cycle resource behavior
- fail-closed behavior
- secret/log redaction
- persistence corruption/migration rollback
- stale/cached state
- cross-component error propagation
- contract substitutability
- boundary exposure to frontend/process/network/filesystem

A historical PASS statement is not evidence if the current implementation has materially changed since that evidence was produced.


### 8.1 Execution-environment fallback during retrospective audit

A retrospective batch is not `REPAIR_REQUIRED` merely because the local Codex sandbox cannot execute one of its verifiers.

When a stable sandbox/runtime limitation blocks a required deterministic check:

1. classify it according to `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`
2. stop equivalent retries after the failure fingerprint is confirmed
3. run the unchanged verifier in an approved isolated CI environment when that environment can prove the criterion
4. record the CI run/commit evidence in `docs/RETRO_AUDIT_STATUS.md`
5. create a code finding only if the verifier fails in a capable approved environment

Unsandboxed host diagnostics may help classify the problem but are never the sole `LEGACY_VERIFIED` evidence.

Workflow/action changes needed only to make a verifier portable remain human-gated and must not weaken the historical acceptance meaning.

## 9. Finding model

Every confirmed issue receives a stable finding ID:

```yaml
id: RA-03-F001
batch: RA-03
origin_loop: 12
severity: HIGH
type: RESOURCE_FAILURE
criteria: [RES-004, IPC-006]
status: OPEN
evidence:
  - "<test/reproduction/file reference>"
root_cause: null
repair: null
verification: null
```

Finding severities:

- `CRITICAL`: exploitable security/data-loss/release-blocking safety issue
- `HIGH`: material correctness/security/resource/architecture failure
- `MEDIUM`: real defect or blocking maintainability/quality issue with contained impact
- `LOW`: real but non-blocking issue
- `ADVISORY`: improvement suggestion; not a failed criterion

Any failed acceptance criterion is `MUST_FIX` regardless of cosmetic severity.

Do not create a finding for speculative style preferences or optional redesign.

## 10. Repair rule

Only confirmed `MUST_FIX` findings reopen code mutation.

For each repair:

1. capture the failing criterion and stable evidence
2. state the smallest plausible root-cause hypothesis
3. make the smallest causal correction
4. rerun the focused reproducer/check
5. run affected regression checks
6. run required security/resource/quality review
7. close the finding only with final-state evidence
8. commit the verified repair before another RA batch; push/CI only when required and authorized

Do not stack broad refactors on top of an audit finding. If the repair requires a material architecture change, stop with `NEEDS_ADR`.

## 11. Independent reviewers

Use fresh-context read-only reviewers for audit work where practical.

Recommended Codex pattern:

- `retro_explorer`: read-only code/evidence mapping
- `retro_reviewer`: correctness/test/architecture/quality review
- `retro_security_reviewer`: security/trust-boundary review for RA-01/02/03/04/05 as applicable

Parallel subagents are preferred for read-heavy exploration/review. Avoid concurrent code writers during retrospective repair unless write sets are explicitly disjoint and isolated.

Reviewer output must contain concrete findings with file/test/criterion references. "Looks good" is not sufficient evidence.

## 12. Human gates

Human review is required when:

- an applicable criterion already requires human verification
- the audit discovers a product/UX/policy ambiguity
- a proposed repair changes architecture/security/acceptance meaning
- a verifier/test must be weakened, removed, bypassed, or reinterpreted
- the audit cannot distinguish defect from intended behavior
- a baseline exception is requested
- the final baseline has an unresolved blocking item

Do not ask for human approval merely to replace deterministic testing that the agent can run.
Human checks must be simple and user-visible; do not require internal IDs, DB/AppData edits, fixture internals, adapters, IPC, or repository internals.

## 13. Context and token control

The root must not load all Loops 01–22 code into one context.

Per batch, load:

1. root `AGENTS.md`
2. `docs/RETRO_AUDIT_STATUS.md`
3. exact relevant criteria
4. exact relevant architecture/security sections
5. relevant source/tests/evidence
6. broader docs only when a conflict/ambiguity requires them

Subagents return distilled findings, not raw logs. Store long evidence in tests/CI/artifacts/Git and reference it.

Checkpoint `docs/RETRO_AUDIT_STATUS.md` and `docs/AGENT_HANDOFF.md` after each batch and before any limit/intentional stop.

## 14. Retrospective baseline completion

`RETRO_BASELINE_PASS` requires:

- RA-01 .. RA-06 all `LEGACY_VERIFIED`
- no open `MUST_FIX` finding
- no applicable criterion in FAIL / BLOCKED / NEEDS_HUMAN / NOT_RUN
- all repairs verified on final integrated state
- final architecture/code-quality review has zero unresolved MUST_FIX
- security-sensitive batches have required security review
- current full regression suite required by the repository passes
- `docs/RETRO_AUDIT_STATUS.md` records the verified HEAD
- any required human gates are `HUMAN_PASS`

Advisory/nit improvements may remain and must not block the baseline unless they represent an actual criterion failure.

After `RETRO_BASELINE_PASS`, update `docs/IMPLEMENTATION_STATUS.md`, checkpoint the handoff for Loop 24, and continue normal v2.4.1 loop execution.

## 15. Status authority

When retrospective sources disagree:

1. current human instruction
2. product/architecture/security/ADR/acceptance docs
3. current deterministic tests/CI/runtime evidence
4. current source/Git state
5. retained historical evidence/status
6. old agent/chat claims

Never rewrite current correct behavior solely to match an obsolete historical note.

## 16. Required outputs

The audit maintains:

- `docs/RETRO_AUDIT_STATUS.md`
- finding IDs/evidence in that file
- tests added for confirmed gaps/bugs
- normal Git history/diff
- updated `docs/IMPLEMENTATION_STATUS.md` at baseline completion
- bounded `docs/AGENT_HANDOFF.md`

Do not create one report file per trivial finding.

## 17. References

Primary/official:

- OpenAI Codex AGENTS.md: https://learn.chatgpt.com/docs/agent-configuration/agents-md
- OpenAI Codex Subagents: https://learn.chatgpt.com/docs/agent-configuration/subagents
- OpenAI Codex Worktrees: https://learn.chatgpt.com/docs/environments/git-worktrees
- OpenAI — Running Codex safely: https://openai.com/index/running-codex-safely/
- OpenAI Codex CLI commands: https://learn.chatgpt.com/docs/developer-commands?surface=cli
- GitHub Actions run history: https://docs.github.com/en/actions/how-tos/monitor-workflows/view-workflow-run-history
- GitHub CLI `gh run view`: https://cli.github.com/manual/gh_run_view
- GitHub Actions workflow-run API permissions: https://docs.github.com/en/rest/actions/workflow-runs
- NIST SSDF project / SP 800-218: https://csrc.nist.gov/projects/ssdf
- OpenSSF OSPS Baseline v2026.08.28: https://baseline.openssf.org/versions/2026-08-28
- OWASP Secure Code Review Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Secure_Code_Review_Cheat_Sheet.html
- Google Engineering Practices — Code Review: https://google.github.io/eng-practices/review/

Protocol research provenance: `docs/12_PROTOCOL_RESEARCH_NOTES.md` (read only when auditing/changing protocol behavior).

Secondary operational signals (not normative):

- OpenAI Codex GitHub issue tracker for current compaction/resume/permission regressions
- r/codex reports on context management/worktree workflows

Issues/community reports can justify defensive checks but never override current official behavior or project contracts.
