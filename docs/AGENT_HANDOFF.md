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
  None. Loop 37d (Reliable starts and more synopses) passed on 2026-10-23 (docs/loops/LOOP37D.md; de3c9ed; CI 39917415783 green): stored episode list first (backend action StoredEpisodes), Anime1 page timeout + retry, 20 s start check, migration 0011, synopsis matching 193/228 with 0 wrong.
  Release v0.4.1-beta (patch, owner request 2026-10-23) in progress: release commit pushed to main; tag after its CI is green; then check the published ZIP.
  Loop 37c (Skip the opening) passed 2026-10-22; v0.4.0-beta published 2026-10-22. Next loop: 38 (hardening and release, v1.0.0); needs owner approval to start.  Never drive the owner's desktop to investigate (owner 2026-10-22); reproduce headlessly.
  Loop 37c (Skip the opening) passed on 2026-10-22 (docs/loops/LOOP37C.md; af53275; CI 39877272016 green): AniSkip 跳過片頭, new action OpeningTimes, builds with -H windowsgui (no console window).
  Pre-release v0.4.0-beta published 2026-10-22 (tag on 2caceaa; CI 39880655456; release job 39880999897): 37b, 37c, app icon (apps/desktop/appicon, rsrc_windows_amd64.syso), no console. Owner to try it.
  Loop 37b (Language) passed on 2026-10-22 (docs/loops/LOOP37B.md; 17d13be; CI 39845099717 green): live switch, uitext table, migration 0010.
  Loop 37 (Settings) passed on 2026-10-22 (docs/loops/LOOP37.md; f582da8 + fixes 1966449, f2c4f51; CI 39734539484 green).
  Pre-release v0.3.0-alpha published 2026-10-22 (tag on b005d3a; release job 39738301609 reused the CI-tested exe; DLL hash matches the pin).
  Next loop: 38 (hardening and release, v1.0.0); needs owner approval to start.
active_acceptance: none
next_actions:
  - Migrations 0007-0011 are applied to the owner's test data; never edit them.
  - New UI text goes into apps/desktop/uitext (all three languages); a fixed label uses labelled, a status a localText, so a live language switch reaches it.
  - A new FLTK window outside fltkhome newDialog must close leaked groups at the source (see LOOP37 final review).
  - Build exes with -ldflags "-H windowsgui" (test builds too); CI fails on a console-subsystem exe.
  - Ask before any GUI test run while the owner may be using the PC (owner request 2026-10-22).
  - 2. After each push, confirm GitHub Windows CI for that commit; classify any failure with the ci-log template.
  - Open owner choices: 追蹤 更新集數 wording; remove unused NewEpisode (docs/STATUS.md); Home 今日播出 from the stored schedule.
  - Releases: bump internal/release.Version + notes in docs/releases/<tag>.md, push to main, wait for CI, then push the v* tag; the release job reuses that run's exe (owner approval each time). CI is small by owner decision: no window or libmpv tests there; run them locally (ask the owner first).
  - When a followed anime gets a new episode, ask the owner to confirm a live Home 近期更新 row (open it, Back returns Home).
  - If playback-failures.log shows repeated proxy_upstream 502, propose one bounded automatic retry to the owner.
do_not_repeat:
  - Do not remember a MyAnimeList-id miss that searched only the Anime1 (Chinese) title; metadata stored later may find it (Loop 37c review).
  - Do not hard-code versions newer than the current one in tests; derive them from release.Version (v0.4.0-beta release commit failed CI, Loop 37c).
  - Do not slice a uitext array into storage that is later appended to; it writes into the shared table (Loop 37b review). Copy with slices.Clone.
  - Do not destroy an FLTK group or window whose children have draw handlers: go-fltk's deletion handler unregisters the draw id from the callback map and a later callback crashes at address 0 (Loop 37 CI). Destroy children one by one first (dialogs), or hide (test teardown).
  - Do not use the runner's UCRT C:\mingw64 gcc in CI; go-fltk libraries need msvcrt.
  - Do not research libmpv licensing again; settled in ADR-023.
  - Do not enable mpv log-file in production; it records the proxy session token.
  - Do not remove fltk.Lock() from main; without it libmpv frame notifications are delayed and video stutters.
  - Do not use a Go OpenCC library; they link GPL-2.0 cedar-go. Use the embedded Apache OpenCC dictionaries.
  - Do not show Simplified Chinese; convert Bangumi text with s2twp. Japanese-only synopses stay unconverted.
  - Do not add backend actions without owner approval; main_test.go allowlist is fixed (CachedSchedule approved 2026-10-20, StoredEpisodes 2026-10-23).
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
  - docs/loops/LOOP37D.md
  - docs/loops/LOOP37C.md
  - docs/loops/LOOP37B.md
  - docs/loops/LOOP37.md
  - artifacts/evidence/loop37/
  - docs/loops/LOOP36.md
  - docs/loops/LOOP35.md
  - docs/loops/LOOP34B.md
  - docs/STATUS.md
```
