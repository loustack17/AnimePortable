<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 38R: Cleanup before 2.0

Status: IN PROGRESS (slice list approved 2026-10-25; S1-S4 committed, CI green; S5 done, waiting for commit approval). Owner asked for this loop on 2026-10-25. Base: origin/main 8639175.

## Goal

Close the findings in slices S1-S11 below: delete the listed dead code, keep one copy of each listed duplicate, and fix the listed stale comments and docs. The app's behavior, UI text and security checks do not change. Version 2.0 (Loops 39-46) starts after this loop.

## Scope

In scope: behavior-preserving refactors in every Go package, test deduplication that keeps coverage, stale code comments, stale docs.

Out of scope: features; UI, text or product changes; new dependencies; new backend actions (the `main_test.go` allowlist stays fixed); edits to applied SQL migrations (0001-0011); abstractions that only 2.0 would need; anything in the "Owner decisions" list until the owner decides.

Rules for every slice:

- No test is weakened, skipped or deleted to get green. A test is moved or merged only when the same cases still run; the slice notes which test now covers what.
- Security checks in [03_SECURITY.md](../03_SECURITY.md) stay where they are, even when they look redundant (list under "Kept on purpose").
- If a refactor turns out risky or unclear, stop and ask the owner instead of forcing it.

## How the audit was done

- Six read-only code reads, one per area: (1) `core/` and `internal/`; (2) `adapters/network/*`, `adapters/playback/proxy`, `adapters/update/github`, `adapters/player/libmpv`; (3) `adapters/source/anime1`, `adapters/metadata/*`, `adapters/persistence/sqlite`; (4) `apps/desktop/backend`, the `apps/desktop` main package, `tools/`, `tests/`; (5) `apps/desktop/fltkhome`; (6) `apps/desktop/fltkplayer`, `apps/desktop/fltkengine`. A separate read-only reviewer then checked the findings against the code.
- Unreachable functions: `deadcode` from golang.org/x/tools v0.51.0, already in the Go module cache, run without a global install and without network:

  ```
  $env:GOPROXY = "file:///C:/Users/Lou/go/pkg/mod/cache/download"; $env:GOSUMDB = "off"
  go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./...         # production reachability
  go run golang.org/x/tools/cmd/deadcode@v0.51.0 -test ./...   # including tests
  ```

- `staticcheck -checks U1000` did not run: the module cache lacks two of its dependencies (a golang.org/x/tools pseudo-version and BurntSushi/toml), and downloading them was not approved. Unused constants and fields were found by repo-wide grep instead.
- Long functions: the code-review-graph tool listed 28 functions over 70 lines (graph built at 9e68e48, two commits before the base).
- Oracle check of this plan (2026-10-25): three MUST_FIX (1.4 constants read by tests, 9.1 navigation keys differ, 6.7 tests add cases) and eight SHOULD_FIX, all applied above; it confirmed by grep that S1 and S2 have no production caller.

## Findings and slices

One slice is one commit. Each slice heading gives its risk (L low, M medium) and the estimated net change in lines, for example "about -110". Rows cite `file:line` at base 8639175; paths are shortened to the file name when the package is clear from the slice.

### S1. Dead code: adapters and core (L, about -110)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 1.1 | `adapters/network/securehttp/client.go:41-55` | 13 exported alias constants, no user | delete |
| 1.2 | `securehttp/client.go:652-676` | `RedactURL`, `RedactHeaders`, `normalizeHeaders` used only by `TestRedactionClonesAndRemovesSecrets` | delete with that test; header stripping stays covered by the redirect test (`client_test.go:159-205`) |
| 1.3 | `adapters/source/anime1/anime1.go:61` | `type Source = Client`, unused | delete |
| 1.4 | `bangumi.go:39-40`, `anilist.go:32-34`, `sqlite/validation.go:16-17` | 7 constants that production never reads; three are read by tests (`anilist_test.go:528`, `sqlite/library_test.go:144-145`) | delete; those tests use the equal `internal/metadata` policy constants instead |
| 1.5 | `adapters/player/libmpv/libmpv.go:148-150, 230-232, 303-310, 338-340` | `position`, `duration`, `paused` written, never read | delete fields and writes; `Snapshot` keeps its closed check |
| 1.6 | `core/metadata_normalize.go:13-20` | `NormalizeMetadataTitle` used only by tests | move to a `_test.go` file |
| 1.7 | `core/app_test.go:86-94, 324` | `fakeMetadataProvider` used only by a compile-time assertion | delete |
| 1.8 | `apps/desktop/backend/portable.go:36-41`, `service.go:510-516` | `ValidatePortableStatePath`, `parseTime`: no caller | delete |
| 1.9 | `backend/service.go:100-102, 134-150` | `New` and `newWithDependencies` used only by tests | move `newWithDependencies` to a `_test.go` file; tests call `NewAt("")` |

