<!-- SPDX-License-Identifier: MPL-2.0 -->

# Codex App / Codex CLI Execution Profile

This project intentionally targets **OpenAI Codex App and Codex CLI** as the supported implementation harness for Loop Engineering Protocol v2.4.1.

OpenCode Go is not required by this protocol. If it is reintroduced later, do not assume behavioral equivalence: its sandbox, instruction loading, multi-agent isolation, approvals, tool permissions, and evidence semantics must be reviewed separately before it may participate in a passing loop.

## 1. Supported model policy

Primary root models:

- GPT-5.6 Sol
- GPT-6 Astra

Use the model explicitly selected for the active Codex thread/session.

Subagents should inherit the root model by default. Do not silently switch worker or reviewer models to reduce cost or increase speed. A mixed-model run is allowed only when the human intentionally configures it and the loop evidence records the override.

Model choice does not change acceptance criteria. A stronger model does not permit weaker verification.

## 2. Repository instructions: AGENTS.md

The repository root must contain `AGENTS.md`.

Codex App/CLI loads applicable `AGENTS.md` instructions from the repository hierarchy. These instructions are the short operational entry point; the numbered engineering documents remain the detailed source of truth.

Rules:

- keep the root `AGENTS.md` concise enough to remain reliably loaded
- nested `AGENTS.md` may add local conventions but must not weaken root safety, verification, or quality requirements
- do not create a nested `.git` marker that changes Codex's detected project root
- when instruction loading is uncertain, stop mutation and verify the effective project root/instructions

## 3. Approved Codex execution modes

### 3.1 Direct local development

Minimum acceptable posture when Codex runs directly on the user's machine:

```toml
# ~/.codex/config.toml — HUMAN-MANAGED EXAMPLE ONLY
# The agent must not edit this host file itself.

sandbox_mode = "workspace-write"
approval_policy = "on-request"
approvals_reviewer = "user"

[sandbox_workspace_write]
network_access = false
```

Project policy:

- repository is the only intended persistent writable workspace
- no Full Access / `danger-full-access`
- no `--dangerously-bypass-approvals-and-sandbox`
- no autonomous edits to `~/.codex`, shell profile, GitHub config, SSH config, keychain, or global package state
- network remains off unless the current loop requires an explicitly allowed destination

The built-in Codex sandbox is a **write boundary**, not a promise that every host file is unreadable on every OS/configuration. For strict confidentiality/isolation, use the external-isolation profile below.

### 3.2 Preferred unattended / long multi-agent runs

For long autonomous runs, use defense in depth:

```text
Host
└── externally isolated dev environment (container / dedicated VM / dedicated WSL environment)
    ├── /workspace/repo      <- only project bind mount, read-write
    ├── /tmp + tool caches   <- sandbox-owned ephemeral
    ├── Codex App/CLI agent  <- still uses workspace-write where practical
    └── scoped GitHub auth   <- only connected repository
```

Requirements:

- do not mount the user's whole home directory
- do not mount host Docker/Podman control sockets
- do not mount host SSH/keychain/browser credential stores
- bind-mount only the active repository paths required for visible edits
- use disposable temp/cache paths inside the isolated environment
- keep GitHub credentials inside the isolated environment and repository-scoped

A bind-mounted real repository preserves live visibility: edits performed inside the isolated environment appear in the normal host working tree while unrelated host paths remain outside the agent environment.

## 4. GitHub and CI access

CI observability is required and does not justify host access.

Read operations may include:

```text
gh pr view
gh pr checks
gh run list
gh run view <run-id>
gh run view <run-id> --log-failed
```

Equivalent Codex GitHub connector/API reads are acceptable.

The agent may read for the connected repository:

- commits/branches
- PRs/reviews
- checks/statuses
- Actions run/job/step state
- failure logs
- diagnostic artifacts required by the loop

