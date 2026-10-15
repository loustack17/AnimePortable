<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 9](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.1"
status: IN_PROGRESS
branch: claude/funny-lalande-549e51 (worktree, uncommitted)
observed_head: see git log  # check `gh run list --limit 1` for the latest CI result
active_work: >
  Loop 31 (Following), approved 2026-10-14. Built and reviewed, uncommitted; test build in artifacts/test-build/.
  Owner check passed 2026-10-14 (docs/loops/LOOP31.md). Waiting for commit and push approval, then CI.
active_acceptance: docs/loops/LOOP31.md
next_actions:
  - 1. With owner approval, commit and push Loop 31; then mark LOOP31.md PASS and update STATUS.md.
  - 2. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - 3. Pre-existing flaky failurelog tests (dropped write on Windows rename) block a fully green local race; propose a fix to the owner.
  - 4. If playback-failures.log shows repeated proxy_upstream 502, propose one bounded automatic retry to the owner.
do_not_repeat:
  - Do not use the runner's UCRT C:\mingw64 gcc in CI; go-fltk libraries need msvcrt.
  - Do not research libmpv licensing again; settled in ADR-023.
  - Do not enable mpv log-file in production; it records the proxy session token.
  - Do not remove fltk.Lock() from main; without it libmpv frame notifications are delayed and video stutters.
  - Do not use a Go OpenCC library; they link GPL-2.0 cedar-go. Use the embedded Apache OpenCC dictionaries.
  - Do not show Simplified Chinese; convert Bangumi text with s2twp. Japanese-only synopses stay unconverted.
  - Do not add a backend action for Following refresh; main_test.go allowlist is fixed. The UI refreshes via Episodes.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Commit, push, workflow and release need owner approval.
references:
  - docs/loops/LOOP31.md
  - docs/loops/LOOP30.md
  - docs/STATUS.md
  - artifacts/evidence/loop31/
```