### S2. Dead code: UI and DLL bindings (L, about -160)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 2.1 | `fltkhome/search_model.go:21, 55, 104-127`, `search_view_windows.go:419-431`, `play_entry_test.go` | search playback path (`beginPlayback`, `finishPlayback`, `searchRequestPlayback`, `playbackFailed`) never reached in production | delete; `renderPreview` keeps the loading and failed branches; the uitext keys it used stay (owner decision O5) |
| 2.2 | `fltkhome/detail_model.go:145-148` | `episodeHistory` test-only | delete; `detail_model_test.go:80-83` still covers completed history |
| 2.3 | `fltkhome/search_model.go:292-297` | `selectedItem` test-only | delete; tests index `results[selected]` |
| 2.4 | `fltkengine/internal/mpvwin/api_windows.go:27,32,47,52,80,85,270-274`, `render_windows.go:88-92` | `Mpv.Wakeup`, `RenderContext.ReportSwap` and their bindings: no caller | delete; loader tests rerun with the pinned DLL |
| 2.5 | `mpvwin/types_windows.go:46, 110, 113-123`, `api_windows.go:247-249` | `Event.Error`, `EndFileEvent.Error` never read | delete |

### S3. Shared Anime1, proxy and cover validation (M, security-relevant, about -25)

Checks are moved, never loosened; each moved check keeps its existing tests.

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 3.1 | `adapters/playback/proxy/proxy.go:338-348` = `securehttp/client.go:694-704` | header-name validator copied byte for byte | export one from `securehttp`, call it from the proxy |
| 3.2 | `anime1.go:815-822` and `:868-875` | same reference parse and anime1.me target check twice | one `resolveSiteReference(base, href)` |
| 3.3 | `cover/cover.go:22-23, 120`, `anilist.go:26`, `bangumi.go:32, 548-550` | cover host names spelled in five places | one pair of host constants in `internal/metadata`; each check stays as it is (AniList accepts only its host, Bangumi only its host, the cover loader both, with its trailing-dot trim) |

### S4. HTTP and metadata provider boilerplate (M, about -180)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 4.1 | `securehttp`-level copies: `isJSONResponse`/`isJSON` (anime1 `:379`, bangumi `:637`, anilist `:420`, aniskip `:138`), `sanitizeRequestError` and variants (anime1 `:391`, `:489`, `resolver.go:153`, bangumi `:649`, anilist `:432`), `checkContext` (anime1 `:384`, bangumi `:642`, anilist `:425`), request tail (bangumi `:343-355`, anilist `:227-239`) | one policy in up to seven copies across four adapters | one function each in `securehttp`, with the fallback sentinel passed in, so every returned error stays the same sentinel or `*securehttp.Error` |
| 4.2 | metadata-only copies in `bangumi.go:555-626` vs `anilist.go:355-409`: `decodeResponse`, `validSearchQuery`, `boundedText`, `parseMetadataID`, `parseJSONMetadataID`, `validCoverURL`; `containsDisallowedControl` also in `internal/metadata/policy.go:143` | copied between the two metadata adapters | move to `adapters/metadata/internal`; export the existing `internal/metadata` predicate instead of a fourth copy; `boundedText` keeps its `*string` (nil gives `("", true)`) and uses `TitleLimits()` or, for the studio call (`anilist.go:338`), `StudioLimits()` |
| 4.3 | `adapters/metadata/internal/helpers.go:41-94` = `anime1/resolver.go:378-431` | JSON scanner copied; the resolver's decoder uses `UseNumber` | one scanner in a new `adapters/internal/jsonscan` package (importable by anime1 and the metadata adapters only); the caller builds the decoder, so `UseNumber` stays where it is; add it to 02_ARCHITECTURE |
| 4.4 | `anime1.go:333-368` vs `:762-790` | two visible-text extractors | one iterative `visibleText(...)` with the limits and the sentinel passed in (title and episode limits differ) |
| 4.5 | `adapters/metadata/internal/helpers.go` | file name breaks the no-`helpers` naming rule | rename by content (e.g. `json.go`, `text.go`) |

