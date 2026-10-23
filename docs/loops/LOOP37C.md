<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 37c: Skip the opening

Status: PASS (owner accepted 2026-10-22, owner check 1). Commit af53275; GitHub Windows CI 39877272016 green (includes the new GUI-subsystem check). Owner allowed window tests 2026-10-22. Scope approved 2026-10-22 with the MyAnimeList id kept in memory ("存記憶體, 沒影響太大就這樣做"); the owner approved one new backend action, `OpeningTimes` (same day).

## Goal

Like Netflix, a 跳過片頭 button appears only while the opening plays and jumps to its end. It replaces the fixed-seconds skip the owner removed in Loop 37 (owner check 3: openings often start after a short scene).

## Current state (2026-10-22)

- AniSkip v2 (`GET https://api.aniskip.com/v2/skip-times/{malId}/{episode}?types=op&types=mixed-op&episodeLength={seconds}`, no key) returns `found` and `results[]` with `interval.startTime`/`endTime` in seconds, `skipType` and the `episodeLength` of the submission. `episodeLength` 0 returns all submissions; a real length makes AniSkip pick timestamps from a matching cut. No rate limit is documented.
- AniSkip needs the MyAnimeList id. AniList has it (`idMal`); Bangumi does not. Stored metadata is usually Bangumi, because Bangumi wins score ties (Chinese synopsis, ADR-009). So the stored record alone gives a MyAnimeList id only for some anime.
- The player knows the episode number (Anime1, per season, like AniList and MyAnimeList) and, once loaded, the length (libmpv `duration`).

## Proposed scope

1. MyAnimeList id, looked up the first time an episode of that anime plays: AniList search with the anime's title, accepted only through the existing `core.MatchMetadata` (only AniList candidates), then a small AniList query for that anime's `idMal`. No accepted match or no `idMal` means no skip for that anime. The answer (id or none) is kept in memory for the app run; nothing is stored, so no migration. (Alternative for the owner: store it in SQLite with migration 0011, so it survives a restart and AniList is asked once per anime ever.)
2. Opening times: one AniSkip request per episode load, after libmpv reports the length, for `op` and `mixed-op`; the episode number must be a plain whole number (no `12.5`, no ranges). Results are validated (see 03_SECURITY) and the one whose `episodeLength` is closest to the real length is used; none within 5 seconds of the real length means no skip. Opening times are not kept: a replay asks again.
3. Core ports `core.OpeningSource` (adapter `adapters/metadata/aniskip`, its own secure client with origin `https://api.aniskip.com` only) and `core.MyAnimeListSource` (the AniList client); `App.MyAnimeListID`, `App.FindOpening` and `ChooseOpening` hold the policy. Backend action `OpeningTimes` (owner-approved) keeps the ids in memory; `main` (`playerOpening`) asks once per load when libmpv first reports the length, and gives the player the interval. A slow or failed lookup never delays or breaks playback; it only means no button.
4. Player: a 跳過片頭 button at the lower right, above the control bar, shown only while the position is inside the opening (start ≤ position < end − 1 s), also when the other controls are hidden; it disappears when the opening ends or after it is used. Click, Enter (when focused) and the S key jump to the opening's end. Seeking back into the opening shows it again. In the Tab order when shown. Text in uitext (跳過片頭 / Skip Intro / OPをスキップ).
5. No setting: the button only appears when data exists.
6. Added by the owner 2026-10-22 (workflow edit approved): no console window next to the app. CI and test builds link with `-H windowsgui`; CI fails when the executable is not a Windows GUI program (`objdump` subsystem 2); the one startup error that printed to the console (FLTK threading unavailable) shows in a Windows message box (`StartFailed` in uitext).

## Out of scope

Detecting openings from audio; endings, recaps and previews; submitting timestamps to AniSkip; automatic skipping without a press; storing opening times.

## Tests

- AniSkip adapter: request path and query exactly (only the three numbers), response validation (bad JSON, too large, negative, end ≤ start, end beyond length, too many results, wrong type) fails closed, `found: false` and 404 mean no data, choice of the closest-length result.
- AniList: `idMal` mapped and bounded; MyAnimeList lookup uses only an accepted AniList match.
- Core: lookup happens only for a playing episode; no request for a non-integer episode number or an unknown length; per-run memory prevents a second AniList lookup for the same anime.
- Player (window tests, run locally after asking the owner): button visible only inside the interval, S and click jump to the end and hide it, seeking back shows it, focus leaves when it hides, fits in all three languages.
- Full native suite and race; simplify; oracle review with no MUST_FIX.

## Owner check

1. Play an episode of a popular series (for example a current-season show on Anime1). During the opening, 跳過片頭 appears at the lower right; outside it, it does not.
2. Press it (or S): playback continues right after the song, and the button is gone.
3. Seek back into the opening: the button appears again.
4. Play an episode of an obscure or very old series: no button, playback unaffected.
5. Start the app from its folder: only the app window opens, no black console window.

