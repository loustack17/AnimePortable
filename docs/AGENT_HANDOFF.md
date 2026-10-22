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
  None. Loop 37 (Settings) passed on 2026-10-22 (docs/loops/LOOP37.md; check CI for the push commit).
  Next loops: 37b (language: zh-TW default, English, Japanese) and 37c (skip the opening with AniSkip), then 38; each needs owner approval to start.
active_acceptance: none
next_actions:
  - 1. Confirm GitHub Windows CI for the Loop 37 push; classify any failure with the ci-log template.
  - Migrations 0007-0009 are applied to the owner's test data; never edit them.
  - Loop 37c: AniSkip origin api.aniskip.com needs a 03_SECURITY section and owner approval; a new FLTK window outside fltkhome newDialog must close leaked groups at the source (see LOOP37 final review).
  - Ask before any GUI test run while the owner may be using the PC (owner request 2026-10-22).
  - 2. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - Open owner choices: console window next to the app (exe built as a console program; -H windowsgui?); 追蹤 更新集數 wording; remove unused NewEpisode (docs/STATUS.md); Home 今日播出 from the stored schedule.
  - Releases: push a v* tag with notes in docs/releases/<tag>.md; CI packages and publishes a pre-release (owner approval each time).
  - When a followed anime gets a new episode, ask the owner to confirm a live Home 近期更新 row (open it, Back returns Home).
  - If playback-failures.log shows repeated proxy_upstream 502, propose one bounded automatic retry to the owner.
do_not_repeat:
  - Do not destroy an FLTK group or window whose children have draw handlers: go-fltk's deletion handler unregisters the draw id from the callback map and a later callback crashes at address 0 (Loop 37 CI). Destroy children one by one first (dialogs), or hide (test teardown).
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
  - Never add offline playback or video downloads (owner, 2026-10-21): playback is online only; offline shows stored lists only.
  - Do not add refresh limits, timeouts or throttles in a screen; use core.Refresher (Loop 35). Home makes no network call.
  - Do not use a playback fixture that can reach EOF before a test's last seek; a slow CI runner unloads it (fixed 828028f).
  - Do not chain `| Select-Object -First N` after `Tee-Object` for evidence: it stops the pipeline and truncates the file.
  - Do not key per-end or per-load guards in the UI by load generation alone; it restarts at 1 in every playback session (Loop 36 review). Use viewGeneration.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Commit, push, workflow and release need owner approval.
references:
  - docs/loops/LOOP37.md
  - artifacts/evidence/loop37/
  - docs/loops/LOOP36.md
  - docs/loops/LOOP35.md
  - docs/loops/LOOP34B.md
  - docs/STATUS.md
```