### S5. SQLite adapter (L-M, about -85)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 5.1 | `portable_import.go:62-100` vs `snapshot.go:39-76` | snapshot staging and install copied | one `stageSnapshot` that takes the temp prefix and returns errors the caller maps, so `ImportPortable` keeps `ErrImportUnavailable`/`ErrImportInvalidSource` and its extra `ctx.Err()`, and `SnapshotTo` keeps `ErrStorage`/`ErrMigration`/`ErrInvalidInput`; only after O4 is decided (if O4 removes `ImportPortable`, 5.1 is void) |
| 5.2 | `portable_import.go:171-183, 357-369`, `path.go:101-114` | three DSN builders | one `sqliteFileDSN`; DSN strings byte-identical (test added that pins all three) |
| 5.3 | `migrations.go:220-224` = `playback.go:114-118` | two identical rollback helpers, one misnamed | one `rollback` |
| 5.4 | `path.go:15` vs `portable_import.go:105` | same path predicate | `databasePath` calls `importPath` |
| 5.5 | six list queries (`metadata.go:70`, `anime.go:56`, `episode_mappings.go:94`, `schedule.go:67`, `playback.go:247`, `following.go:36`) | same query/scan/close/err loop six times | one generic `queryAll` whose scan callback can return an error (schedule returns `ErrStorage` mid-scan); results still start empty, not nil; error shape unchanged |
| 5.6 | `source_ingest.go:32, 51-52` | anime column list and insert repeat `anime.go:13, 24` | reuse `animeColumns` and the insert |

### S6. Core (M, about -60)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 6.1 | `core/app.go:87, 103`, `metadata_refresh.go:21`, `opening.go:53, 87`, `following.go:18`, `history.go:17` | same nil guard seven times | `app.ready(ctx)` covers only `ctx == nil \|\| app == nil \|\| app.store == nil` → `ErrInvalidPlayback`, with no `ctx.Err()`; each caller keeps its extra conditions and its own `ctx.Err()` checks |
| 6.2 | `playback_tracking.go:60-79, 394-416, 605, 620` | pending and active state structs and transitions copied; terminal-owner check twice | embed the shared fields; one `applyEvent`; hoist the check; risk M: focused `TestTracked*` tests under `-race` after each step |
| 6.3 | `metadata_match.go:122-127, 147-148` | "names match" rule twice | one `candidateTitleScore` |
| 6.4 | `metadata_normalize.go:175-214` | hand-written numeral switch | lookup map, same results |
| 6.5 | `apps/desktop/backend/actions.go:266-277` | episode ordering policy lives in the backend | move next to `core.EpisodeNumberValue` (adds one exported core function; owner confirms with the slice list) |
| 6.6 | `internal/release/release.go:124-133` | `sign`, `compareInt` duplicate `compareUint64` | use `compareUint64` |
| 6.7 | `core/app_test.go:713-727` vs `following_test.go:178-205` | overlapping tests in two files; the `app_test.go` ones add cases (history read with no followed anime; store `context.Canceled` passes through) | fold them into the `following_test.go` tables as new rows; no case is dropped |
| 6.8 | `core/proxy_address_test.go:57` | speed test in the proxy test file | move to the models test file |

