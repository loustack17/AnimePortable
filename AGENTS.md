# Anime Client / AnimePortable — Codex Instructions

Instruction version: **v2.4.1**

These instructions apply to the whole repository unless a deeper `AGENTS.md` adds stricter local rules. Deeper instructions must not weaken this file's safety, verification, architecture, or code-quality requirements.

## Supported harness

Use OpenAI Codex App or Codex CLI. Multi-agent delegation is explicitly allowed under the rules below. OpenCode Go or another coding harness is not required and must not become a hidden dependency of the workflow.

## Session bootstrap and durable memory

Do not rely on chat/session memory as the project state.

For a **new session continuing the current active loop**:

1. use the applicable `AGENTS.md` instructions
2. read `docs/AGENT_HANDOFF.md`
3. reconcile it with `git status --short --branch`, recent Git log, and the current diff
4. read only the source-of-truth sections/files referenced by the handoff and required for the next action
5. expand to broader canonical docs only if the handoff is stale, conflicting, incomplete, security/architecture-sensitive, or the loop is changing

For a **new loop**, protocol/instruction change, missing/stale handoff, or unresolved conflict, perform the broader canonical initialization required by `docs/README.md`, including `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md` when verification/runtime boundaries are relevant.

`docs/AGENT_HANDOFF.md` is a bounded recovery index, not a specification. Git, tests/CI, ADRs, acceptance criteria, and canonical docs override it.

The root agent updates the handoff only at recoverability boundaries defined in `docs/10_DURABLE_AGENT_STATE.md`. Workers do not concurrently edit the global handoff.

Do not implement from an old chat summary alone.

## Scope and safety

- Work only on the assigned loop/vertical slice.
- Persistent local writes are limited to this repository.
- Sandbox-owned ephemeral temp/cache is allowed.
- Do not edit host dotfiles, `~/.codex`, SSH/GitHub global config, credential stores, OS settings, services, or unrelated repositories.
- Do not use sudo/global installs to make a check pass.
- Do not use Full Access, `danger-full-access`, `--dangerously-bypass-approvals-and-sandbox`, or equivalent on the host.
- GitHub access is limited to the connected repository; CI/check/log reads are allowed. Protected remote administration is human-gated.
- If a boundary blocks required work, stop with `NEEDS_HUMAN`/`ENVIRONMENT_BLOCKED`; never grant yourself broader access.

## Architecture and code quality

The intended Clean Architecture form is the repository's **minimal Ports & Adapters** design: `UI -> Core -> Adapters`, dependencies inward.

Apply SOLID pragmatically:

- SRP: cohesive responsibilities; no god files/types.
- OCP: extend at real adapter/policy seams, not provider conditionals in core.
- LSP: adapters preserve shared contract/error/cancellation/lifecycle/security behavior.
- ISP: narrow consumer-relevant ports; no interface-per-helper ceremony.
- DIP: core owns abstractions/policy and never imports concrete infrastructure.

Keep code readable and maintainable:

- clear domain names and straightforward control flow
- code in the narrowest correct existing package/directory
- no generic `utils`/`helpers`/`common`/`misc` dumping grounds
- no duplicate domain/security policy
- no unnecessary public API, dead/debug scaffolding, or speculative abstraction
- comments explain non-obvious constraints/why, not obvious syntax

Functional tests passing is not enough if the final diff violates architecture, maintainability, readability, or file placement.

## Verification integrity

- Define acceptance/verifier/human gates before new mutation under Protocol v2.4.1.
- Never weaken, delete, skip, or special-case required tests/criteria/CI/security policy to get PASS.
- On failure: classify -> diagnose with evidence -> make the smallest causal fix -> rerun focused check -> regression.
- Stop on repeated identical failure/no progress/budget limit as defined by the runbook.
- Final PASS requires final-state evidence after all fixes and cleanup.
- After a verified logical change, commit it before unrelated work. Push only with approval; if CI is required, close only after green CI.
- When remote mutation is approved, target `main` by default. Use a branch/PR only when repository protection requires it, the human requests it, or the change is high-risk/incompatible.

### Approved verifier environments

The local Codex sandbox is the default verifier environment, but it is not the only approved isolated verifier.

If the local sandbox works generally but cannot execute a required deterministic verifier because of a stable sandbox/runtime capability limitation:

- classify the environment failure; do not call it a repository test failure
- confirm the failure fingerprint once, then stop equivalent retries
- do not use Full Access / `danger-full-access`
- an unsandboxed host run may diagnose the problem but cannot count as PASS
- use a GitHub-hosted runner or another human-approved isolated CI environment when it can reproduce the criterion
- preserve the exact criterion/test semantics and record the run/commit evidence
- never substitute Linux/browser CI for a criterion that specifically requires native Windows/MPV/human behavior

Edits to `.github/workflows/**` or `.github/actions/**` require explicit human-approved scope before mutation and independent verifier/security review. Remote application remains separately human-authorized.

See `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`.

## Multi-agent rules

One root agent owns the contract, integration, budgets, and final result.

Code-changing workers:

- receive concrete self-contained tasks
- must have disjoint write sets
- should use separate Codex App worktrees / isolated subagent workspaces for parallel writes
- must report files changed and checks run
- must not duplicate another active worker's unresolved task

Reviewers:

- use fresh context
- are read-only during the review pass
- do not approve final code they authored
- code-quality review is required for every production-code loop
- add security/concurrency/performance/platform reviewers when triggered

Do not create an unbounded subagent tree. Parallelism must materially improve the task and stay inside the loop budgets.


## Retrospective baseline gate

After Loop 23 reaches PASS, do **not** begin Loop 24 until the one-time retrospective baseline audit in `docs/11_RETROSPECTIVE_BASELINE_AUDIT.md` reaches `RETRO_BASELINE_PASS`.

Audit historical Loops 01–22 as current-state verification, not implementation replay. Missing old-format evidence means missing verification, not a defect. Reopen code only for confirmed `MUST_FIX` findings.

Maintain `docs/RETRO_AUDIT_STATUS.md`. For audit parallelism, prefer read-only subagents; parallel write-heavy work is exceptional and requires disjoint write sets plus isolation.

## Completion

Do not mark a loop done until all required deterministic checks, `QUAL-*` criteria, specialized reviews, human gates, regression checks, and execution-boundary criteria pass. Human checks must be simple and user-visible; do not require internal IDs, DB/AppData edits, fixture internals, adapters, IPC, or repository internals. Report exact commands/evidence and unresolved issues. Before an intentional pause/handoff or loop exit, checkpoint the bounded `docs/AGENT_HANDOFF.md` according to `docs/10_DURABLE_AGENT_STATE.md`.
