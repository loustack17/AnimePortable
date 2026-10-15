<!-- SPDX-License-Identifier: MPL-2.0 -->

# Pre-release v0.1.0-alpha.1

Status: IN PROGRESS (owner approved on 2026-10-15: "如果適合就發吧", after agreeing it is an alpha, not a beta).

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
