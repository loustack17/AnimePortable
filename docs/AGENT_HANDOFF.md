<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 10](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.0"
status: IN_PROGRESS
branch: main
observed_head: see git log; CI speed-up (D3) merged to main  # check `gh run list --limit 1` for the latest result
active_work: >
  Loop 30 is next. CI speed-up (D3) is done: local native suite (plain, race, 20x repeat) passes in about 90 s;
  CI uses MSYS2 MINGW64 gcc without system upgrade plus pinned mesa-dist-win 26.2.4, about 3.3 min (run 38504108599).
active_acceptance: Loop 30 checks in docs/loops/LOOP30.md.
budgets: >
  D1, D2 and D3 are complete.
  Loop 30 corrections are counted in docs/loops/LOOP30.md section 7.
loop30:
  state: Loop 30 (anime detail and episode UI) is in progress, not PASS, and is resumed.
  owner_accepted: Player controls hide after idle time and reappear on input.
  owner_pending: Detail page, cover, episode click/Enter/Space, Continue, back/retry and minimum window size.
  open_problem: >
    Video stutters while audio is normal. Commit e8f0640 removes a frame-swap report that fired
    before the frame was shown, and adds a native test that requires at least 2 new frames.
    CI run 38444662617 for it is green. No improvement is claimed until the owner tests it.
  details: docs/loops/LOOP30.md
next_actions:
  - 1. Note for users: after migration 0003 an older app version refuses to open the database; keep a backup before downgrading.
  - 2. Resume Loop 30. Deliver the latest green CI build of main (it contains e8f0640), remove old binaries and caches (keep data/ and evidence), and ask the owner for one 30 to 60 second test of the same clip.
  - 3. After that test, judge the new diagnostic report with TypeSafe (de-identified numbers only) before any further fix. If there is no improvement, record that.
do_not_repeat:
  - Do not use the runner's UCRT C:\mingw64 gcc in CI; go-fltk libraries need msvcrt (CI run 38503790967 link failure).
  - Do not research libmpv licensing again; the owner settled it in ADR-023.
  - Do not tune playback options without a measured cause.
boundaries:
  - Keep .codex/, artifacts/ and portable data untouched.
  - Do not commit, push, change workflows, release, or start Loop 31 without owner approval.
references:
  - docs/STATUS.md
  - docs/loops/LOOP30.md
  - artifacts/evidence/loop30/
  - artifacts/evidence/docs-audit/
```
