<!-- SPDX-License-Identifier: MPL-2.0 -->

# ADR-022 proposal: resource-first desktop UI decision

Status: Historical proposal. The owner selected Windows FLTK and in-process libmpv implementation in ADR-023; its release and resource gates remain open.

## Decision needed

The owner requires the complete AnimePortable experience to consume less resource than OneAnime in comparable Windows idle, focus, playback and cleanup states. This is a necessary condition, not an optional optimization target. ADR-021 currently requires Fyne. Loop 28's existing `PERF-008` idle criterion fails: the current Fyne build reached 169.50 MiB private resident after 15 minutes minimized, above its under-100 MiB limit. A separate fixed-clip MPV diagnostic reached 345.64 MiB combined private resident, but did not exercise app-initiated playback or a matched OneAnime run. OneAnime's reported low resident readings coexist with about 300 MiB private commit in the owner's earlier screenshots. No architecture is yet proved to beat OneAnime across all required measures.

## Options for owner review

1. Retain Fyne and keep Loop 28 open only for a bounded, attributable toolkit/graphics-memory investigation. Current evidence offers no demonstrated route to the required resource result; further work must stop if it cannot close the gap without forced GC, working-set trimming or weakened behavior.
2. Prototype a Windows GUI built from Win32 standard controls over the existing Go service boundary. This could avoid Fyne's renderer and preserve mouse, keyboard, focus and portable launch, but its memory result is unknown. It would require separate Linux and macOS frontends later. A small Home/navigation/play vertical slice with identical data and resource counters must pass before a full rewrite is approved.
3. Prototype a Bubble Tea terminal UI over the existing Go services if the owner accepts text-based presentation, terminal-host dependency, non-portable cover art and terminal-dependent mouse behavior. Measure the application plus terminal host and external MPV, not only the TUI process. This changes the product interface and needs revised interaction criteria.

A higher fixed numerical limit alone cannot satisfy the owner's lower-than-OneAnime requirement. This proposal does not select an option, alter ADR-021, authorize a new dependency or begin a rewrite.

The owner's later resource-control candidate is specified in [the Go/FLTK/libmpv prototype plan](18_RESOURCE_CONTROL_PROTOTYPE_PLAN.md). It adds an in-process player and explicit module budgets to a bounded experiment; it does not supersede the current Fyne or external-MPV decisions before a measured prototype and owner acceptance.

## Acceptance and migration impact

Compare the exact release builds on one owner-approved Windows host with documented profile/content, window and decoder states. Record private resident, private commit, GPU allocation, CPU, startup and responsiveness for Home foreground, 15-minute idle, refocus, app-initiated fixed-clip playback and stop/cleanup. Count the complete process tree, including terminal host or external MPV as applicable. OneAnime and AnimePortable content/player differences must be labeled; a low trimmed resident point alone cannot establish lower total resource cost. The owner stopped the prior 15-minute test and has not authorized a repeat.

Go core, SQLite data and MPV trust/lifecycle contracts remain shared in any UI option. A UI replacement must redo Windows keyboard/mouse/focus/portable checks and specialized reviews; Linux/macOS native acceptance stays deferred. No Loop 29 feature work, main merge, release or removal of retained historical evidence occurs before a new architecture is accepted and its Windows migration gates pass.
