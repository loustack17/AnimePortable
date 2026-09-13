<!-- SPDX-License-Identifier: MPL-2.0 -->

# Home acceptance

## End-user checks

Already accepted: Home layout, text, navigation, hover/click, resizing, populated Continue Watching, populated Following and the saved position 2:05. Do not repeat these checks.

Loop 23 human acceptance is complete. On 2026-09-13 the user confirmed that disconnected relaunch preserved Continue Watching, Following and saved position 2:05. No repeat is required. The following is a reference procedure for future relevant regressions, not an outstanding task:

1. With the prepared app closed, disconnect the network using your usual controls.
2. Open the prepared app again.
3. Report whether Continue Watching, Following and the saved position still appear.
4. Close the app and reconnect the network.

If you already observed this successfully, simply report that; another test is unnecessary. Do not discover source IDs, enter technical parameters, edit data, or repair the test setup. If the prepared app cannot open, report that visible failure; setup/recovery belongs to the maintainer, not the tester.

Real MPV playback is not an additional Loop 23 human gate. This loop changes Home presentation and invocation of the existing Play binding, not the playback pipeline. Existing pipeline evidence and automated Home boundary verification remain required; this is not a new claim that native playback was tested in Loop 23.

## Maintainer responsibilities

The current command-line tool is development tooling, not an end-user acceptance interface. Its preparation, launch, ownership, recovery and reset commands must not be delegated to the tester. A turnkey prepared-app entry point has not yet been implemented; do not describe the CLI as one. Any further tester session must have its setup and safe recovery handled by tooling/maintainer first, leaving only launch and visible observations to the tester.

This fixture uses existing Store/domain APIs and production executable behavior. Only the child APPDATA is isolated; normal user data and global environment must remain untouched. Isolation also affects MPV AppData configuration discovery; it does not establish normal-profile MPV configuration acceptance.

Synthetic state is sufficient for the remaining Home offline observation. No live source discovery or user-supplied Anime1 IDs are required for Loop 23. Optional live-mode internals do not create an additional human gate. Future source discovery, resolution and membership validation must be automated before any end-user playback acceptance; never request IDs, tokens, cookies or media URLs from the tester.

Guard recovery remains fail-closed: canonical fixed acceptance root, valid ownership, exclusive OS lease, dead recorded operation process, no active app/other launcher, and safe settled inventory are required. Recovery preserves data. Reset validates exact inventory/digests and removes only owned leaves. Missing ownership, unknown state, reparse points or uncertain liveness must refuse; no manual deletion or production-data fallback.

Maintainer verification:

```powershell
go test -count=1 ./tools/native-home-acceptance
go test -race -count=1 ./tools/native-home-acceptance
go vet ./tools/native-home-acceptance
go build -o apps/desktop/bin/native-home-acceptance.exe ./tools/native-home-acceptance
```

Scope and evidence reconciliation is recorded in `docs/IMPLEMENTATION_STATUS.md`. Loop 23 is PASS. RA-01 is a separate subsequent phase and has not started.
