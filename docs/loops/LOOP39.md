<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 39: Core slimming

Status: NOT STARTED. Planned by the owner on 2026-11-06. Base: 6c99b05. Starting this loop needs the owner's go-ahead. Loop 39T (2026-11-07) has since deleted many tests: recheck every audit row that names a test or a test line against the current files before acting on it.

## Goal

The same app with less code. Only code is removed or merged; nothing is added.

## Hard rules

These rules beat any other reading of this file. If a change would break one, do not make it: write it under "Noticed, not done" and go on.

1. **Only listed work.** Change only what a row in "Approved slices" says. A row exists only after the owner approved it in chat. Phase A writes no code.
2. **Only these paths:** `core/`, `adapters/` (not `adapters/persistence/sqlite/migrations/`), `apps/desktop/backend/`, `internal/`, `tests/contract/`, and this file. Never touch `apps/desktop/fltk*`, `apps/desktop/*.go`, `apps/desktop/uitext/`, `tools/`, `.github/`, `go.mod`, `go.sum` or other docs (except STATUS.md and the handoff when closing).
3. **Nothing new.** No new package, production file, exported name, interface, type, dependency, setting, backend action, feature or abstraction. A file may be merged into another or deleted, never split. Test helpers may move only into an existing `_test.go` file.
4. **Every slice shrinks.** Each slice must lower the production line count of its package group, and must not raise its test line count. A slice that does not shrink is dropped, not reshaped into something else.
5. **Behaviour stays.** No change to UI text, saved data, network requests, security checks or errors the UI shows. The 03_SECURITY.md checks stay, even when they look redundant (see LOOP38R "Kept on purpose").
6. **Tests:** a test is deleted only when the owner approved its row and the row names the remaining test that still covers that behaviour. Never weaken, skip or special-case a test to get green.
7. **No 2.0 preparation.** Loop 40 (UI boundary) and Loop 41 (UI toolkit) are out of scope. Do not move code toward them.
8. **Stop and ask** when: a row turns out to need a file outside rule 2; a test fails twice after a fix; a row is unclear; the change would alter behaviour. Do not guess.
9. **One slice = one commit**, after the owner approves the commit. Do not start the next slice before that.

## Phase A: audit (read-only)

Output: the "Audit findings" table below, filled in. Nothing else.

For each package group, find:

- **Duplicate:** the same logic in two places.
- **Unused:** code, settings or branches no production path reaches (`deadcode`, as in LOOP38R).
- **Over-defended:** a check that guards nothing listed in 03_SECURITY.md, or that is repeated inside one trust boundary. Mark these "owner decides"; never cut a security check on your own reading.
- **Feature the owner may drop:** a whole behaviour, with its line count. Owner decides.
- **Repeated test:** two or more tests that check the same behaviour with the same kind of input; name the one to keep.

Each finding row: `#`, `file:line`, kind, what it is, proposed change, estimated net lines (production / test), risk (L/M).

Phase A ends with a short report to the owner: the table and the total estimate. **Stop there and wait.** The owner marks each row approve or reject. Only approved rows move to "Approved slices".

## Phase B: slices

Group the approved rows into slices, one package group each, in table order. For each slice:

1. Make the change.
2. Run the focused tests for the touched packages, then the full suite once (05_WORKFLOW.md section 5; no race run since Loop 39T; owner rule: light testing, no repeated runs).
3. Measure lines (command below) and record before and after in the slice row.
4. Fresh read-only review (02_ARCHITECTURE.md quality rules plus these hard rules); fix every MUST_FIX.
5. Ask the owner to approve the commit.

## Measuring lines

```powershell
foreach ($d in 'core','adapters','apps\desktop\backend','internal') {
  $f = Get-ChildItem $d -Recurse -Filter *.go
  $p = ($f | Where-Object Name -notlike '*_test.go' | Get-Content | Measure-Object -Line).Lines
  $t = ($f | Where-Object Name -like '*_test.go' | Get-Content | Measure-Object -Line).Lines
  "$d production $p test $t"
}
```

Baseline at 6c99b05 (non-blank lines as `Measure-Object -Line` counts them, Go only):

| Group | Production | Tests |
| --- | --- | --- |
| `core` | 2,574 | 3,135 |
| `adapters/persistence/sqlite` | 1,770 | 2,999 |
| `adapters/source/anime1` | 1,429 | 1,450 |
| `adapters/metadata/*` | 1,597 | 2,646 |
| `adapters/network/*` | 839 | 1,229 |
| `adapters/playback/proxy` | 829 | 1,430 |
| `adapters/player/libmpv` | 413 | 380 |
| `adapters/update`, `adapters/internal` | 158 | 160 |
| `apps/desktop/backend` | 2,779 | 3,895 |
| `internal/*` | 512 | 205 |
| **Total** | **12,900** | **17,529** |

