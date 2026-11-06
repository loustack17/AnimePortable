<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 39: Core slimming

Status: NOT STARTED. Planned by the owner on 2026-11-06. Base: 6c99b05. Starting this loop needs the owner's go-ahead.

## Goal

The same app with less code. Only code is removed or merged; nothing is added.

## Hard rules

These rules beat any other reading of this file. If a change would break one, do not make it: write it under "Noticed, not done" and go on.

1. **Only listed work.** Change only what a row in "Approved slices" says. A row exists only after the owner approved it in chat. Phase A writes no code.
2. **Only these paths:** `core/`, `adapters/` (not `adapters/persistence/sqlite/migrations/`), `apps/desktop/backend/`, `internal/`, `tests/contract/`, and this file. Never touch `apps/desktop/fltk*`, `apps/desktop/*.go`, `apps/desktop/uitext/`, `tools/`, `.github/`, `go.mod`, `go.sum` or other docs (except STATUS.md and the handoff when closing).
3. **Nothing new.** No new package, production file, exported name, interface, type, dependency, setting, backend action, feature or abstraction. A file may be merged into another or deleted, never split. Test helpers may move only into an existing `_test.go` file.
4. **Every slice shrinks.** Each slice must lower the production line count of its package group, and must not raise its test line count. A slice that does not shrink is dropped, not reshaped into something else.
5. **Behaviour stays.** No change to UI text, saved data, network requests, security checks or errors the UI shows. The 03_SECURITY.md checks stay, even when they look redundant (see LOOP38R "Kept on purpose").
6. **Tests:** a test is deleted only when the owner approved its row and the row names the remaining test that still covers that behaviour. Never weaken, skip or special-case a test to get green.
7. **No 2.0 preparation.** Loop 40 (UI boundary) and Loop 41 (UI toolkit) are out of scope. Do not move code toward them.
8. **Stop and ask** when: a row turns out to need a file outside rule 2; a test fails twice after a fix; a row is unclear; the change would alter behaviour. Do not guess.
9. **One slice = one commit**, after the owner approves the commit. Do not start the next slice before that.

## Phase A: audit (read-only)

Output: the "Audit findings" table below, filled in. Nothing else.

For each package group, find:

- **Duplicate:** the same logic in two places.
- **Unused:** code, settings or branches no production path reaches (`deadcode`, as in LOOP38R).
- **Over-defended:** a check that guards nothing listed in 03_SECURITY.md, or that is repeated inside one trust boundary. Mark these "owner decides"; never cut a security check on your own reading.
- **Feature the owner may drop:** a whole behaviour, with its line count. Owner decides.
- **Repeated test:** two or more tests that check the same behaviour with the same kind of input; name the one to keep.

Each finding row: `#`, `file:line`, kind, what it is, proposed change, estimated net lines (production / test), risk (L/M).

Phase A ends with a short report to the owner: the table and the total estimate. **Stop there and wait.** The owner marks each row approve or reject. Only approved rows move to "Approved slices".

## Phase B: slices

Group the approved rows into slices, one package group each, in table order. For each slice:

1. Make the change.
2. Run the focused tests for the touched packages, then the full suite once and the race suite once (05_WORKFLOW.md section 5; owner rule: light testing, no repeated runs).
3. Measure lines (command below) and record before and after in the slice row.
4. Fresh read-only review (02_ARCHITECTURE.md quality rules plus these hard rules); fix every MUST_FIX.
5. Ask the owner to approve the commit.

## Measuring lines

```powershell
foreach ($d in 'core','adapters','apps\desktop\backend','internal') {
  $f = Get-ChildItem $d -Recurse -Filter *.go
  $p = ($f | Where-Object Name -notlike '*_test.go' | Get-Content | Measure-Object -Line).Lines
  $t = ($f | Where-Object Name -like '*_test.go' | Get-Content | Measure-Object -Line).Lines
  "$d production $p test $t"
}
```

Baseline at 6c99b05 (non-blank lines as `Measure-Object -Line` counts them, Go only):

| Group | Production | Tests |
| --- | --- | --- |
| `core` | 2,574 | 3,135 |
| `adapters/persistence/sqlite` | 1,770 | 2,999 |
| `adapters/source/anime1` | 1,429 | 1,450 |
| `adapters/metadata/*` | 1,597 | 2,646 |
| `adapters/network/*` | 839 | 1,229 |
| `adapters/playback/proxy` | 829 | 1,430 |
| `adapters/player/libmpv` | 413 | 380 |
| `adapters/update`, `adapters/internal` | 158 | 160 |
| `apps/desktop/backend` | 2,779 | 3,895 |
| `internal/*` | 512 | 205 |
| **Total** | **12,900** | **17,529** |

## Audit findings

Empty until Phase A.

## Approved slices

Empty until the owner approves rows.

## Noticed, not done

Anything outside the hard rules goes here with `file:line` and one sentence. It is not fixed in this loop.

## Done when

- Every approved row is done or listed as dropped with the reason.
- Production lines in scope are lower than the baseline; test lines are not higher.
- Full suite and race suite pass once on the final state; CI is green for the last commit.
- The owner's normal-session check of a test build in `artifacts/test-build/` passes (open Home, search, open a detail page, play an episode, check History and Settings).
