<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
updated_at_utc: "2026-09-27"
status: IN_PROGRESS
repository:
  branch: codex/loop28-release-probe-20260927
  observed_head: 3db8a534595158d20c70fa9e4e054953e9829a30
  working_tree: "Scoped Loop-28 probe commits pushed; plain-GL build adjustment and FLTK notice research pending commit. Preserve all untracked .slim/.codex/experiments and unrelated files; no reset/clean."
active_loop:
  number: 28
  state: "Windows FLTK accepted visually; release runtime, exact CI, portable and matched-resource gates open. No Loop 29."
  goal: "Windows-only FLTK/libmpv portable product with MPL-2.0 own source and matched whole-product resource use below OneAnime."
  criteria: "ADR-023 Windows replacement gates 1–5; docs/13 isolated verification; docs/06 QUAL/security; docs/LOOP28_M4_CONTRACT historical Fyne diagnosis."
decisions:
  - "Windows first; Linux then macOS later. Remove active Fyne/Wails/Svelte/NSIS/external-MPV paths; main holds Windows FLTK development snapshot."
  - "No third-party repository clone/vendor; test DLL is external. No further 15-minute idle test; owner approved five-minute comparison only."
  - "Do not merge/release without relevant user authorization. Own source stays MPL-2.0."
  - "Owner authorized local ci.yml source-build/license-audit work and a scoped temporary Loop-28 branch commit/push for isolated CI; no merge/release authorization. Current manual-dispatch build probe is diagnostic only, not release evidence."
current_slice:
  - "Scoped temporary branch commit fc27271 passed exact Windows CI run 36357946493, including Go/race/vet/vuln/build/bare-EXE smoke; main remains 36a7391. Manual run 36357998999 passed Windows but libmpv probe stopped before build because the former pinned GHCR image digest returned manifest unknown."
  - "Local fixes: spatial keyboard focus, preserved FLTK scroll origin, deferred focus to avoid Right double-hop, guarded Enter, Stop resets playback in player, timeline, loading cue, episode cursor/cancel/retry, generation-gated failures."
  - "Owner 2026-09-27 visually confirmed final Home Right focus after switching pages has no jump, and player switching 7→2/5/6/8 has no failure/reversion in that run. Earlier owner confirmed Stop stays at 00:00 and replay works. Intermittent episode failures previously occurred; one clean run is not repeated reliability evidence."
  - "Current pin is OneAnime 1.4.7 DLL SHA256 24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a from Predidit/libmpv-win32-video-cmake 20260811. Exact archive has no complete build-specific BOM/notices/source; test only, do not distribute."
  - "License audit: own MPL preserved. THIRD_PARTY_NOTICES now lists Windows-linked Go modules and full pinned go-fltk MIT text; remaining full/nested Go notices unresolved. Pinned go-fltk build recipe uses FLTK 1.4.5 plus Windows patch and bundled JPEG/PNG/zlib static archives. Modified FLTK source and image library version/notice obligations need final distribution review. ADR-023 records mpv/FFmpeg uncertainty."
  - "ADR-023 release research: oneAnime/mpv.net choose GPL; media-kit MIT wrapper separately fetches libmpv; Mozilla says MPL app may link LGPL library without relicensing its own files. Preferred verified LGPL build; GPL combined distribution is an owner-choice fallback."
  - "Explicit replacement path now accepts an absolute DLL path plus SHA-256 through two environment variables and preserves default pin and loader checks; independent read-only review found no MUST_FIX. Actual replacement runtime still needs verification."
  - "Predidit, zhongfly and mpv development DLL candidates lack complete matching source/notices. ADR-023 records exact hashes and evidence; none may be published from current evidence. No product pin change."
  - "New packaging gate requires schema-1 libmpv provenance JSON + source ZIP, binds DLL/source hashes and checks listed members. New Go notice generator includes linked modules' root/nested license texts; Windows package script requires both provenance inputs. Independent package review fixed corrupt-entry and test-isolation defects, then found no remaining MUST_FIX."
  - "Authorized ci.yml manual build probe uses fixed mpv/FFmpeg source archive hashes and pinned cross-build image, with container network disabled and no third-party clone. After GHCR removed the prior digest, a2e91dd changed it to sha256:15b4fa39e2f33a8c93842ea9a01c116eea1ecef5af9020c625c837dce3368fed, verified by registry HEAD and docker manifest inspect; independent workflow/security re-review found no MUST_FIX. This remains diagnostic and lacks linked-component source/license closure."
  - "Exact a2e91dd manual run 36360005977 passed Windows CI and compiled FFmpeg; mpv Meson setup failed because D3D11 was enabled while shaderc and spirv-cross were disabled. Official mpv fixed-commit meson.build requires both; 3db8a53 enabled both for the next probe."
  - "Exact 3db8a53 manual run 36361126878 passed Windows CI and found shaderc/spirv-cross, but mpv setup failed because GL was enabled with every GL output disabled. Product uses FLTK-provided OpenGL libmpv rendering, not mpv's D3D11 video output. Local build.sh now selects plain-gl, Windows WASAPI and D3D hwaccel while disabling D3D11 output/shaderc/spirv-cross; independent review and CI rerun pending."
  - "Official FLTK release-1.4.5 tag resolves to a9b1113516ffd15fc7602a6d425a317df30f4720; its bundled image sources identify IJG JPEG 9f, libpng 1.6.44 and zlib 1.3.1. THIRD_PARTY_NOTICES and ADR updated locally; source/notice package still open."
verification:
  passed:
    - "Exact branch commit fc27271 Windows CI run 36357946493 passed formatting, module verify, full/repeated/race Go tests, Anime1 acceptance, vet, govulncheck, dependency closure, Windows build and bare-EXE startup. Local full tests/vet/build and git diff --check also passed."
    - "Owner final keyboard/episode check passed; host screenshots showed stable Browse geometry and one Right focus after page switch. Host real player smoke played/stopped/exited, but host diagnostics are not isolated PASS."
    - "Five-minute minimized diagnostic: AnimePortable 6.26 MiB private resident/107.18 commit; OneAnime 98.32/309.48; data, startup and profile differed, so not matched PASS."
  pending:
    - "Repeat episode-switch reliability under comparable real source conditions; final native extracted-ZIP first-run/import/move/reopen and clean-host runtime/dependency checks."
    - "Successful libmpv probe compilation after Meson option fix, then exact DLL/component license/BOM/source/notice evidence plus FLTK patched-source and image-library source/notice inventory; do not publish current DLL/ZIP."
    - "Matched same-content/home/playback/cleanup AnimePortable–OneAnime resource comparison with private resident/commit, CPU/GPU and repeatability; final release-state isolated CI."
next_actions:
  - "Review plain-GL option correction, commit/push only Loop-28 build script, notice/ADR and this handoff to the authorized temporary branch, then rerun manual probe on exact commit. Diagnose only new failure fingerprints. Finish LGPL libmpv linked-component closure and source/notice bundle without any third-party clone/vendor; local Docker daemon is absent."
  - "Prepare matched resource and extracted ZIP checks only after a distributable runtime is identified; do not infer PASS from unmatched host samples."
  - "At next recoverability boundary update this file; do not mark Loop 28 PASS or start Loop 29 until all ADR-023 gates and exact-state CI close."
references:
  - "docs/README.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; docs/10_DURABLE_AGENT_STATE.md; THIRD_PARTY_NOTICES.md"
```
