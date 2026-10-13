<!-- SPDX-License-Identifier: MPL-2.0 -->

# Legacy cleanup (D2)

A maintenance loop with no product loop number. Loop 30 stays paused until it is committed.

## Contract

- Owner direction (2026-10-13): all legacy must be removed; the project stays simple, clean, maintainable and readable. `docs/archive/` is kept.
- Baseline: `main` at `f790dfc`.
- In scope:
  1. Remove the unused `github.com/Microsoft/go-winio` module requirement.
  2. Delete empty placeholders `testdata/.gitkeep` and `tests/integration/.gitkeep`, and the unused icons `apps/desktop/build/appicon.png` and `apps/desktop/build/windows/icon.ico`.
  3. Remove the dead `APP_NAME` Taskfile variable and the unused `-tags production` build flag everywhere it appears, including `.github/workflows/ci.yml` (owner approved this workflow edit on 2026-10-13).
  4. Remove the unused strict packaging mode from `tools/portable-package` (owner approved).
  5. Drop the SQLite `settings.mpv_path` column with a new migration that keeps every other setting (owner approved).
  6. Update docs that mention removed items.
- Out of scope: product behavior, UI, player, CI test steps other than the build flag, dependency upgrades.
- Acceptance: QUAL-008, QUAL-009, QUAL-011, ARCH-001..005, CI-001, CI-002, CI-007, CI-008, CI-010; plus no criterion regresses.
- Verification:
  - Local (CGO disabled): `gofmt -l`, `go vet` and `go test -count=1` for `./core ./adapters/... ./apps/desktop/backend ./tests/... ./tools/...`; `go mod verify`.
  - Migration test: an old database that still has `mpv_path` opens, migrates, and keeps its other settings.
  - Native Windows build and tests: GitHub Windows CI on the pushed commit (push needs owner approval).
  - Independent code-quality review and a workflow security review of the `ci.yml` diff.
- Budget: 5 corrective iterations, 2 per failure fingerprint.

## Results

| Check | Result |
| --- | --- |
| Local gofmt, vet, tests (CGO disabled) for core, adapters, backend, tests, tools; `go mod verify`; `git diff --check` | PASS |
| Migration 0003 keeps every other setting and constraint; old database with `mpv_path` upgrades; fresh schema has no `mpv_path` | PASS (new tests in `adapters/persistence/sqlite/migrations_test.go`) |
| `ci.yml` diff removes only `-tags production` from two commands; permissions, triggers and guards unchanged | PASS (reviewed diff; no Go file uses that tag) |
| Independent review | First review: 1 `MUST_FIX` (DLL digest test passed for the wrong reason). Fixed in correction 1 of 5. |
| Native Windows build and tests | Pending: GitHub Windows CI after the owner approves the push |

Known effects:

- After migration 0003 runs, an older app version refuses to open the database (unknown migration 3). It does not corrupt it. Rolling back the app needs a database backup.
- Dropped `mpv_path` text can stay in free database pages until SQLite reuses them. Low privacy impact; no VACUUM is added.
