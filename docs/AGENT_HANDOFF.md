<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 9](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.1"
status: OWNER_CHECK_PENDING
branch: main
observed_head: see git log  # check `gh run list --limit 1` for the latest CI result
active_work: >
  Loop 34 (Keyboard navigation everywhere, docs/loops/LOOP34.md) implemented in worktree hungry-kowalevski-1f0f72,
  not committed. Suite and race green, review has no MUST_FIX, owner check PASS 2026-10-20.
active_acceptance: docs/loops/LOOP34.md (owner check steps 1-7)
next_actions:
  - 1. Owner check passed. Confirm the Ctrl+K player test change with the owner; then ask to commit and push to main and confirm Windows CI.
  - 2. After PASS, move Loop 34 to the completed table in docs/04_ROADMAP.md and update its header count. Open owner choices: 追蹤 更新集數 wording; remove unused NewEpisode (docs/STATUS.md).
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
  - Do not add backend actions without owner approval; main_test.go allowlist is fixed (CachedSchedule was approved 2026-10-20).
  - Never change a migration already applied to the owner's test data; the checksum check then refuses the database (happened 2026-10-20, repaired; broken copy in artifacts/evidence/loop33/).
  - Do not let Esc reach FLTK's default on the main window: unhandled Esc runs the window callback and closed the app (fixed Loop 34 by isEscapeClose).
  - Do not take the schedule from Anime1 (owner, 2026-10-20): its season pages lag; the schedule is Bangumi's calendar.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Commit, push, workflow and release need owner approval.
references:
  - docs/loops/LOOP34.md
  - docs/loops/LOOP33.md
  - docs/loops/LOOP32.md
  - docs/loops/LOOP31.md
  - docs/loops/LOOP30.md
  - docs/STATUS.md
  - artifacts/evidence/loop31/
  - artifacts/evidence/loop32/
  - artifacts/evidence/loop33/
```
