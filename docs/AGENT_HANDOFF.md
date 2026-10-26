<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 9](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.1"
status: IN_PROGRESS
branch: main
observed_head: see git log  # check `gh run list --limit 1` for the latest CI result
active_work: >
  Loop 38R (cleanup before 2.0, owner request 2026-10-25): audit done, plan in docs/loops/LOOP38R.md (slices S1-S11, owner decisions O1-O8), oracle-checked, TypeSafe doc-quality in artifacts/evidence/loop38r/ of worktree relaxed-montalcini-e848e2. Slice list approved 2026-10-25. S1 (5d60093), S2 (5fbd5f3), S3 (47bf4e8), S4 (01982a5, CI 40232435530) committed, CI green. S5 (SQLite, rows 5.2-5.6) committed 6d0e064, CI 40258238743 green; 5.1 waits for owner decision O4. S6 (core) committed c7e976d, CI 40260054936 green. S7 (backend) done and checked, uncommitted, waiting for owner approval to commit and push; next S8 (main package). Evidence for 38R is in artifacts/evidence/loop38r/ of worktree relaxed-montalcini-e848e2 (copy it to the main checkout before removing that worktree). Known flake: fltkhome window tests sometimes time out ("FLTK event did not reach the expected Search state") in the normal full run only; rerun passes. One slice per commit, owner approval before each commit and push.
  Loop 38 (hardening and release) PASSED 2026-10-24 (docs/loops/LOOP38.md; v1.0.0 published, tag on 5d9c068).
  Next after 38R: version 2.0 (Loops 39-46, docs/04_ROADMAP.md) needs owner approval to start.
  Open owner decisions: update check offers pre-releases to users on 1.0.0; 03_SECURITY close order (N8); numbering of versions after 1.0.0.
  Loop 37d passed 2026-10-23 (LOOP37D.md); v0.4.1-beta published 2026-10-23. Never drive the owner's desktop to investigate; reproduce headlessly.
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
  - Real-libmpv headless tests need artifacts/runtime/libmpv-2.dll in the worktree (copy from main checkout artifacts/test-build) or they download it.
do_not_repeat:
  - Do not let mpv probe the stream format: a video/mp4-labelled playlist made libmpv fetch listed URLs past the proxy (Loop 38a). Keep demuxer=lavf, demuxer-lavf-format=mp4.
  - Do not set mpv options the pinned DLL lacks (ytdl, osc): no Lua in it, and an unknown option fails player creation.
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
  - docs/loops/LOOP38R.md
  - docs/loops/LOOP38.md
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