Remote write requires authorization. Target `main` by default; use a branch/PR only when repository protection requires it, the human requests it, or the change is high-risk/incompatible. Force push, workflow administration, reruns/cancellation, releases/tags, secrets, variables, environments, branch protection, repository settings, webhooks, deploy keys, and organization administration remain human-gated.

## 5. Codex multi-agent policy

Multi-agent work is explicitly authorized by this repository's `AGENTS.md`, but it is bounded.

### Root agent

Owns:

- loop contract
- decomposition
- acceptance/evidence state
- integration
- budgets
- final verification
- human handoff

### Code-changing workers

Use only when the task can be partitioned into concrete, coherent slices.

Rules:

- each worker gets a disjoint write set
- Codex App: prefer one isolated worktree per parallel code-changing worker
- Codex CLI/subagents: use isolated/forked workspaces when available; otherwise do not run overlapping writers concurrently
- each worker reports exact files changed and checks run
- root reviews/integrates returned changes before final verification

### Reviewer agents

Reviewers are read-only for the review pass and use fresh context.

At minimum, every production-code loop requires a code-quality reviewer. Add specialized reviewers for security, concurrency/lifecycle, performance, or platform behavior when triggered by the runbook.

A reviewer must not approve its own authored final code. If a reviewer proposes a repair and then becomes the author of that repair, use a different fresh reviewer for the final state.

### Concurrency

Do not maximize agent count for its own sake. Parallelism is useful only for independent work.

Recommended project default: cap concurrent subagent threads to a small reviewable number (for example 4) unless the human deliberately changes it.

Current Codex builds expose multi-agent settings such as:

```toml
# HUMAN-MANAGED EXAMPLE ONLY
[agents]
enabled = true
max_concurrent_threads_per_session = 4
```

Do not set `default_subagent_model` unless mixed-model behavior is deliberate; leaving it unset preserves the normal inheritance policy.

## 6. Codex App worktrees

Codex App supports isolated worktrees for parallel agents. Use them when two code-changing agents operate on the same repository concurrently.

A worktree is isolation, not acceptance. Before integrating a worker result:

1. inspect the diff
2. run focused verification in that worktree when practical
3. check for protected-verifier edits
4. integrate through the root agent/selected branch
5. run final regression and quality review on the integrated state

Never mark the loop PASS from separate green worktrees without verifying the combined result.

## 7. Codex CLI safety invariants

- run from the intended Git repository root or a known subdirectory
- verify `git status`/repository identity before mutation
- do not use danger bypass/full-access on the host
- do not respond to sandbox denial by broadening permissions automatically
- a missing tool is `ENVIRONMENT_BLOCKED`, not permission to install globally
- inspect GitHub CI through scoped network/API access rather than host credential discovery
- do not mutate verification criteria/tests merely to obtain green status

Codex versions evolve. When an upgrade removes/deprecates a configuration key or changes sandbox/approval behavior, fail safe: stop, check the current Codex documentation/schema, update this profile through human-reviewed documentation, then resume.

## 8. OpenCode Go status

Protocol v2.4.1 does not require OpenCode Go and no acceptance evidence may depend exclusively on it.

If the user stops using OpenCode Go, no engineering-plan change is required.

If it is reintroduced later, require a harness-equivalence review covering:

- repository instruction loading
- sandbox filesystem boundary
- network boundary
- GitHub credential/repository scoping
- multi-agent write isolation
- reviewer independence
- trace/evidence capture
- stop-loss/budget enforcement

Until that review passes, alternate-harness output is advisory only and cannot be the sole evidence for a loop PASS.


## 9. Current Codex implementation notes and source basis

This profile intentionally uses currently supported Codex concepts rather than older CLI examples.

As of the current Codex generation used for this document:

