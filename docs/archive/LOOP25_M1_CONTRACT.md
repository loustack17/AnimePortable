<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 25 — Fyne lifecycle and Home

Status: M1 PASS. Baseline HEAD `3e15651e3d7761122f0587f0ed476b224b79f1de`, with pre-existing uncommitted planning/probe files and the Loop 24 M0 record. Preserve the transitional Svelte implementation and its verifiers until native parity is proven.

| Field | Contract |
| --- | --- |
| Scope | Framework-neutral `Start`/`Close` service lifecycle, Fyne production entry, six-section shell and existing cached Home preview with safe playback action; no portable import, full Search/detail/etc. screen, Wails dependency removal, workflow edit or resource benchmark |
| Criteria | ARCH-001/010/014/015; UX-002/003/008..010; SEC-014..019; PRIV-010..012; RES-002..004/010; PERF-001..003/005; RA-06 native boundary; QUAL-001..012 |
| Success | Production entry creates Fyne rather than Wails WebView; startup/shutdown/cancellation and safe DTO/action boundaries remain intact; six destinations, keyboard arrows/Tab/Enter/Space, visible focus and mouse work; Home cache/history/following preview, retry/empty/error states and Play guard are testable; no direct UI provider/SQL/MPV access |
| Deterministic verifiers | Focused Go service/native tests, existing frontend tests while transitional code remains, `go vet`, native build, source scan and scoped whitespace check; classify sandbox capability failures separately |
| Independent review | Fresh-context code-quality plus security/lifecycle/resource review required before M1 PASS |
| Human gate | Windows visible native shell/Home interaction, including keyboard focus, mouse and resize; no long-idle test |
| Protected assets | Existing backend/frontend tests, acceptance and verification matrix, `.github/workflows/**`; do not weaken |
| Budget | At most 5 corrective iterations, 2 repairs per fingerprint, 1 replan, 2 unchanged flaky reruns; provider/token/usage budgets unavailable |

M1 reached PASS after deterministic, independent and Windows human gates. Loop 26 may start with its own contract.

## Implemented and checked

`apps/desktop/main.go` now starts a Fyne window through `backend.Runtime`. The wrapper exposes framework-neutral `Start`/`Close` without expanding `backend.Service`'s 15-method Wails binding surface. `apps/desktop/native/` implements six destinations, Home cache/history/following reads, cancellation/stale-result suppression, safe Play feedback and pending guard, keyboard/mouse navigation, visible focus transfer, scroll control, retry/empty states and a six-item preview matching `home.ts`. The Wails frontend, generated bindings and existing tests remain transitional until later migration gates. No WebView is created by the production entry, but Wails remains a transitive dependency and final no-WebView artifacts are not yet claimed.

Final-state checks with repository-local `GOCACHE`: `go test -count=1 ./...` PASS, `go test -race -count=1 ./apps/desktop/native ./apps/desktop/backend` PASS, `go vet ./apps/desktop/...` PASS, `go build -tags production -o .slim/animeportable-m1.exe ./apps/desktop` PASS, `go mod verify` PASS. Transitional frontend `npm test` PASS 8/8 and `npm run check` PASS 0 diagnostics. Scoped whitespace checks on M1 paths pass. The unscoped `git diff --check` still reports the pre-existing `docs/09_CODEX_EXECUTION_PROFILE.md` EOF blank line; M1 did not edit it. Default Go cache access is denied by the sandbox, so final sandbox checks used an isolated repository-local cache; no unsandboxed run is counted as PASS.

Independent fresh-context review found and resolved direct UI adapter coupling, retry-focus removal, and missing-MPV guidance to an unavailable Settings screen. Final review reported no remaining M1 code MUST_FIX. The owner then checked the visible Windows Fyne window launched with an isolated profile and reported **全部通過** for six destinations, Tab/Shift+Tab, arrows, Enter/Space, mouse, visible focus, content entry/exit, scroll control and reduced-window scrolling. The process exited after the check. No long-idle comparison was run.
