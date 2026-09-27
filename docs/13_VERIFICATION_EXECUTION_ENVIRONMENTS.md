<!-- SPDX-License-Identifier: MPL-2.0 -->

# Verification Execution Environments and Fallback Policy

Protocol version: **v2.4.1**

This document defines which execution environments may produce authoritative verification evidence when the local Codex sandbox cannot execute a required verifier.

The objective is to preserve two independent properties:

1. **execution safety** — the coding agent must not gain broader host privileges merely to make a check run
2. **verification integrity** — a real required check must still run in an environment capable of exercising it

A failure of one sandbox implementation is not automatically a failure of the repository.

## 1. Evidence hierarchy

For deterministic verification, use the first capable environment in this order:

1. **Local Codex sandbox** using the approved project execution profile
2. **GitHub-hosted Actions runner** for this repository
3. Another **human-approved isolated CI runner/environment** with equivalent or stronger isolation and reproducible configuration

The following are never authoritative PASS evidence by themselves:

- unsandboxed execution on the user's normal host
- `danger-full-access`
- `--dangerously-bypass-approvals-and-sandbox`
- a shell/process started outside the approved execution boundary solely to bypass a sandbox restriction

Unsandboxed host execution may be used for diagnosis only, for example to distinguish a repository failure from a sandbox/runtime limitation.

## 2. Sandbox failure classes

Do not classify every command failure as a repository test failure.

### `SANDBOX_INFRA_FAILURE`

Use when the sandbox cannot initialize or cannot start even a minimal process.

Examples:

- sandbox setup/helper failure
- `CreateProcessAsUserW` failure before the requested command starts
- sandbox bootstrap/ACL/token initialization failure

Required response:

1. capture the exact failure fingerprint
2. run at most one minimal sandbox health check if it provides new evidence
3. do not repeatedly try alternate shells with the same process-creation fingerprint
4. do not switch to Full Access
5. checkpoint and use a supported sandbox mode or human-approved verifier environment

### `SANDBOX_CAPABILITY_LIMITATION`

Use when the sandbox works generally, but a specific capability required by a verifier is not available.

Examples:

- Node itself runs, while `child_process.spawn()` / `fork()` returns `EPERM`
- Chromium/helper startup fails because the sandbox blocks the required child process

`SANDBOX_CHILD_PROCESS_LIMITATION` is accepted as a more specific status label for this child-process case.

This is not `TEST_FAILURE` unless the same verifier also fails in a capable approved environment.

## 3. Failure-fingerprint circuit breaker

For a stable sandbox/runtime fingerprint:

- one failure establishes the initial observation
- one targeted health/reproduction check may be used to confirm the classification
- after the same fingerprint is confirmed, stop repeating equivalent commands, shells, paths, or agents

A retry is justified only by new evidence, a materially changed sandbox/runtime configuration, or an upstream version change.

Repeated attempts that cannot change the causal layer waste provider budget and can create misleading repository evidence.

## 4. GitHub-hosted CI as an approved verifier

GitHub documents standard hosted runner labels such as `ubuntu-latest` and `windows-latest`; standard hosted runner jobs execute on GitHub-managed runner infrastructure.

A GitHub-hosted run may satisfy a deterministic acceptance criterion when all of the following hold:

- the workflow belongs to the connected repository
- the run is tied to the exact commit/branch state being verified
- the verifier command and acceptance meaning are preserved
- required dependencies are repository-declared and reproducible; lockfiles/pinned versions are used where practical
- no test, assertion, threshold, fixture, repetition count, or failure behavior is weakened to make CI green
- the workflow uses least-privilege `GITHUB_TOKEN` permissions
- new secrets are not added merely to run the verifier
- the exact workflow run/job/check ID and conclusion are recorded
- the runner OS is capable of proving the criterion

A Linux Chromium check can prove platform-neutral browser/controller behavior. It cannot replace a criterion that explicitly requires native Windows desktop behavior, Windows process integration, MPV integration, visual/interaction judgment, or another platform-specific property.

## 5. Workflow-change protection

`.github/workflows/**` and `.github/actions/**` are protected verifier/security surfaces.

Before an agent edits them:

- the human must approve the exact intended local scope
- the agent must preserve existing checks/triggers/security behavior unless the approved task explicitly changes them
- the final workflow diff receives independent verifier/security review