- `workspace-write` is the intended sandbox mode for repo editing; it permits writes in the workspace/writable roots while keeping other writes behind the sandbox/approval boundary
- `approval_policy = "on-request"` is the restrictive interactive policy used here; legacy `approval_policy = "untrusted"` is no longer supported in current Codex builds and must not be copied from old examples
- `approvals_reviewer` can be routed to user or auto-review; this project chooses `user` for sandbox/permission expansion because boundary changes are human authority
- Codex config exposes `[agents]` controls including `enabled` and `max_concurrent_threads_per_session`
- Codex multi-agent tooling normally lets subagents inherit the current model unless a model is explicitly selected/overridden
- Codex App supports isolated worktrees for parallel agents
- applicable `AGENTS.md` files are loaded hierarchically from repository root toward the working directory and are the intended repository instruction mechanism

Authoritative references to re-check after major Codex upgrades:

- OpenAI — Running Codex safely at OpenAI: https://openai.com/index/running-codex-safely/
- OpenAI — Building a safe, effective sandbox to enable Codex on Windows: https://openai.com/index/building-codex-windows-sandbox/
- OpenAI — Introducing the Codex app: https://openai.com/index/introducing-the-codex-app/
- OpenAI — GPT-6 Astra / current model guidance: https://developers.openai.com/api/docs/guides/latest-model
- OpenAI Codex repository config schema: https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json
- OpenAI Codex AGENTS.md implementation/docs: https://github.com/openai/codex/blob/main/codex-rs/core/src/agents_md.rs

Treat configuration snippets in this document as human-reviewed project guidance, not as permission for the agent to modify host Codex configuration itself.
## 10. Session continuity, compaction, and resume

Codex thread/session state is useful but is not the project memory authority.

Repository policy:

- persistent project memory lives in `docs/AGENT_HANDOFF.md` plus referenced Git/status/test/CI/ADR evidence
- a continuing root session reads the bounded handoff and reconciles current Git before broader discovery
- do not inject all engineering docs into every new worker/session when the handoff can point to the exact required sections
- keep `AGENTS.md` stable and concise because it is part of Codex's injected repository instructions
- do not put an ever-growing project diary into `AGENTS.md`

For context compaction:

- assume compaction can omit or distort nonessential historical detail
- checkpoint durable state at material recoverability boundaries instead of waiting for a compaction event
- after compaction, if the agent appears to repeat completed work or loses the current task point, reread/reconcile `docs/AGENT_HANDOFF.md` and Git state rather than reconstructing from chat history

For resume:

- ordinary resume is acceptable when repository instructions/protocol did not change
- after `AGENTS.md`, protocol, architecture/security contract, or handoff schema changes, prefer a **new session** initialized from current repository state
- do not trust an old resumed thread to have automatically refreshed every injected instruction surface

Token discipline:

- handoff target <= 4 KiB, hard cap <= 8 KiB/160 lines
- long evidence stays out-of-band and is referenced by path/run ID/criterion
- workers receive task-specific context rather than the full project history
- root aggregates worker findings into the single bounded global handoff

See `docs/10_DURABLE_AGENT_STATE.md` for the authoritative project policy.


## 11. v2.4.1 verified Codex operating notes

Current Codex documentation confirms:

- the `AGENTS.md` instruction chain is built once per run/session; protocol/instruction changes should therefore start a fresh session rather than assuming an old resumed session has refreshed instructions
- project instruction discovery walks from Git/project root to the current working directory; the default combined instruction cap is 32 KiB
- current Codex releases enable subagent workflows; subagents can be requested by prompt or applicable `AGENTS.md`
- subagent workflows consume more tokens than comparable single-agent work
- OpenAI recommends starting parallel delegation with read-heavy exploration/tests/triage/summarization and being more careful with parallel write-heavy workflows
- subagents inherit the parent turn's current permission/sandbox mode unless a custom agent narrows it
- project-scoped custom agents are supported under `.codex/agents/`; this repository provides read-only retrospective reviewers without hard-coding a model
- the Codex App worktree feature supports parallel branches/checkouts, but a Git worktree is not a security sandbox
- Codex CLI `/status` reports active model, approval policy, writable roots, and current token usage; inspect it before a long run when practical
- `workspace-write` controls where commands may write; it does not guarantee that unrelated host files are unreadable. Use an external container/VM/dedicated environment when confidentiality/read isolation is required
- GitHub workflow history/logs can be read with repository read access; workflow-run API access can use repository `Actions: read`, and `gh run view --log-failed` can retrieve failed-step logs

