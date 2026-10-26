<!-- SPDX-License-Identifier: MPL-2.0 -->

# Pre-release v0.1.0-alpha.1

Status: PASS. Published 2026-10-15 (owner approved: "如果適合就發吧", after agreeing it is an alpha, not a beta). https://github.com/loustack17/AnimePortable/releases/tag/v0.1.0-alpha.1

## Goal

Publish the current Windows app as a GitHub pre-release so the core flow (search, detail, play, continue watching, following, 近期更新) can be tried from a downloaded ZIP.

## Scope

- A `release` job in `.github/workflows/ci.yml` that runs only for `v*` tags, after the Windows job passes. It reuses that job's executable, downloads the upstream libmpv archive named in `docs/LIBMPV_RUNTIME_NOTICE.txt` (archive SHA-256 pinned), checks the DLL against `internal/runtimepin`, packages the ZIP with `apps/desktop/build/windows/package.ps1`, and creates a GitHub release marked pre-release with `docs/releases/v0.1.0-alpha.1.md` as notes.
- Out of scope: the Loop 38 security and resource passes, code signing, installers, new features.

## Checks

- Local dry run of the same packaging: ZIP contains `animeportable.exe`, `LICENSE`, `THIRD_PARTY_NOTICES.md`, `libmpv-2.dll` (pinned hash) and `licenses/libmpv.txt`; the unzipped app starts and creates `data/`.
- CI for the tagged commit is green and the release job uploads the ZIP; the downloaded ZIP's DLL hash matches the pin.

## Owner check

Download the ZIP from the GitHub release page, unzip it to a new folder, run `animeportable.exe`, search for an anime and play an episode.

## Result

- Release job added in 399b5e0 (CI 38682949488 green on main, release skipped). Tag v0.1.0-alpha.1 on 399b5e0: CI 38683446055 green, windows and release jobs passed.
- Published asset `AnimePortable-v0.1.0-alpha.1-windows-amd64.zip` (SHA-256 `5AB9B70F…B4DC`), marked pre-release. Downloaded and checked: 5 files as listed above, `libmpv-2.dll` SHA-256 equals the pin, the unzipped app starts and creates `data/`.
- Release title changed to the version alone (owner request); the release job now uses the tag as the title.
- Owner check 2026-10-15: PASS. The downloaded ZIP runs; search and playback work.