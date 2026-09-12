<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop Engineering v2 Change Summary

## Migration decision

- Effective from Loop 23.
- Loop 23 may continue from its current in-progress implementation; it must satisfy v2 gates before PASS.
- Loops 01-22 are not replayed automatically.
- Loops 01-22 receive retrospective evidence audit only; implementation reopens only on an actual failed requirement.

## What v2 fixes

### Closed-loop control

Old behavior was primarily implement -> test/review -> report -> stop.

v2 adds an explicit state machine:

`CONTRACT -> BASELINE_VERIFY -> IMPLEMENT -> VERIFY -> CLASSIFY -> DIAGNOSE -> CORRECT -> REVERIFY -> REGRESSION -> REVIEW -> HUMAN -> FINAL_VERIFY -> PASS`

### Explicit PASS/FAIL semantics

A criterion cannot pass from agent self-report. Objective deterministic evidence has precedence. Subjective/product criteria route to a documented human gate.

### Bounded self-debugging

Project defaults:

- 5 total corrective iterations
- 2 repair attempts for the same stable failure fingerprint
- 1 major replan
- 2 unchanged reruns only for diagnosing flakiness
- 30 agentic tool-use turns when the harness can enforce a turn cap
- stop after two consecutive no-progress correction cycles

These are project stop-loss defaults, not universal industry constants.

### Resource stop-loss

When exact remaining-usage telemetry exists:

- <=20%: no new broad implementation approach
- <=10%: stop mutation and preserve handoff/evidence only

If quota is not observable, the agent must record `unavailable` rather than guess.

### Anti-bypass / reward-hacking control

Product/security/architecture contracts, acceptance meaning, required CI checks, relevant regression tests, and verifier logic are protected during ordinary feature loops.

An agent cannot earn PASS by deleting/skipping/loosening a failing test or changing the criterion to match its implementation.

### Independent verification

The implementer, deterministic verifier, independent AI reviewer, and human owner are separate roles.

- deterministic failure cannot be overridden by AI review
- independent AI review must be read-only during review
- required human judgment cannot be replaced by another AI vote

### Human gates

Human review is mandatory for subjective UX/product judgment, spec/ADR/security-policy changes, verifier weakening, new trust/privilege boundaries, destructive changes, non-convergence decisions, exceptions, and final MVP/release acceptance.



### Sandboxed execution and repository mutation boundary

Agent execution is now required to be isolated from the user's host system:

- the active repo is the only persistent local writable project area by default
- sandbox-owned temp/cache paths may be writable but are ephemeral and discarded
- host home/dotfiles/credentials/system settings/services/other repos are outside the mutation boundary
- no root/sudo or host Docker/Podman control socket
- GitHub access is scoped to one connected repo
- CI/check/Actions logs remain readable through repository-scoped CLI/API/connector access
- remote autonomous writes are limited to the working branch/PR when granted
- default-branch writes, workflow administration, releases/tags, secrets/settings, and other GitHub administration require a human gate
- network egress is constrained/allowlisted rather than open-ended
- any sandbox/repository/GitHub boundary violation stops mutation as `NEEDS_HUMAN`

This preserves local repo visibility by allowing the real repo to be mounted read-write into the sandbox while protecting the rest of the host system.

### Human-check procedure

UI review now requires exercising the running app with keyboard and mouse, focus checks, error/empty/loading states, small-desktop reflow, clipping/overlap checks, and concrete observations rather than `looks good`.

### Evidence integrity

PASS evidence is tied to the final state. Relevant changes after verification invalidate the affected PASS and require re-verification.

## Files changed/added

- `docs/README.md` — adds Protocol v2 and verification matrix to source-of-truth reading order.
- `docs/05_LOOP_ENGINEERING_RUNBOOK.md` — replaced with bounded convergence/control protocol.
- `docs/06_ACCEPTANCE_CRITERIA.md` — adds exact criterion status/evidence semantics and loop-integrity criteria.
- `docs/08_VERIFICATION_MATRIX.md` — new mapping from acceptance criteria to automated/AI/human verifier requirements.
- `IMPLEMENTATION_STATUS_TEMPLATE.md` — adds loop contract, budgets, failure fingerprints, evidence, integrity review, AI review, and human gate fields.

## Research basis used

Primary/official sources were preferred:

- Anthropic Agent SDK / engineering guidance
- Microsoft Agent Framework looping guidance
- OpenAI evaluation and coding-agent monitoring guidance
- NIST SSDF
- OpenSSF OSPS Baseline
- OWASP secure code review guidance
- METR evaluation-integrity research as an independent empirical cross-check


## v2.1 — Codex-targeted execution and code-quality gates

Added after the project standardized on Codex App / Codex CLI with GPT-5.6 Sol or GPT-6 Astra and multi-agent workflows.

Changes:

- added root `AGENTS.md` as Codex's persistent repository operating instructions
- added `docs/09_CODEX_EXECUTION_PROFILE.md` for Codex App/CLI sandbox, approvals, worktrees, GitHub CI, and multi-agent behavior
- made OpenCode Go non-required; alternate harnesses need an explicit equivalence/security review before their evidence can be authoritative
- added Codex-specific loop criteria `LOOP-021..025`
- added explicit `QUAL-001..012` blocking criteria for SOLID, cohesion, readability, maintainability, file/package placement, public surface, and independent code-quality review
- clarified that the intended Clean Architecture form is minimal Ports & Adapters, not enterprise layer proliferation
- required fresh-context code-quality review on every production-code loop
- required isolated worktrees/disjoint write sets for parallel code-changing agents
- added execution harness/model/multi-agent fields to loop/status evidence
- corrected stop-loss subsection numbering under section 12
## v2.3 — Durable agent state / bounded session handoff

Added repository-resident recovery memory for Codex App/CLI without turning context into an unbounded transcript.

Key changes:

- added `docs/10_DURABLE_AGENT_STATE.md`
- added bounded runtime `docs/AGENT_HANDOFF.md`
- changed session bootstrap from unconditional full-doc reread to handoff + Git reconciliation + progressive source loading
- added checkpoint triggers for interruption, correction, worker integration, human gates, and loop exits
- added strict 4 KiB target / 8 KiB + 160-line hard cap
- prohibited chain-of-thought, full logs/diffs/source copies, and secrets in durable memory
- made the root agent the sole owner of global handoff state
- added safe stale-state/reconciliation behavior
- added fresh-session guidance after instruction/protocol changes
- added `STATE-001..010` acceptance criteria and verification matrix coverage

This memory is explicitly a cache/index. Git, tests/CI, ADRs, canonical engineering docs, and human instructions remain higher authority.
