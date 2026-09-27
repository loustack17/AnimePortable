<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
updated_at_utc: "2026-09-26"
status: WINDOWS_FLTK_CLEANUP_IN_PROGRESS
repository:
  branch: codex/fyne-migration-verification-20260924
  observed_head: bde9298
  working_tree: "Windows FLTK snapshot committed/pushed on verification branch; preserve unrelated untracked probes. C++20 icon type fix pending. No reset/clean."
active_loop:
  number: 28
  state: "Windows FLTK replacement in progress; no Loop 28 PASS, main merge or Loop 29."
  goal: "Windows-only FLTK/libmpv product, clean legacy removal, matched lower resource use than OneAnime, release gates."
historical:
  - "Loop 23 and RA-01..06 passed at 3e15651e; do not replay."
  - "Loops 24–26 M0–M2 passed. Fyne Loop 27 M3 WINDOWS_PASS at 93fa5c4; Linux/macOS native acceptance deferred."
  - "Fyne M4 15-minute minimized 169.50 MiB private WS missed <100 target; external-MPV playback 345.64 MiB resident/613.86 MiB commit. Fyne M4 not PASS."
owner_decisions:
  - "Windows only now. Pause Linux/macOS implementation until Windows stable; Linux second, macOS third."
  - "Remove active Fyne, Wails, Svelte, NSIS, Linux/macOS and external-MPV adapter; use Go+FLTK+in-process libmpv. Preserve clean core/data and historical evidence."
  - "No third-party repo clone/vendor. Do not commit/push or release without request. Long-idle rerun requires fresh owner consent."
  - "Whole-product resource objective remains below matched OneAnime; short unmatched samples are diagnostic only."
  - "Owner now prioritizes replacing Wails on main before Loop 28 PASS. Main may carry a reviewed, exact-CI Windows FLTK development snapshot; release and Loop 28 gates remain open."
current_changes:
  - "Windows amd64 default entry/Taskfile uses FLTK. fltkhome/fltkplayer/fltkengine, first-party pinned mpvwin binding, libmpv core.Player, portable new/import and play/stop queue are committed on the verification branch."
  - "Removed Fyne apps/desktop/native, non-Windows entry/build recipes, Fyne/Wails resource probes, external adapters/player/mpv, backend player_error, and root Fyne deps. Windows-only CI retains full Go/race/vet/vuln/build/dependency checks."
  - "Removed MPVPath from active core/backend settings and player factory. SQLite legacy mpv_path column remains for old databases; current settings ignore and preserve it. Added regression test."
  - "Windows ZIP tool now accepts only Windows amd64 and requires hash-pinned libmpv plus license file; no distributable ZIP/CI artifact while exact license unresolved."
  - "Exact CI 36281457807 at f6e2391 failed Test Go: runner GCC 15 could not link go-fltk prebuilt libraries (_setjmp); mpvwin rejected valid temp DLL. ea7239e corrected canonical DLL loading: exact CI 36281867374 passed mpvwin tests but GCC 14 on windows-2022 still failed _setjmp. bde9298 selected pinned MSYS2 MINGW64 GCC; exact CI 36282386404 passed toolchain setup and found C++20 char8_t icon literal type mismatch. Current production fix adds guarded overload; not yet exact-CI verified."
verification:
  passed:
    - "Final go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., go mod verify, Windows FLTK production build and git diff --check passed."
    - "Independent workflow/security and production-quality reviews after cleanup found no MUST_FIX. CI smoke is bare EXE only, not package acceptance."
    - "After FFI uintptr correction, host real-player smoke opened player, Stop returned Home, exit 0. PrintWindow video capture blank; CopyFromScreen failed with invalid handle, so visible video not verified by this run."
    - "Earlier host real Anime1 720p Play/seek/Stop/Close passed with lazy pinned DLL; latest 12s unmatched 181.57 MiB private WS/618.07 MiB private bytes, 5s post-stop 55.45/181.61. Missing/corrupt DLL keeps Home usable."
  pending:
    - "Exact-state isolated CI after C++20 correction and full owner-visible Windows playback/keyboard/mouse/focus and portable ZIP check."
    - "Owner native keyboard/mouse/focus/episode/portable ZIP gate; prior test window was closed without full checklist result."
    - "Matched OneAnime resource comparison; new 15-minute idle authorization; exact libmpv build/codec license notices; clean-host Vulkan dependency."
    - "Candidate DLL remains outside repo in TEMP; SHA256 e466e34e425cb3b4546b18ad6dd66ad020dcb389bb3cd8fe9c843e8d6d8ee743. Test-only."
next_actions:
  - "Review and verify CI corrections, commit/push current branch, require exact green Windows CI before main integration."
  - "Review exact libmpv build provenance/notices and clean-host DLL dependencies; do not distribute current test-only DLL."
  - "Keep CI Windows only; do not publish/package until license and clean-host runtime gates close."
  - "Do not claim Loop 28 PASS or release until Windows native, resource, package and exact-state isolated verification pass."
references:
  - "docs/README.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/LOOP28_M4_CONTRACT.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md"
```
