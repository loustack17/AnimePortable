<!-- SPDX-License-Identifier: MPL-2.0 -->

# Durable Agent State and Session Handoff

This document defines how Codex persists the minimum state required to stop, resume, compact, or replace an AI session without re-discovering the whole repository.

The goal is **recoverability with bounded context**, not a transcript archive.

## 1. Memory model

Repository-resident agent memory has four layers:

1. **Instructions** — `AGENTS.md` and the numbered engineering documents. These define policy and intent.
2. **Current handoff** — `docs/AGENT_HANDOFF.md`. This is a small, replaceable checkpoint for the active loop/session.
3. **Durable evidence** — `docs/IMPLEMENTATION_STATUS.md`, tests, CI results, ADRs, and other explicitly referenced artifacts.
4. **Code truth** — Git branch/HEAD, working-tree diff, and the repository itself.

`docs/AGENT_HANDOFF.md` is a cache/index over the other layers. It is never authoritative over Git, tests, acceptance criteria, security policy, or ADRs.

Do not depend on chat history, hidden model memory, or Codex session compaction as the only copy of material project state.

## 2. What belongs in the handoff

Record only information whose absence would cause the next agent to repeat material discovery, lose an unresolved problem, or take a materially different implementation path.

Required fields:

- protocol/instruction version
- current loop and loop state
- branch and last observed HEAD
- loop goal and acceptance criterion IDs
- current completed/in-progress work
- unresolved failures and failure fingerprints
- exact next action(s), ordered
- touched/relevant paths
- last meaningful focused/regression verification results
- pending human gate / ADR / environment blocker
- current retry/budget counters when relevant
- references to the exact docs/headings/evidence needed next
- short `do_not_repeat` notes for already disproven approaches

Do **not** store:

- hidden chain-of-thought or long reasoning transcripts
- full command output, CI logs, diffs, source files, or test logs
- information already easy to obtain from `git status`, `git diff`, tests, or CI
- secrets, cookies, tokens, credentials, authenticated URLs, or sensitive environment values
- speculative ideas that did not affect the accepted plan
- copied sections of the numbered engineering documents

When a detail needs long-term normative force, promote it to an ADR, acceptance criterion, test, architecture/security document, or code comment as appropriate. Do not let `docs/AGENT_HANDOFF.md` become a shadow specification.

## 3. Size and token discipline

Target handoff size: **<= 4 KiB**.

Hard limit: **8 KiB** and **160 lines**.

If the file exceeds either hard limit:

1. remove duplicated/history detail
2. move durable evidence to `docs/IMPLEMENTATION_STATUS.md` or an evidence artifact
3. promote architectural decisions to an ADR
4. keep only the current state, unresolved items, next actions, and references

The deterministic size check is:

```text
wc -c docs/AGENT_HANDOFF.md
wc -l docs/AGENT_HANDOFF.md
```

A large handoff is a protocol failure, not a reason to load it anyway.

## 4. Checkpoint triggers

The root Codex agent updates `docs/AGENT_HANDOFF.md` at **recoverability boundaries**, not after every tool call.

Checkpoint after:

- the Loop Contract becomes stable
- a material implementation slice is completed and focused verification has run
- a verification failure has been classified and a diagnostic direction is established
- a corrective iteration materially changes the diagnosis or implementation
- worker/subagent results are integrated
- a human decision/approval changes the allowed path
- entering `BLOCKED`, `NEEDS_HUMAN`, `NEEDS_ADR`, `ABORTED_*`, or `PASS`

Checkpoint before:

- intentionally stopping/pausing the Codex session
- handing work to another root agent/session
- a known long-running operation where interruption would otherwise lose state
- asking the human for a gate/decision

Do not checkpoint every read, test command, or trivial edit. Git and tool output already preserve those facts more efficiently.

## 5. New-session bootstrap

For a **new Codex session continuing an active loop**, use this bounded bootstrap instead of rereading the entire repository/docs:

1. confirm repository identity and working directory
2. read the applicable `AGENTS.md` (normally injected by Codex) and `docs/AGENT_HANDOFF.md`
3. run:
   - `git status --short --branch`
   - `git log -5 --oneline --decorate`
   - `git diff --stat`
   - `git diff --name-only`
4. reconcile the handoff's branch/HEAD/touched paths with current Git state
5. read only the referenced engineering-document sections, acceptance criteria, source files, tests, and evidence needed for the next action
6. continue from the recorded loop state only after reconciliation

If the working tree is dirty, inspect the relevant diff before mutation even when the handoff describes it.

Do not rescan the whole repository merely to rebuild context already captured by a valid handoff.

## 6. When full/re-expanded reading is required

The bounded bootstrap is invalid and the root agent must expand its reading when any of these apply:

- starting a new loop/phase
- `docs/AGENT_HANDOFF.md` is missing, malformed, over the size cap, or marked stale
- current branch/HEAD/worktree materially contradicts the handoff
- protocol/instruction version changed
- `AGENTS.md`, architecture, security, acceptance criteria, or active ADRs changed since the checkpoint
- the recorded next action cannot be justified from referenced evidence
- a security/architecture boundary is unclear
- a failure occurs outside the handoff's known scope

Even then, read progressively: relevant sections first, whole documents only when the scope or conflict requires it.

## 7. Handoff freshness and stale-state handling

`docs/AGENT_HANDOFF.md` records the last observed branch and HEAD. They are reconciliation hints, not locks.

On startup:

- same branch/HEAD + compatible working tree -> handoff may be used as current index
- HEAD advanced -> inspect commits/diff since the recorded HEAD and refresh handoff
- branch changed -> treat handoff as stale until reconciled
- dirty paths differ materially -> inspect the actual diff; Git wins
- test/CI evidence conflicts with handoff text -> verifier evidence wins

Never reset or discard working-tree changes merely because they are absent from the handoff.

## 8. Multi-agent memory ownership

Only the **root orchestrator** owns the global `docs/AGENT_HANDOFF.md`.

Workers/subagents return concise structured results to the root:

- task/result
- files changed
- verification run/results
- unresolved findings
- assumptions/risks

Workers must not concurrently rewrite the global handoff. The root updates it after integrating or rejecting worker results.

A reviewer may read the handoff for scope/context but must independently inspect the final diff/evidence required by its rubric.

## 9. Session resume and instruction changes

Do not assume a resumed Codex thread is an authoritative memory source.

If `AGENTS.md` or Protocol v2.x instructions changed after a session started, prefer a **new Codex session** bootstrapped from the current repository and `docs/AGENT_HANDOFF.md` rather than relying on the old resumed session's injected instruction state.

Session compaction is an optimization, not durable project storage. The repository checkpoint must remain sufficient to recover if compaction loses detail or a session is discarded.

## 10. Loop-boundary behavior

At loop PASS:

1. write final acceptance/evidence status to `docs/IMPLEMENTATION_STATUS.md`
2. promote durable decisions to ADR/docs/tests where needed
3. replace the active handoff with a compact next-loop handoff or `NO_ACTIVE_LOOP`
4. remove stale failure notes and disproven temporary context that no longer matters

Do not accumulate every previous loop in `docs/AGENT_HANDOFF.md`. Historical truth belongs in Git, status/evidence, tests, and ADRs.

## 11. Authority order on resume

When sources conflict, use:

1. current human instruction
2. applicable `AGENTS.md` + numbered source-of-truth docs/ADRs
3. deterministic verifier / current Git and CI state
4. `docs/IMPLEMENTATION_STATUS.md` and retained evidence
5. `docs/AGENT_HANDOFF.md`
6. old chat/session summaries

The handoff accelerates recovery; it never overrides the project contract.
