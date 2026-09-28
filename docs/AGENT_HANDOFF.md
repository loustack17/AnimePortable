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
  observed_head: df2c6b45c838c8cfb23cd24de209700c7050587b
  working_tree: "Tracked files were clean before this handoff edit. Preserve unrelated untracked research files; no reset/clean."
active_loop:
  number: 28
  state: "Windows FLTK accepted visually; release runtime, exact CI, portable and matched-resource gates open. No Loop 29."
  goal: "Windows-only FLTK/libmpv portable product, lawful bundled runtime, and matched whole-product resource use below OneAnime."
  criteria: "ADR-023 Windows replacement gates 1–5; docs/13 isolated verification; docs/06 QUAL/security; docs/LOOP28_M4_CONTRACT historical Fyne diagnosis."
decisions:
  - "Windows first; Linux then macOS later. Remove active Fyne/Wails/Svelte/NSIS/external-MPV paths; main holds Windows FLTK development snapshot."
  - "No third-party repository clone/vendor; test DLL is external. No further 15-minute idle test; owner approved five-minute comparison only."
  - "Do not merge/release without relevant user authorization. Owner now accepts GPL if needed for a simpler legally distributable Windows product; do not change license merely to bypass corresponding-source duties."
  - "Owner directed the shortest, simplest maintainable Loop-28 route: keep the working in-process libmpv integration and stop custom libmpv compilation experiments. No further build-probe runs."
  - "Owner authorized scoped ci.yml work and temporary Loop-28 branch commit/push for isolated CI. Main integration and branch removal are authorized after Loop-28 gates pass; release publication is not authorized. Unused manual libmpv build probe and script were removed after owner stopped custom builds."
  - "Owner 2026-09-28 directed one temporary development branch only. After all Loop-28 gates pass and before closing the loop, integrate to main, inspect local/remote branch inventory, remove all other branches, and leave only main. Do not prune branches before acceptance."
current_slice:
  - "main remains 36a7391 Windows FLTK development snapshot. Temporary branch df2c6b4 removed the abandoned manual libmpv build probe; exact Windows CI run 36367707894 passed."
  - "Local fixes: spatial keyboard focus, preserved FLTK scroll origin, deferred focus to avoid Right double-hop, guarded Enter, Stop resets playback in player, timeline, loading cue, episode cursor/cancel/retry, generation-gated failures."
  - "Owner 2026-09-27 visually confirmed final Home Right focus after switching pages has no jump, and player switching 7→2/5/6/8 has no failure/reversion in that run. Earlier owner confirmed Stop stays at 00:00 and replay works. Intermittent episode failures previously occurred; one clean run is not repeated reliability evidence."
  - "Current pin is OneAnime 1.4.7 DLL SHA256 24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a from Predidit/libmpv-win32-video-cmake 20260811. Exact archive has no complete build-specific BOM/notices/source; test only, do not distribute."
  - "Own files remain MPL for now. Owner authorized GPL distribution if required; no license change has yet been made, and it would not remove source/notice duties. ADR-023 records provenance research."
  - "Explicit replacement path now accepts an absolute DLL path plus SHA-256 through two environment variables and preserves default pin and loader checks; independent read-only review found no MUST_FIX. Actual replacement runtime still needs verification."
  - "Current Predidit, zhongfly and official mpv development DLLs lack complete matching source/notices in their binary archives. A source-bundled third-party runtime was screened but uses AGPL and a 101 MiB DLL, outside the currently authorized GPL route; it was discarded. No product pin change."
  - "New packaging gate requires schema-1 libmpv provenance JSON + source ZIP, binds DLL/source hashes and checks listed members. New Go notice generator includes linked modules' root/nested license texts; Windows package script requires both provenance inputs. Independent package review fixed corrupt-entry and test-isolation defects, then found no remaining MUST_FIX."
  - "Historical manual build-probe runs 36360005977–36364632847 never produced a DLL. The probe job and tools/libmpv-build/build.sh are removed; Windows CI remains unchanged. Exact linked-component license/source closure remains open."
  - "FLTK release-1.4.5 tag resolves to a9b1113516ffd15fc7602a6d425a317df30f4720; bundled IJG JPEG 9f, libpng 1.6.44 and zlib 1.3.1 identified. Official source tar SHA 7715e69c...f593ea and pinned go-fltk patch SHA 44688325...f3a4f6 verified. Local package script pins both, Go packager requires/includes both under sources/; focused tests and independent review passed, real package/notice audit pending."
  - "Exact 949a8ed run 36364632847 passed Windows CI and compiled all mpv C objects but link failed on missing D3D11 helper symbols. Owner then stopped this custom build route; the later uncommitted workaround was discarded. Existing OneAnime-derived DLL remains the working test runtime."
  - "User requested repo cleanup. Verified generated Go/Fyne/Gio caches and old test binaries/fixtures, then removed only those after safety checks. Repo shrank from about 11.7 GiB to 0.10 GiB; .slim is 19.6 MiB. Research notes, scripts, screenshots and unrelated untracked files remain."
verification:
  passed:
    - "Exact df2c6b4 Windows CI run 36367707894 passed formatting, module verify, full/repeated/race Go tests, Anime1 acceptance, vet, govulncheck, dependency closure, build and bare-EXE startup. Probe removal received independent verifier/security review."
    - "Owner final keyboard/episode check passed; host screenshots showed stable Browse geometry and one Right focus after page switch. Host real player smoke played/stopped/exited, but host diagnostics are not isolated PASS."
    - "Five-minute minimized diagnostic: AnimePortable 6.26 MiB private resident/107.18 commit; OneAnime 98.32/309.48; data, startup and profile differed, so not matched PASS."
  pending:
    - "Repeat episode-switch reliability under comparable real source conditions; final native extracted-ZIP first-run/import/move/reopen and clean-host runtime/dependency checks."
    - "Use the existing working DLL path; establish actual redistribution license, corresponding sources/notices and clean-host ZIP evidence without building a custom DLL. GPL option is authorized if required, but source obligations remain. FLTK build-match and image-library notices also remain."
    - "Matched same-content/home/playback/cleanup AnimePortable–OneAnime resource comparison with private resident/commit, CPU/GPU and repeatability; final release-state isolated CI."
next_actions:
  - "Keep product code unchanged unless a concrete acceptance defect appears. Pursue a minimal legal package using the already working DLL; do not rerun custom build probe. Current Predidit build recipe has many linked dependencies, so merely changing AnimePortable to GPL does not complete corresponding-source duties. Avoid third-party clone/vendor in Git."
  - "Prepare matched resource and extracted ZIP checks only after a distributable runtime is identified; do not infer PASS from unmatched host samples."
  - "At next recoverability boundary update this file; do not mark Loop 28 PASS or start Loop 29 until all ADR-023 gates and exact-state CI close."
references:
  - "docs/README.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; docs/10_DURABLE_AGENT_STATE.md; THIRD_PARTY_NOTICES.md"
```
