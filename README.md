<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable

Windows development uses Go, FLTK and in-process libmpv. Loop 28 Windows acceptance is complete. Loop 29 Search has passed automated Windows checks and is awaiting human acceptance. Linux and macOS development is paused; release publication remains pending.

## Current test build

The local acceptance build is [artifacts/loop29/AnimePortable.exe](artifacts/loop29/AnimePortable.exe). Generated artifacts are not tracked; a fresh clone can download the executable from [the verified Windows CI run](https://github.com/loustack17/AnimePortable/actions/runs/37558539916). Follow [the test objects, actions and expected results](docs/LOOP29_HUMAN_CHECK.md).

See [the engineering status](docs/IMPLEMENTATION_STATUS.md) and [Windows desktop decision](docs/19_ADR_FLTK_WINDOWS_DESKTOP.md).

AnimePortable is licensed under the Mozilla Public License 2.0. See `LICENSE` and `THIRD_PARTY_NOTICES.md` for licensing scope, dependencies, generated files, and remote-service content boundaries.
