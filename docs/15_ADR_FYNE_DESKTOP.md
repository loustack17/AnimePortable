<!-- SPDX-License-Identifier: MPL-2.0 -->

# ADR-021 — Fyne for the non-WebView desktop migration

## Status

Accepted for migration implementation by the owner on 2026-09-23. Superseded for the Windows desktop by ADR-023 on 2026-09-25 after the Loop 28 resource failure. The earlier Fyne verification results remain historical evidence.

## Context

ADR-020 requires removal of Wails and every WebView runtime. The owner prefers Fyne because its interface is more suitable for the intended desktop style and its native widgets are easier to test in this project. The earlier tiny-shell Windows probe measured Fyne at a 76.4 MiB private-resident median, versus Gio at 45.5 MiB, so Fyne is not claimed to be the lowest-memory toolkit from that test. More representative Fyne Home-only host diagnostics measured 172.3, 130.1 and 125.5 MiB private resident across three changing keyboard revisions, with no production database, provider work, cover loading, or MPV. These single samples are not a release benchmark and are above the owner's preferred sub-100 MiB idle target. The owner observed that the first Fyne prototypes displayed normally but had incomplete keyboard flow; a third visible check of a later revision passed. Independent review then found a narrower offscreen-focus defect, reproduced by a new failing test and repaired with a persistent visible scroll control. Focused tests now pass, while the newest revision still needs a fresh user-visible check.

## Decision

Use Fyne as the desktop UI toolkit for the Wails-to-native migration. Keep Go core and adapters, external user-installed MPV, and portable Windows/Linux/macOS distribution. Do not ship a WebView fallback or retain Wails as a release option. The Fyne UI must support the complete workflow by both keyboard and mouse, with visible focus, navigation into and out of content, Enter/Space activation, and keyboard scrolling. It must retain the existing security, error, cancellation, lifecycle, and data-boundary behavior.

The toolkit choice does not waive the resource objective. Benchmark the integrated application and app-plus-MPV playback with representative content, whole process-tree metrics, repeated cycles, and all supported platforms. If Fyne cannot meet an owner-approved resource envelope after bounded causal optimization, stop and present the measured conflict to the owner; do not silently lower the target or switch frameworks.

## Alternatives considered

- Gio: lower measured tiny-shell resident memory, but the owner's Home prototype check reported keyboard failure and the owner prefers Fyne's interface. The Gio prototype is not a proven full-product baseline.
- Wails/Svelte: disallowed by ADR-020 regardless of measured resource use.
- Flutter desktop with Go backend: non-WebView candidate, but not chosen by the owner for this migration. No matched-content measurement was completed.

## Security impact

Fyne removes the WebView execution boundary, but the native UI must not gain direct provider/network access, raw MPV IPC, secret-bearing playback URLs, or unvalidated persistence access. Map every former binding/DTO, redaction and cancellation test to equivalent native boundaries before deleting the old checks.

## Performance/resource impact

The current Home-only diagnostic shows material headroom risk, not proof of final behavior. Measure private resident and commit memory, GPU allocation, CPU, startup and cleanup separately; distinguish app-only and app-plus-external-MPV. Preserve all three 172.3/130.1/125.5 MiB observations rather than selecting the lower run. The owner's later OneAnime long-idle screenshots show private resident falling to 26.6 MiB while private commit remains about 300 MiB, consistent with working-set trimming; Fyne's 40-second samples are not directly comparable. Repeat with matched long-idle duration, window state, wake-up and playback conditions. Investigate causes and document any CPU/memory tradeoff from garbage-collector or rendering changes.

## Migration impact

Replace the Wails desktop adapter/UI in bounded slices. Keep the existing implementation transitional until equivalent Fyne features and tests pass. Remove Wails/Svelte/WebView/NSIS runtime and build dependencies from final portable artifacts; define local state location and safe import without deleting existing AppData. Windows, Linux and macOS runtime evidence and a final human keyboard/mouse check are required before release.

## Consequences

Fyne is the chosen implementation path, not a claim that the current prototype meets the final memory or interaction goals. No production Wails code has yet been replaced.
