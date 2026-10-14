<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable

[繁體中文](README.md) | English

A lightweight, keyboard-first Windows anime player inspired by [OneAnime](https://github.com/Predidit/oneAnime). Find an anime, pick an episode, watch it in the built-in player, and resume later. Unzip and run: no installer, no account, no telemetry.

## What it does

- Browse and search the Anime1 catalog.
- Anime detail page with cover and synopsis from Bangumi (AniList as a cross-check), shown in Traditional Chinese.
- Episodes in order, with watch progress on each episode.
- Built-in player (libmpv) with auto-hiding controls, episode switching and fullscreen.
- Continue Watching on Home, resuming where you stopped.
- Mouse and keyboard both work.

Following, history, schedule, settings and autoplay are in progress; see the [roadmap](docs/04_ROADMAP.md).

## Run it

Unzip the portable ZIP and run `AnimePortable.exe`. The ZIP includes `libmpv-2.dll`; the app checks its SHA-256 before loading it. Your data (history, progress, cached metadata) lives in the `data/` folder next to the executable, so the whole folder can be moved or copied.

Windows only for now. Linux and macOS come after the Windows release.

## Build from source

Requirements: Go (version in `go.mod`), cgo, and an msvcrt MinGW gcc on `PATH` (MSYS2 MINGW64 or nuwen MinGW; a UCRT MinGW cannot link the FLTK libraries).

```
CGO_ENABLED=1 go build -trimpath -o AnimePortable.exe ./apps/desktop
CGO_ENABLED=1 go test ./...
```

Put the pinned `libmpv-2.dll` (see [docs/LIBMPV_RUNTIME_NOTICE.txt](docs/LIBMPV_RUNTIME_NOTICE.txt)) next to the executable to play video.

## Project status

In development; there is no public release yet. Browsing, the detail page and playback work. Before the first Windows release come: following, history, schedule, full keyboard navigation, offline startup from saved data, autoplay of the next episode, settings, and a security and release pass.

- Detailed state: [docs/STATUS.md](docs/STATUS.md)
- Engineering docs: [docs/README.md](docs/README.md)
- Contributors and AI agents: [AGENTS.md](AGENTS.md)

## License

Mozilla Public License 2.0. See `LICENSE`, and `THIRD_PARTY_NOTICES.md` for dependencies, the bundled libmpv DLL and remote-service content.
