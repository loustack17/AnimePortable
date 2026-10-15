<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 9](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.1"
status: PASS
branch: main
observed_head: see git log  # check `gh run list --limit 1` for the latest CI result
active_work: >
  None. Loop 31 passed on 2026-10-14; Home 近期更新 follow-up f7dffd5 on 2026-10-15 (docs/loops/LOOP31.md, CI 38681263489). Loop 32 (History and Continue
  Watching) is next and needs owner approval to start.
active_acceptance: none
next_actions:
  - 1. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - 2. v0.1.0-alpha.1 passed the owner check. Ask the owner before starting Loop 32.
  - Releases: push a v* tag with notes in docs/releases/<tag>.md; CI packages and publishes a pre-release (owner approval each time).
  - 3. When a followed anime gets a new episode, ask the owner to confirm a live Home 近期更新 row (open it, Back returns Home).
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
