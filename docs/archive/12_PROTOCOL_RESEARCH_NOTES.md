<!-- SPDX-License-Identifier: MPL-2.0 -->

# Protocol v2.4.1 External Validation Notes

Validated: **2026-09-12**

This file records why the execution/audit controls in Protocol v2.4.1 are considered feasible. It is reference/provenance material and is **not** normal Codex bootstrap context.

## Evidence hierarchy

1. Current OpenAI/Codex, GitHub, NIST, OWASP, OpenSSF, and Google official documentation.
2. OpenAI `openai/codex` GitHub implementation/issues for operational failure modes.
3. Community reports for workflow friction and corroboration only.

Community reports and open issues never override current official behavior or repository contracts.

## Verified Codex facts used by this protocol

- Applicable `AGENTS.md` guidance is assembled once when a Codex run/session starts. Project discovery begins at the project/Git root and walks toward the current directory. Combined project instruction content defaults to a 32 KiB cap.
- Current Codex releases support subagents. Subagents consume additional tokens; official guidance recommends starting parallelism with read-heavy exploration/tests/triage/summarization and using more care for parallel write-heavy work.
- Subagents inherit the parent turn's effective sandbox/permission overrides unless deliberately narrowed. Project-scoped custom agents under `.codex/agents/` are supported; `sandbox_mode = "read-only"` is a supported narrowing.
- `[agents].max_concurrent_threads_per_session` is an official concurrency control. Protocol v2.4.1 caps it at 4 and does not hard-code a model so the user's active Sol/Astra selection can flow through.
- Git worktrees are an official Codex parallel-work primitive. They isolate checkouts/branches but are not a security sandbox.
- Codex CLI `/status` reports the effective model, approval policy, writable roots, and current token usage. `/debug-config` can explain effective config layers.
- OpenAI's current Windows sandbox design intentionally gives broad read access comparable to the actual user while restricting writes to allowed roots. Therefore `workspace-write` is treated here as write containment, not strict confidentiality/read isolation.
- GitHub Actions run status/logs can be inspected with repository read access; fine-grained workflow-log access supports `Actions: read`; `gh run view --log-failed` is a supported diagnostic command.


## Verified execution-environment fallback basis

Primary documentation supports the security model:

- OpenAI states that sandboxing defines the technical execution boundary while approvals govern requests that cross it. OpenAI's internal Codex guidance constrains sandbox modes and network access instead of treating Full Access as the normal repair path.
- OpenAI's Windows sandbox design is implemented with Windows process/token/ACL primitives and explicitly describes an unelevated prototype plus the newer elevated design. This is separate from Microsoft's Windows Sandbox VM feature.
- The Codex app-server documentation exposes Windows sandbox setup modes named `elevated` and `unelevated`.
- GitHub documents standard GitHub-hosted runner labels, including Linux and Windows hosted environments.
- GitHub documents that repository read access can view workflow-run history/logs; the workflow-run REST API supports fine-grained `Actions: read` for read operations.
- GitHub's workflow syntax and security guidance support explicit least-privilege `GITHUB_TOKEN` permissions.

Operational upstream evidence supports the circuit breaker:

- `openai/codex` #21470 reproduces Node itself running while Node child-process creation returns `EPERM`, affecting Chromium/esbuild/ffmpeg-style workflows.
- `openai/codex` #35070 reproduces the same `child_process.spawn/fork` limitation under the Windows unelevated sandbox.
- `openai/codex` #37272/#37415 document Windows Computer Use/helper `spawn EPERM` and elevated-sandbox setup failures.

These open issues are not normative product contracts. Protocol v2.4.1 uses them only to distinguish sandbox/runtime capability failures from repository failures and to stop repeated non-causal retries.

Community reports independently describe similar Windows sandbox failures and unelevated workarounds. They are corroboration only; no security rule is based solely on community advice.

## Verified retrospective-audit basis

- OWASP Secure Code Review explicitly distinguishes comprehensive **baseline reviews** from diff-based reviews and lists legacy-system onboarding as a baseline-review use case.
- NIST SSDF is outcome/risk oriented and emphasizes secure verification and addressing root causes to prevent recurrence. Protocol v2.4.1 uses current-state evidence and root-cause repair rather than replaying historical implementation.
- OpenSSF OSPS Baseline requires automated tests in CI before changes are accepted and, at higher maturity, documented test operation and non-author human approval for primary-branch merge.
- Google Engineering Practices expects review of design, functionality, complexity, tests, readability, and maintainability while explicitly avoiding perfectionism that blocks progress. This supports MUST_FIX vs ADVISORY rather than unlimited retrospective cleanup.

## Operational signals that justify defensive durable state

Open `openai/codex` issues in 2026 report cases where compaction/resume can restore stale task state, lose recent context, or cause repeated discovery. Community reports similarly describe using small state files plus Git/worktrees to recover across sessions. These are not specifications; they justify checkpoint/reconciliation controls because those controls fail safely even when compaction works correctly.

## Primary references

- https://developers.openai.com/codex/guides/agents-md
- https://developers.openai.com/codex/subagents
- https://developers.openai.com/codex/app/worktrees
- https://developers.openai.com/codex/cli/reference
- https://openai.com/index/running-codex-safely/
- https://openai.com/index/building-codex-windows-sandbox/
- https://learn.chatgpt.com/docs/app-server
- https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- https://docs.github.com/en/actions/reference/security/secure-use
- https://docs.github.com/en/actions/how-tos/monitor-workflows/view-workflow-run-history
- https://docs.github.com/en/rest/actions/workflow-runs
- https://cli.github.com/manual/gh_run_view
- https://cheatsheetseries.owasp.org/cheatsheets/Secure_Code_Review_Cheat_Sheet.html
- https://csrc.nist.gov/pubs/sp/800/218/final
- https://baseline.openssf.org/versions/2026-08-28
- https://google.github.io/eng-practices/review/

## Secondary references

Examples of current operational reports considered during design:

- https://github.com/openai/codex/issues/27731
- https://github.com/openai/codex/issues/25900
- https://github.com/openai/codex/issues/25394
- https://github.com/openai/codex/issues/21470
- https://github.com/openai/codex/issues/35070
- https://github.com/openai/codex/issues/37272
- https://github.com/openai/codex/issues/37415
- https://www.reddit.com/r/codex/comments/1v3ta5e/codex_on_windows_falls_back_to_the_unelevated/
- https://www.reddit.com/r/codex/comments/1vjt8qr/solutions_to_windows_sandbox_issues/

Community discussions were used only as corroborating workflow signals.
