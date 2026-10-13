<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

Recovery index only. Git, CI, tests and the linked docs override it. Rules for this file: [05_WORKFLOW.md section 10](05_WORKFLOW.md).

```yaml
schema_version: 2
instruction_version: "3.0"
status: IN_PROGRESS
branch: main
observed_head: f790dfc  # docs restructure; the legacy cleanup commit follows it (run git log)
active_work: >
  The docs restructure (D1) and the legacy cleanup (D2) are committed locally and not pushed.
  Pushing needs owner approval; GitHub Windows CI must then pass for the pushed commit.
active_acceptance: The checks listed in docs/loops/LEGACY_CLEANUP.md.
budgets: >
  D1 is complete. D2 corrections used: 1 of 5.
  Loop 30 corrections are counted in docs/loops/LOOP30.md section 7.
loop30:
  state: Loop 30 (anime detail and episode UI) is in progress, not PASS, and is paused until the docs commit.
  owner_accepted: Player controls hide after idle time and reappear on input.
  owner_pending: Detail page, cover, episode click/Enter/Space, Continue, back/retry and minimum window size.
  open_problem: >
    Video stutters while audio is normal. Commit e8f0640 removes a frame-swap report that fired
    before the frame was shown, and adds a native test that requires at least 2 new frames.
    CI run 38444662617 for it is green. No improvement is claimed until the owner tests it.
  details: docs/loops/LOOP30.md
next_actions:
  - 1. Docs restructure is committed (f790dfc) and legacy cleanup is committed after it (docs/loops/LEGACY_CLEANUP.md). Ask the owner to approve the push, then confirm GitHub Windows CI is green for the pushed commit; classify any failure with the ci-log template.
  - 2. Note for users: after migration 0003 an older app version refuses to open the database; keep a backup before downgrading.
  - 3. Resume Loop 30. Deliver the CI-built exe and launcher of e8f0640, remove old binaries and caches (keep data/ and evidence), and ask the owner for one 30 to 60 second test of the same clip.
  - 4. After that test, judge the new diagnostic report with TypeSafe (de-identified numbers only) before any further fix. If there is no improvement, record that.
do_not_repeat:
  - Do not run the local native cgo build; it is a known environment failure.
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
