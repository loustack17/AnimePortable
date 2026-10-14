<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 10](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.0"
status: IN_PROGRESS
branch: main
observed_head: see git log  # check `gh run list --limit 1` for the latest CI result
active_work: >
  Loop 30. The video stutter root cause (missing fltk.Lock) is fixed and confirmed smooth by the owner on 2026-10-14;
  see docs/loops/LOOP30.md sections 3 and 4. Playback-start failures now show a dialog and are recorded in
  playback-failures.log. Remaining: owner checks in LOOP30.md section 6.
active_acceptance: Loop 30 IDs in docs/loops/LOOP30.md section 1.
budgets: >
  Loop 30 corrections are recorded in docs/loops/LOOP30.md section 7.
next_actions:
  - 1. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - 2. Deliver the latest green CI build to artifacts/test-build/ (replace it; keep data/), then ask the owner for the section 6 checks.
  - 3. If playback-failures.log shows repeated proxy_upstream 502, propose one bounded automatic retry to the owner.
do_not_repeat:
  - Do not use the runner's UCRT C:\mingw64 gcc in CI; go-fltk libraries need msvcrt.
  - Do not research libmpv licensing again; settled in ADR-023.
  - Do not enable mpv log-file in production; it records the proxy session token.
  - Do not remove fltk.Lock() from main; without it libmpv frame notifications are delayed and video stutters.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Commit, push, workflow, release and Loop 31 need owner approval.
references:
  - docs/loops/LOOP30.md
  - docs/STATUS.md
  - artifacts/evidence/loop30/
```
