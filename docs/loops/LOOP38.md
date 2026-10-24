<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 38: Hardening and release (Windows MVP)

Status: PASS (owner accepted 2026-10-24: "Owner check of v1.0.0: downloaded the release ZIP, unzipped, ran it and used it for a normal session; everything works."; TypeSafe owner-report HUMAN_PASS 0.82). 38a PASS (66f8730, CI 40054548137); 38b PASS (35a6729, CI 40061188113); 38c PASS: v1.0.0 published 2026-10-24 (tag on 5d9c068, CI 40069127213, release job 40069518830). Owner approved starting Loop 38 on 2026-10-23 and asked for it to be split into stages; stage plan approved 2026-10-23 ("I approve the Loop 38 stage plan as written. Start stage 38a."; TypeSafe owner-report HUMAN_PASS 0.42, adjudicated: explicit approval). Base: origin/main 9e68e48.

## Goal

Publish v1.0.0 as a full GitHub Release: a CI-built ZIP that runs after extraction, after a security pass and a resource pass with no open MUST_FIX finding (roadmap Loop 38).

## Scope

Three stages, in order:

- 38a: check the 20 boundaries in the 38a table against 03_SECURITY.md; fix MUST_FIX findings.
- 38b: measure goroutines, Go heap and private bytes over 200 cycles of five scenarios; fix leaks.
- 38c: release notes, version 1.0.0, READMEs, release-job flag, tag and publish, ZIP check.

Out of scope: new features, new settings, screens or backend actions, installers, Linux, macOS, the 2.0 plan (Loops 39-46), the deferred OneAnime resource comparison (ADR-023), libmpv licensing (ADR-023).

Each stage ends with a report to the owner. The next stage starts only after the owner approves. Every stage ends with a handoff checkpoint.

## Common checks (every stage that changes code)

- Every fix has a test that fails without it.
- `gofmt -l .` empty; `go mod verify`; `go vet ./...`.
- `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` (the version CI pins in `.github/workflows/ci.yml`).
- Full native suite with an msvcrt MinGW gcc on PATH: `CGO_ENABLED=1 go test -count=1 ./...`, then `CGO_ENABLED=1 go test -race -count=1 ./...`. Window and real-libmpv tests run only after the owner says the PC is free.
- simplify pass over every changed file, tests rerun.
- Independent read-only oracle review with no open MUST_FIX; triage with TypeSafe `review-triage.json`.
- Test build only in `artifacts/test-build/` (exe built with `-ldflags "-H windowsgui"`, pinned DLL, notice, existing `data/` kept).
- Commit and push only after owner approval; confirm GitHub Windows CI for the pushed commit.

## 38a: Security pass

### Boundaries checked

Each row is checked against its rule in [03_SECURITY.md](../03_SECURITY.md) with a named test or code reference, recorded in the result table below. Rows 7-20 were added after the original six-row table.