### S7. Backend (L, about -80)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 7.1 | `backend/service_test.go` `newWithDependencies` (moved there in S1) and `service.go` `Start` | same dependency adoption twice | one `adopt(deps)` in `service.go`, called by `Start` and by the test helper, so tests cannot drift from production |
| 7.2 | `service.go:196-205`, `252-274`, `378-411` | close-on-failure blocks repeated | one close path |
| 7.3 | `service.go:566-665` | enum name and parse tables mirror each other | one table per enum; aliases stay on the parse side; move to `dto.go` |
| 7.4 | `actions.go:55-61` vs `81-87` | same DTO list build | one helper |
| 7.5 | `failurelog.go:142, 160` | same recent-upstream test | one helper |
| 7.6 | `ingest.go:17-19`, `store_guard.go:116-118` | same anonymous interface | one named interface |
| 7.7 | `store_guard.go:17-18`, `failurelog.go:244-259` | data-location error and `stageSource` in the wrong file | move |
| 7.8 | `tests/contract/assertions.go:30-42, 74-87, 120-133` | same membership loop and ref validator twice | one generic helper each |

### S8. Main package playback wiring (M, about -60)

`apps/desktop/main_fltk_windows.go` `main()` is about 590 lines of closures. This slice only removes copies; it keeps the closures and their order.

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 8.1 | lines 182, 198, 268, 358, 391, 462, 477, 494 | engine lock triplet eight times | `activeEngine()` |
| 8.2 | lines 175-177, 229-231, 247, 474 | notice pairs copied | `showPlaybackNotice` |
| 8.3 | lines 211-216, 323-329 | episode index lookup twice | one `episodeIndex` (shared with `player_autoplay.go:48`) |
| 8.4 | lines 123-124, 137-138, 148-149, 515-516 | same start and awake closures | define once |

Splitting `main()` into a playback controller type is owner decision O7, not part of S8.

### S9. fltkhome (L-M, about -110)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 9.1 | `window_windows.go:990-1013` | identical key cases | `case keyUp, keyPrevious:` and `case keyDown, keyNext:` there only (the navigation handler at `:852-865` treats `keyDown` and `keyNext` differently and stays) |
| 9.2 | `detail_view_windows.go:410-413` | `playbackPositionLabel` equals `formatPosition` | use `formatPosition` |
| 9.3 | about 17 uses of the literal `4` and 13 of `0` as section indexes (e.g. `window_windows.go:470, 480, 570, 602, 710`, `keys_windows.go:217`) | `SearchSection` exists; Home has no name | use `SearchSection` and a new `HomeSection` beside it |
| 9.4 | `search_view_windows.go:264-280` | same preview block on both branches | run it once |
| 9.5 | `history_view_windows.go:193`, `following_view_windows.go:361`, `row_list_windows.go:213`, `schedule_view_windows.go:246` | focus frame drawn four times | `drawFocusFrame` (draw only) |
| 9.6 | eight destroy-and-remove loops | same loop | one helper, same order |
| 9.7 | `search_model.go:282-327`, `detail_model.go:106-113`, `detail_view_windows.go:352-357`, `search_view_windows.go:532-533` | page arithmetic repeated | `pageCount`/`pageRange` on both models, one clamp |
| 9.9 | `following_view_windows.go:188-236, 305-401` | detail-screen code in the following file | move to the detail files |
| 9.10 | `following_view_windows.go:246`, `history_view_windows.go:130`, `schedule_view_windows.go:120` | same status switch three times | one helper, strings unchanged |

Kept for now: `detailEpisodesDone` (`window_windows.go:91`) looks like a copy of `detailModel.refreshed`, but `detailModel.close()` resets only the model, so the two can differ. Also kept (medium risk, generation-counter ordering): merging the detail open and retry flows (`detail_view_windows.go:177-209` vs `search_view_windows.go:254-301`), the three list loaders, and the row key handlers. Done only if S9 lands cleanly and the owner agrees.

### S10. fltkplayer, fltkengine and the libmpv adapter (L-M, about -90)

