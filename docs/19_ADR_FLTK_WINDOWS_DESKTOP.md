<!-- SPDX-License-Identifier: MPL-2.0 -->

# ADR-023 — FLTK and in-process libmpv for the Windows desktop

## Status

Accepted for Windows implementation by the owner on 2026-09-25. Supersedes ADR-021 for the Windows desktop implementation. Release, resource and Loop 28 acceptance remain open. Linux follows a stable Windows release; macOS follows stable Linux support.

## Context

The Fyne Windows implementation passed Loop 27 portable and native-interaction checks but missed the Loop 28 idle resource limit: 169.50 MiB private working set after 15 minutes minimized against the under-100 MiB target. The owner requires the complete product to use less resource than OneAnime under comparable conditions. ADR-020 still prohibits WebView, Wails and Svelte as release UI paths.

An isolated Go, FLTK and in-process libmpv Windows prototype has an approved visual direction and working mouse playback, pause, seek, volume, episode selection, fullscreen and stop controls. One 20-second synthetic 720p24 playback run sampled maxima of 167.82 MiB private working set and 568.43 MiB private bytes; four seconds after Stop it sampled 52.99 and 178.45 MiB respectively. Its Home is demonstration content, its clips are local command-line arguments, and it does not use the production service. The short run used different content and conditions from the owner's OneAnime observations and did not measure GPU allocation. It is a feasibility screen, not a completed resource audit or Loop 28 PASS.

## Decision

Replace the Windows Fyne desktop adapter with FLTK and replace external MPV playback with in-process libmpv. Keep the existing Go core, source and metadata adapters, SQLite data, portable `<extracted folder>/data` contract, explicit legacy-data import, and existing security, cancellation, error and playback-lifecycle behavior. Reuse the core `Player`/`PlaybackSession` boundary; do not make widgets responsible for provider validation or persistence policy. Native player controls remain inside the video surface.

The Windows release and Loop 28 PASS require product parity with the currently accepted Fyne slice, equivalent or stronger tests, native keyboard/mouse/focus and portable-artifact checks, exact-state isolated verification, and a matched resource comparison with OneAnime. The unchanged resource objective is whole-product use below OneAnime in comparable idle, wake, playback and cleanup states. Do not infer a pass from the synthetic short run, trim working sets, force GC, weaken security/tests or raise the target to make the result pass.

On 2026-09-26 the owner prioritized replacing the Wails code on `main` before Loop 28 closes. This permits an explicitly marked Windows FLTK development snapshot on `main` after scoped review and exact-commit CI, while release artifacts and Loop 28 PASS remain gated by the requirements above. The snapshot must not bundle the test-only libmpv DLL or claim complete playback portability.

The Windows amd64 default build selects FLTK directly. The owner subsequently paused Linux and macOS development and directed removal of all Fyne, Wails, Svelte, NSIS and external-MPV product paths now. The active source, dependencies and CI are Windows-only; historical documentation and the legacy SQLite `mpv_path` column remain for traceability and old-data compatibility. Later platform work requires new native adapters and their own acceptance evidence.

## Windows replacement gates

1. Production startup: extracted-folder data, first-run new/import choices, failure behavior, close cancellation and portable move/reopen.
2. UI parity: six destinations with the current Home and placeholders, real Library/History/Following data, bounded rows, safe resume Play, loading/error/retry behavior, visible focus, Tab/Shift+Tab, arrows, Enter/Space, mouse and reduced-window scrolling.
3. Player parity: validated source and headers, protected local proxy boundary where needed, progress checkpoints, resume, episode switch, stop and close cleanup, no secret-bearing errors, and no orphan player session.
4. Windows build/package: no Fyne, Wails, Svelte, NSIS or WebView runtime; no installer requirement; actual ZIP extraction and native operation.
   The package must include a versioned, hash-pinned libmpv runtime and its build-specific license notice without committing third-party binaries or source to the repository. Distribution review must resolve mpv and linked codec license terms for that exact build.
   The executable must verify the bundled DLL hash and load it by absolute path only when playback starts; a missing or altered DLL must leave Home available and produce a safe playback error. Reject runtime dependencies outside the package or Windows system directories.
5. Resource comparison: matching content and counters for AnimePortable and OneAnime across comparable Home, idle, refocus, playback and cleanup states, with CPU/GPU and repeatability. The owner must approve a new long-idle run before one is performed.

Loop 29 feature work remains gated by Windows Loop 28 acceptance. Linux and macOS acceptance are separate later phases and are not passed by Windows evidence.

## Windows integration evidence, 2026-09-26

The opt-in production-tag FLTK executable played the existing Loop 23 Anime1 acceptance episode through the real backend, protected local proxy and in-process libmpv. Its 720p player displayed video, episode selection and controls; the stop button returned to Home, and closing Home exited with code 0. Two 12-second host diagnostics measured private working set of 181.07/181.67 MiB and private commit of 609.35/610.31 MiB. One earlier cold sample was 233.41/665.82 MiB. Five seconds after Stop, working set/commit was 56.86/192.48 MiB. A separate 10-second Home run with 200 local anime, 39 following entries and 19 history entries measured 13.28/125.02 MiB. These short, unmatched samples do not establish the OneAnime comparison or long-idle gate.

Independent security review found that the first integration's go-mpv purego backend loaded libmpv by bare DLL name during Go package initialization. This could panic before Home if the DLL was missing and did not prove that the loaded DLL matched the hash-pinned package. A narrow, lazy Windows binding with absolute-path loading and runtime hash verification is required before Windows release; the integration and resource measurements above predate that correction. Two bounded mpv option trials (four software decoder threads and 8 MiB backward cache) gave no material private-commit reduction and were removed.

The replacement now uses a first-party narrow Windows binding. It verifies the adjacent DLL against the pinned SHA-256 and retains a read-only, non-shareable-for-write/delete file handle through `LoadLibraryEx` and symbol binding; the loader limits dependency search to the DLL directory and Windows System32. A focused race test, a native real 720p playback/seek/stop/close run, and native missing/corrupt DLL runs passed. The latter keep Home usable and show an inline playback error. Independent security re-review found no remaining evidenced MUST_FIX in this loader path. The current 12-second playback sample was 181.14 MiB private working set and 612.77 MiB private bytes; five seconds after Stop it was 55.63/181.98 MiB. These remain unmatched host diagnostics. The candidate DLL imports `vulkan-1.dll` from System32 on this host; clean-machine portability and the exact build's license/codec notices remain open.

On 2026-09-26 the Windows amd64 default `go build` and production Taskfile were switched to the FLTK entry. A Windows ZIP request without bundled libmpv fails in the packaging CLI. The owner then directed a Windows-only cleanup: Fyne/native, external MPV IPC, non-Windows build/package source and runtime dependencies were removed. Windows CI now builds and tests FLTK and libmpv without distributing a ZIP. The exact libmpv build's license/codec notices and clean-host runtime dependencies remain unresolved, so a distributable package and Windows Loop 28 PASS remain blocked. Linux and macOS native implementation and acceptance are deferred.