Project policy therefore distinguishes:
1. **write containment** — built-in Codex workspace sandbox
2. **read/confidentiality isolation** — external isolation when required
3. **parallel source isolation** — Git worktrees
4. **remote authorization** — repository-scoped GitHub credentials/permissions

These are separate controls and must not be described as interchangeable.

Operational warning:

Open Codex GitHub issues in 2026 document real compaction/resume and permission-profile regressions. They are secondary evidence, not specification, but justify the project's durable handoff and effective-runtime verification. If actual runtime `/status`/permission behavior conflicts with expected config, fail safe and ask the human instead of silently broadening access.

Current references:

- https://learn.chatgpt.com/docs/agent-configuration/agents-md
- https://learn.chatgpt.com/docs/agent-configuration/subagents
- https://learn.chatgpt.com/docs/environments/git-worktrees
- https://learn.chatgpt.com/docs/developer-commands?surface=cli
- https://openai.com/index/running-codex-safely/
- https://openai.com/index/building-codex-windows-sandbox/
- https://github.com/openai/codex/issues/27731
- https://github.com/openai/codex/issues/34322


## 12. Windows sandbox capability failures and isolated verifier fallback

The Windows Codex sandbox is distinct from Microsoft's optional Windows Sandbox VM feature. The native Codex Windows design uses Windows process/token/ACL mechanisms; WSL, Docker, Hyper-V, or the Windows Sandbox feature are not prerequisites for the native Codex sandbox itself.

OpenAI documents Windows sandbox setup modes named `elevated` and `unelevated`. This project does not require one universal mode across all machines. The human-managed effective mode is acceptable when it:

- is supported by the installed Codex build
- passes a minimal process-execution health check
- preserves the repository write boundary
- does not require Full Access

If the sandbox cannot start a minimal process, classify `SANDBOX_INFRA_FAILURE`.

If normal commands run but a verifier requires child-process behavior that the sandbox blocks (for example Node `child_process.spawn()` / Chromium helper startup returning `EPERM`), classify `SANDBOX_CAPABILITY_LIMITATION` / `SANDBOX_CHILD_PROCESS_LIMITATION`.

Do not burn retries changing shells or executable paths after the same causal fingerprint is confirmed. Current `openai/codex` issue reports #21470 and #35070 contain minimal reproductions where Node itself runs while child-process spawning fails with `EPERM`; Computer Use/Chromium helper reports show the same class of failure. These are issue reports, not contractual product behavior, so the protocol uses them only to justify fail-safe classification/circuit breaking.

For a deterministic check blocked only by that local sandbox capability, use the canonical fallback policy in `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`:

1. local capable Codex sandbox
2. GitHub-hosted runner for the connected repository
3. another human-approved isolated CI environment

The normal host outside the sandbox is diagnostic only. Full Access/danger bypass is never an acceptance fallback.

GitHub-hosted CI must preserve the criterion and use least privilege. GitHub documents standard hosted runner labels such as `ubuntu-latest` and `windows-latest`, repository read access for viewing workflow run history/logs, and `Actions: read` for fine-grained workflow-run read APIs. Workflow changes remain a protected/human-gated surface.

References:

- https://openai.com/index/building-codex-windows-sandbox/
- https://openai.com/index/running-codex-safely/
- https://learn.chatgpt.com/docs/app-server
- https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- https://docs.github.com/en/actions/how-tos/monitor-workflows/view-workflow-run-history
- https://docs.github.com/en/rest/actions/workflow-runs
- https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- https://github.com/openai/codex/issues/21470
- https://github.com/openai/codex/issues/35070
- https://github.com/openai/codex/issues/37272