| # | Boundary | Rules to verify |
| --- | --- | --- |
| 1 | Internet -> Go backend (`adapters/network/securehttp`) | one client; HTTPS only, scheme allowlist; TLS verify, no `InsecureSkipVerify`; timeouts; bounded bodies; bodies closed; no unmanaged `http.Client` or `http.DefaultClient` anywhere outside it |
| 2 | Go backend -> FLTK UI (`apps/desktop/backend`, `fltkhome`) | typed actions only (fixed allowlist in `main_test.go`); no URLs, tokens, cookies, raw HTML or stack traces in results |
| 3 | Backend -> loopback proxy (`adapters/playback/proxy`) | loopback bind only; token entropy; session invalidated on close/switch; stream caps; no credentials in errors |
| 4 | Proxy -> libmpv | libmpv gets only the loopback session URL; no headers or cookies passed; empty `http_proxy`; `config=no`; mpv `log-file` off |
| 5 | libmpv DLL pin and loader (`internal/runtimepin`, `adapters/player/libmpv`, `fltkengine`) | absolute path; hash verified on a held, non-write-shareable handle; reparse points and non-regular files rejected; dependency search limited; override pair `ANIMEPORTABLE_LIBMPV_OVERRIDE`/`_SHA256` gets the same checks; missing or altered DLL leaves Home usable |
| 6 | SQLite (`adapters/persistence/sqlite`) | absolute path; regular file, `nofollow`; no secrets in any table; migration checksums |
| 7 | Anime1 resolver and cookies (`adapters/source/anime1`) | origin allowlist; cookies and signed URLs in memory only; parser-only HTML extraction; page timeout and retry bounded |
| 8 | Playback CDN origins | HTTPS, public IP, redirect revalidation, expected media response, fail closed |
| 9 | Redirects | every hop rechecked (scheme, host, resolve, SSRF); sensitive headers not forwarded across origins |
| 10 | SSRF and DNS rebinding | dial-time IP check of the connected address; blocked ranges as listed |
| 11 | Forward proxy (`adapters/network/forwardproxy`) | CONNECT only; TLS to the original host; manual address syntax; only that host:port exempt; WinINet read-only parse; proxy address never logged |
| 12 | Metadata providers (AniList, Bangumi) | origin allowlist; bounded plain text; cached metadata rechecked on read |
| 13 | AniSkip (`adapters/metadata/aniskip`) | only MAL id, episode, length sent; 64 KiB / 20 results; value checks; nothing logged; no response text in UI |
| 14 | GitHub update check (`adapters/update/github`) | user-triggered only; GitHub API origin; bounded body; never downloads or runs; release page opened only from a fixed origin/path pattern |
| 15 | Cover images (`adapters/metadata/cover`) | fixed origins; status 200; JPEG/PNG with agreeing MIME, signature, decoder; 4 MiB; pixel limits before decode; 4 concurrent |
| 16 | Failure log and start timeline (`apps/desktop/backend/failurelog.go`, `timeline.go`) | fixed vocabulary; no URLs, tokens, cookies, titles, ids, proxy address; timeline off unless `ANIMEPORTABLE_START_TIMELINE=1`; size bound |
| 17 | Portable data folder and its relocation setting | `<folder>/data` default; relocation never silently moves or deletes data; path validation; no elevated rights |
| 18 | Other environment variables read by the shipped exe | every `ANIMEPORTABLE_*` variable read outside `_test.go` listed, and each one either harmless or covered above |
| 19 | `-H windowsgui` build and process | GUI subsystem enforced by CI; no admin manifest; no console; restart path does not pass secrets on the command line |
| 20 | Supply chain | `go mod verify`; govulncheck clean or each finding classified; libmpv archive SHA-256 pinned in the release job; `THIRD_PARTY_NOTICES.md` current |

### Method

1. Explorer agents map each row to its code and existing tests (read-only).
2. Root agent checks each rule with a test name or `file:line`. A rule with no test and no clear code evidence gets a test.
3. govulncheck and `go mod verify`, output saved to `artifacts/evidence/loop38/38a/`.
4. Independent review: an oracle agent and a security-auditor agent, read-only, each given the boundary table and the whole tree. Their findings are merged and triaged with TypeSafe.
5. Classification: MUST_FIX (a rule in 03_SECURITY.md is broken or a secret can leak), SHOULD_FIX (defence weaker than the doc says but no rule broken), NOTE (stale doc, residual risk already accepted). MUST_FIX gets the smallest causal fix and a test. SHOULD_FIX is fixed in 38a only if small and inside scope; otherwise listed for the owner. A fix that would change security policy goes to the owner first.
6. 03_SECURITY.md updated where it is stale (for example the six-row boundary table), with no change of meaning unless the owner approves.

### Done when

- Every row above has a recorded verdict with evidence.
- govulncheck shows no reachable vulnerability (or each is classified and accepted by the owner).
- No open MUST_FIX; reviewers' findings and their dispositions recorded below.
- Common checks pass. Commit after owner approval.

## 38b: Resource pass

### Harness

A repeatable Go test harness (build tag `resourcepass`, so the normal suite is unchanged), run headless: no FLTK window, no screenshots, no video on screen, no network. Upstreams are local TLS test servers; the media is a local test fixture.

