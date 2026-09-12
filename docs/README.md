<!-- SPDX-License-Identifier: MPL-2.0 -->

# Anime Client MVP Engineering Plan

This directory is the source of truth for implementing the MVP. The supported AI implementation harness is OpenAI Codex App / Codex CLI; repository-level operating instructions live in `AGENTS.md`.

The product is a lightweight, fast, keyboard-friendly Anime1 desktop client focused on immersive playback through MPV. The application handles discovery, metadata, scheduling, following, history, episode selection, playback orchestration, and progress tracking. MPV handles actual media playback and all playback UX.

## AI bootstrap and reading policy

Codex receives applicable `AGENTS.md` instructions as repository guidance. Project continuity is stored in the bounded `docs/AGENT_HANDOFF.md` defined by `docs/10_DURABLE_AGENT_STATE.md`.

A session **continuing an active loop** should not blindly reread every document. It first reconciles `docs/AGENT_HANDOFF.md` with current Git state, then reads the referenced/relevant source-of-truth sections needed for the next action.

A **new loop**, protocol/instruction change, stale/missing handoff, architecture/security ambiguity, or material state conflict requires broader canonical initialization from these documents:

1. `docs/01_PRODUCT_CONTRACT.md`
2. `docs/02_ARCHITECTURE.md`
3. `docs/03_SECURITY.md`
4. relevant phase in `docs/04_MVP_IMPLEMENTATION_PLAN.md`
5. `docs/05_LOOP_ENGINEERING_RUNBOOK.md`
6. `docs/06_ACCEPTANCE_CRITERIA.md`
7. active ADRs in `docs/07_ADR_BASELINE.md` / later ADRs
8. `docs/08_VERIFICATION_MATRIX.md`
9. `docs/09_CODEX_EXECUTION_PROFILE.md`
10. `docs/10_DURABLE_AGENT_STATE.md`

The durable handoff reduces repeated discovery; it does not replace these sources.

## Source-of-truth rule

If code, comments, or previous implementation conflict with these documents, these documents win unless a new ADR explicitly changes the decision.

## Non-negotiable product principles

- Lightweight
- Fast
- Intuitive
- Minimal and visually calm
- Easy to use
- Keyboard-first on desktop
- Mouse fully supported
- Immersive playback
- No danmaku
- No social features
- No ads
- No telemetry by default
- Local-first privacy
- MPV-first playback
- Modular but not over-engineered
- Maintainable, readable code with clear responsibilities and file placement
- SOLID principles applied pragmatically, not ceremonially
- External dependencies are replaceable
- Anime1 must not be allowed to become a permanent architectural dependency of the core

## MVP technology baseline

- Go
- Wails v3
- Svelte + TypeScript
- SQLite
- MPV as the only MVP player
- MPV JSON IPC
- Anime1 as the initial anime source adapter
- AniList as primary metadata provider
- Bangumi as fallback / cross-check metadata provider
- Windows, Linux, macOS
- No mobile in MVP

## Core design rule

Use a minimal Ports & Adapters architecture.

Only create replaceable ports for real external boundaries:

- `AnimeSource`
- `MetadataProvider`
- `Player`
- `Store`

Do not introduce enterprise-style abstraction layers for every helper or utility.

## Loop engineering protocol

Loop Engineering Protocol v2.3 is mandatory beginning with Loop 23. Loops 01-22 are not automatically replayed; they receive retrospective evidence audit under `docs/05_LOOP_ENGINEERING_RUNBOOK.md`.

A loop is complete only when the required deterministic, independent-review, and human gates defined in `docs/05_LOOP_ENGINEERING_RUNBOOK.md` and `docs/08_VERIFICATION_MATRIX.md` pass. The implementation agent's self-report is not a completion signal.

## Agent execution security

AI implementation must run inside a sandboxed workspace. The active local repository is the only persistent local project area writable by default; sandbox-owned temporary/cache state may exist only as disposable sandbox state. Host files, credentials, services, unrelated repositories, and privileged control sockets are outside the mutation boundary.

The sandbox may retain repository-scoped GitHub connectivity so the agent can read PR checks, GitHub Actions results/logs, and CI artifacts for the connected repository. Remote writes are restricted to the connected repository and normal working-branch/PR operations; GitHub administration, workflow control, protected-branch writes, secrets/settings, releases/tags, or scope expansion require a human gate. See `docs/05_LOOP_ENGINEERING_RUNBOOK.md`.


## Durable agent state

`docs/AGENT_HANDOFF.md` stores only the compact active-loop checkpoint needed to recover after pause, session replacement, or context compaction. It is size-bounded and points to durable evidence instead of copying logs/diffs/docs. Historical decisions belong in ADRs/status/tests/Git, not in an ever-growing AI memory file.

## MVP completion definition

The MVP is complete only when all functional, architecture, code-quality, security, performance, resource-lifecycle, keyboard-accessibility, cross-platform, CI/release, and required loop-integrity criteria in `docs/06_ACCEPTANCE_CRITERIA.md` pass with evidence for the final state, including all required human gates.
