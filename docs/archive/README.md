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
11. `docs/11_RETROSPECTIVE_BASELINE_AUDIT.md`
12. `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md` when sandbox/verifier execution boundaries or CI fallback are relevant
13. `docs/14_ADR_NO_WEBVIEW.md`, `docs/15_ADR_FYNE_DESKTOP.md`, and `docs/16_FYNE_MIGRATION_PLAN.md` for the Fyne migration baseline
14. `docs/17_ADR_UI_RESOURCE_DECISION_PROPOSAL.md` when reviewing the unresolved Fyne/resource architecture conflict; this proposal does not supersede ADR-021
15. `docs/18_RESOURCE_CONTROL_PROTOTYPE_PLAN.md` for the proposed Go/FLTK/libmpv resource-control experiment and its stop gates; no architecture change is approved by this plan
16. `docs/19_ADR_FLTK_WINDOWS_DESKTOP.md` for the owner-selected Windows replacement path and its still-open verification gates

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

## Transitional implementation baseline

- Go
- Windows amd64 uses FLTK with in-process libmpv under ADR-023. Fyne, Wails, Svelte, NSIS and external-MPV product code have been removed from the active tree. Linux and macOS development is paused until the Windows version is stable; release and resource gates remain open.
- SQLite
- MPV as the only MVP player
- In-process libmpv behind the core player port
- Anime1 as the initial anime source adapter
- AniList as primary metadata provider
- Bangumi as fallback / cross-check metadata provider
- Windows amd64 for the current phase; Linux and macOS are later phases
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

Loop Engineering Protocol v2.4.1 is mandatory beginning with Loop 23. Loops 01–22 are not replayed. After Loop 23 PASS and before Loop 24, they receive the one-time current-state retrospective baseline audit defined in `docs/11_RETROSPECTIVE_BASELINE_AUDIT.md` and tracked in `docs/RETRO_AUDIT_STATUS.md`.

A loop is complete only when the required deterministic, independent-review, and human gates defined in `docs/05_LOOP_ENGINEERING_RUNBOOK.md` and `docs/08_VERIFICATION_MATRIX.md` pass. The implementation agent's self-report is not a completion signal.

## Agent execution security

AI implementation must run inside a sandboxed workspace. The active local repository is the only persistent local project area writable by default; sandbox-owned temporary/cache state may exist only as disposable sandbox state. Host files, credentials, services, unrelated repositories, and privileged control sockets are outside the **mutation** boundary. Codex `workspace-write` is not a confidentiality/read-isolation guarantee: where the host itself must be unreadable, use an external isolation layer in addition to the Codex sandbox.

The local Codex sandbox is the preferred deterministic verifier, but Protocol v2.4.1 does not make one sandbox implementation the only acceptable verifier. If a stable sandbox capability limitation prevents a required deterministic check from running, GitHub-hosted CI (or another human-approved isolated CI environment) may provide authoritative evidence when it preserves the exact criterion. Unsandboxed host runs are diagnostic only, and Full Access/danger bypass is never an acceptance fallback. Platform-specific/native/human criteria cannot be replaced by a non-equivalent CI runner. See `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`.

The sandbox may retain repository-scoped GitHub connectivity so the agent can read PR checks, GitHub Actions results/logs, and CI artifacts for the connected repository. Remote writes are restricted to the connected repository and normal working-branch/PR operations; GitHub administration, workflow control, protected-branch writes, secrets/settings, releases/tags, or scope expansion require a human gate. Workflow/action definitions are protected verifier/security surfaces and require explicitly approved local scope before editing. See `docs/05_LOOP_ENGINEERING_RUNBOOK.md`.


## Durable agent state

`docs/AGENT_HANDOFF.md` stores only the compact active-loop checkpoint needed to recover after pause, session replacement, or context compaction. It is size-bounded and points to durable evidence instead of copying logs/diffs/docs. Historical decisions belong in ADRs/status/tests/Git, not in an ever-growing AI memory file.

## MVP completion definition

The MVP is complete only when all functional, architecture, code-quality, security, performance, resource-lifecycle, keyboard-accessibility, cross-platform, CI/release, and required loop-integrity criteria in `docs/06_ACCEPTANCE_CRITERIA.md` pass with evidence for the final state, including all required human gates.

## Retrospective baseline

Loop 23 was the protocol migration/Home loop and reached PASS. RA-01 through RA-06 subsequently reached `RETRO_BASELINE_PASS` at `3e15651e`. No standalone RA work remains; changed desktop boundaries are reverified in the Fyne migration under `docs/16_FYNE_MIGRATION_PLAN.md`.

`LEGACY_VERIFIED` means current code from a historical loop passes applicable present-day criteria; it does not claim the historical agent process complied with the current protocol.

Loop 24 begins the Fyne migration, not Search. Search resumes in Loop 29 only after the migration passes.

## Protocol research provenance

`docs/12_PROTOCOL_RESEARCH_NOTES.md` records the external sources used to validate Protocol v2.4.1. It is provenance/reference material, not part of normal per-session bootstrap, so agents should not read it unless protocol behavior itself is being audited or changed.
