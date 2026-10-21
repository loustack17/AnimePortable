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
  None. Loop 35 (Cache-first startup and refresh) passed on 2026-10-21 (docs/loops/LOOP35.md; 8bb524b, pushed with
  the resize hotfix 0371f05 and Loop 34b 18b6c6a in ef97b6c; CI Go 1.26.10 in 92868eb; CI 39532702396 green).
  Loop 36 (Autoplay next episode) is next and needs owner approval.
active_acceptance: none
next_actions:
  - 1. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - 2. Ask the owner before starting Loop 36.
  - Done 2026-10-21: Loop 34b (follow from 歷史紀錄 and the player) PASS (docs/loops/LOOP34B.md). CI flake in TestPlayerControlsRenderAfterEngineReentry fixed in 828028f: the 3 s fixture reached EOF before the seek on slow CI and mpv unloaded it; the test now uses a 60 s fixture (CI 39564341290 green).
  - Open owner choices: 追蹤 更新集數 wording; remove unused NewEpisode (docs/STATUS.md); Home 今日播出 from the stored schedule.
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
  - Never add offline playback or video downloads (owner, 2026-10-21): playback is online only; offline shows stored lists only.
  - Do not add refresh limits, timeouts or throttles in a screen; use core.Refresher (Loop 35). Home makes no network call.
  - Do not use a playback fixture that can reach EOF before a test's last seek; a slow CI runner unloads it (fixed 828028f).
  - Do not chain `| Select-Object -First N` after `Tee-Object` for evidence: it stops the pipeline and truncates the file.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Commit, push, workflow and release need owner approval.
references:
  - docs/loops/LOOP35.md
  - artifacts/evidence/loop35/
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
