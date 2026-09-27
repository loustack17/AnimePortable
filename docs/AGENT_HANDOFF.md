<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
updated_at_utc: "2026-09-27"
status: NEEDS_HUMAN
repository:
  branch: codex/loop28-release-probe-20260927
  observed_head: 36a7391
  working_tree: "Dirty: Windows FLTK UI/playback/Anime1 fixes, runtime pin, ADR and notices. Preserve all untracked .slim/.codex/experiments and unrelated files; no reset/clean."
active_loop:
  number: 28
  state: "Windows FLTK accepted visually; release runtime, exact CI, portable and matched-resource gates open. No Loop 29."
  goal: "Windows-only FLTK/libmpv portable product with MPL-2.0 own source and matched whole-product resource use below OneAnime."
  criteria: "ADR-023 Windows replacement gates 1–5; docs/13 isolated verification; docs/06 QUAL/security; docs/LOOP28_M4_CONTRACT historical Fyne diagnosis."
decisions:
  - "Windows first; Linux then macOS later. Remove active Fyne/Wails/Svelte/NSIS/external-MPV paths; main holds Windows FLTK development snapshot."
  - "No third-party repository clone/vendor; test DLL is external. No further 15-minute idle test; owner approved five-minute comparison only."
  - "Do not commit/push/release or edit .github/workflows without relevant user authorization. Own source stays MPL-2.0."
  - "Owner authorized local ci.yml source-build/license-audit work and a scoped temporary Loop-28 branch commit/push for isolated CI; no merge/release authorization. Current manual-dispatch build probe is diagnostic only, not release evidence."
current_slice:
  - "main 36a7391 passed exact Windows CI runs 36282618538 (branch) and 36282892016 (main): Go/race/vet/vuln/build/bare-EXE smoke. Current dirty diff is newer and has no exact-state CI."
  - "Local fixes: spatial keyboard focus, preserved FLTK scroll origin, deferred focus to avoid Right double-hop, guarded Enter, Stop resets playback in player, timeline, loading cue, episode cursor/cancel/retry, generation-gated failures."
  - "Owner 2026-09-27 visually confirmed final Home Right focus after switching pages has no jump, and player switching 7→2/5/6/8 has no failure/reversion in that run. Earlier owner confirmed Stop stays at 00:00 and replay works. Intermittent episode failures previously occurred; one clean run is not repeated reliability evidence."
  - "Current pin is OneAnime 1.4.7 DLL SHA256 24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a from Predidit/libmpv-win32-video-cmake 20260811. Exact archive has no complete build-specific BOM/notices/source; test only, do not distribute."
  - "License audit: own MPL preserved. THIRD_PARTY_NOTICES now lists Windows-linked Go modules and full pinned go-fltk MIT text; remaining full/nested Go notices unresolved. Pinned go-fltk build recipe uses FLTK 1.4.5 plus Windows patch and bundled JPEG/PNG/zlib static archives. Modified FLTK source and image library version/notice obligations need final distribution review. ADR-023 records mpv/FFmpeg uncertainty."
  - "ADR-023 release research: oneAnime/mpv.net choose GPL; media-kit MIT wrapper separately fetches libmpv; Mozilla says MPL app may link LGPL library without relicensing its own files. Preferred verified LGPL build; GPL combined distribution is an owner-choice fallback."
  - "Explicit replacement path now accepts an absolute DLL path plus SHA-256 through two environment variables and preserves default pin and loader checks; independent read-only review found no MUST_FIX. Actual replacement runtime still needs verification."
  - "Predidit, zhongfly and mpv development DLL candidates lack complete matching source/notices. ADR-023 records exact hashes and evidence; none may be published from current evidence. No product pin change."
  - "New packaging gate requires schema-1 libmpv provenance JSON + source ZIP, binds DLL/source hashes and checks listed members. New Go notice generator includes linked modules' root/nested license texts; Windows package script requires both provenance inputs. Independent package review fixed corrupt-entry and test-isolation defects, then found no remaining MUST_FIX."
  - "Authorized ci.yml manual build probe uses fixed mpv/FFmpeg source archive hashes and pinned cross-build image, with container network disabled and no third-party clone; independent verifier/security review found no execution MUST_FIX. It has not run and lacks the image's linked-component source/license closure, so cannot be a release verifier."
verification:
  passed:
    - "After DLL override and notice edits, full go test -count=1 ./..., vet, Windows build and git diff --check passed locally on 2026-09-27. Focused override tests include malformed and missing settings. Independent production review found no MUST_FIX in override or prior UI/player fixes."
    - "Owner final keyboard/episode check passed; host screenshots showed stable Browse geometry and one Right focus after page switch. Host real player smoke played/stopped/exited, but host diagnostics are not isolated PASS."
    - "Five-minute minimized diagnostic: AnimePortable 6.26 MiB private resident/107.18 commit; OneAnime 98.32/309.48; data, startup and profile differed, so not matched PASS."
  pending:
    - "Repeat episode-switch reliability under comparable real source conditions; final native extracted-ZIP first-run/import/move/reopen and clean-host runtime/dependency checks."
    - "Exact DLL/component license/BOM/source/notice evidence plus FLTK patched-source and image-library source/notice inventory; do not publish current DLL/ZIP."
    - "Matched same-content/home/playback/cleanup AnimePortable–OneAnime resource comparison with private resident/commit, CPU/GPU, repeatability; final dirty-state isolated CI requires approved commit/push."
next_actions:
  - "Push only the scoped Loop-28 change set to a temporary branch, run the isolated build probe and exact-state Windows CI, classify failures. Then finish exact LGPL libmpv linked-component closure and source/notice bundle without any third-party clone/vendor; local Docker daemon is absent."
  - "Prepare matched resource and extracted ZIP checks only after a distributable runtime is identified; do not infer PASS from unmatched host samples."
  - "At next recoverability boundary update this file; do not mark Loop 28 PASS or start Loop 29 until all ADR-023 gates and exact-state CI close."
references:
  - "docs/README.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; docs/10_DURABLE_AGENT_STATE.md; THIRD_PARTY_NOTICES.md"
```
