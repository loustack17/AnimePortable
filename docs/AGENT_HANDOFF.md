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
  observed_head: 4fd72f8121cc303c920dc39ffc610f2c585556b3
  working_tree: "Reviewed DXGI probe compatibility fix and ADR evidence pending commit. Preserve all untracked .slim/.codex/experiments and unrelated files; no reset/clean."
active_loop:
  number: 28
  state: "Windows FLTK accepted visually; release runtime, exact CI, portable and matched-resource gates open. No Loop 29."
  goal: "Windows-only FLTK/libmpv portable product with MPL-2.0 own source and matched whole-product resource use below OneAnime."
  criteria: "ADR-023 Windows replacement gates 1–5; docs/13 isolated verification; docs/06 QUAL/security; docs/LOOP28_M4_CONTRACT historical Fyne diagnosis."
decisions:
  - "Windows first; Linux then macOS later. Remove active Fyne/Wails/Svelte/NSIS/external-MPV paths; main holds Windows FLTK development snapshot."
  - "No third-party repository clone/vendor; test DLL is external. No further 15-minute idle test; owner approved five-minute comparison only."
  - "Do not merge/release without relevant user authorization. Own source stays MPL-2.0."
  - "Owner authorized scoped ci.yml source-build/license-audit work and temporary Loop-28 branch commit/push for isolated CI. Main integration and branch removal are authorized after Loop-28 gates pass; release publication is not authorized. Current manual probe is diagnostic only."
  - "Owner 2026-09-28 directed one temporary development branch only. After all Loop-28 gates pass and before closing the loop, integrate to main, inspect local/remote branch inventory, remove all other branches, and leave only main. Do not prune branches before acceptance."
current_slice:
  - "main remains 36a7391 Windows FLTK development snapshot. Scoped temporary branch commits through c09b727 passed exact Windows CI on runs 36357946493, 36360005977, 36361126878 and 36362293745; manual libmpv probe remains diagnostic."
  - "Local fixes: spatial keyboard focus, preserved FLTK scroll origin, deferred focus to avoid Right double-hop, guarded Enter, Stop resets playback in player, timeline, loading cue, episode cursor/cancel/retry, generation-gated failures."
  - "Owner 2026-09-27 visually confirmed final Home Right focus after switching pages has no jump, and player switching 7→2/5/6/8 has no failure/reversion in that run. Earlier owner confirmed Stop stays at 00:00 and replay works. Intermittent episode failures previously occurred; one clean run is not repeated reliability evidence."
  - "Current pin is OneAnime 1.4.7 DLL SHA256 24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a from Predidit/libmpv-win32-video-cmake 20260811. Exact archive has no complete build-specific BOM/notices/source; test only, do not distribute."
  - "Own MPL preserved; preferred verified LGPL libmpv. GPL combined distribution is an owner-choice fallback. ADR-023 records provenance and license research."
  - "Explicit replacement path now accepts an absolute DLL path plus SHA-256 through two environment variables and preserves default pin and loader checks; independent read-only review found no MUST_FIX. Actual replacement runtime still needs verification."
  - "Predidit, zhongfly and mpv development DLL candidates lack complete matching source/notices. ADR-023 records exact hashes and evidence; none may be published from current evidence. No product pin change."
  - "New packaging gate requires schema-1 libmpv provenance JSON + source ZIP, binds DLL/source hashes and checks listed members. New Go notice generator includes linked modules' root/nested license texts; Windows package script requires both provenance inputs. Independent package review fixed corrupt-entry and test-isolation defects, then found no remaining MUST_FIX."
  - "Manual CI probe uses hash-pinned mpv/FFmpeg archives and reachable digest-pinned LGPL-labeled BtbN image, container network disabled, no clone. Runs 36360005977/36361126878 diagnosed D3D11 shader dependencies and missing GL output. c09b727 selects plain-gl/WASAPI/D3D hwaccel for the actual libmpv render path. Run 36362293745 compiled 236/250 then failed because auto-features disabled native win32 threads, causing timer-win32.c/threads-posix.h redefinition; local build.sh now enables win32-threads. Exact linked-component license/source closure remains open."
  - "FLTK release-1.4.5 tag resolves to a9b1113516ffd15fc7602a6d425a317df30f4720; bundled IJG JPEG 9f, libpng 1.6.44 and zlib 1.3.1 identified. Official source tar SHA 7715e69c...f593ea and pinned go-fltk patch SHA 44688325...f3a4f6 verified. Local package script pins both, Go packager requires/includes both under sources/; focused tests and independent review passed, real package/notice audit pending."
  - "Exact 4fd72f8 run 36363610980 passed Windows CI and compiled mpv 248/249; vf_d3d11vpp.c then failed because Meson omitted HAVE_DXGI_DEBUG_D3D11 when D3D hardware acceleration was enabled without D3D11 video output. Pinned MinGW header defines the GUID. Local build.sh now checks the header symbol with the cross compiler and sets the C define; independent review and rerun pending."
verification:
  passed:
    - "Latest 4fd72f8 Windows CI run 36363610980 passed formatting, module verify, full/repeated/race Go tests, Anime1 acceptance, vet, govulncheck, dependency closure, build and bare-EXE startup. FLTK source package changes passed focused Go test, PowerShell syntax/mismatch checks and independent review."
    - "Owner final keyboard/episode check passed; host screenshots showed stable Browse geometry and one Right focus after page switch. Host real player smoke played/stopped/exited, but host diagnostics are not isolated PASS."
    - "Five-minute minimized diagnostic: AnimePortable 6.26 MiB private resident/107.18 commit; OneAnime 98.32/309.48; data, startup and profile differed, so not matched PASS."
  pending:
    - "Repeat episode-switch reliability under comparable real source conditions; final native extracted-ZIP first-run/import/move/reopen and clean-host runtime/dependency checks."
    - "Successful libmpv probe compilation after DXGI fix, then exact DLL/component license/BOM/source/notice evidence plus FLTK build-match and image-library notice audit; do not publish current DLL/ZIP."
    - "Matched same-content/home/playback/cleanup AnimePortable–OneAnime resource comparison with private resident/commit, CPU/GPU and repeatability; final release-state isolated CI."
next_actions:
  - "After independent DXGI flag review, commit/push only scoped Loop-28 build/docs changes to authorized temporary branch and rerun exact-state CI/probe. Diagnose only new failure fingerprints. Finish LGPL libmpv linked-component closure and source/notice bundle without clone/vendor; local Docker daemon is absent."
  - "Prepare matched resource and extracted ZIP checks only after a distributable runtime is identified; do not infer PASS from unmatched host samples."
  - "At next recoverability boundary update this file; do not mark Loop 28 PASS or start Loop 29 until all ADR-023 gates and exact-state CI close."
references:
  - "docs/README.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; docs/10_DURABLE_AGENT_STATE.md; THIRD_PARTY_NOTICES.md"
```