| # | Where | Problem | Change |
| --- | --- | --- | --- |
| 10.1 | `fltkplayer/player_windows.go:676-725, 495-525` | Space, Enter and mouse commit the same panel actions | `confirmOpenPanel` and per-panel commit helpers; each call site keeps its close-before or close-after order |
| 10.2 | `player_windows.go:644-652, 780-786`, `native_input_windows.go:130-145` | mute and volume step copied | `toggleMute`, `stepVolume` |
| 10.3 | `player_windows.go:158-164` vs `447-452` | deactivate and hide handling twice | one helper |
| 10.4 | `fltkengine/engine_windows.go:431-445, 635-641` | position and duration fallback twice (the tick at `:705-716` differs: duration falls back to 0, float seconds, and stays) | one helper for the two equal copies |
| 10.5 | `mpvwin/api_windows.go:37-88` | symbol list written twice | one table of name and destination; every symbol is still resolved before any is registered; the hash pin (`loader_windows.go:134`) still runs first |
| 10.6 | `adapters/player/libmpv/libmpv.go:166-241` | discard block five times; start time validated again inside `load` | one discard closure; validate once per entry point |
| 10.7 | `engine_demuxer_windows_test.go:50-89`, `resourcepass_windows_test.go:38-83`, `player_reentry_windows_test.go:275-292, 443-453`, `engine_end_windows_test.go:264-269` | mpv bring-up and wait loops copied in tests | shared test helpers, same build tags |
| 10.8 | `player_windows.go:108-109` | comment omits PlayerSkipIntro | fix the comment |

### S11. Stale comments and docs (L, no code behavior)

- Code comments that cite loop numbers instead of the reason (`detail_view_windows.go:99-100`, `bangumi.go:93`, `anime1.go:405-409`): keep the why, drop the history.
- `docs/STATUS.md` is about 10.7 KB; the layout rule says one page. Shorten; move history to `archive/`.
- `docs/loops/`: LOOP30-LOOP36 and `PRERELEASE_0.1.0-alpha.1.md` are past loops; move to `archive/` (the README says `loops/` holds active or recent loops). Every link to them in docs, the handoff and code comments is updated (grep).
- `docs/02_ARCHITECTURE.md`: add `adapters/internal/jsonscan` if 4.3 lands.

## Kept on purpose

- Security checks that look repeated but guard different paths: `proxy.go:296-298` source checks; `securehttp/client.go:416-417` and `426-428` literal-address checks (first request and redirects); `client.go:305-307` body close on error; `client.go:495-498` URL checks before origin parsing; `stripSensitiveHeaders` and its list; proxy redaction methods (`proxy.go:373-381`); `forwardproxy.go:82` re-validation of a stored address.
- `securehttp/client.go:287` body copy: removing it changes nil versus empty bodies.
- Event drain loops (`engine_windows.go:306-314`, `595-597`): they share only a 3-line header; a helper would add indirection for no gain.
- Shared test fakes for bangumi and anilist (`bangumi_test.go:41-98`, `anilist_test.go:22-67`): sharing needs a new test-support package for about 40 lines, and the fakes differ (nil-body guard).
- `video/mp4` media-type checks in `proxy.go:868` and `resolver.go:357`: two different boundaries (source answer and proxy answer), each with its own tests.
- SQLite transaction styles (`BEGIN IMMEDIATE` versus `BeginTx`): different locking.
- `writeGuard` repeated checks: a plain decorator.
- Test seams (`probeProxy`, `environmentProxyAllowed`, `export_windows_test.go`) and independent test oracles (`skipLabelWidth`, the libmpv hash literal in `player_reentry_windows_test.go:34`).
- Unused libmpv enum values: part of the C ABI.
- Long FLTK build functions (`buildSettingsView`, `build`, `buildSearchView`, `populateHome`, `loadDetailResources`, `handleKey`, `handle`, `pollEvents`): split only where a slice above already touches them; layout code is not reshaped for length alone.

## Owner decisions (not done without an answer)

| # | Finding | Why it is not a pure refactor |
| --- | --- | --- |
| O1 | Season length limits differ: core 64 bytes / 32 runes (`core/metadata_match.go:49-52`) versus `internal/metadata/policy.go:17-22` 128/128; AniList seasons of 65-128 bytes pass the adapter and are then dropped by core | aligning them changes which candidates match |
| O2 | "Resumable" and "finished" differ: Home uses position < duration (`fltkhome/model.go:67-73`), detail uses completed and position > 0 (`detail_model.go:131`), core uses 90% (`core/playback_tracking.go:496-498`); Home orders by LastPlayed, detail by UpdatedAt | one rule changes what the user sees |
| O3 | `backend/failurelog.go:57-70` omits `proxy` and `proxy_refused`, so proxy failures are logged as `unknown` | a bug fix; changes log output |
| O4 | Legacy AppData import (`PortablePlan.OfferImport`, `LegacyPath`, `Import`, `sqlite.ImportPortable`) and the copy/import branches of `startupErrorMessage` have no production caller | removing a product path |
| O5 | uitext keys left unused by S2.1 and O4 (`PreparingPlayback`, `NoPlayableEpisodes`, `DataCopyFailed`, `DataCopiedUnopenable`) | UI text tables |
| O6 | Port and model fields used only by tests: `Store.SaveProgress`, `core.ErrUnsupported`, `MetadataMatchConfidence`/`Score`/`CrossChecked`, `MetadataQuery.Season`, Anime1 `Title` | changes a core port or model |
| O7 | Split `main()` (about 590 lines) into a playback controller type | high risk: UI-thread order and generation counters |
| O8 | Shared limits spelled in several layers (episode label 128, title 4096, year 3000, episode cap 100000, season names) | moving them to core adds core constants |

