<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
status: IN_PROGRESS
repository:
  branch: main
  observed_head: c2a7fef
  working_tree: "Scoped input/startup fixes and evidence docs; preserve unrelated AGENTS.md and .codex/."
active_loop:
  number: 29
  state: "Loop29 PASS by owner; bounded player follow-up reopened, not Loop30."
  goal: "Random initial/Continue loading failure; hardware media keys; Space on focused Play/Pause."
  criteria: "PLAYER_KEYBOARD_FOLLOWUP contract; preserve default progress/Tab/contextual arrows; QUAL and native Windows/owner gates."
completed:
  - "Previous exact dc2e361 CI37730888867 all gates PASS; delivered exe/ZIP retained."
  - "Source/proxy and saved-Continue diagnostics succeeded; exact delivered native exe played on DB copy. No 20-second failure reproduced. Host diagnostics are not verifier PASS."
  - "Only synchronous Service.Play error returns Home. Optional Episodes fetch blocked play/control queue; root now starts Play first and fetches labels independently with cancellation/stale guards."
  - "Worker added scoped WM_APPCOMMAND subclass parent/video HWND lifecycle; Space/Enter SHORTCUT dispatch; fixed pinned/hover menu priority and cached native procedures."
verification:
  - "gofmt/diff PASS; CGO0 go test ./apps/desktop PASS with sandbox temp GOCACHE. Default host cache denied, classified environment."
  - "Independent platform/input review completed; final startup/concurrency review no MUST_FIX. CI37734182237 stalled: t.Run cross-thread SendMessage test deadlock; corrected same cases onto HWND owner thread, review PASS. CI37735227917 then failed invalid synthetic keyup LPARAM and empty menu fixture; corrected test inputs with unchanged assertions, review pending."
  - "Native local cgo breaker closed: parse_cgo_.o +unlinkAccessDenied. No equivalent retry or hostPASS. Windows CI authoritative."
budget:
  corrections_used: 5
  corrections_max: 5
  remaining: 0
boundaries:
  - "Prior scoped main commit/push/exact exe/package authorization applies; no workflow/release mutation. Planned Loop30 remains Phase24."
  - "Do not claim network failure fixed without owner playback. No speculative decoder/cache/retry changes."
  - "Native diagnostic process727728 exited normally; disposable data copy only. Do not repeat hidden-window screenshot rectangle capture; it captures background."
next:
  - "Resolve final test-only review; commit scoped test/docs and push main; exact Windows CI. Stop on further required failure: budget exhausted."
  - "On green CI deliver exact exe/ZIP, preserve data hashes and five-entry package closure. Full portable notices, pinned DLL hash."
  - "Update evidence/handoff; request simple owner Search/Continue, Space/Enter, physical media-key/fullscreen checks. Remain NEEDS_HUMAN until owner passes."
references:
  - "docs/PLAYER_KEYBOARD_FOLLOWUP.md; docs/IMPLEMENTATION_STATUS.md latest subsection."
  - "apps/desktop/playback_startup.go/test; main_fltk_windows.go; fltkplayer/native_input_windows.go/test and player keyboard files."
  - "artifacts/evidence/player-keyboard/dc2e361-test-object.json; diagnosis/ ignored diagnostic artifacts."
object:
  exe: artifacts/loop29/AnimePortable.exe
  package: artifacts/loop29/AnimePortable-player-keyboard-dc2e361.zip
  rollback: artifacts/loop29/AnimePortable-loop29-2d4bf48.zip
```
