# Implementation history

Historical checkpoints retain their original observations and paths. Superseded local probes and evidence were consolidated into artifacts/evidence/legacy-verification.zip; its entry hashes are recorded in legacy-manifest.json. Current state is in ../IMPLEMENTATION_STATUS.md.

### Windows FLTK replacement — historical 2026-09-26 checkpoint

On 2026-09-26 the owner directed Windows-only development and complete removal of active Fyne/Wails/Svelte/NSIS, Linux/macOS, and external MPV code. The local uncommitted cleanup removes those source/build paths, makes the Go module and CI Windows-only, and keeps the legacy SQLite player-path column solely for existing-data compatibility while removing it from current settings and playback. Final `go test -count=1 ./...`, go test -race -count=1 ./..., `go vet ./...`, `go mod verify`, Windows FLTK production build and `git diff --check` pass. Independent workflow/security and production-quality reviews found no MUST_FIX. A real-player host smoke opened the player, returned Home on Stop and exited 0 after the FFI pointer correction; its `PrintWindow` video capture was blank and `CopyFromScreen` failed in this environment, so visible video remains unverified in this run. Windows native UX, matched OneAnime resource comparison, exact libmpv license review, clean-host DLL portability, distributable ZIP and exact-state CI remain open. This is not Loop 28 PASS or authorization to merge.