## Known limits

- Anime1 sometimes numbers a later season on from the first (13, 14, …) while MyAnimeList starts each season at 1; such episodes find no data, so no button (never a wrong skip).
- Only anime whose Japanese title AniList matches get a MyAnimeList id; stored metadata without a Japanese title falls back to the Anime1 title, which AniList rarely knows.

## Review (oracle, 2026-10-22)

- No MUST_FIX. SHOULD_FIX fixed: (1) a miss on the Anime1 title (no stored Japanese title yet, as on a first play while the detail page still refreshes metadata) was remembered for the run; now only a settled answer is kept (`MyAnimeListID` reports it). (2) A lookup kept running after the episode changed and held an operation slot; each load's lookup now has its own context, cancelled when the next load starts and when the player leaves (re-review: no MUST_FIX or SHOULD_FIX). NITs: two lookups at once for one anime are accepted (same answer, said in the comment); the spec line on `idMal` corrected; seek rounded up to a whole second and the optional AniList shortcut left as is.

## Runs

- 2026-10-22: AniSkip checked by hand for MyAnimeList 52991 episode 1 (one `op` result with `episodeLength` 1420.1) and episode 999 (HTTP 404), matching the adapter. Non-window suite (CI package set), `gofmt` and `go vet` green; window tests not run yet (owner to allow).
- 2026-10-22, owner allowed window tests. Full native suite 1: one failure, `TestNativeLanguageSwitchesEveryShownTextAtOnceAndIsSaved` (`ja: focus left the language control`; Loop 37b test, fltkhome not changed here); alone `-count=5` passed; TypeSafe failure-class FLAKY_OR_NONDETERMINISTIC conf 1.00 (`artifacts/evidence/loop37c/failure-native-1.json`). Full native suite 2 and `-race` green with no change (`native-suite-2.txt`, `race-1.txt`).
- Simplify pass: the AniSkip adapter now decodes with the shared bounded decoder (`adapters/metadata/internal.DecodeObject`: size, nesting, trailing data). Adapter tests green.
- Console check: `objdump -p` reads subsystem 2 (Windows GUI) for a `-H windowsgui` build and 3 (console) without it, so the new CI check catches both.
- Test build 1: `artifacts/test-build/AnimePortable.exe` (EXE SHA-256 `AE649369…2059`, Windows GUI subsystem; libmpv DLL matches the pin; data backed up to `artifacts/evidence/loop37c/data-backup-before-build-1/`).
- Owner check 1 (2026-10-22, test build 1): "Owner check 1 passed：跳過片頭和沒有黑色視窗都正常。" TypeSafe owner-report HUMAN_PASS (conf 1.00, `owner-report-1.json`).

## Release v0.4.0-beta (owner, 2026-10-22)

- Release commit d81b995 failed CI 39879500545: `TestCheckForUpdateComparesWithThisBuild` hard-coded tags newer than 0.3.0-alpha, so the bump to 0.4.0-beta made them older (only `internal/release` was run locally before the push). The test now raises the current version's minor and patch, so it stays valid after every bump; the checks are unchanged.
- Owner, before tagging: beta needs an icon (Explorer showed a default one); the owner supplied the image. `apps/desktop/appicon`: the white background removed, squared; `icon.ico` (16-256) linked into the exe through `rsrc_windows_amd64.syso` (`make_syso.ps1`, MinGW windres); 16/32/48 px PNGs set on the Home, player and dialog windows (`Window.SetIcons`). Test checks every size loads. Windows extracts the icon from test build 2.
- Test build 2: EXE SHA-256 `C627BC70…9977` (Windows GUI, `.rsrc` present); data backed up to `data-backup-before-build-2/`. Owner: "Test build 2 圖示正常。" Full native suite and ``-race`` green (``native-suite-3.txt``, ``race-2.txt``); simplify: no change.
- Icon and test fix: commit 2caceaa, CI 39880655456 green. Tag `v0.4.0-beta` (annotated) on 2caceaa; release job 39880999897 green reused that run's exe; pre-release https://github.com/loustack17/AnimePortable/releases/tag/v0.4.0-beta.
- Published ZIP checked (`artifacts/evidence/loop37c/release-v0.4.0-beta/`): SHA-256 `6BCF19A8…8509`; files `animeportable.exe`, `libmpv-2.dll` (matches `internal/runtimepin`), `LICENSE`, `THIRD_PARTY_NOTICES.md`, `licenses/libmpv.txt`; exe Windows GUI subsystem with the app icon; unzipped and started: window `AnimePortable`, `data/animeportable.db` created, no console process; closed cleanly. Owner to try the release.