## Audit findings

Phase A ran 2026-11-06 at HEAD f8d61e6. Four read-only reviewers covered the package groups. Each finding was checked by a repo-wide caller grep, and the root agent spot-checked the key claims. `deadcode ./...` (offline, as in LOOP38R) reports nothing unreachable outside `tests/contract`, and `deadcode -test ./...` reports nothing. Line counts are hand estimates, given as net production / test. "Owner" means the owner decides, as rule 3 requires for over-defended and drop rows.

| # | file:line | Kind | What | Proposed change | Net prod/test | Risk |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `core/app.go:33-35, 74-105` | Unused | `App.Search`, `Library`, `Follow`, `Unfollow` and `setFollowing` are called only from `app_test.go`; the backend calls the source and the store directly | delete them, their tests (`app_test.go:311-330, 576-693`) and the fake fields only those tests read | -27 / -140 | L |
| 2 | `core/models.go:54-59` | Unused | type `core.Episode` has no reference; every `Episode{` is `backend.Episode` | delete | -7 / 0 | L |
| 3 | `core/playback_tracking.go:565-724` (+ staging `:420-453`, `:296-301`) | Drop (owner) | the tracked session relays events to callers, but no production code reads `tracked.Events()`; the backend only calls `Close` or `SwitchEpisode` | drop the relay and the staging; `Events()` returns a channel that closes when the session ends; checkpoints unchanged; `TestTrackedRelay*` and the event asserts go, `TestTracked*` checkpoint tests stay | -200 / -170 | M |
| 4 | `core/playback_tracking.go:171-191` vs `:193-213` | Duplicate | tracked `Load` repeats `switchEpisode` | `Load` checks `StartAt < 0`, then calls `switchEpisode`; same errors in the same order | -12 / 0 | L |
| 5 | `core/playback_tracking.go:159-169` | Unused | tracked `Snapshot` is reached only by two test type assertions (`playback_tracking_test.go:552, 637`) | delete; those two tests drive `refreshSnapshot` through `SwitchEpisode` or `Close` | -11 / 0 | M |
| 6 | `core/playback_tracking.go:44-58`, `core/app.go:15-18, 26-29, 167-174` | Unused | `normalizePlaybackTrackingConfig` fills defaults that `NewApp` already set; only the zero test config needs it | `App` holds one `playbackTrackingConfig` set in `NewApp`; delete `trackingConfig()` and the normaliser | -25 / 0 | L |
| 7 | `core/app.go:50-56`, `core/playback_tracking.go:123-125`; `core/app.go:64` + `playback_tracking.go:194`; `internal/release/page.go:25` | Over-defended (owner) | the `closeErr` branch returns the same error on both paths; `snapshotter == nil` is impossible when `ok`; `store == nil` is already checked by the only caller; nil session and resolve checks are done in both caller and callee; `RawQuery`/`Fragment` are already covered by `ContainsAny("?#")` | drop the dead conditions | -7 / 0 | L |
| 8 | `core/metadata_match.go:12-28, 52-87` | Unused | production asks only "accepted or not"; `NeedsCrossCheck` and `NoMetadata` are told apart only in tests | `MatchMetadata(query, candidates) (MetadataCandidate, bool)`; delete the decision type, its constants and the request/result types; tests assert `ok` | -22 / -10 | L |
| 9 | `core/metadata_normalize.go:66-69, 108-111, 122-125` | Duplicate | the rule "numeral if known, else trim leading 0" is written three times | `seasonNumeral` does both; three call sites | -9 / -1 | L |
| 10 | `internal/metadata/policy.go:32-55` | Unused | every `PlainTextLimits` sets `MaxInputBytes == MaxOutputBytes`; the `<= 0` guard covers only hand-built zero limits, and nothing builds them | delete `MaxOutputBytes` (both length checks use `MaxInputBytes`; both checks stay) and the `<= 0` guard | -5 / 0 | L |
| 11 | `core/metadata_variants.go` | Duplicate (file) | the file only holds regexps that `metadata_normalize.go` uses | merge into `metadata_normalize.go` | -5 / 0 | L |
| 12 | `core/app_test.go:27-29, 48-51, 134-143, 189-195, 276-300` | Unused (tests) | fake fields core never reaches (`episodes*`, `sourceRefs*`) and two never set (`settingsErr`, `progressErr`) | delete them | 0 / -18 | L |
| 13 | `core/app_test.go:486-499` | Repeated test | `TestAppSwitchEpisodeDoesNotLoadWhenResolveFails` | keep `TestSwitchWithFailingResolveKeepsSessionUsable` (`playback_autoplay_test.go:90`), which checks more | 0 / -14 | L |
| 14 | `core/app_test.go:751-756` | Repeated test | `TestPlaybackEventIdentifiesCanonicalEpisode` checks only its own struct literal | delete | 0 / -6 | L |
| 15 | `adapters/metadata/internal/json.go:19-33` | Over-defended (owner) | `DecodeObject`'s limit guard and reflect target checks repeat `json.Unmarshal` and jsonscan; the fresh-value decode guards a partial decode no caller can see | jsonscan check, then `json.Unmarshal`; delete the two no-partial-decode tests | -11 / -23 | M |
| 16 | `adapters/metadata/internal/json_test.go:10-44` | Repeated test | the rows of `TestDecodeObject` and the nesting checks | keep `jsonscan_test.go` `TestObject`, `TestObjectNestingLimitIsInclusive` and `provider_test.go:68` `TestRulesDecode` | 0 / -30 | L |
| 17 | `adapters/source/anime1/anime1.go:150, 194-205` (+ `escapedPrefix` `:446-447, 787-794, 858-860`) | Over-defended (owner) | the page-count check, `seenURLs` and `escapedPrefix` can never decide: pagination must rise by exactly one under a fixed decoded prefix, capped at 32 | delete the three; `TestEpisodePageAndCountExactLimits` and `TestEpisodesRejectsPaginationViolations` must still pass unchanged | -14 / 0 | M |
| 18 | `adapters/source/anime1/resolver.go:286, 301, 329, 362, 374, 385, 409-425` | Over-defended (owner; next to 03_SECURITY row 7) | sub-conditions implied by checks already made: `url.Parse` rejects control bytes; the `https://` prefix fixes the scheme; the `.v.anime1.me` suffix plus the label loop fix case and ASCII; percent-decoding only shrinks; the body size was checked in `fetchResolver` | delete those sub-conditions, `isASCII` and `containsASCIIControl`; the host, HTTPS, port, user, fragment, `.mp4` and label checks stay | -18 / 0 | M |
| 19 | `adapters/source/anime1/anime1.go:316-331, 425-435, 888-899, 640-643, 695-697` | Duplicate | three hand-written digit loops repeat `allASCIIDigits`; `parsePostID` and `normalizeEpisodeText` only forward | call `allASCIIDigits`; inline both wrappers | -18 / 0 | L |
| 20 | `adapters/metadata/anilist/anilist.go:241-248` vs `:222-229` | Duplicate | `mapMetadata` repeats `mapCandidate` | `mapMetadata` calls `mapCandidate` (as Bangumi's `mapSubject` does), with checks in the same order | -6 / 0 | L |
| 21 | `adapters/source/anime1/html_policy.go` | Duplicate (file) | one function, one caller (`resolver.go:156`), always passed `errResolverMalformed` | merge into `resolver.go`; use the sentinel directly | -9 / 0 | L |
| 22 | `adapters/metadata/bangumi/bangumi_test.go:404, 418, 443, 557, 630, 765-774` | Repeated test | six "change one field, expect malformed" loops | one field/value table that keeps every row; `typeName` and `mustJSONForName` go | 0 / -55 | L |
| 23 | `adapters/metadata/anilist/anilist_test.go:350-470` | Repeated test | `:470` repeats `:460`; the reject tests `:424, :451, :364` and the drop tests `:350, :394, :410` are the same pattern | delete `:470`; one reject table and one drop table that keep every row | 0 / -60 | L |
| 24 | `adapters/source/anime1/acceptance_test.go` | Repeated test (owner) | its malformed-body checks repeat `anime1_test.go:260, 875` and `resolver_test.go:167`; the composition test only checks `New` is non-nil | delete the file; add an "empty token" row to `resolver_test.go:169-182` | 0 / -133 | M |
| 25 | `anime1_test.go:712-716, 683-688`, `resolver_test.go:295-297` | Repeated test | three cases repeat `anime1_test.go:829, 733-736` and `resolver_test.go:306-308` | keep the later case of each pair | 0 / -13 | L |
| 26 | `adapters/source/anime1/anime1.go:63-69`, `adapters/metadata/anilist/anilist.go:59-65` | Unused | `newWithDo` only builds `&Client{do: do}` | `New` builds the struct; tests use `&Client{do: fake}` (optional: many mechanical test edits) | -8 / 0 | L |
| 27 | `adapters/network/securehttp/client.go:81-96, 126, 203-240, 636-673` | Unused (the `ExtraSensitiveHeaders` part: owner) | production sets only `AllowedOrigins`, `MaxResponseBytes` and `Proxy`; six timeout and limit fields have no user, and `ConnectTimeout` and `ExtraSensitiveHeaders` are set only in tests | delete the 8 fields, their checks, `normalizeSensitiveHeaders` and the `extra` parameter; use the defaults directly; the fixed strip list stays | -50 / -3 | M |
| 28 | `adapters/playback/proxy/proxy.go:25-50, 210-233` | Unused | libmpv never sets `SessionTTL`, `MaxSessions`, `MaxConcurrentStreams` or `MaxStreamsPerSession`; the bounds, `errInvalidConfig` and the clamp are dead | drop the 4 fields and `normalizeConfig`; tests use the default constants (`proxy_test.go:240-255, 469, 967, 1332`) | -35 / -8 | M |
| 29 | `adapters/player/libmpv/libmpv.go:20, 82-84, 178, 242-244, 397-402, 412-413` | Over-defended (owner) | `validateStartAt` and `ErrInvalidStartAt` repeat core's `startAt < 0` check in the same process | delete the function, the error, both calls and the sanitise case | -16 / 0 | L |
| 30 | `adapters/player/libmpv/libmpv.go:89-98, 438-449` | Over-defended (owner) | reflection `isNil` guards against typed nils no factory returns | delete it; plain nil checks | -14 / 0 | L |
| 31 | `adapters/persistence/sqlite/playback.go:35-82` | Duplicate | `loadProgressCheckpoint` and `loadHistoryCheckpoint` are the same scan and validation | one function that takes the query; progress selects `''` for `last_played_at` | -20 / 0 | L |
| 32 | `sqlite/metadata.go:12-30` vs `playback.go:184-191`; `playback.go:12-14, 107-112` | Duplicate | `animeExists`/`requireAnime` repeat `requireAnimeWithQueryer`; `maxStoredTime` is the built-in `max`; `validEpisodeID` only forwards | one `requireAnime(ctx, queryer, id)` for all 6 callers; `max`; inline | -20 / 0 | L |
| 33 | `sqlite/path.go:15-21, 45-48`, `snapshot.go:86-92` | Duplicate | `absoluteLocalPath` runs twice on the same value; the `\\.\` and `\\?\` prefixes are already caught by `\\` | call `ValidatePortablePath` once; drop the two prefixes | -6 / 0 | L |
| 34 | `sqlite/path.go:79, 98-108`, `store.go:67-70`, `snapshot.go:49-53`, `path_test.go:66-68, 100-153` | Unused (owner: Unix groundwork) | the package builds only on Windows, so the non-Windows chmod branches and `secureDatabaseArtifacts` never run, and their test always skips | delete | -20 / -57 | L |
| 35 | `sqlite/settings.go:66-75, 85-87`, `playback.go:48-50, 71-73` | Over-defended (owner) | read-side re-validation of settings and checkpoint rows that only the app writes and the schema CHECKs already enforce | delete them and their corrupt-row tests (`persistence_test.go:410-454, 518-530`); metadata and schedule read checks stay (03_SECURITY "Metadata text"); the enum range check stays wherever narrowing could wrap | -19 / -60 | M |
| 36 | `adapters/playback/proxy/proxy.go:855-858` | Duplicate | `validVideoType` equals `securehttp.HasMediaType(v, "video/mp4")` | use it; both video/mp4 checks stay in place | -4 / 0 | L |
| 37 | `adapters/network/forwardproxy/forwardproxy.go:86-88` | Unused | the `system == nil` branch is reached only by `NewWithReader(nil)` in a test | delete the branch and `TestSourceWithoutReaderIsDirectInSystemMode` | -3 / -7 | L |
| 38 | `securehttp/client.go:279-282, 387-389`, `sqlite/store.go:112-114` | Unused | `transport == nil`, `c.proxy != nil` and `fn == nil` branches no caller reaches | delete | -8 / 0 | L |
| 39 | `securehttp/response_checks.go`, `sqlite/migrations_embed.go`, `sqlite/errors.go`, `sqlite/time.go` | Duplicate (file) | small files that repeat a header | merge into `client.go`, `migrations.go`, `store.go` | -20 / 0 | L |
| 40 | `sqlite/path_test.go:155-164` | Repeated test | `TestDatabaseDSNUsesNoFollow` | keep `TestSQLiteDSNsAreExact` (its exact string has `nofollow=1`) | 0 / -10 | L |
| 41 | `sqlite/store_test.go:74-88` | Repeated test | `TestCloseIsIdempotentAndRejectsNewOperations` | keep `lifecycle_test.go:18` `TestCloseWaitsForActiveOperationAndConcurrentClosers` | 0 / -15 | L |
| 42 | `sqlite/migrations_test.go:179-404` (6 copies), `episode_mappings_test.go:177, 315`, `schedule_test.go:243` | Repeated test code | the same "migrations up to N" loop nine times | `loadMigrations(embeddedMigrations)` plus `all[:N]`, existing code only | 0 / -40 | L |
| 43 | `sqlite/persistence_test.go:252-284` | Repeated test (owner) | `TestPlaybackCheckpointReopensConsistently` overlaps `:34` and the contract checkpoint subtest; it is the only check that non-zero times survive a reopen | keep `TestPlaybackAndSettingsPersistAcrossReopen` and the contract subtest | 0 / -33 | M |
| 44 | `proxy/proxy_test.go:168-174, 177-237, 940-951` | Repeated test | the range test, the unknown-host/closed test, a redaction block and close checks | keep `:496`, `:551`, `:418`, `:776-828`, `:986` and `:920-925` | 0 / -74 | L |
| 45 | `securehttp/proxy_test.go:259-264`, `client_test.go:193-202, 526-532` | Repeated test | three repeats | keep `client_test.go:51`, `:205` and `TestOverallTimeout` (`:372`) | 0 / -25 | L |
| 46 | `forwardproxy/forwardproxy_test.go:88-95` | Repeated test | two `TestSourceModes` steps | keep `TestResolveSaysWhenTheSystemProxyIsUnusable` (`:118, :124`) | 0 / -8 | L |
| 47 | `apps/desktop/backend/service.go:319-329` + 17 call sites in `actions.go` and `opening.go:37` | Over-defended (owner) | `appSnapshot` re-checks what `begin()` checked under the same lock | return the values with no error; each call goes from 4 lines to 1; only during the shutdown race does a caller get `context.Canceled` instead of `ErrClosed` (the UI never checks `ErrClosed`) | -58 / 0 | M |
| 48 | `backend/service.go:361-371` | Unused | the `databasePath == ""` fallback in `openProduction`: production always passes a plan with a path | delete | -11 / 0 | L |
| 49 | `backend/service.go:105-128` | Unused | `NewAtWithPlayback` only forwards (one production caller, one test) | fold it into `NewAtWithPlan`; `network_test.go:202` calls that | -8 / 0 | L |
| 50 | `backend/service.go:153-162` | Unused (owner) | `Start`'s "already started" branch: production calls `Start` once | delete; a second `Start` would then open a second store | -10 / 0 | M |
| 51 | `backend/service.go:446-452, 562-564`, `actions.go:131-132, 335-336`, `opening.go:12-14` | Duplicate | `episodeID` repeats `localID`; `sortAnime` has no visible effect (SQLite orders by id, the UI maps or re-sorts); the timeout aliases rename core constants | delete; convert at the two callers; use the core constants; drop `TestSortAnime` | -14 / -10 | L |
| 52 | `backend/dto.go:25-26`, `actions.go:127` | Unused | `Metadata.Title` and `Metadata.NativeTitle` are never read by the UI | delete both fields | -2 / 0 | L |
| 53 | `backend/dto.go:111-116`, `actions.go:738` | Unused (owner; 03_SECURITY:149 says the action returns "bytes and dimensions") | `Cover.Width`/`Height` are never read; the UI decodes the bytes itself | delete | -2 / 0 | L |
| 54 | `backend/dto.go`, `data_location.go:18-37`, `service_integration_test.go:819-840` | Unused (owner) | JSON tags on DTOs that nothing serializes, plus the test that pins them | delete the tags and the test; goes in a slice with other rows (it saves no production lines) | 0 / -22 | L |
| 55 | `backend/home_integration_test.go:74-133` | Repeated test | `TestHomeHistoryAfterRestartInvokesExistingPlayback` | keep `TestStopPlaybackPersistsFinalPositionAndResumesAfterRestart` (`resume_checkpoint_test.go:85`) and move the one extra request check into it | 0 / -58 | L |
| 56 | `backend/failurelog_test.go:79-94, 108-134`, `service_test.go:484-488` | Repeated test | the proxy-name test repeats `TestFailureLineFormat`; two cap-at-50 tests; the newPlayer error case | keep `TestFailureLineFormat`; one cap test that keeps the across-restart and no-`.tmp` checks; keep the `service_integration_test.go:379` "generic" row | 0 / -30 | L |
| 57 | `backend/data_location_test.go:95-107`, `portable_location_test.go:305-322` | Repeated test | two use-existing tests | keep `portable_location_test.go:265-303` and `data_location_test.go:273-295` (it takes the "location file untouched" check) | 0 / -28 | L |
| 58 | `tests/contract/player.go` (whole file), `fakes_test.go:82-169, 316-349` | Unused (owner) | no adapter runs `RunPlayer`; the suite tests only its own fake | delete; `02_ARCHITECTURE.md:137` then overstates (see below) | -202 / -100 (contract) | L |
| 59 | `tests/contract/fakes_test.go:16-80, 171-311, 321-353` | Repeated test | the suites are run against in-package fakes | keep the real adapter runs (`anime1_test.go:938`, `bangumi_test.go:143`, `anilist_test.go:127`, `sqlite/persistence_test.go:20`) and `assertions_test.go` | 0 / -235 (contract) | L |
| 60 | `tests/contract/store.go:152-167, 226-242`, `source.go:45-68` | Repeated test / duplicate | the "progress" subtest is a subset of "playback checkpoint"; two contains loops; the catalog and search blocks repeat each other | fold the `ErrNotFound` check into "playback checkpoint"; `slices.Contains`; one loop over both calls | -40 / 0 (contract) | L |

**Total if every row is approved:** about -754 production / -1,154 test in the measured groups, which is -5.8% / -6.6% of the 12,900 / 17,529 baseline. `tests/contract` adds -242 / -335; the measuring script does not count it (it is 891 / 479 today). The owner rows (3, 7, 15, 17, 18, 24, 27 in part, 29, 30, 34, 35, 43, 47, 50, 53, 54, 58) make up about -400 / -500 of that. Without them: about -360 / -650.

## Approved slices

On 2026-11-06 the owner answered the Phase A report with "核准非 owner 的 L 列". That approves every L-risk row not marked "owner". The rows are grouped into slices, one package group per slice, in table order. Rows that span groups are split by group.

| Slice | Group | Rows | Est. prod/test |
| --- | --- | --- | --- |
| S1 | `core` | 1, 2, 4, 6, 8, 9, 11, 12, 13, 14 | -107 / -189 |
| S2 | `internal/*` | 10 | -5 / 0 |
| S3 | `adapters/source/anime1` | 19, 21, 25, 26 (anime1 part) | -31 / -13 |
| S4 | `adapters/metadata/*` | 16, 20, 22, 23, 26 (anilist part) | -10 / -145 |
| S5 | `adapters/network/*` | 37, 38 (securehttp part), 39 (`response_checks.go`), 45, 46 | -15 / -40 |
| S6 | `adapters/playback/proxy` | 36, 44 | -4 / -74 |
| S7 | `adapters/persistence/sqlite` | 31, 32, 33, 38 (`store.go` part), 39 (sqlite part), 40, 41, 42 | -66 / -65 |
| S8 | `apps/desktop/backend` | 48, 49, 51, 52, 55, 56, 57 | -35 / -126 |
| S9 | `tests/contract` | 59, 60 | -40 / -235 |

Not approved yet, so these rows stay out of every slice:

- The "owner" rows: 3, 7, 15, 17, 18, 24, 27, 29, 30, 34, 35, 43, 47, 50, 53, 54, 58.
- The M-risk rows not marked owner: 5 and 28.

Slice results:

| Slice | Result |
| --- | --- |
| S5 | Done 2026-11-07, not committed. Rows 37, 38 (`securehttp` part), 39 (`response_checks.go`), 45 and 46 are closed. Removed: forwardproxy's nil-reader branch and its test, and the `transport == nil` and `c.proxy != nil` branches in securehttp. `response_checks.go` and its test file are merged into `client.go` and `client_test.go`. Three repeated securehttp checks and two `TestSourceModes` steps are deleted. Review fixes: `TestOpenDisablesOverallTimeoutButDoRemainsBounded` is renamed to `TestOpenIgnoresOverallTimeoutButObservesCancellation`, since its Do check is gone, and the now-constant reader variables in `TestSourceModes` became literals. `adapters/network` production went from 839 to 822 lines and tests from 1,123 to 1078. `go test ./adapters/network/... ./adapters/playback/... ./apps/desktop/backend/`, `go vet` and one full native suite run (two window walks) passed. Independent oracle review: PASS, no MUST_FIX; its SHOULD_FIX was applied. |
| S4 | Done 2026-11-07. Committed as 6ea1c0e, CI green (run 42419930932). Rows 16, 20, 22, 23 and the AniList part of row 26 are closed. AniList `mapMetadata` calls `mapCandidate`, and AniList `newWithDo` is gone. `TestDecodeObject` is removed, since `jsonscan` `TestObject` covers it. The Bangumi date, type, control-character and ID cases are now one table, `TestBangumiGetRejectsInvalidFieldsAtomically`, which keeps all 19 values. The AniList unsafe-studio case is a second row of `TestAniListGetDropsInvalidCoverAndUnusableStudios`. The other test cases rows 16 and 23 named were already gone after Loop 39T. `adapters/metadata` production went from 1,597 to 1,590 lines (-7) and tests from 2,318 to 2,239 (-79). `go test ./adapters/metadata/... ./tests/...`, `go vet` and one full native suite run (two window walks) passed. Independent oracle review: PASS, no MUST_FIX. |
| S3 | Done 2026-11-07. Committed as 4cb62b5, CI green (run 42418461303). Rows 19, 21, 25 and 26 are closed. The three Anime1 digit parsers use `allASCIIDigits`. `parsePostID`, `normalizeEpisodeText` and `newWithDo` are inlined. `html_policy.go` is merged into `resolver.go` and returns `errResolverMalformed` directly. One repeated `URL limit +1` check is removed from `resolver_test.go`; the two `anime1_test.go` repeats row 25 named were already gone after Loop 39T. `adapters/source/anime1` production went from 1,429 to 1,394 lines (-35) and tests from 1,119 to 1,116 (-3). `go test ./adapters/source/anime1/ ./tests/...`, `go vet` and one full native suite run (two window walks) passed. Independent oracle review: PASS, no MUST_FIX. Its SHOULD_FIX, the stale `02_ARCHITECTURE.md:14` reference, is recorded under "Noticed, not done". |
| S2 | Done 2026-11-07. Committed as 7cfaf7d, CI green (run 42417762480). Row 10 is closed. `PlainTextLimits` now has one byte limit, `MaxInputBytes`, which both length checks use. The `<= 0` guard is removed. With the owner's approval (2026-11-07: "刪兩列照做") the `policy_test.go` rows "output bytes" and "invalid limits" were deleted: the first can no longer be built, and the second tested the removed guard. `internal` production went from 308 to 307 lines and tests from 198 to 196; the row's -5 estimate was too high. `tests/contract/assertions.go` changed by one line. The full native suite (`CGO_ENABLED=1 go test -count=1 ./...`, two window walks) passed once, and `go vet ./...` passed. Independent oracle review: PASS, no MUST_FIX. Its claim that `&nGt;` grows when unescaped is wrong: Go returns it unchanged, as checked by running it. |
| S1 | Done 2026-11-06. Committed as 7f2416c, CI green on b300f3e (run 42331591622, after the x/net update for GO-2026-6860). Rows 1, 2, 4, 6, 8, 9 and 11-14 are closed. Production `core` went from 2,574 to 2,473 lines (-101) and tests from 3,135 to 2,902 (-233), counted with the "Measuring lines" command. Checks: `go test -count=1 ./core/` passed. Every package except `apps/desktop` and `apps/desktop/fltk*` passed vet, a normal test run and a `-race` run. `CGO_ENABLED=1 go vet ./apps/...` passed. `deadcode ./...` reports nothing outside `tests/contract`. With the owner's OK, the window and libmpv tests (`apps/desktop`, `fltk*`) passed one normal run and one `-race` run. Independent oracle review: no MUST_FIX. Its SHOULD_FIX, the stale `02_ARCHITECTURE.md` file list, is recorded under "Noticed, not done". |

## Noticed, not done

Anything outside the hard rules goes here with `file:line` and one sentence. It is not fixed in this loop.

From Phase A (2026-11-06):

- `core/refresh.go:57`: `interval` is always `RefreshInterval`. Removing it means editing `apps/desktop/fltkhome/window_windows.go:253` (rule 2).
- `apps/desktop/backend/actions.go:24-28`: the `Catalog` action has no UI caller (about -9 lines). Removing it needs the allowlist in `apps/desktop/main_test.go:15` (rule 2).
- `backend/dto.go:44-50`, `actions.go:293-299`, `core/following.go:76`: the UI never reads `Following.LatestAvailable`, `HasAvailable` or `NewEpisode` (owner decision 2026-10-19, `fltkhome/model.go:188-190`). Removing them needs fltkhome test edits (rule 2).
- `core/store.go:8`: no production path calls `Store.SaveAnime`. Removing it changes a port, and about 40 test setups would grow (rule 4).
- `core/playback.go:55-58`: `PlaybackSnapshotter` could merge into `PlaybackSession`. That is a port change. If row 3 lands, `PlaybackEvent.Err` (`core/models.go:225`, `libmpv.go:334-336`) loses its last reader.
- Features the owner may drop, each needing files outside rule 2. Line counts are estimates for the in-scope part:
  - Start timeline (`ANIMEPORTABLE_START_TIMELINE`): `backend/timeline.go` plus the proxy `StreamEvent` observer, about -230 / -160. Also needs `main_fltk_windows.go` and 03_SECURITY:173.
  - Failure log: `backend/failurelog.go`, `playError` and the stage tagging, about -310 / -265. Also needs `main_fltk_windows.go` and the proxy `UpstreamRejected` callback.
  - Data-location move: `data_location.go`, the move half of `portable_location.go`, and `store_guard.go`, about -470 / -640. Reading an existing `data-location.txt` stays. Also needs fltkhome settings rows and two allowlist entries.
  - The `resourcepass` harness: `internal/resourcepass`, -204 production, plus 437 test lines in scope and 163 outside it. Loop 38 used it as an acceptance tool.
- `backend/actions.go:161-185`: `fillCatalogInfo` back-fills anime stored before migration 0011, and no test covers it. Removing it changes network requests (rule 5).
- `adapters/playback/proxy/proxy.go:55, 636-637, 674-675`: `OnUpstreamFailure` duplicates `OnStream`'s `StreamFailed`, about -13 / -32. Merging them needs `main_fltk_windows.go:480`.
- `adapters/metadata/aniskip/aniskip.go:34-51` and `adapters/update/github/github.go:26-50` set up the client the same way. `anilist.go:118-131` and `:157-170` share a request prefix. Six DOM walks in `anime1.go` and `resolver.go` share their scaffolding. Each merge needs a new package or helper (rule 3).
- `backend/service_integration_test.go:220-500`: seven tests build the same setup block, about -55 test lines. A shared setup function would be a new test helper (rule 3).
- `docs/02_ARCHITECTURE.md:137` says every adapter passes `tests/contract`, but libmpv never runs `RunPlayer`. That doc is outside rule 2.
- `adapters/persistence/sqlite/install_exclusive_windows.go:1`: `installPortableSnapshot` has no non-Windows version, so sqlite does not build off Windows. This matters for later Linux work.
- `backend/actions.go:315-333`: the backend's follow path does not check that the anime exists, while the unused core version (row 1) does. Row 1 keeps today's behaviour.
- `docs/02_ARCHITECTURE.md:13`: the core file list still names `metadata_variants.go`, which S1 merged into `metadata_normalize.go`. That doc is outside rule 2.
- `docs/02_ARCHITECTURE.md:14` still names `html_policy.go`, which S3 merged into `resolver.go`. That doc is outside rule 2.
- `adapters/metadata/bangumi/bangumi_test.go:32, 36, 38`: the constants `testMaxDescriptionBytes`, `testMaxEpisodeCount` and `testMaxMetadataIDDigits` have no user since Loop 39T deleted their tests. No approved row covers them.
- `adapters/network/securehttp/client_test.go`: the test type `idleClosingTransport` has no user, and already had none at 6ea1c0e. No approved row covers it.
- `core/following.go:33`: since S1, Following reads its clock from `app.tracking.now`, a field of the playback-tracking config. That is a slightly misleading place for it.
- Owner, 2026-11-06, during the S1 window-test run: tests should check how the app is really used, not open new windows over and over. Most of the 260 or so tests in `apps/desktop` and `fltk*` each open their own FLTK window or libmpv player, and the `-race` run repeats all of them. Changing that touches `apps/desktop/fltk*` and the test rules in `05_WORKFLOW.md`, so it is outside this loop.
- The `LOOP39.md` measuring script leaves out `tests/contract` (891 / 479 lines) and counts `adapters/persistence/sqlite/migrations/` Go files. The latter has none today.

## Done when

- Every approved row is done or listed as dropped with the reason.
- Production lines in scope are lower than the baseline; test lines are not higher.
- Full suite and race suite pass once on the final state; CI is green for the last commit.
- The owner's normal-session check of a test build in `artifacts/test-build/` passes (open Home, search, open a detail page, play an episode, check History and Settings).
