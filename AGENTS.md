<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable — agent instructions

Instruction version: 3.1 (2026-10-14)

These rules apply to every AI agent or human working in this repository, whatever tool or model they use. Deeper `AGENTS.md` files may add stricter rules, never weaker ones.

## Start every session

1. Read `docs/AGENT_HANDOFF.md`.
2. Run `git status --short --branch`, `git log -5 --oneline`, `git diff --stat`. Git wins over the handoff.
3. Read only what the next action needs. `docs/README.md` says which doc covers what.
4. Do not work from old chat summaries.

The full process is [docs/05_WORKFLOW.md](docs/05_WORKFLOW.md). It overrides anything shorter here.

## Hard rules

- Work only on the active loop's scope. One coherent slice per loop.
- Persistent writes only inside this repository. No global installs, sudo, host config, credential stores or other repositories. Never widen permissions to make something pass; report `ENVIRONMENT_BLOCKED` instead.
- Commit, push, workflow edits (`.github/**`), CI reruns, releases, new dependencies, and changes to product, architecture, security or acceptance meaning need explicit owner approval first. Default push target is `main`.
- Never weaken, skip or special-case a test, check or criterion to get PASS.
- On failure: classify, diagnose with evidence, make the smallest causal fix, rerun the focused check, then regressions. After three failed attempts at the same problem, stop and report to the owner.
- PASS needs final-state evidence: required checks, independent code-quality review with no `MUST_FIX`, and the owner's check when required.
- Run the full native suite locally before pushing (`CGO_ENABLED=1 go test -count=1 ./...`, then `-race`, with an msvcrt MinGW gcc such as MSYS2 MINGW64 or nuwen MinGW on PATH). GitHub Windows CI confirms the build and the tests that need no window or libmpv for the pushed commit; the local run is the only run of the window and playback tests.
- Preserve unrelated user files (`.codex/`, portable `data/`, `artifacts/` evidence).

## Roles

The root agent owns the plan, integration, final verification and the handoff. It may delegate bounded, independent work to: explorer (read-only search), librarian (external docs), oracle (independent read-only review and debugging), fixer (implementation with a stated write set), designer (UI work). Workers get disjoint write sets, never start agents, and never edit the handoff. Reviewers are read-only and never approve their own code. Delegate only when it saves usage or adds independence.

## TypeSafe

Use TypeSafe for every judgment step it can answer (classification, triage, scope and coverage checks, owner-reply reading, doc quality). Use tests, builds and exact commands for facts. Run it with `tools/typesafe-judge/run.ps1`; the API key comes from Bitwarden Secrets Manager into that process only. Details, templates and thresholds: [docs/08_TYPESAFE.md](docs/08_TYPESAFE.md). If TypeSafe is unavailable (no key, `bws` failure, API outage), report `ENVIRONMENT_BLOCKED` for that judgment step and tell the owner; do not silently replace it with free-form judgment. A TypeSafe answer never replaces a test, a review or the owner's check.

## Code quality

Minimal Ports & Adapters: `UI -> Core -> Adapters`, dependencies point inward, core never imports concrete infrastructure. Clear names, straightforward control flow, code in the narrowest correct package, no `utils`/`helpers`/`common`/`misc`, no duplicated policy, no dead or debug code, no speculative abstraction. Comments explain why, not what. See [docs/02_ARCHITECTURE.md](docs/02_ARCHITECTURE.md).

## Settled owner decisions (do not reopen unless the owner does)

- The player has on-video controls that auto-hide (the product is inspired by OneAnime).
- The portable ZIP bundles the SHA-256-pinned libmpv DLL with a disclosure notice. Licensing is decided (ADR-023); do not research it again.
- Windows first; Linux and macOS later.

## Before pausing

Checkpoint `docs/AGENT_HANDOFF.md` (max 8 KiB / 160 lines) and report exact commands, results and open issues.
