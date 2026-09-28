<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
updated_at_utc: "2026-09-28"
status: IN_PROGRESS
repository:
  branch: codex/loop28-release-probe-20260927
  observed_head: 12d1adb
  working_tree: "Community package implementation and docs are tracked-modified; docs/LIBMPV_RUNTIME_NOTICE.txt is new. Preserve unrelated untracked .codex, .ignore, .slim, docs/CODEX_*, experiments."
active_loop:
  number: 28
  state: "Windows FLTK integration and prior owner UI checks passed; the official community-mode Windows ZIP has passed extracted-folder startup and player-window lifecycle checks. Final exact-state review/CI and repository integration remain."
  goal: "Windows portable FLTK/libmpv product using the existing Go core and the OneAnime-identical libmpv runtime."
  criteria: "ADR-023 and docs/06 as amended 2026-09-28: prior resource/interaction evidence accepted for this loop; official Windows ZIP and transparent community provenance notice required."
decisions:
  - "Windows first; Linux and macOS deferred. Keep FLTK and the same OneAnime 1.4.7 DLL; no more custom libmpv builds or alternative-runtime screens unless needed."
  - "Owner permits GPL or AGPL if necessary, but changing AnimePortable's license does not itself establish the linked DLL's terms or provide corresponding sources. Own files remain MPL."
  - "Owner says unresolved source-build metadata alone should not block Loop 28 if no actual legal/security issue is identified; record the uncertainty accurately. Do not claim verified license compliance or fabricate unrun tests."
  - "Owner waived new resource comparison and other repeat tests, allowing the earlier checks to stand; only Windows ZIP testing is requested now. No long-idle rerun."
  - "Owner clarified AnimePortable is an individual community open-source project and selected pinned, tested DLL plus upstream provenance links and explicit unverified disclosure as the Windows Loop 28 package gate. This is not a legal-compliance certification. Source files remain MPL-2.0."
  - "Prior authorization permits temporary-branch commit/push and main integration plus removal of other branches after Loop 28 gates pass. No release publication authorization. No workflow edit without specific scope and independent review."
evidence:
  - "Exact Windows CI run 36368230376 at 6a46f53 passed tests, race, vet, govulncheck, build and bare startup. Subsequent commits through 12d1adb were documentation only."
  - "Owner previously accepted Windows Home focus, episode switching, Stop/replay and visual direction. Three-cycle same-DLL host diagnostic played and cleaned up without orphan; short host samples are not a matched resource comparison."
  - "Pinned DLL SHA256 24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a matches D:/Download/Tools/oneAnime_windows_1.4.7/libmpv-2.dll and Predidit 20260811 release. Builder pins mpv but fetches a moving FFmpeg branch; exact static-component source inventory is not established. No known legal violation or security defect was found, but distribution compliance is unverified. Independent reviewer rejected an earlier attempt to declare the DLL release-verified without that evidence; bypass code was reverted."
  - "A current production-tag EXE and the pinned DLL were placed in .slim/loop28-zip-test/animeportable-windows-amd64-diagnostic.zip. After extraction, hash check, fresh-folder window startup and clean close passed. With a copy of the existing Loop 23 acceptance database added under extracted/data, the extracted app opened a player window, returned from player close, and exited 0. The first playback automation selected the wrong hidden Home window; a targeted window-title diagnostic identified this, and the corrected check passed. Actual decoded video frames were not asserted in this ZIP test; prior player tests are the evidence for playback."
  - "The official apps/desktop/build/windows/package.ps1 community ZIP was produced at .slim/loop28-zip-test/animeportable-windows-amd64-community.zip, SHA256 2B13FFF1B286817B36948C4E8154C66EBF9CBDD7FA3DE0B0E754FF5AD1992D8C. It includes EXE, MPL LICENSE, generated Go module notices, the pinned DLL and docs/LIBMPV_RUNTIME_NOTICE.txt. Extraction confirmed the DLL hash; fresh extracted window startup/clean exit passed; after copying the Loop23 acceptance DB to data, player window opened, returned Home on close, and app exited 0. This ZIP test did not assert decoded video frames. The strict package mode and tests remain available."
  - "go test ./tools/portable-package passed with a repository-local Go cache; default host cache produced sandbox Access denied. git diff --check passed. Independent read-only code review found a notice validation/write race, fixed by pinning the notice digest in the archive writer; it also identified stale ADR text, now marked as historical."
  - "Owner supplied a licensing analysis: distribution form does not alter DLL obligations and matching sources need not be embedded in the ZIP. ADR-023 now distinguishes that legal delivery choice from the packager's stricter source-in-ZIP design. The analysis's go-mpv/mpv-1.dll and negative-strings examples do not apply or establish LGPL status for this product, which uses purego/libmpv-2.dll."
  - "A proposed media-kit/libmpv-builds repository returned 404. The actual media-kit/libmpv-win32-video-cmake 20241021 release has binary mpv archives but no separate corresponding-source/build-log asset; automatic GitHub Source code.zip covers the builder repository only. ADR-023 records this; no new runtime was selected or tested."
  - "Owner chose a fourth, community provenance disclosure acceptance for this individual project. docs/06 CI-010 and ADR-023 record the changed project gate; exact DLL source/license details remain explicitly unverified. Do not claim formal legal PASS or publish a release without authorization."
pending:
  - "Finish final review of exact diff and, if authorized prior temp-branch commit/push remains applicable, commit/push then check exact-head CI. Do not edit workflow without new explicit authorization."
  - "Close Loop 28 only after final-state checks and handoff evidence agree. Prior owner direction requests main integration and removal of extra branches after PASS; never publish a release without separate authorization."
references:
  - "docs/06_ACCEPTANCE_CRITERIA.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; docs/10_DURABLE_AGENT_STATE.md; tools/portable-package; apps/desktop/build/windows/package.ps1"
```
