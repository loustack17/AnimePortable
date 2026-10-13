<!-- SPDX-License-Identifier: MPL-2.0 -->

# ADR-020 — No WebView in the desktop application

## Status

Accepted by the owner on 2026-09-23. ADR-021 selected Fyne for migration work; ADR-023 selects FLTK with in-process libmpv for the Windows replacement. Final resource, keyboard, platform and release acceptance remains pending.

## Context

ADR-002 selected Wails v3 and Svelte/TypeScript. The owner requires the lowest practical resource use and explicitly rejects every WebView-based desktop UI, regardless of a Wails benchmark result. The product must remain portable on Windows, Linux and macOS, use Go for the backend, and launch the user's external MPV for playback. Complete keyboard operation and visible focus are required; desktop screen-reader support is not currently required.

## Decision

The shipped desktop application must not use WebView2, WKWebView, WebKitGTK, CEF, Chromium, browser-backed widgets, or an HTML UI runtime. Wails/Svelte and their WebView runtime must be removed from the final application, its runtime dependency tree, and release artifacts. This decision supersedes ADR-002's desktop UI choice and the older product-contract instruction to use the OS WebView. Existing Wails code may remain only as a temporary migration source until equivalent non-WebView behavior is verified; it is not an acceptable final release. No installer is required: distribution must be portable. MPV remains a separately installed external player and is not bundled.

The owner selected Fyne in ADR-021 after the initial candidate probes. Verify a representative Home implementation, matched-content resource use, Windows keyboard/focus behavior, and Linux/macOS build and runtime feasibility before release. Resource measurements cannot reinstate Wails. Preserve existing core, adapter, security, cancellation, error, and lifecycle contracts. Do not weaken acceptance tests merely because DOM-based checks need equivalent native verifiers.

## Alternatives considered

- Retain Wails if its measured RAM is competitive: rejected because the owner forbids WebView independently of RAM.
- Gio and Flutter desktop with Go backend: non-WebView alternatives considered before the owner selected Fyne in ADR-021. Reopen toolkit selection only after an explicit owner decision, not as an automatic fallback.
- Embed a browser or video engine: rejected for the desktop UI and current external-MPV playback contract.

## Security impact

Removing WebView changes the UI trust boundary but does not permit direct provider/network access or raw MPV commands from UI controls. Recheck every former frontend DTO, input-validation, redaction, cancellation, and error boundary in the native replacement. Remove WebView-specific CSP tests only after equivalent native security tests and removal of the WebView runtime.

## Performance/resource impact

Aim for the lowest practical startup, idle, interaction, and playback resource use. Measure private resident memory, private commit, process-tree and GPU costs, CPU, startup, repeated open/close and playback switching with matched content. Existing tiny-shell and empty-profile observations are insufficient to select a toolkit or promise universal 100 MB idle / 250 MB playback limits. No further Wails launch is required to decide this architecture choice; prior Wails measurements are historical context only.

## Migration impact

ADR-021 records the owner's Fyne selection for migration, without declaring release gates passed. Replace the desktop adapter/UI in bounded slices, preserve equivalent tests, and remove Wails/Svelte/NSIS and associated runtime/build dependencies before final release. Define portable application-state location and safe import of existing data without deleting the current AppData profile.

## Consequences

The present Wails build is transitional and does not satisfy this ADR. Candidate research can proceed without a WebView benchmark gate. Final migration acceptance requires no WebView process or dependency in packaged artifacts on Windows, Linux, or macOS.