## Done when

- Every finding in S1-S11 that the owner approved is closed or listed as kept, with its reason, in the result table below.
- `deadcode ./...` (same version and offline proxy) reports no production-unreachable function outside `tests/contract` (test-only package) and the items under owner decisions.
- No two copies remain of any duplicated item listed in an approved slice, and no removed constant, field or type is still referenced (checked by grep per item, since `deadcode` sees only functions).
- No coverage loss: `go test -cover` per package before and after each slice; any drop is explained by removed dead code or fixed before commit.
- `gofmt -l .` empty, `go mod verify`, `go vet ./...`, `govulncheck@v1.1.4 ./...` clean.
- Full native suite green: `CGO_ENABLED=1 go test -count=1 ./...`, then `-race` (msvcrt MinGW gcc; window and libmpv tests after telling the owner).
- The Loop 38b resource harness (`-tags resourcepass`, headless) passes all scenarios with the Loop 38 thresholds, and no scenario's growth is worse than the Loop 38b result by more than the run-to-run spread (compared in a table).
- Each slice: simplify pass, independent oracle review with no MUST_FIX, owner approval before commit and push, GitHub Windows CI green for the pushed commit.
- Production Go lines (non-test) counted before and after with the same command; the reduction is reported.

## Owner check

No user-visible change is expected. After the last slice, an optional test build in `artifacts/test-build/` for a normal session (search, open, play, switch episode, settings).

## Result

Owner approved the slice list 2026-10-25 ("I approve the Loop 38R slice list S1-S11 as written. Start S1."; TypeSafe owner-report HUMAN_PASS 0.71).