Loop 24 M0 passed with action/DTO/test inventory, read-only OneAnime 1.4.7 source comparison, resource protocol and portable data/import contract in `docs/LOOP24_M0_CONTRACT.md`. Loop 25 M1 passed deterministic and independent reviews plus the owner's visible Windows six-section keyboard/mouse/focus check in `docs/LOOP25_M1_CONTRACT.md`. Loop 26 M2 passed in `docs/LOOP26_M2_CONTRACT.md`: the production entry and 15 typed actions use framework-neutral `Start/Close`; the native Home retains cached data, safe Play, cancellation and focus behavior. Exact-SHA hosted [CI run 35974229139](https://github.com/loustack17/AnimePortable/actions/runs/35974229139) at `4deabe684df5424f81ae0eb6097a120717318b9d` passed Linux full regression/vulnerability/build and Windows native build plus real external MPV IPC/three-media tests. The owner visibly confirmed Home-to-MPV playback and responsive/focused Fyne Home after closing MPV. Loop 27 M3 is `WINDOWS_PASS` under the owner's Windows-first sequence: [Windows job 107901003874](https://github.com/loustack17/AnimePortable/actions/runs/36080446540/job/107901003874) at production/workflow SHA `e0ea18a` passed, extracted Windows portable import and focus were owner-confirmed, active Wails/Svelte/NSIS/WebView files/dependencies are absent, and final independent review found no MUST_FIX. The aggregate workflow is red only at the deferred macOS graphics smoke; Linux/macOS native acceptance remains open for later phases. Loop 28 M4 Windows resource/final integration gates remain open: an owner-authorized 15-minute host diagnostic reached 169.50 MiB private resident against the unchanged under-100 MiB idle target; it is diagnostic, not PASS. No Loop 29 feature work or main merge yet. The temporary CI branch contains only migration code/tests/dependencies/workflow; earlier dirty planning/probe files remain local.

### Retrospective baseline — CLOSED / RETRO_BASELINE_PASS

RA-01 through RA-06 are `LEGACY_VERIFIED` at `3e15651e3d7761122f0587f0ed476b224b79f1de`; all confirmed MUST_FIX findings are closed. RA-06 repaired missing arrow-key focus navigation and restrictive WebView CSP in the four-file commit `3e15651e`, now pushed to `main` together with the previously local RA-05 repair `c4bf12b0`. Final local frontend test/check/build and full Go test/race/vet passed. [Exact-SHA CI 35943800633](https://github.com/loustack17/AnimePortable/actions/runs/35943800633) passed frontend/Chromium, Go test/race/vet, vulnerability and desktop-build checks. The owner confirmed the repaired native Windows Home, visible arrow-key focus and Enter activation. Fresh final architecture/quality and security/resource reviews found no remaining code MUST_FIX; audit closeout evidence is in `docs/RETRO_AUDIT_STATUS.md`. The earlier MPV live IPC timeout was a limited host diagnostic, not a playback-resource measurement; M2 later supplied separate isolated Windows and visible playback evidence. The owner explicitly requires no WebView in the final desktop app and selected Fyne for migration. Wails removal remains mandatory; full M4 resource and cross-platform gates remain open. Entries below retain their historical point-in-time status.

### RA-05 Metadata — CLOSED / LEGACY_VERIFIED

Current-state audit of historical Loops 17–20 is complete at local repair commit `c4bf12b0f5705419715e087a3ca1bfca839de6c6`. Audited AniList, Bangumi, shared metadata parsing/plain-text policy, provider-neutral matching, cover loader, SQLite metadata validation/cache, and narrow desktop Detail/GetCover/DTO boundaries. `RA-05-F001` confirmed that indistinguishable distinct IDs from one provider could be accepted as a high-confidence match. The new fixture failed before the two-file matcher fix; focused tests, ten shuffled affected-package rounds, final-state full Go test/race/vet and independent correctness/security reviews passed. The repair is committed locally and not pushed. Exact-SHA CI 34874294714 at prior `028b0221` remains evidence for unchanged components, not the repair; no new CI or live upstream check was run. Runtime metadata orchestration/refresh remains scheduled for application wiring/Phase 29 and was not changed. No human gate. RA-06 remains NOT_STARTED and requires a separate session.

### RA-04 Persistence/state — CLOSED / LEGACY_VERIFIED

Current-state audit of historical Loops 14–16 is complete at `028b02210d0ab6e986ccae2b8ac3a77799019343`. SQLite schema/migrations and Store lifecycle, progress/history checkpoints and resume, following/mappings, corruption/error/concurrency behavior, resource cleanup, identifier and secret boundaries, and desktop cached-state mapping were inspected. Focused local `go test -count=1`, `go test -race -count=1`, and `go vet` for `./core ./adapters/persistence/sqlite ./apps/desktop/backend ./tests/contract` passed. Exact-HEAD CI 34874294714/linux 104077534611 is SUCCESS. Independent correctness and security/resource reviews found no MUST_FIX; no repair or new human gate was needed. Offline desktop Following exposes cached watch state; richer availability remains a future UI concern. No production/test change, commit, push or new CI run occurred in RA-04.

### RA-03 Playback — CLOSED / LEGACY_VERIFIED

Current-state retrospective audit of historical Loops 10–13 is `CLOSED / LEGACY_VERIFIED` at repair commit `028b02210d0ab6e986ccae2b8ac3a77799019343`. Audited the secure playback proxy, MPV locator/process/IPC/player, same-process switching, current progress/resume/checkpoint orchestration, and narrow SQLite/desktop lifecycle and error propagation boundaries. Focused normal/race/vet checks, 50 race-enabled switching repetitions, 120 repeated helper-process start/close/reap cycles, portable Linux/Darwin builds and full Go regression passed. Earlier exact-HEAD CI 34790321962/linux 103813204911 remains reused for unchanged playback components.

`RA-03-F001` (`PLAY-003`) is closed. Within the explicitly authorized playback-backend/Home-error boundary, the backend now preserves only fixed, redacted missing-player and invalid-configured-path sentinels while keeping all other failures generic. Home exact-whitelist maps those two cases across synchronous and asynchronous Wails errors to concise Traditional Chinese installation/correction guidance; unknown and non-Error values retain the generic fallback. Focused tests cover direct and wrapped secret-bearing errors, both actionable categories, generic failure and successful playback. No proxy, IPC, process, persistence, layout, navigation, search or episode-selection behavior changed.

Post-repair backend tests/race/vet, frontend tests/check/build and serialized full Go tests/race/vet passed. Fresh correctness and security/resource reviews plus oracle closeout found no remaining MUST_FIX. Controller mapping, Wails `Error.message` behavior and the unchanged HomeView `role="alert"` render path prove the bounded message change, so no human gate remains. The focused six-file repair was committed and pushed as `028b02210d0ab6e986ccae2b8ac3a77799019343`; exact-SHA GitHub Actions run 34874294714, linux job 104077534611, completed SUCCESS with all frontend/Home, Go test/race/vet, vulnerability, desktop build and clean-worktree checks green. Advisory only: backend/frontend duplicate the two fixed wire strings; no broader shared/generated contract was introduced. Local live MPV v0.41 discovery/version execution succeeded, but both native IPC smokes timed out and left no orphan; cause remains unresolved and is not claimed as a repository or sandbox failure.

### RA-02 Anime source — LEGACY_VERIFIED

Current-state retrospective audit of historical Loops 05–09 is complete at `2160e5debe1e4ff9cbe1508ddcfed3a9dd4c58c0`. Audited the AnimeSource port, Anime1 catalog/search, archive episodes/pagination, resolver, schedule, adapter acceptance, parser policy, and narrow core/persistence/desktop DTO secret-flow traces. Focused adapter/contract, race, and vet checks plus `go test -count=1 ./...` passed in the local Codex sandbox with repository-local GOCACHE. Exact-HEAD hosted CI 34790321962/linux 103813204911 is reused for full race, vet, module, vulnerability, build and clean-worktree evidence.

Fresh correctness/quality and security reviews found no MUST_FIX. Current tests cover untrusted JSON/HTML, URL/origin/stream/cookie handling, MIME and resource bounds, cancellation/no-partial-result behavior, redaction and output/persistence isolation. No live upstream request was needed or made; deterministic fixtures certify current behavior, not current upstream compatibility. No repair, production change, human gate, commit or remote mutation. RA-03 later began and is recorded above without reopening RA-02.

### RA-01 Foundation — LEGACY_VERIFIED

Current-state retrospective audit of historical Loops 01–04 is complete at `2160e5debe1e4ff9cbe1508ddcfed3a9dd4c58c0`. Audited repository/module and architecture boundaries, core models/ports/App facade, shared contract/fake/validator suites and secure HTTP. Focused test, race and vet commands for `./core ./tests/contract ./tests/architecture ./adapters/network/securehttp` passed in the local Codex sandbox with repository-local GOCACHE. Reused exact-HEAD hosted CI 34790321962/linux 103813204911 for final integrated regression, module, vulnerability, build and clean-worktree evidence; no Loop 23 check was rerun.

Independent correctness/architecture and security reviews found no MUST_FIX. Secure HTTP current-state coverage includes SSRF/DNS actual-destination pinning, redirect validation/header stripping, TLS, response limits, cancellation, closure and redaction. The bounded streaming proxy trace confirms context cancellation and transfer-owned body closure. Two review advisories (early App nil-guard consistency; source fixture membership allowing extras) are not confirmed defects or failed criteria. Historical process-only requirements are `NOT_APPLICABLE_PRE_V2_4_1`; LEGACY_VERIFIED certifies current implementation only. No repair, production change, human gate, commit or remote mutation. RA-02 is complete.

### Loop 23 — CLOSED / PASS

Closeout published one focused commit on main: `2160e5debe1e4ff9cbe1508ddcfed3a9dd4c58c0` (`test: complete Loop 23 Home acceptance tooling`), pushed directly to origin/main. Exact nine-file scope: apps/desktop/backend/home_integration_test.go and tools/native-home-acceptance/{README.md,fixture.go,fixture.json,fixture_test.go,main.go,process_other.go,process_windows.go,process_windows_test.go}. Existing Home implementation and required CI were already published; no workflow change was needed. Staged enumeration, whitespace check and formatting were clean. Protocol documentation, .codex/, .ignore, recovery/state files, local config, temporary artifacts and generated binaries were excluded. No feature branch, PR or repository administration changes.

[CI run 34790321962](https://github.com/loustack17/AnimePortable/actions/runs/34790321962) at that exact SHA: completed SUCCESS. [linux job 103813204911](https://github.com/loustack17/AnimePortable/actions/runs/34790321962/job/103813204911): SUCCESS, all 24 steps success; started 2026-09-13T23:39:16Z, completed 23:41:11Z. Chromium interaction verifier PASS; log confirms 20/20 controller round labels and successful step completion. Complete frontend verification, Go tests (including new backend and acceptance-tool packages), race, vet, module/format checks, vulnerability check (none found), desktop build and clean-worktree gate PASS. No CI failures or corrective commits required. Linux CI does not replace prior Windows-specific/native evidence.

Evidence collected through read-only `gh run view 34790321962 --json headSha,status,conclusion,jobs,url` and `gh run view 34790321962 --log`. Git index/publication and GitHub read permissions were explicitly escalated; no verifier used host privilege expansion. Human Home/offline PASS remains as recorded below. Loop 23 is CLOSED/PASS. RA-01 was not started. Local durable closeout updates remain uncommitted; unrelated working-tree changes are preserved.

### Loop 23 — PASS

On 2026-09-13 the user explicitly reported Offline state PASS after disconnecting the network and relaunching: Continue Watching persisted, Following persisted, and saved position 2:05 persisted. This closes the last Home human gate under the reconciled scope below. Earlier NOT_CONFIRMED/BLOCKED/NEEDS_HUMAN entries are historical and superseded.

Final evidence combines accepted native Home layout/text/navigation/hover-click/resizing and populated state; this disconnected-relaunch observation; hosted production CI 34738919996 at 2ab60163 (Chromium, all 20 controller rounds and full CI); locally passing supplemental Home boundary tests/race/vet; and recorded independent quality/security/verifier reviews without MUST_FIX. Playback implementation is unchanged and its prior evidence remains inherited; this does not claim a new real-MPV human run or waive global playback criteria.

No remaining Loop 23 human checks. Main remains 2ab60163; supplemental test/tooling and state documentation remain local/uncommitted, so hosted evidence is not claimed for those unpublished additions. No production changes, user-data operations, commit/push or remote mutation in this closure. No completed verification repeated. RA-01 has not started; retrospective baseline is the next separate phase and blocks Loop 24.

### Home acceptance scope reconciled — NEEDS_HUMAN

Final local evidence: `go test -count=1 ./apps/desktop/backend`, `go test -race -count=1 ./apps/desktop/backend`, and `go vet ./apps/desktop/backend` all PASS using repository-local GOCACHE. Scoped diff whitespace PASS. Independent read-only `/root/home_scope_review` PASS with no MUST_FIX: boundary test is valid; synthetic coverage does not prove native audio/video and does not remove global playback criteria. Reviewer's default-cache test attempt was denied; root's actual isolated tests above passed. No completed Chromium/20-round CI replay.

This entry supersedes earlier instructions requiring the user to find Anime1 IDs, operate fixture/recovery commands or retest the entire MPV pipeline. Latest explicit user acceptance records PASS for layout, text, navigation, hover/click, resizing, populated Continue Watching, populated Following and saved position 2:05. No explicit disconnected-relaunch observation is present in the available record: offline human evidence remains NOT_CONFIRMED, not FAIL and not fabricated PASS.

Scope evidence: `git show --stat 19c21075` contains Home frontend, tests and verifier changes; `git diff d12da690 HEAD -- core adapters internal apps/desktop/backend/actions.go apps/desktop/backend/service.go` is empty. Playback, persistence and production composition are unchanged. MVP plan Phase 22 is Home (execution Loop 23), with cached SQLite/no-network validation. Verification matrix classifies PLAY-004..008 and PLAY-010..013 as automated/integration (`Y`), unlike FUNC-001's visible offline flow (`YH`). Prior blanket native playback wording in the Loop 23 recovery contract over-expanded the UI scope; latest user instruction authorizes this correction, not deletion or relaxation of global playback criteria.

Inherited pipeline evidence: completed-items records include live MPV 0.41 detection/start/reap, Windows named-pipe property/stop/cleanup, same-PID three-media loading with capability revocation, and live IPC/media smoke after playback tracking. Recorded commands include `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 ./adapters/player/mpv -run 'TestLive' -v`. These are historical evidence, not newly executed native acceptance or a retrospective baseline audit. Current hosted CI 34738919996 at 2ab60163 passed the existing deterministic pipeline regression suite; it does not substitute Linux for native MPV. No playback change invalidates the inherited evidence in this loop.

Home boundary: hosted Chromium verifier asserts the actual generated binding call arguments `{animeId:'a', episodeId:'opaque-latest', startAt:125500}`, plus pending/error/cancellation behavior. New `TestHomeHistoryAfterRestartInvokesExistingPlayback` uses real SQLite Store APIs, closes/reopens persisted history/mapping, passes the returned Home DTO through unchanged Service.Play/core path, and asserts exactly one Player start with canonical identity, resolved test source and 125500 ms resume. Fake source/player intentionally isolate the Home boundary; this is not real-stream or MPV execution. Existing offline integration test retains its uncallable source assertion. Production code and existing tests are unchanged.

Remaining legitimate human check: only whether the populated Home state remains visible after a disconnected relaunch. If already observed, record the user's confirmation without repeating it. Full real MPV playback is inherited subsystem coverage, not a new blocking human task for this UI-only loop. Setup/recovery/cleanup and any future live-source discovery belong to automated tooling, never the end user. README now separates simple observations from maintainer responsibilities; current CLI is not falsely claimed to provide a turnkey launcher. No actual profile mutation or remote action performed. Loop 23 remains NEEDS_HUMAN; RA-01 unstarted.

### Guard lifecycle repaired locally — NEEDS_HUMAN

Supersedes the blocked investigation below. First-launch output confirms post-Wait WebView2 `EBWebView/Default/*.tmp` sharing violation: immediate inventory failed and retained the old guard. The separate `application unavailable` binding log is not diagnosed as a production defect. No production code changed.

Tool-only repair: bounded inventory settling (25 scans, 200 ms separation, two matching successful snapshots), OS exclusive lease, root/PID metadata and explicit `recover`. Legacy recovery requires canonical root/ownership, dead recorded PID, no native app/other launcher, unchanged ownership and safe settled inventory. Unknown/access-denied/live process evidence refuses. Recovery refreshes only inventory metadata and releases the guard; database/browser data is preserved. Reset remains leaf-only with exact inventory/digest validation. Incomplete ownership/state remains fail-closed.

Changed main.go, fixture_test.go, README.md; added process_windows.go, process_other.go, process_windows_test.go under tools/native-home-acceptance. Existing fixture.go/fixture.json unchanged by repair. Root self-review and fresh read-only /root/guard_final_review PASS, no MUST_FIX.

Final default-sandbox verification using repository-local GOCACHE: `go test -count=1 ./tools/native-home-acceptance` PASS (18 top-level tests plus subtests); `go test -race -count=1 ./tools/native-home-acceptance` PASS; `go vet ./tools/native-home-acceptance` PASS; `go build -o apps/desktop/bin/native-home-acceptance.exe ./tools/native-home-acceptance` PASS; Linux amd64 CGO-disabled cross-build PASS. Includes real Windows sharing-lock regression, own-process liveness, simulated abnormal exit, active/unknown/leased/native-process refusal, legacy/v2 recovery, ownership/path refusal and repeated lifecycle. Windows tool SHA256: `5CBC5A3CD7FE030745D48E5C1B5C1E00F013F9E714EEA36DB43C775DAD59EEDE`.

Actual acceptance state/legacy guard untouched. User closes app/launcher instances, runs `./apps/desktop/bin/native-home-acceptance.exe recover`, then `./apps/desktop/bin/native-home-acceptance.exe launch --exe ./apps/desktop/bin/animeportable.exe`. No prepare/reset needed for recovery. Native recovery/close/relaunch, populated history, offline restart and actual Home-to-MPV playback/resume remain human-gated. Accepted visual checks stand. No production/CI diff or remote mutation; unrelated edits preserved. Loop 23 not PASS; RA-01 unstarted.

### Native acceptance blocked — fixture/launcher guard defect

User reports that after normal native app exit, `launch`, `reset`, and `prepare` all return `operation active or stale guard exists; preserve state and request recovery`. Classify this as `TEST_FAILURE` in the development acceptance fixture/launcher, fingerprint `NATIVE_ACCEPTANCE_GUARD_RETAINED`; it is not a production defect or the prior sandbox child-process limitation. The earlier tooling automated PASS/review does not override this actual native failure.

Human evidence: populated history is NOT_CONFIRMED because the report contains `PASS / FAIL` without selecting the first observed result. Populated offline state is BLOCKED. Real playback is BLOCKED. Previously accepted layout/text/navigation/hover-click/resizing remain accepted; no request to repeat them.

Read-only inspection confirms the acceptance operation guard exists and contains `pid=42456`. No `animeportable` or `native-home-acceptance` process was observed by name during inspection; this is a point-in-time observation, not proof sufficient for forced lock removal. Source inspection shows launch retains the guard after runner failure or post-exit inventory/metadata failure, and guard release can also fail. The triggering branch/root cause is not established; normal window closure alone does not establish successful launcher completion. Existing automated launch coverage uses a simulated runner and did not establish the real native close/cleanup sequence.

Current action is preservation and checkpoint only: no guard/profile deletion, no prepare/launch/reset retry, no acceptance database writes, no production/tooling code edits, no rebuild, and no remote mutation. Main remains `2ab60163`. Next investigation must reproduce/localize the launcher lifecycle failure without changing production or discarding the original acceptance state, then define a reviewed tooling-only repair/recovery. Loop 23 is BLOCKED and not PASS; RA-01 remains unstarted.

### Native fixture complete — NEEDS_HUMAN

Resumed at unchanged main `2ab60163c9e690f6edfbfae97b80926c307e9893`; prior local implementation authorization persisted. The interrupted drafts are now replaced by complete local tooling in exactly the five approved files: `tools/native-home-acceptance/main.go`, `fixture.go`, `fixture_test.go`, `fixture.json`, `README.md`. This entry supersedes the incomplete/budget-blocked tooling entries below. No production, schema, workflow, dependency or normal-user data changes; no commit/push/PR/remote mutation. Scope check `git diff --name-only -- apps core adapters .github` is empty.

Implementation: embedded versioned fixture; validated Store APIs and close/reopen state checks; synthetic populated state or explicit real category/post pair with bounded secure Episodes membership preflight; no Resolve call/stream secrets in tooling. Only launched child APPDATA is overridden. Fixed profile, canonical ancestors/reparse rejection, shared exclusive guard, exact ownership/manifest/inventory/file hashes and individual leaf deletion protect reset. Clean child completion inventories allowed SQLite/WebView state; abnormal/uncertain state retains guard. No forced cleanup. README includes compile-once offline workflow, runtime MPV path input, normal-database/sidecar hash comparison, reset/refusal behavior and the three remaining native human gates.

Final local evidence in default sandbox, using process-local `GOCACHE=<repo>/.slim/deepwork/go-cache`:

- PASS: `go test -count=1 -v ./tools/native-home-acceptance` — 11 top-level tests plus subtests; zero skips. Real Store state/relationships, deterministic reset/reprepare, normal-data sentinel, real identity mapping using injected membership result, preflight/invalid-input refusal, ownership/path/hash/unknown/incomplete-state checks, guard exclusion, simulated runtime WebView/progress updates, failed-launch guard retention, actual symlink ancestor/leaf refusal, fixture validation and environment replacement.
- PASS: `go test -race -count=1 ./tools/native-home-acceptance` and `go vet ./tools/native-home-acceptance`.
- PASS: Windows `go build -o apps/desktop/bin/native-home-acceptance.exe ./tools/native-home-acceptance`. Artifact SHA256 `B5990EE708C9502399649ECD1316FC0345861743034CCF5826A27E7E1E7E6047`.
- PASS: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o .slim/deepwork/native-home-acceptance-linux ./tools/native-home-acceptance`; confirms added tool package compiles for existing Linux CI, not native acceptance.
- PASS: gofmt produces no differences; scoped status/handoff whitespace checks. Global `git diff --check` reports inherited EOF blank lines in `docs/08_VERIFICATION_MATRIX.md:387` and `docs/09_CODEX_EXECUTION_PROFILE.md:350`; those unrelated edits are preserved.
- Environment note: initial default Go cache denial was resolved by allowed repository-local cache placement. Go emitted nonfatal telemetry `upload.token` Access denied messages, even with GOTELEMETRY=off; actual compiler/tests exited 0. No host config change, permission escalation or unsandboxed verifier was used.
- Root self-review completed; fresh read-only `/root/fixture_security_review` returned final PASS_REVIEW after reviewing code, README and expanded tests, with no MUST_FIX. Its earlier default-cache denial is superseded by the above actual test evidence.

Not performed: live source network preflight, user AppData preparation/reset, native application launch, real WebView2/MPV execution or actual normal-database hash comparison. Automated launch tests use a controlled runner callback and temp directories; they do not claim native process behavior. Production CI at 2ab60163 remains valid and was not repeated. General native layout/text/navigation/hover-click/resizing remain accepted. User must now follow README to verify populated viewing history, populated offline restart, and actual Home-to-MPV audio/video with persisted progress/resume after app restart, recording Windows version and executable SHA256. Upstream failure stays BLOCKED. Loop 23 is not PASS; RA-01 remains unstarted.

### Native fixture implementation interrupted — ABORTED_BUDGET

User approved local-only implementation of the five proposed tooling files, tests and independent review; authorization remains valid. Plan review `/root/fixture_plan_review` returned PASS_PLAN after shared operation-guard, exact AppData isolation, source-membership preflight and fail-closed reset corrections. Worker `/root/fixture_implementation` then terminated with a provider usage-limit error (reported retry 5:30 AM, timezone unspecified).

Four incomplete untracked drafts exist: main.go, fixture.json, fixture_test.go and README.md; fixture.go is missing. No tooling tests, native launch, AppData fixture installation, final self-review or independent implementation review completed. Root read-only triage identified unused imports, real-mode arguments not applied to persisted state, absent membership preflight, inadequate path/marker/link checks, incomplete reset/browser-cache ownership handling, inadequate tests and machine-specific README examples. These are unfinished tooling issues, not production defects. README is explicitly marked DO NOT RUN; its draft claims/commands are not verified instructions. Preserve drafts for recovery but do not execute them. Resume from `.slim/deepwork/loop23-native-fixture.md`; complete and review the approved implementation before giving usable commands.

No production/CI changes, commit, push, PR or remote mutation occurred. Prior CI PASS and partial native UI approval remain valid; remaining populated/offline/real playback acceptance is still pending. RA-01 remains unstarted.

### Partial human acceptance and native acceptance setup proposal

User approves currently reachable native layout, displayed text, navigation, hover/click behavior and resizing. Do not request these again unless implementation materially changes. This is partial acceptance, not blanket UX/playback PASS. Remaining evidence is populated Home/Continue Watching behavior, populated cached startup without network, and real native MPV playback/resume/progress after restart. Search and episode-selection UI are out of scope. The blocker is testability, not a confirmed production defect.

Inspection found no native seed/install mechanism. `apps/desktop/backend/home_integration_test.go` uses validated Store APIs but its temporary state lacks playback source/mapping records and is removed by testing. `apps/desktop/frontend/tests/browser/home.mjs` mocks bindings and cannot certify native playback. MPV live tests use test-internal proxy/media injection rather than the production desktop path. `Service.Play` requires a persisted episode mapping before player creation. `openProduction` uses `os.UserConfigDir()/AnimePortable/animeportable.db`; the installed Go Windows implementation reads AppData.

HUMAN_GATE proposal, not implemented: add only `tools/native-home-acceptance/main.go`, `fixture.go`, `fixture_test.go`, `fixture.json`, and `README.md`. A development command seeds through sqlite.Open migrations and existing SaveAnime/SaveSourceRef/SaveEpisodeMapping/SetFollowing/SavePlaybackCheckpoint/SaveSettings validation, closes/reopens and verifies expected state. No SQL editing, production imports of tooling, or schema/validation changes. Fixed canonical IDs/timestamps/resume positions and source references live in the versioned manifest; machine-specific MPV paths are optional runtime inputs, never committed. Offline data is deterministic; a separately requested live preflight validates the explicit provider episode through the existing secure Anime1 adapter, without storing stream URLs/tokens. Upstream availability is not deterministic and remains separately reported.

Proposed commands: prepare, launch, reset. Tool-owned profile under the normal AppData AnimePortable/acceptance/loop23 subtree, with child-process-only AppData redirection, launches the unchanged native executable. No normal database overwrite, registry/global environment changes, or host-wide installs. The redirected profile can also affect inherited MPV configuration, so record that boundary and do not claim normal-profile config preservation. Prepare refuses existing state; reset is allowed only for the exact owned, validated profile after the app exits, refusing symlink/reparse escapes and unknown contents. Reset then prepare restores the manifest. A manifest/ownership marker and executable identity are recorded locally. Implementation must verify isolation, valid state reopen/mappings, deterministic reset, invalid-input rejection, and independent security/verifier review before native use. No new harness, production edit, network preflight, fixture install or commit performed in this proposal turn. Await explicit approval; RA-01 remains unstarted.

Loop 23 — NEEDS_HUMAN: final hosted deterministic verification PASS; Windows native UX/offline/real playback acceptance pending. Historical pending-publication/CI entries below are superseded by this final evidence.

### Final hosted evidence at 2ab60163

- Commit: `2ab60163c9e690f6edfbfae97b80926c307e9893`, main and origin/main; local app files match HEAD.
- [CI 34738919996](https://github.com/loustack17/AnimePortable/actions/runs/34738919996), [linux job 103675155050](https://github.com/loustack17/AnimePortable/actions/runs/34738919996/job/103675155050), attempt 1, push event: SUCCESS, all 24 steps success. Run started `2026-09-13T04:52:34Z`, final update `2026-09-13T04:54:05Z`.
- GitHub-hosted `ubuntu-latest` resolved to Ubuntu 24.04.5, image `20260907.300.1`. Actual isolated runner evidence; host diagnostics excluded.
- `npm ci`, binding generation, `npm run check`, `npm test`, `npm run build`, `npm audit --audit-level=high`: PASS; Svelte zero errors/warnings, npm zero vulnerabilities.
- `node node_modules/playwright/cli.js install --with-deps chromium`: PASS, matching Chromium revision 1223 / Chrome for Testing 148.0.7778.96 installed.
- `node tests/browser/home.mjs`: PASS log confirms typed fixtures, latest history, plain text, keyboard Play, pending guard, redaction, six destinations, skip focus, responsive layout, cancellation, retry, empty state and local-only requests.
- `for cycle in {1..20}; do node --test --test-name-pattern='controller' tests/home.test.mjs || exit 1; done`: PASS. Log extraction counted 20 round labels, 20 summaries with 3 passes, 20 with zero failures, 20 with zero skipped tests: 60 controller test executions total.
- Go formatting, `go mod verify`, `go test ./...`, Anime1 adapter acceptance, `go test -race ./...`, `go vet ./...`, `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`, Linux desktop build and clean-worktree gate: PASS. govulncheck reports no vulnerabilities.
- Evidence retrieved with `gh run view 34738919996 --repo loustack17/AnimePortable --json headSha,status,conclusion,jobs,url` and `gh run view 34738919996 --repo loustack17/AnimePortable --log`; all round summaries independently counted from logs. Read-only network escalation does not change the hosted verifier boundary.

No repository/verifier failure occurred in this run. Native Windows acceptance is NOT_RUN: record Windows version, actual app build/artifact, display resolution/scaling, cached offline startup, keyboard and mouse navigation/Play, visible focus/no trap, loading/error/empty states, small-window clipping and actual MPV playback. Report per-observation PASS/FAIL/BLOCKED and overall HUMAN_PASS/HUMAN_FAIL/HUMAN_BLOCKED. Existing local executable predates publication; record its identity if used and do not claim Linux build proves native behavior. Loop 23 is not PASS until this human evidence passes. RA-01 has not started. State updates remain uncommitted.

### 2026-09-13 runner-label follow-up

User explicitly requested changing the runner to `ubuntu-latest` and then verifying CI. Commit `2ab60163c9e690f6edfbfae97b80926c307e9893` (`ci: use ubuntu-latest runner`) contains only that one-line change in `.github/workflows/ci.yml` and was pushed to `origin/main`. Focused diff/whitespace checks passed; independent read-only `/root/runner_review` returned PASS_REVIEW with unchanged triggers, steps, tests, versions, timeouts and permissions. No recovery/config files were committed.

[Final revision CI run 34738919996](https://github.com/loustack17/AnimePortable/actions/runs/34738919996), linux job `103675155050`, is in progress. The preceding ubuntu-24.04 run also started; queue causality is unconfirmed. Record final verification against 2ab60163, not the preceding revision. No CI rerun/cancellation or administration operation was performed.

### 2026-09-13 authorized publication

User authorized one commit containing only the ten listed Loop 23 implementation/verifier files, directly on main. Commit `19c21075e9e2028b172b5c196471e8139b3e8c9c` (`feat: add Home UI and hosted verification`) was pushed successfully to `origin/main`. No branch, PR, administration change, or protocol/config/recovery file was included. Staged file enumeration and whitespace check passed before commit; the resulting commit contains exactly ten files. Git index writes and network operations required tool escalation; no verifier ran unsandboxed.

[CI run 34738400184](https://github.com/loustack17/AnimePortable/actions/runs/34738400184) is queued for that exact SHA. Chromium interactions, all 20 controller rounds and the complete run remain pending. Windows native UX/offline/real playback acceptance must follow CI PASS; Loop 23 remains NEEDS_HUMAN and RA-01 has not started. This publication record supersedes the earlier pending-publication entries below. Recovery/state changes remain local and uncommitted.

Loop 23 — Home UI — NEEDS_HUMAN (approved local verifier changes complete; publication authority and hosted evidence pending)

Current repository instructions require Protocol v2.4. Recovery observed `main` at `d12da690`, no staged changes, and inherited uncommitted Home implementation plus protocol documentation changes. No production code was changed during recovery. Historical completed items and command lists below remain historical evidence, not final Loop 23 verification.

Home already contains typed local reads, Continue Watching selection and Play, Following titles, empty Recently Updated/Today sections, cancellable request ownership, generic errors, controller tests, SSR checks and a SQLite reopen integration test. All six Home paths remain preserved; see `docs/AGENT_HANDOFF.md` for scope and recovery actions.

The previous execution blocker is cleared. Git status, recent log and Home diff reconcile with inherited main/d12da690. Earlier recovery preserved all implementation. The subsequently approved local workflow/verifier/dependency changes are recorded below; production Home, existing tests, bindings and host configuration remain unchanged this turn. Current evidence below supersedes historical command lists.

The sandbox-mode prerequisite is cleared. The user verified `/debug-config`: highest-precedence managed platform session flags from the custom launcher set `windows.sandbox = "unelevated"`. The older global `elevated` value does not describe the effective session. Normal-session process health is also confirmed. Host configuration and filesystem permissions remain unchanged.

Verification resumed on the inherited implementation. RA-01 through RA-06 is authorized only after Loop 23 PASS; Loop 24 must not begin.

### Current recovery evidence

Frontend commands ran in `apps/desktop/frontend`; desktop build in `apps/desktop`; Go checks at repository root. Default tool sandbox results:

- PASS: `go test -count=1 ./apps/desktop/backend -run '^TestHomeBindingsReadSQLiteAfterRestartWithoutSource$'`.
- PASS: `npm test` (5/5), `npm run build`, `npm audit --audit-level=high` (zero vulnerabilities).
- PASS: `npm run check` on later unchanged default-sandbox retry, zero errors/warnings. Initial Vite `exec(net use)` spawn EPERM and escalated diagnostic pass are retained as transient environment evidence; root cause is unconfirmed.
- PASS: `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go mod verify`.
- PASS: `go build -o bin/animeportable.exe .`; ignored Windows desktop artifact, native runtime not verified.
- PASS: `git diff --check` and `gofmt -l apps/desktop/backend/home_integration_test.go`; existing CRLF warning only.
- BLOCKED: `node .slim/deepwork/home-browser.mjs`, Chromium spawn EPERM, including one unchanged default retry.
- BLOCKED: 20 iterations of `node --test --test-name-pattern='controller' tests/home.test.mjs`, Node test-worker spawn EPERM on cycle 1.
- BLOCKED network execution: `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` and connected GitHub CI read encountered sandbox proxy refusal.

Tool-required escalated diagnostics, excluded from sandbox acceptance: browser fixture PASS (real generated bindings with local fixtures, canonical Play milliseconds, one pending Play, plain text, sanitized failures, keyboard navigation/skip focus, resize, cancellation/late import, retry and empty state); controller tests PASS for all 20 cycles; govulncheck reported no vulnerabilities. No RAM/goroutine-growth claim is made. `gh run list --repo loustack17/AnimePortable --limit 3 --json databaseId,headSha,status,conclusion,url` observed [CI 34717734043](https://github.com/loustack17/AnimePortable/actions/runs/34717734043) SUCCESS at d12da690 via escalated read. CI does not cover uncommitted Home.

Fresh-context read-only reviews `/root/home_quality`, `/root/home_security`, and `/root/home_final` found no production MUST_FIX or weakened verifier. Final reviewer withheld Loop PASS for remaining gates; its stale typecheck-blocker note was corrected with the later default PASS. Simplify inspection found no necessary structural change; dense App else markup remains advisory. Recently Updated/Today empty notices fit absent local update/schedule storage. No speculative cleanup or implementation replay.

Containment record: no danger-full-access, host configuration edit or filesystem permission expansion. Escalated retries are segregated above, not sandbox certification. Initial PowerShell profile host-cache write attempts were denied; all subsequent calls used `login:false`. Generated build and browser screenshot artifacts are ignored.

Remaining gates: obtain approved isolated hosted verification for affected checks, complete required security and reproducible final integrated verification, then obtain native `HUMAN_PASS`. Human procedure must record Windows/build/display, cached offline startup, keyboard-only and mouse Play, visible focus/no trap, loading/error/empty states, small-desktop resize/clipping and actual playback. No RA batch has started. Resume remaining verification from the handoff without revisiting effective sandbox mode.

### Hosted verifier decision and approved local implementation

The user classifies the unresolved Windows Node child-process fingerprint as `SANDBOX_CHILD_PROCESS_LIMITATION`, an execution-infrastructure limitation, not `TEST_FAILURE`. Effective `unelevated` configuration and normal sandbox execution are confirmed. Do not retry alternate shells, Node/Chromium paths, ACL changes, broader permissions or danger-full-access. Unsandboxed host diagnostics remain diagnostic only. GitHub-hosted Actions is approved as the isolated verifier for affected deterministic checks; workflow changes still require explicit approval.

Read-only inspection reconciled main/d12da690 and the inherited dirty Home state. The only repository workflow is `.github/workflows/ci.yml`, using `ubuntu-24.04` with push/pull_request triggers. Its frontend step runs check, test, build and audit. `package.json` maps test to `node --test tests/*.test.mjs`: a single suite invocation, with no 20-cycle execution. There is no Playwright dependency, Chromium install or browser step. The existing browser fixture in ignored `.slim/deepwork/home-browser.mjs` imports a user npm-cache path and launches a fixed Windows Chromium executable. It is unavailable/nonportable on a clean hosted checkout. Existing CI therefore does not cover the missing verification; no run was triggered or accepted for these checks.

The user approved the following local-only scope. It is now implemented; no commit, staging, push, PR, release, workflow dispatch or other remote mutation occurred:

1. Existing Linux CI adds three steps after frontend verification: Chromium install, browser interactions and 20 controller rounds. Both verification steps have a five-minute timeout. Existing steps, triggers, clean-worktree gate and security settings are structurally identical to HEAD. No secrets or permissions added.
2. New non-ignored `apps/desktop/frontend/tests/browser/home.mjs` adapts the existing fixture; it is ready for version control but intentionally not staged/committed. Built assets resolve relative to the script, Playwright is imported by package name, and its managed Chromium is used without a host executable path. Screenshots stay in memory. Existing 19 assertion ASTs are unchanged; page/navigation waits are bounded and server cleanup runs even if browser close fails. Existing ignored fixture is preserved.
3. Frontend manifest and lockfile pin `playwright` and `playwright-core` to `1.60.0` with registry integrity hashes. The exact optional macOS fsevents dependency is nested, preserving the existing fsevents version. npm also synchronized the existing root package license metadata; no existing dependency versions changed. CI calls `node node_modules/playwright/cli.js install --with-deps chromium`, so it cannot select an unpinned global/npx Playwright.
4. CI runs `node tests/browser/home.mjs` and, in a separate step, `for cycle in {1..20}; do node --test --test-name-pattern='controller' tests/home.test.mjs || exit 1; done` with a round label. Every round must pass; no failed result is discarded. This provides the missing Chromium interaction and repeated controller evidence when run on the hosted revision, not native playback or RAM/goroutine-growth evidence.

Local verification and review:

- PASS: `node --check apps/desktop/frontend/tests/browser/home.mjs` and `node --check apps/desktop/frontend/tests/home.test.mjs`.
- PASS: `npm install --package-lock-only --offline --ignore-scripts --no-audit --no-fund --logs-max=0` after adding cached exact-version/integrity records. First attempt lacked offline registry metadata (`ENOTCACHED`); no network retry or permission expansion. npm then accepted/normalized the final lock graph.
- PASS: `npm ci --dry-run --offline --ignore-scripts --no-audit --no-fund --logs-max=0`; plans only Playwright/core additions on Windows. This is lock consistency/planning evidence, not an actual clean install or package audit.
- PASS: PyYAML 6.0.3 unique-key YAML parsing and configuration checks against `git show HEAD:.github/workflows/ci.yml`: exact added commands, working directories, timeouts, 20-round fail-fast loop and insertion position; removing the three added steps produces the identical original workflow object. Dedicated actionlint/yamllint executables are unavailable; no hosted Actions parser/runtime claim.
- PASS: Acorn AST comparison preserves all 19 inherited assert calls; package/lock integrity fields and portable dist resolution validated. An initial ad-hoc audit helper guessed more than 20 assertions; this audit-helper assumption was corrected using the actual source count. No repository assertion/test was weakened.
- PASS: root reviewed final scoped diff; fresh-context read-only security/verifier reviewer `/root/ci_verifier_review` returned `PASS_REVIEW`, no MUST_FIX. `git diff --check` passes with the pre-existing App.svelte CRLF warning.

Not run this turn: Chromium launch, repeated Node workers or alternate-path/shell retries; these retain `SANDBOX_CHILD_PROCESS_LIMITATION`. No unsandboxed diagnostics, dependency runtime install, Chromium download or external operation was performed. Production tests/builds above are historical evidence and were not rerun for this verifier-only static-validation slice.

Await separate publication authorization. Hosted verification must use the exact revision containing inherited Home files plus the new workflow/verifier/lockfile and record commit SHA, run/job/check identity and relevant logs. Actual Linux npm ci, matching Chromium install, browser interactions, all 20 controller rounds and existing checks including dependency audit remain unverified for this revision. The current HEAD run does not certify these changes. Windows native UX/offline/real playback HUMAN_PASS remains mandatory afterward; Linux Chromium does not replace it. Loop 23 is not PASS; RA-01 has not started.

## Completed

- [x] Six-section desktop Svelte shell with native keyboard/mouse navigation and explicit active-page state
- [x] Functional skip link, visible focus, Traditional Chinese document language and small-desktop reflow
- [x] Dependency-free Node/Svelte SSR shell contract test integrated into Linux CI
- [x] Shell simplify and independent review, browser interaction checks, full Go/race/vet and Windows build verification

- [x] Root Go module and minimal architecture boundaries
- [x] Wails v3.0.0-beta.12 desktop shell
- [x] Svelte and TypeScript frontend shell
- [x] Source-of-truth documents copied exactly
- [x] Linux CI
- [x] Architecture dependency test
- [x] Local test, build, security, and desktop-start validation
- [x] Clean-checkout CI-order validation
- [x] Independent final review
- [x] Provider-neutral core models and canonical IDs
- [x] AnimeSource, MetadataProvider, Player, and Store ports
- [x] Typed application façade with source-independent local library
- [x] Opaque, cloned, and redacted transient playback source
- [x] Fake-based core replacement and security tests
- [x] Reusable AnimeSource, MetadataProvider, Player, and Store contract suites
- [x] Fake adapters covering supported, unsupported, cancellation, lifecycle, and persistence behavior
- [x] Contract validator rejection tests for invalid adapter output
- [x] Independent code-quality and Oracle final review
- [x] Reusable exact-origin HTTPS client and transport policy
- [x] Connection-time DNS/IP validation with literal-address pinning
- [x] Redirect revalidation and cross-origin sensitive-header removal
- [x] TLS verification, timeout, response-size, and response-header limits
- [x] Sanitized typed errors and centralized URL/header redaction
- [x] Deterministic SSRF, redirect, TLS, cancellation, and body-lifecycle tests
- [x] Anime1 `animelist.json` retrieval through the shared secure HTTP client
- [x] Catalog rows normalized to provider-scoped `SourceRef`s
- [x] Local case-insensitive substring search over the loaded catalog
- [x] Bounded HTML title normalization and JSON catalog parsing
- [x] Fixture tests for malformed input, cancellation, and the `AnimeSource` contract
- [x] Live catalog smoke: 1,893 valid entries and a successful query result
- [x] Oracle and simplify review approval
- [x] Anime1 category archives parsed into provider-scoped episode references
- [x] Bounded sequential pagination with canonical same-origin validation
- [x] Deterministic oldest-to-newest episode order across long-running series
- [x] Non-numeric episode labels preserved without exposing playback tokens
- [x] Exact-limit, malformed-page, cancellation, and no-partial-result coverage
- [x] Live episode smoke: 8-episode and 170-episode archives
- [x] Anime1 episode pages resolved through the fixed `v.anime1.me/api` control endpoint
- [x] Resolver tokens, signed stream URLs, and `e`/`h`/`p` cookies kept transient and redacted
- [x] Non-GET/HEAD redirects rejected before sensitive request bodies can be replayed
- [x] Dynamic Anime1 CDN URLs and playback authorization validated with strict bounds
- [x] Live resolver smoke completed without logging or persisting playback secrets
- [x] Provider-neutral day/time schedule precision and unknown-episode representation
- [x] Anime1 seasonal tables parsed with embedded `Asia/Taipei` calendar rules
- [x] Exact season headers, weekday columns, trusted category links, and stable source order
- [x] Schedule parser limits, malformed schema, cancellation, half-open range, and season-boundary tests
- [x] Live schedule smoke against the current Anime1 seasonal table
- [x] Resolver and schedule simplify passes and independent code-quality reviews
- [x] Named Anime1 adapter acceptance gate covering all five source methods
- [x] Malformed-response zero-value and `%v`/`%+v`/`%#v` secret-redaction regression coverage
- [x] Public `securehttp` to Anime1 adapter composition test without production wiring
- [x] Current Anime1 category-slug compatibility with same-page and cross-page integrity checks
- [x] Live end-to-end adapter smoke for catalog, search, episodes, resolver, and schedule
- [x] CI binding generation before frontend verification, full race detection, and clean-worktree gate
- [x] IPv4 loopback-only playback proxy on an ephemeral port
- [x] High-entropy per-session capability URLs with bounded TTL, registry, and stream limits
- [x] Resolver-owned source URLs and credentials isolated from frontend and local player requests
- [x] Shared exact-origin HTTPS, DNS pinning, SSRF, TLS, and redirect policy for streaming requests
- [x] Strict GET/HEAD, single-range, 200/206/416, MP4 MIME, encoding, and response-header validation
- [x] Deterministic session/server revocation with in-flight request, body, and blocked-writer cancellation
- [x] Concurrent close, saturation, expiry, malformed request, redaction, and body-lifecycle race coverage
- [x] Live Anime1 resolver-to-proxy 1 KiB Range smoke without logging or persisting playback secrets
- [x] Secure playback proxy simplify pass and independent security review approval
- [x] User-configured, PATH, and fixed-platform MPV executable detection
- [x] Fail-closed configured-path validation with sanitized actionable errors
- [x] Windows `.exe`, Scoop Junction, Unix regular-file, and executable-bit validation
- [x] Fixed `--idle=yes` launch without shell, source credentials, or MPV config overrides
- [x] Stable PID plus repeatable concurrent `Done`, `Wait`, and idempotent `Close` lifecycle
- [x] Unix TERM-to-KILL and Windows direct-kill cleanup with bounded stop failures and process reaping
- [x] Deterministic helper-process cancellation, exit, escalation, race, and repeated-cycle coverage
- [x] Windows, Linux, and macOS MPV package cross-compilation
- [x] Live local MPV 0.41 detection, start, PID, close, and reap smoke
- [x] MPV lifecycle simplify pass and independent security/concurrency review approval
- [x] Typed MPV JSON IPC commands for loopback proxy loading, playback properties, observation, stop, and quit
- [x] Random short-lived Unix socket and Windows named-pipe endpoints with bounded startup dialing
- [x] Unix private runtime directory, socket permissions, trusted temp fallback, path bounds, and exact cleanup
- [x] Windows current-user protected named-pipe DACL applied before backend connection
- [x] Bounded JSON framing, request-response demultiplexing, malformed-event tolerance, and sanitized errors
- [x] Coalesced progress events with preserved terminal events and cancellable reader/dispatcher lifecycles
- [x] Deterministic timeout, cleanup, redaction, invalid-media, close, and process-reap coverage
- [x] Live local MPV 0.41 named-pipe property, stop, close, and cleanup smoke
- [x] MPV IPC simplify pass and independent security/lifecycle review approval
- [x] Concrete MPV Player and PlaybackSession adapter with one process per viewing session
- [x] Typed application episode switching with resolver secrets confined to the backend
- [x] Per-episode proxy capability rotation with old-session revocation only after commit
- [x] MPV receive-sequence, pre-load barrier, and post-rejection drain against stale event races
- [x] ACK plus validated `file-loaded` commit with same-PID EP01 → EP02 → EP03 switching
- [x] Fail-closed timeout, cancellation, NACK race, partial cleanup, and Load/Close lifecycle handling
- [x] Coherent canonical playback events with bounded non-blocking delivery and terminal priority
- [x] Deterministic sequence, backpressure, redaction, cleanup, concurrency, and high-count race coverage
- [x] Live local MPV three-episode same-PID smoke with capability revocation checks
- [x] Same-session switching simplify, Oracle, and security/lifecycle review approval
- [x] SQLite Store adapter with canonical local IDs and provider-neutral library state
- [x] Embedded, checksum-verified transactional migrations and schema-tamper rejection
- [x] Durable anime, source references, metadata, following, playback progress/history, and settings CRUD
- [x] Store contract, persistence/reopen, migration, lifecycle, input-validation, and path-safety coverage
- [x] Local database path validation, private Unix artifacts, final-path symlink rejection, and SQLite `nofollow`
- [x] Optional synchronous playback snapshots with fail-closed production capability checks
- [x] Durable resume policy with explicit-start precedence and completed/near-complete suppression
- [x] In-memory progress tracking with 15-second and pause/switch/end/stop/failure/exit checkpoints
- [x] Atomic SQLite progress/history checkpoints with stale-write rejection and monotonic completion
- [x] Double-snapshot episode switching with persistence-before-resolve/load failure safety
- [x] EOF-only completion semantics with distinct stopped/failed terminal states
- [x] Bounded, terminal-safe event delivery across backpressure, reload, and episode ownership changes
- [x] Concurrent idempotent close with cancellable final persistence, raw-player cleanup, and owned-run shutdown
- [x] Restart resume, terminal race, corrupt-state, rollback, backpressure, and lifecycle race coverage
- [x] Live MPV IPC and same-process three-media smoke after playback tracking integration
- [x] Progress/history simplify pass plus independent concurrency and Oracle review approval
- [x] Canonical local follow/unfollow application operations with anime preflight and Store error propagation
- [x] Durable canonical episode-to-provider reference mappings with checksum-verified SQLite migration 0002
- [x] Idempotent mapping writes, global provider-episode conflict protection, composite anime-source ownership, and multi-provider canonical support
- [x] Provider-neutral watched sets with source-order latest-watched and new-episode calculation
- [x] Correct canonical/provider identity handling when local and external episode IDs differ
- [x] Multi-source-ref fallback with caller-cancellation propagation and remote-source graceful degradation
- [x] Playback mapping persistence after successful resolve and before player/session mutation without storing resolver secrets
- [x] Fresh, upgrade, reopen, validation, cancellation, concurrent-conflict, rewatch, and legacy-unmapped regression coverage
- [x] Following simplify pass plus independent code-quality and Oracle review approval
- [x] AniList anonymous GraphQL MetadataProvider through the shared exact-origin HTTPS client
- [x] Bounded Search/Get queries for the MVP title, native title, cover, synopsis, season, year, episode count, and main animation studio fields
- [x] Atomic malformed-candidate rejection, nullable-schema handling, positive IDs, enum/numeric/text/URL/depth bounds, and sanitized HTTP/GraphQL failures
- [x] Plain-text description normalization with linear HTML/Markdown processing and no frontend network access
- [x] Provider contract, adversarial malformed/resource-bound, cancellation, redaction, request-shape, and live Search-to-Get coverage
- [x] AniList simplify pass plus independent code-quality and Oracle final review approval
- [x] Bangumi fallback/cross-check MetadataProvider through the shared exact-origin HTTPS client
- [x] Versioned project User-Agent, anonymous Search/Get requests, and no Authorization propagation
- [x] Chinese-title fallback, original-title preservation, strict date mapping, explicit total/regular episode precedence, and no inferred season/studio
- [x] Provider-neutral atomic JSON and bounded remote plain-text helpers shared with the AniList adapter
- [x] Required search envelope, Anime type, ID/text/date/numeric/cover/body/depth/result bounds, duplicate-key rejection, and sanitized status/error handling
- [x] Tests-first provider contract, request-shape, nullable/malformed/adversarial/cancellation/redaction coverage, plus live Bangumi Search-to-Get smoke
- [x] Bangumi simplify pass plus independent code-quality and Oracle final review approval
- [x] Provider-neutral metadata title normalization for NFKC/full-width, punctuation, conservative season/episode suffixes, and bounded Traditional/Simplified variants
- [x] Deterministic metadata confidence scoring across title/native title, season/year, and episode-count hints without first-result selection
- [x] Fail-closed low-confidence and conflicting-match handling with medium-confidence cross-provider confirmation and stable AniList/Bangumi tie ordering
- [x] Metadata matching fixture coverage for variants, native titles, hints, ambiguity, conflicts, malformed candidates, and empty input
- [x] Metadata matching simplify pass plus focused, shuffle, race, vet, and full repository validation
- [x] MPL-2.0 project license, SPDX coverage, dependency notices, and package metadata migration
- [x] Local standard-library qsort compatibility module replacing the unlicensed upstream source
- [x] Generated build artifacts removed from the repository workspace
- [x] Shared bounded plain-text policy for remote metadata titles, descriptions, seasons, and studios
- [x] AniList/Bangumi Search/Get display-field normalization with provider-neutral contract enforcement
- [x] Fixed-origin on-demand cover loader isolated from provider API clients and frontend network access
- [x] JPEG/PNG status, encoding, MIME, signature, full-decode, byte, dimension, and pixel validation
- [x] Four-operation cancellable cover concurrency bound with owned result bytes and sanitized errors
- [x] SQLite metadata write validation and fail-closed cached-row revalidation
- [x] Malformed/config-only image, unsafe cache, URL, cancellation, redaction, and resource-bound coverage
- [x] Metadata content-security simplify pass plus full validation and independent final review approval
- [x] Typed Wails desktop service covering library, catalog/search, detail/episodes, following, schedule, history, settings, play, and cover-by-local-ID
- [x] Atomic source-anime ingestion transaction with persisted canonical identity reuse and description preservation
- [x] Bounded request admission, lifecycle-scoped cancellation, and ordered shutdown (session before dependencies and store)
- [x] Frontend DTO allowlist with explicit json tags; no provider refs, URLs, credentials, SQL, or raw MPV operations
- [x] Player recovery on terminal playback errors with close-before-restart and actionable MPV configuration errors
- [x] Binding-boundary plain-text normalization for persisted anime titles and offline Following with latest-watched history
- [x] Schedule input/response validation before persistence; episodes multi-ref fallback with correct parent ownership
- [x] Generated TypeScript bindings and reflection-gated method surface test
- [x] Backend simplify, independent security/lifecycle/final review approval

## In progress

Loop 24 Fyne migration M0 contract, parity inventory and resource reference under `docs/16_FYNE_MIGRATION_PLAN.md`. Retrospective baseline is closed; no standalone RA work or production Wails replacement has started. No further long-idle test is authorized for now.

## Blocked

None for retrospective baseline. The earlier Windows Node/Chromium sandbox limitation and completed Loop 23 gates are recorded in the historical entries above.

## Known technical risks

- Wails v3 remains pre-stable at v3.0.0-beta.12
- Windows and macOS release validation remain deferred to their planned phases

## Last verified commands

- `go test -count=1 ./...`
- `go test ./adapters/source/anime1 -run '^TestAnime1AdapterAcceptance' -count=1`
- `go test -race -count=1 ./...`
- `go vet ./...`
- `go build -o bin/animeportable.exe .`
- `go mod verify`
- `npm run check`
- `npm run build`
- `npm audit --audit-level=high`
- `npm ci`
- `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
- `go tool wails3 doctor`
- `go tool wails3 build`
- `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 -run '^TestLiveMPVLoadsThreeMediaURLsOnOneProcess$' ./adapters/player/mpv`
- `go tool wails3 dev -config ./build/config.yml -port 9245`
- `go test -count=1 ./adapters/persistence/sqlite`
- `go test -race -count=1 ./adapters/persistence/sqlite`
- `ANIMEPORTABLE_MPV_LIVE=1 go test -count=1 ./adapters/player/mpv -run 'TestLive' -v`
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./core ./adapters/... ./tests/...`
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./core ./adapters/... ./tests/...`
- `go test -shuffle=on -count=10 ./adapters/metadata/anilist`
- `go test -race -shuffle=on -count=3 ./adapters/metadata/anilist`
- `ANIMEPORTABLE_ANILIST_LIVE=1 go test -count=1 -run '^TestLiveAniListAdapter$' -v ./adapters/metadata/anilist`
- `go test -shuffle=on -count=10 ./adapters/metadata/bangumi`
- `go test -race -shuffle=on -count=3 ./adapters/metadata/bangumi`
- `ANIMEPORTABLE_BANGUMI_LIVE=1 go test -count=1 -run '^TestLiveBangumiAdapter$' -v ./adapters/metadata/bangumi`
- `go test -shuffle=on -count=10 ./core ./tests/contract ./adapters/metadata/...`
- `go test -race -shuffle=on -count=3 ./core ./tests/contract ./adapters/metadata/...`
- `go test ./...`
- `go vet ./...`
- `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./core ./adapters/... ./tests/...`
- `go mod verify`
- `git diff --check`
- `go test -shuffle=on -count=10 ./internal/metadata ./adapters/metadata/... ./adapters/persistence/sqlite ./tests/contract`
- `go test -race -shuffle=on -count=3 ./internal/metadata ./adapters/metadata/... ./adapters/persistence/sqlite ./tests/contract`

Loops 07–21 passed focused and full tests, race detection, vet, live smoke validation where applicable, simplify review, and independent code-quality review.

## Next loop

ADR-020 bans WebView and ADR-021 selects Fyne. `docs/16_FYNE_MIGRATION_PLAN.md` now makes Loops 24–28 the migration and integrates native revalidation of changed RA boundaries; feature progression resumes with Fyne Search at Loop 29 only after migration PASS. Three short Fyne Home host samples (172.3/130.1/125.5 MiB private resident) exceed the preferred idle target; the owner's OneAnime long-idle sample (26.6 MiB resident, 300.3 MiB commit) is not directly comparable. The newest prototype focus fix still lacks a fresh human check, and long-idle testing was declined for now. No production code has been migrated or benchmark gate passed.