Local permission to edit a workflow does **not** imply permission to:

- commit
- push
- open/update a PR
- rerun/cancel workflows
- modify repository Actions settings
- add/change secrets, variables, environments, branch protection, or repository permissions

Those remote actions remain governed separately by the runbook.

## 6. CI permissions

Prefer the minimum workflow token permissions required by the job.

For verifier-only jobs, `contents: read` or an even narrower effective permission set is normally sufficient unless the job genuinely requires another repository capability.

Do not grant `write-all`, broad administration access, secrets, or OIDC merely because a verifier moved to CI.

Observation of workflow runs/logs does not require workflow write authority. GitHub documents repository read access for viewing workflow-run history, and fine-grained workflow-run API access can use repository `Actions: read`.

## 7. Evidence record

When a fallback verifier is used, record:

```yaml
verification_environment:
  local_sandbox:
    result: CAPABILITY_LIMITATION
    classification: SANDBOX_CAPABILITY_LIMITATION
    fingerprint: "<stable error>"
  fallback:
    type: github_hosted_runner
    workflow: "<workflow path/name>"
    run_id: "<id>"
    job: "<job/check>"
    commit: "<sha>"
    runner: "<label>"
    result: PASS | FAIL
  host_diagnostic:
    performed: true | false
    authoritative: false
```

If CI fails, treat the CI failure as real verification evidence and classify it normally. Do not dismiss it merely because the local host diagnostic passed.

## 8. Windows Codex note

OpenAI's Windows Codex sandbox is an OS-process sandbox; it is distinct from Microsoft's optional Windows Sandbox VM feature.

OpenAI documents Windows sandbox setup modes named `elevated` and `unelevated`. The project's accepted mode is whichever human-managed configuration:

- is supported by the installed Codex build
- passes a minimal process health check
- preserves the repository write boundary
- does not require Full Access

Current `openai/codex` issue reports document Windows cases where Node child-process creation and Chromium/Computer Use helpers fail with `spawn EPERM` even though the parent Node process starts. These issue reports are operational evidence, not a guarantee of product behavior. They justify the capability-failure classification and circuit breaker; they do not justify weakening the sandbox.

## 9. Human gates remain human

An isolated CI substitute can replace only a deterministic verifier whose semantics it can reproduce.

It cannot replace:

- native UX judgment
- keyboard/mouse interaction judgment requiring the real app
- offline behavior that depends on the packaged/native runtime unless reproduced equivalently
- real MPV playback acceptance
- subjective visual quality
- an explicit platform-specific human criterion

## 10. References

Primary / normative product documentation:

- OpenAI — Running Codex safely at OpenAI: https://openai.com/index/running-codex-safely/
- OpenAI — Building a safe, effective sandbox to enable Codex on Windows: https://openai.com/index/building-codex-windows-sandbox/
- OpenAI Codex App Server — Windows sandbox setup modes: https://learn.chatgpt.com/docs/app-server
- GitHub — GitHub-hosted runners reference: https://docs.github.com/en/actions/reference/runners/github-hosted-runners
- GitHub — Viewing workflow run history: https://docs.github.com/en/actions/how-tos/monitor-workflows/view-workflow-run-history
- GitHub — Workflow-run REST API permissions: https://docs.github.com/en/rest/actions/workflow-runs
- GitHub — Workflow syntax / `GITHUB_TOKEN` permissions: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- GitHub — Secure use reference: https://docs.github.com/en/actions/reference/security/secure-use

Operational upstream evidence:

- openai/codex #21470 — Windows sandbox blocks Node child processes: https://github.com/openai/codex/issues/21470
- openai/codex #35070 — unelevated Windows sandbox child-process `EPERM`: https://github.com/openai/codex/issues/35070
- openai/codex #37272 — Windows Computer Use `spawn EPERM`: https://github.com/openai/codex/issues/37272
- openai/codex #37415 — Computer Use `spawn EPERM` / elevated setup failures: https://github.com/openai/codex/issues/37415

Community corroboration only:

- https://www.reddit.com/r/codex/comments/1v3ta5e/codex_on_windows_falls_back_to_the_unelevated/
- https://www.reddit.com/r/codex/comments/1vjt8qr/solutions_to_windows_sandbox_issues/

Community posts are not normative and must not be used to override official product/security behavior.