Per scenario, after a warm-up of 20 cycles, 200 cycles, sampled every 20 cycles:

- goroutines: `runtime.NumGoroutine()`;
- Go heap: `HeapInuse` after `runtime.GC()` (forced GC is used only to detect leaks here, never to meet the idle target);
- process private bytes: Windows `GetProcessMemoryInfo` `PrivateUsage`.

Scenarios:

1. Player open/close through the real backend, proxy and `core` playback session with a fake player.
2. The same with real in-process libmpv using `vo=null`, `ao=null` and a local fixture through the proxy, if libmpv runs without a window. If it needs a window, the owner is asked before it runs.
3. Episode switches, manual and autoplay (end-of-file to next episode).
4. Cancelled searches (cancel before response, during body, after response) with fake Anime1 and metadata servers.
5. Proxy-session churn: create, stream partially, abort mid-body, invalidate, including the per-session (4) and global (16) caps.

Thresholds (no steady growth):

- goroutines at the end equal the warm-up baseline within ±2, and are back to baseline after the scenario closes;
- heap after GC: the mean of the last five samples is at most 1 MiB above the mean of the first five, and the samples do not rise monotonically;
- private bytes: at most 8 MiB growth from the first to the last sample after warm-up (libmpv and the C heap do not return memory at once; any growth above this is investigated with a heap profile before it is called noise).

A breach is a finding: diagnose with `pprof` goroutine and heap profiles, fix causally with a test, rerun the scenario.

### Idle memory target

ADR-023's goal: private working set under 100 MiB after 15 minutes minimized. This needs the real app window, so it runs only when the owner says the PC is free: the test build is started with a copy of the owner's data, minimized, and sampled every minute for 15 minutes by a PowerShell script reading the process counters; no clicks. Results recorded below. No working-set trimming.

### Done when

- All five scenarios meet the thresholds; numbers in the table below and raw CSV in `artifacts/evidence/loop38/38b/`.
- Idle target measured (or the owner defers it in writing).
- Every leak found is fixed with a test. Common checks pass. Commit after owner approval.

## 38c: Release v1.0.0

