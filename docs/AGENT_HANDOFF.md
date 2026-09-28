<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

```yaml
schema_version: 1
protocol_version: v2.4.1
instruction_version: v2.4.1
updated_at_utc: "2026-09-28"
status: PASS
repository:
  branch: main
  verified_code_head: 54c5cc7
  working_tree: "Only this final handoff update is tracked-modified; preserve unrelated untracked .codex, .ignore, .slim, docs/CODEX_*, experiments."
active_loop:
  number: 28
  state: "Windows-only Loop 28 passed under the owner's 2026-09-28 community package and prior-resource-evidence acceptance. Loop 29 may begin on Windows; Linux and macOS remain deferred."
  goal: "Windows portable FLTK/libmpv desktop with existing Go core."
  criteria: "ADR-023; docs/06 Windows phase gate and CI-010 as amended 2026-09-28."
decisions:
  - "Owner accepted prior Windows resource and interaction evidence without another comparison or long-idle run. PERF-008 remains a future formal matched-comparison objective, not an asserted matched PASS."
  - "Owner selected the tested OneAnime-identical DLL with SHA-256 pin, upstream links and explicit unverified source/license disclosure for this individual open-source project's Windows package gate. This is not a legal-compliance certification. AnimePortable source stays MPL-2.0."
  - "Do not build custom libmpv, switch player libraries, resume Linux/macOS, edit workflows or publish a release without new scope/authorization."
evidence:
  - "Exact production commit 54c5cc7 passed Windows CI run 36399950933: Go tests, repeated lifecycle/player checks, Anime1 acceptance, race, vet, govulncheck, build and bare startup."
  - "Official package.ps1 ZIP SHA256 2B13FFF1B286817B36948C4E8154C66EBF9CBDD7FA3DE0B0E754FF5AD1992D8C had EXE, MPL LICENSE, generated third-party notices, pinned libmpv DLL and runtime notice. Extracted DLL SHA256 24E848F59C047C9442501FDBE619AD39B98BE7D4DD402691F79931C852C0070A. Fresh extracted startup/clean exit passed; with Loop23 acceptance data, player window opened, returned Home on close and exited 0. This ZIP test did not assert decoded frames; prior playback tests cover that."
  - "go test ./tools/portable-package and git diff --check passed. Independent read-only review found one notice validation/write race; fixed by hashing validated notice during ZIP creation. Final review found no material package-code or disclosure defect."
  - "Previous owner-visible Home, focus, episode switching, Stop/replay and visual checks passed. Earlier same-DLL playback cycles cleaned up without orphan. No new matched resource comparison was run."
  - "main fast-forwarded from 36a7391 to 54c5cc7 and pushed. Same-head main CI run 36400374366 passed. The completed temporary branch was deleted locally and remotely; only main remains. Session-created .slim/loop28-zip-test and .slim/go-cache were removed after evidence was recorded."
pending:
  - "Commit/push this handoff checkpoint. Begin Loop 29 Windows feature work only under its own defined contract. Do not publish a GitHub Release without separate owner authorization."
references:
  - "docs/06_ACCEPTANCE_CRITERIA.md; docs/19_ADR_FLTK_WINDOWS_DESKTOP.md; docs/13_VERIFICATION_EXECUTION_ENVIRONMENTS.md; tools/portable-package; apps/desktop/build/windows/package.ps1"
```