| Slice | Result | Evidence |
| --- | --- | --- |
| S1 | Done, not committed: rows 1.1-1.9 closed; production -116 lines, tests +45/-39 (`newWithDependencies` moved into `service_test.go`, one test of removed code deleted). `deadcode` now lists only the S2 functions outside `tests/contract`. Coverage per package equal or within 0.3 points (removed code was covered). gofmt, `go mod verify`, vet, govulncheck (0 affecting) clean; `-race` suite green. Normal suite: two fltkhome settings key tests timed out once (FLTK event wait), passed in the focused rerun and the package rerun; TypeSafe failure-class FLAKY_OR_NONDETERMINISTIC 1.00; S1 does not touch fltkhome. Oracle review: no MUST_FIX; its SHOULD_FIX is recorded in row 7.1. Committed 5d60093, CI 40226307355 green. | `artifacts/evidence/loop38r/s1-*` |
| S2 | Done, not committed: rows 2.1-2.5 closed; production -71 net lines (+21/-92, mostly struct realignment), tests -68 net (`play_entry_test.go` tested only the removed playback path; a new headless test covers a successful detail load, which only that file's helper exercised). `deadcode` reports no unreachable function outside `tests/contract`. Coverage: fltkhome 87.6 -> 87.7, fltkengine 85.1 -> 85.1, mpvwin 36.7 -> 37.7. gofmt, `go mod verify`, vet (also `-tags resourcepass`), govulncheck (0 affecting) clean; `-race` suite green. Normal suite: `TestNativeSettingsProxyModeSavesAndShowsTheAddressRowOnlyForManual` timed out once with the same FLTK event wait as in S1 and Loop 38; fltkhome passed again normally and with `-race`; TypeSafe FLAKY_OR_NONDETERMINISTIC 1.00. Oracle review: no MUST_FIX; its SHOULD_FIX (detail-load coverage) fixed. uitext keys `PreparingPlayback` and `NoPlayableEpisodes` now unused (owner decision O5). Committed 5fbd5f3, CI 40227802726 green. | `artifacts/evidence/loop38r/s2-*` |
| S3 | Done, not committed: rows 3.1-3.3 closed. One header-name validator (`securehttp.ValidHeaderName`), one Anime1 reference check (`resolveSiteReference`), cover host names defined once in `internal/metadata`; every check runs at the same point with the same predicate, no allowlist widened. Production -2 net lines (+34/-36; doc comments added for the shared checks, the plan's -25 was an overestimate). Coverage equal or higher (proxy 91.4 -> 91.6, anime1 89.1 -> 89.3, others equal). gofmt, `go mod verify`, vet, govulncheck (0 affecting) clean; full suite and `-race` green. Oracle security review: PASS, no MUST_FIX or SHOULD_FIX. Committed 47bf4e8, CI 40228749659 green. | `artifacts/evidence/loop38r/s3-*` |
| S4 | Committed 01982a5, CI 40232435530 green: rows 4.1-4.5 closed. `securehttp.SafeError` and `securehttp.HasMediaType` replace five sanitize-error and five media-type copies; `adapters/internal/jsonscan.Object` replaces both JSON scanners (the resolver keeps `UseNumber`); `adapters/metadata/internal` now holds the provider answer check (`Rules.CheckResponse`/`Decode`), ID, search-query, text and cover-URL checks for Bangumi and AniList; one iterative `visibleText` for Anime1 titles and episode texts; `helpers.go` renamed `json.go`. Two changes from the plan text: `checkContext` stays local in each package (30+ call sites, 4 lines, package-specific error), and the request tail went to `adapters/metadata/internal`, not `securehttp`, because its 404 mapping uses `core.ErrNotFound`. Every caller keeps its own error values. Production -154 net lines (+348/-502); tests +256 (direct tests for the shared code, which otherwise was covered only through the providers). Coverage: metadata/internal 88.0 -> 98.0, securehttp 89.3 -> 90.2, anilist 86.7 -> 89.3, bangumi 92.6 -> 92.9, anime1 89.3 -> 89.9, aniskip 93.5 -> 93.2 (its removed `isJSON` was covered), others equal; new jsonscan 87.2. gofmt, `go mod verify`, vet, govulncheck (0 affecting) clean; full suite and `-race` green. Oracle review: no MUST_FIX; SHOULD_FIX (leftover constants, architecture doc wording) and a stale comment fixed, then metadata and Anime1 rerun with `-race`. | `artifacts/evidence/loop38r/s4-*` |
| S5 | Done, not committed: rows 5.2-5.6 closed; 5.1 deferred until owner decision O4. One `sqliteFileDSN` behind the three DSN functions (new `TestSQLiteDSNsAreExact` pins all three strings and passed before the change); one `rollback` (in `store.go`, timeout renamed `rollbackTimeout`); `databasePath` and the import/snapshot paths share `absoluteLocalPath` (was `importPath`); generic `queryAll` for the six list reads; anime INSERTs use `animeColumns`. Production -85 net lines (+132/-217); tests +13. Coverage sqlite 82.5 -> 82.3 (removed copies were covered). gofmt, `go mod verify`, vet, govulncheck (0 affecting) clean. Full suite and `-race`: every package except fltkhome passed; fltkhome settings window tests failed once in each run (same FLTK event timeout as before, and one focus check) and passed in both reruns; TypeSafe FLAKY_OR_NONDETERMINISTIC 1.00. Oracle review: PASS, no MUST_FIX; its two cleanups (rollback placement and name, `animeColumns` comment) applied, sqlite rerun with `-race`. | `artifacts/evidence/loop38r/s5-*` |

Plan doc-quality (TypeSafe, `artifacts/evidence/loop38r/doc-quality*.json`): clarity 1.19-1.92 per section, filler at most 0.67 after rewriting Goal, How the audit was done and Findings and slices (were filler 1.76 and 1.39, clarity 0.95). Adjudicated: the slice tables score clarity 1.26-1.77 because they are dense file:line tables; their rows were checked against the code by the reviewer.
