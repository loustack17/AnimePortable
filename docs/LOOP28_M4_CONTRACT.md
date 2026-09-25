<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 28 — Windows integrated Fyne migration gate

Status: IN_PROGRESS. Loop 27 is `WINDOWS_PASS` at production/workflow revision `e0ea18a`; Linux/macOS native acceptance remains deferred. No Loop 29 feature work or main merge is authorized by this status.

| Gate | Windows success condition | Verifier |
| --- | --- | --- |
| Native behavior | The extracted Fyne app retains keyboard/mouse operation, visible focus, portable state, and external MPV lifecycle, including error/close/return paths. | Focused native/MPV tests, final-state Windows CI and simple owner-visible checks only where changed behavior needs renewed confirmation. |
| Security and packaging | Existing import/source protection, untrusted data bounds, no-WebView runtime/dependency scan, and installer-free Windows ZIP remain intact. | Full Go regression/race/vet/vulnerability/package checks, extracted-artifact inspection, independent quality/security/platform review. |
| Resource envelope | On an owner-approved Windows reference host with representative data, sustained app-only idle private resident stays under 100 MiB and combined app-plus-external-MPV playback private resident stays under 250 MiB. Record private commit, CPU, GPU, responsiveness and cleanup across repeatable cycles; a trimmed working-set minimum alone is insufficient. | `PERF-008`, `RES-008..010`, and the measurement protocol in `docs/16_FYNE_MIGRATION_PLAN.md`; isolate host diagnostics from authoritative PASS evidence under `docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md`. |

The owner authorized a long-idle Windows diagnostic and then stopped it at 15 minutes. Its confirmed minimized sample was 169.50 MiB private resident and 304.97 MiB private commit, above the idle target. Do not repeat long-idle sampling unless the owner asks. Investigate and optimize only with bounded causal evidence; do not force working-set trimming, relax the target, revive WebView, or declare Loop 28 PASS from Windows job success alone. If bounded optimization cannot meet the envelope, stop with evidence and owner options.
