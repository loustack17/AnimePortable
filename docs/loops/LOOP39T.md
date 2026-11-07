<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 39T: Tests follow real usage

Status: IN PROGRESS. Approved by the owner in chat on 2026-11-06. Runs before Loop 39, so Loop 39 does not pay for the old suite.

## Goal

Tests check the project, not themselves. Each module keeps a few scenario tests that walk what users really do, or what really happens to the app from outside. Everything else is deleted.

## Rules (owner, 2026-11-06)

1. **Keep** a test only if its case happens in real use: the user flow (Home, search, detail, play, switch episode, continue, History, Following, Schedule, Settings, update check) or a real outside event (offline, source site error or change, wrong proxy, failed data move, database upgrade, a real attack on the security checks).
2. **Delete** tests of cases that cannot happen (nil context, impossible values or timing), one test per button, text line or language, stress loops, and tests that exist only for coverage.
3. **Merge** what remains into a few scenario tests per module; one test walks a whole flow.
4. **Windows:** a full run opens at most two windows: one Home window and one player window with real libmpv. Every window check runs inside them.
5. **No `-race` run.** The full suite runs once.
6. Behaviour and production code do not change; only tests, test helpers and test-only packages.

## Slices

| Slice | Content | State |
| --- | --- | --- |
| S1 | Workflow rule: no `-race`, real-usage rule (AGENTS.md, 05_WORKFLOW.md). Delete the five resource-pass stress tests and the then-unused `internal/resourcepass`. | done (5127904) |
| S2 | Window packages (`fltkhome`, `fltkplayer`, `fltkengine`): one shared Home window and one player window per run; logic checks move to headless tests; delete the rest. Done as one walk per window: `TestHomeRealUse` (fltkhome) and `TestPlayerRealUse` (fltkengine); fltkplayer keeps only headless tests; the proxy child plays headless. | done, run once (both walks and the proxy check pass), not committed |
| S3 | Adapters (`anime1`, metadata, network, playback, persistence). | open |
| S4 | `apps/desktop/backend`, `apps/desktop`, `internal`, `tools`, `tests`. | open |
| S5 | `core` (after Loop 39's uncommitted work is settled, since both touch `core/*_test.go`). | open |

Each slice: list keep, merge or delete per test, the owner approves the list, then the change, one focused run, one commit after owner approval.

## S2 coverage dropped (owner rule: not real use, or not worth a window)

- Per-button, per-language and per-text checks; focus-order and generation-race checks; dialog keys and layout (dialogs open no window in tests now); data-location move, proxy test, update check rows; startup error and settings-load-failure screens; schedule cache-first order; cover decoding.
- Player: native media keys and Space/Enter through the window procedure, fullscreen subclass, star/speed/skip-intro click-and-draw, overlay viewport across reentry; 30-cycle memory growth; opt-in playback diagnostics report.

## Noticed, not done

- `docs/loops/LOOP39.md` still names the resource pass and the old test counts; update it when Loop 39 resumes.
