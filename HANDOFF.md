<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 22 breakpoint

Updated 2026-09-09. Loop21 is committed and pushed as ad3da22c, accepted by the Review task, with successful CI34258733190. Do not redo it.

Loop22 implementation and independent simplify review passed. Six native-button sections, Home default, visible keyboard focus, functional skip link and Traditional Chinese document language. No page data features, backend changes or new dependencies.

Verified locally: npm run check, npm run build, npm test, npm audit --audit-level=high; go test -count=1 ./..., go test -race -count=1 ./..., go vet ./..., go mod verify, Windows desktop go build and git diff --check. Go emitted a telemetry upload-token permission warning; commands exited successfully. Browser checks verified six mouse destinations, Tab/Enter/Space, skip focus and navigation at1000x618,800x600,500x309 with no horizontal overflow or console warnings/errors. The smallest viewport checks reflow, not actual OS zoom. SSR tests cover initial markup only. Playwright CLI lacked Chrome; in-app browser provided interaction validation. Native cross-platform UI acceptance remains deferred.

Delivery checkpoint: commit/push and CI verification are the remaining steps at the time this handoff was written. Inspect the latest commit and its matching GitHub run before resuming; do not repeat implementation if already delivered. Stop after Loop22. Next is Loop23 (Home UI), using local typed bindings; do not bundle search, schedule, following or settings page features.

The user approved removal of the two empty root npm manifests; they were deleted. The temporary .ignore is removed after deepwork. Cleanup must stop the owned preview server and remove owned build/test artifacts, preserving source and committed tests.