1. Release notes `docs/releases/v1.0.0.md` (Traditional Chinese, for users; no AI attribution): what works, how to run it, known limits.
2. `internal/release.Version` set to `1.0.0` (versioning rule in [05_WORKFLOW.md section 11](../05_WORKFLOW.md)); tests derive versions from it.
3. README.md (Chinese, default) and README.en.md updated for v1.0.
4. Release job: it always passes `--prerelease` today. v1.0.0 must be a full release, so `.github/workflows/ci.yml` needs a small change (no `--prerelease` when the tag has no `-` stage suffix). This is a workflow edit: owner approval first. Section 11 asks to test a changed job with a pre-release tag first; the owner decides whether to do that (for example `v1.0.0-rc`) or accept the one-flag difference untested.
5. Push the release commit after approval; CI green on it.
6. Before asking to tag: remind the owner of the open question recorded 2026-10-23, whether to keep the Anime1 source in a publicly distributed release (Taiwan Copyright Act art. 87(1)(8) and 80-2, GitHub DMCA). The owner decides; the agent gives no legal advice.
7. Tag `v1.0.0` and publish only on explicit owner approval at that moment.
8. Verify the published ZIP in a new empty folder: file list (exe, `libmpv-2.dll`, LICENSE, THIRD_PARTY_NOTICES.md, `licenses/libmpv.txt`, which is the libmpv disclosure notice); DLL SHA-256 equals `internal/runtimepin`; exe is GUI subsystem and reports 1.0.0; no `data/`, logs or personal files; it starts after extraction and creates its database (process start and file check only, with the owner's OK for the window).

### Done when

- CI-built ZIP published as a full GitHub Release v1.0.0 and checked as above.
- Owner check passes.

## Owner check (after 38c)

1. 從 GitHub Release v1.0.0 下載 ZIP。
2. 解壓縮到一個新資料夾，雙擊 animeportable.exe：應直接開啟，不需安裝，沒有黑色主控台視窗。
3. 搜尋一部動畫、點開、播放一集、切換下一集、關閉播放器、回首頁：都應正常。
4. 關掉 App 再開：剛才的觀看紀錄與進度應還在。

## Results

### 38a (2026-10-23)

Evidence: `artifacts/evidence/loop38/38a/` (govulncheck, mpv probe, TypeSafe triage, test runs).

Checks: `go mod verify` all modules verified; `go vet` clean; `gofmt -l` empty; govulncheck v1.1.4: 0 reachable vulnerabilities, 6 in required modules not called (x/text GO-2026-6629; x/net HTTP/2 server GO-2026-6603, 6610, 6611, 6612, 6617).

Boundaries: rows 1-3 and 5-20 OK with tests named in the evidence map. Row 4 (proxy -> libmpv) broken, fixed below. Secret scan of tracked files: no keys or tokens (the Bitwarden secret ID in `tools/typesafe-judge/run.ps1` is a lookup ID, not a credential).

Findings (two explorer maps, an oracle review and a security-auditor review; triage with TypeSafe `review-triage`, adjudicated):

| ID | Class | Finding | Disposition |
| --- | --- | --- | --- |
| M1 | MUST_FIX | libmpv detected the stream format itself. A body labelled `video/mp4` that is really an m3u, PLS, EDL or HLS playlist made libmpv fetch the listed address directly, past the proxy, origin allowlist and SSRF checks. Confirmed headless on the pinned DLL (1 fetch each). | Fixed: `demuxer=lavf`, `demuxer-lavf-format=mp4` in `InitOptions`; 0 fetches, real MP4 plays. Test `TestEngineOpensOnlyMP4BodiesNeverListedAddresses` (fails without). |
| S2 | MUST_FIX | Libraries loaded by name after startup (by libmpv, FFmpeg, `opengl32` and `comctl32` lazy loads) used the full search order including the current folder and PATH. | Fixed: `SetDefaultDllDirectories(APPLICATION_DIR, SYSTEM32)` first in `main`. Test `TestRestrictDLLSearchStopsLoadsFromTheCurrentFolder`. |
| A1 | MUST_FIX | Resolver JSON scan had no depth limit: a 1 MiB `[[[[` answer recursed about a million times per Play. | Fixed: depth 16. Test `TestResolverJSONNestingExactLimit` (fails without). |
| T1 | MUST_FIX (TypeSafe 0.69) | Debug timeline capped events per start but not starts per run. | Fixed: 100 starts per run. Test `TestStartTimelineStopsAfterMaxAttempts` (fails without). |
| S1 | NOTE | mpv `ytdl_hook` could start `yt-dlp` by search path. Pinned DLL has no `ytdl`/`osc` option (no scripting), so only an override DLL built with scripting is affected; `ytdl=no` cannot be set because the pinned DLL rejects unknown options. | Recorded in 03_SECURITY.md. (`load-scripts=no` was tried and removed: it only covers the scripts folder, which `config=no` already skips.) |
| N2 | NOTE (TypeSafe said MUST_FIX 0.73; adjudicated) | Failure log and timeline writes follow a link planted in the data folder. Needs same-user write access to the portable folder, the accepted local residual risk. | Listed for the owner. |
| N1 | NOTE | Redirect header stripping compares each hop with the previous one, not the first; not reachable today. | Listed. |
| N3 | NOTE | SQLite `nofollow` is Unix-only; `Lstat` is the Windows guard. | 03_SECURITY.md corrected. |
| N4 | NOTE | Override DLL path accepts UNC; needs same user, hash checked on a held handle. | Listed. |
| N8 | NOTE | Playback close destroys the player before invalidating the proxy session; 03_SECURITY.md lists the reverse. | Owner decision: which order the doc should state. |
| U1 | NOTE | Remote titles drawn with `fltk.Draw` without escaping `@` (display only). | Listed. |
| C1, C2, V1, D1 | NOTE | Notices not diffed in CI (generated at release); Actions `checkout`/`setup-go` by tag; unreachable module vulnerabilities; dev-tool HTTP client. | Listed. |
| Test gap | - | No test for a missing DLL. | Added `TestOpenDoesNotLoadAMissingLibrary`. |

Final review (oracle, read-only): no MUST_FIX. SHOULD_FIX: the `load-scripts` comment overstated it (removed, above); no automated test runs the window, GL or hardware-decoding paths with the DLL search restriction, so the owner's test build (hardware decoding on and off) is the check. NIT: the JSON depth counts the root as 0 (pinned by the test). It agreed N2 is a NOTE.

03_SECURITY.md updated: boundary table rows 7-11, DLL search order, built-in scripts, MP4-only demuxer, override variable names, SQLite guard on Windows, diagnostic files, `tools/` not shipped.

Test build: `artifacts/test-build/AnimePortable.exe` (38a worktree build 2026-10-23 with the 硬體解碼 notice, EXE SHA-256 `4211E2C0…33D4`; the owner check below ran on the previous build `C2B7E676…EDAA`; Windows GUI; libmpv DLL matches the pin; existing `data/` kept).

38a owner check (the DLL search restriction is only exercised by a real run):

1. 開啟 App，到「設定」把「硬體解碼」關閉，關掉 App 再開。點開一部動畫播放一集：應正常播放，畫面與聲音正常。
2. 回「設定」把「硬體解碼」打開，關掉 App 再開，播放一集：應正常播放。
3. 播放中按下一集：應切換並開始播放。

Owner check 2026-10-23: "全部正常" (all three steps pass; TypeSafe owner-report HUMAN_PASS 0.39, adjudicated: explicit pass). New owner request in the same reply: the 硬體解碼 switch should tell the user it needs a restart, or offer to restart after switching. Fact: no app restart is needed; the engine reads the setting each time the player opens (`main_fltk_windows.go`, engine factory), and the row's hint already says 下次開啟播放器時生效; the check steps asked for a restart only to be safe. Owner choice 2026-10-23: "Add a short notice after toggling 硬體解碼 (下次開啟播放器時生效) in 38a, in all three languages, then commit 38a." Built: after a change is saved, a dialog titled 硬體解碼 says 已變更硬體解碼，下次開啟播放器時生效，不需要重新開啟程式。 (English and Japanese too); no notice when nothing is saved (failed settings load, data move). Test `TestNativeSettingsHardwareDecodingToggleSaves` (fails without); the failed-load and data-move tests check that no notice appears. Review (oracle): no MUST_FIX; its two test gaps were closed; open NIT: holding Space past the key repeat delay can close the notice at once (the change is still saved). Whether focus returns to the switch after the notice closes is for the owner to see.

38a PASS: commit 66f8730 pushed to main with owner approval; GitHub Windows CI 40054548137 green.

### 38b (2026-10-24)

Owner approved starting 38b 2026-10-24 ("Start stage 38b (resource pass) as planned in LOOP38.md.").

Harness: `internal/resourcepass` (sampler and limits, Windows only) and one `resourcepass_test.go` per layer, all behind the `resourcepass` build tag, so neither the app nor the normal test run builds them. Run:

```
CGO_ENABLED=1 RESOURCEPASS_OUT=<folder> go test -tags resourcepass -count=1 -run TestResourcePass -v -timeout 40m ./adapters/network/securehttp ./adapters/playback/proxy ./apps/desktop/backend ./apps/desktop ./apps/desktop/fltkengine
```

No window, no screen, no network: upstreams are local servers, the media is the test fixture. Each scenario: 20 warm-up cycles, then 200 cycles (real libmpv: 2000), a sample every 20 (goroutines; Go heap in use after GC; Windows private bytes). Limits: goroutines within 2 of the post-warm-up count and back to the count before setup after teardown; heap and private bytes compared as the mean of the last five samples minus the mean of the first five, at most 1 MiB and 8 MiB; heap not rising at every sample. (The private-bytes comparison of five-sample means replaces the plan's single first and last samples, which a swing of 40 MiB in libmpv made powerless; it is stricter.)

| Scenario | What a cycle does | Goroutines (after warm-up -> end; before setup -> after teardown) | Heap growth | Private growth (first five -> last five means) |
| --- | --- | --- | --- | --- |
| player-open-close | backend Play then StopPlayback: real SQLite, core tracking, libmpv adapter, a new playback proxy and session; fake engine | 3 -> 3; 2 -> 2 | +51 KiB | +0.8 MiB (57.0 -> 57.7) |
| episode-switches | backend Play of the next episode; every other one after an end of file taken in by the session (autoplay), some paused | 7 -> 7; 2 -> 2 | +18 KiB | +0.1 MiB |
| player-helpers | main's start watch (real timer), opening lookup (every other one cancelled when the episode is left) and autoplay, on a simulated UI thread | 3 -> 3; 2 -> 2 | -2 KiB | +0.6 MiB |
| cancelled-searches | backend Search cancelled while the source waits, cancelled before it starts, and one that completes | 3 -> 3; 2 -> 2 | +3 KiB | -0.1 MiB |
| cancelled-requests | secure client against a local TLS server: cancelled before the answer, mid-body, before start, abandoned stream, one complete | 6 -> 6; 2 -> 2 | +88 KiB | +1.0 MiB |
| proxy-session-churn | proxy session: three range requests read in part; every 10th cycle six at once (at least 2 refused, cap 4); every 20th twenty on five sessions (at least 4 refused, cap 16); every 5th closed mid-stream (stream cut) | 3 -> 3; 2 -> 2 | +122 KiB | +1.2 MiB |
| real-libmpv-open-switch-close | pinned DLL loaded, mpv created with the app's options (video and audio output null), two episodes to the first frame, mpv destroyed, DLL unloaded | 3 -> 3; 2 -> 2 | +86 KiB | +3.4 MiB (169.5 -> 172.9) over 2000 cycles |

All scenarios pass. Raw CSV, logs and `summary.csv` in `artifacts/evidence/loop38/38b/` (real libmpv in `long2000/`).

Real libmpv in detail: at 200 cycles the stricter check failed (+18.3 MiB; samples 152 142 149 150 151 then 169 168 167 166 167 168 MiB: one step around cycle 90, flat after; log `run-200-stricter-libmpv-fail.txt`). The 2000-cycle run levels off: means of 400-cycle blocks 168.6, 171.7, 170.4, 170.8, 174.2 MiB, about 3.5 KiB per cycle and within noise of ±7 MiB; goroutines flat, Go heap +86 KiB. A diagnostic run keeping the DLL loaded rose about 20 KiB per mpv session, so what growth there is lies inside libmpv/FFmpeg, not in Go, the proxy or the binding (which frees every string mpv returns); the pinned DLL is fixed (ADR-023). NOTE, nothing to fix here (TypeSafe review-triage ADVISORY 0.61). The scenario now runs 2000 cycles (about 18 minutes) by default, so a step is not read as a trend.

Idle memory (owner approved the run 2026-10-24): test build `4211E2C0…33D4` started minimized from a copy of its folder and data, no clicks, sampled each minute (`artifacts/evidence/loop38/38b/idle/idle-memory.csv`, script `idle-memory.ps1` in the worktree's `artifacts/evidence/loop38/38b/`). Private working set 12.3 MiB at start, 10.3 MiB after 15 minutes (target under 100 MiB, ADR-023); working set 32.1 MiB; private bytes (commit) 111.9 MiB, steady. Minimizing lets Windows trim the working set, as in the Fyne measurement the target came from; the app does not trim itself. Target met.

Harness review (oracle, read-only): no MUST_FIX. Addressed: the end of file before an autoplay switch is now taken in before the switch (unbuffered events); the proxy churn checks refusals at both caps and that closing a session cuts its stream; a private-bytes read failure fails the test; private bytes use five-sample means; at least 200 cycles; waits have timeouts; the libmpv test fails if it could not replace the video output. Not done, recorded as limits: real libmpv reads the fixture from a local server, not through the playback proxy, and the cancelled-search scenario uses a fake source, not the Anime1 adapter: both would need a test-only hook in production code (the proxy and secure client allow only public HTTPS upstreams). Proxy traffic is covered by the churn scenario and the secure client by the cancelled-requests scenario.

Not covered: the engine's OpenGL render context and the FLTK windows (they need a window).

38b PASS: commit 35a6729 pushed to main with owner approval; GitHub Windows CI 40061188113 green.

### 38c (2026-10-24)

Owner approved starting 38c 2026-10-24 ("Start stage 38c (release v1.0.0) as planned in LOOP38.md."). Release-job change approved 2026-10-24 ("Approved: apply the ci.yml release-job change as shown (full release when the tag has no stage) and update 05_WORKFLOW section 11; no pre-release test tag. The window tests may run now.").

Changes: `docs/releases/v1.0.0.md` (user notes, Traditional Chinese; TypeSafe doc-quality clarity 1.75-1.97, intro filler 1.14 kept as the one-line summary earlier notes also have); `internal/release.Version` = `1.0.0`; README.md and README.en.md describe the finished features and v1.0.0 (build command with `-H windowsgui`, file name `animeportable.exe` as in the ZIP); STATUS.md; the release job publishes a tag with a stage as a pre-release and one without as a full release marked latest (`.github/workflows/ci.yml`, 05_WORKFLOW.md section 11). Not decided: how versions after v1.0.0 are numbered.

Checks: full suite and `-race` pass for every package; `go vet` clean; govulncheck v1.1.4 no vulnerabilities affecting the code. Window packages: one failure without `-race` in `TestNativeRowListArrowsScrollAndLeaveToNavigation` ("FLTK event did not reach the expected Search state"; no fltkhome file changed), passed on rerun alone and as the whole package (TypeSafe failure-class FLAKY_OR_NONDETERMINISTIC 1.00). This is the second window-test flake in Loop 38 (38a: language focus); both are focus or event timing on the owner's desktop.

Review (oracle, read-only): no MUST_FIX. Fixed: the handoff, two wording points in the notes (the hardware-decoding notice is the one change in function; the 10 MB figure is what Task Manager shows), the file name in the owner check. Owner decision: the update check offers the newest release, pre-releases included, so a later `v1.1.0-beta` would show 有新版本 to someone on 1.0.0 (`adapters/update/github`). NIT kept: `--latest` would also mark a later patch of an older line as latest; post-1.0 numbering is undecided.

Release commit 5d9c068 pushed to main with owner approval 2026-10-24 ("Commit the v1.0.0 release commit, push to main and confirm CI. Do not tag yet."); GitHub Windows CI 40069127213 green. Not tagged yet.

Release v1.0.0 published 2026-10-24 with owner approval. Before tagging, the owner was reminded of the open legal question of 2026-10-23 and decided: "I have considered the open legal question and decide to keep the Anime1 source. Tag v1.0.0 on 5d9c068 and publish it, then check the ZIP." Annotated tag v1.0.0 on 5d9c068; release job 40069518830 green; GitHub release v1.0.0 is a full release (not a pre-release, not a draft) and the repository's latest.

ZIP check (`artifacts/evidence/loop38/38c/release-check/`): `AnimePortable-v1.0.0-windows-amd64.zip`, 31,419,486 bytes, SHA-256 `98D9B277…F048`. Contents: `animeportable.exe` (SHA-256 `93A0C830…FF52`, Windows GUI subsystem, contains 1.0.0, built with `-trimpath`), `LICENSE`, `THIRD_PARTY_NOTICES.md`, `libmpv-2.dll` (SHA-256 equals the pin in `internal/runtimepin`), `licenses/libmpv.txt` (the libmpv disclosure notice). No `data/`, logs or database; no user or build-machine paths in any file. Unzipped to a new folder and started minimized: the window opened (title AnimePortable), `data/animeportable.db` was created, and the app closed normally (exit code 0).

Owner check 2026-10-24: "Owner check of v1.0.0: downloaded the release ZIP, unzipped, ran it and used it for a normal session; everything works." (TypeSafe owner-report HUMAN_PASS 0.82, no new issue). Loop 38 PASS.
