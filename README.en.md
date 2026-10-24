<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable

[繁體中文](README.md) | English

A lightweight, keyboard-first Windows anime player inspired by [OneAnime](https://github.com/Predidit/oneAnime). Find an anime, pick an episode, watch it in the built-in player, and resume later. Unzip and run: no installer, no account, no telemetry.

## What it does

- Browse and search the Anime1 catalog.
- Anime detail page with cover and synopsis from Bangumi (AniList as a cross-check), shown in Traditional Chinese; the episode list stored last time shows at once and is refreshed from Anime1.
- Episodes in order, with watch progress on each episode.
- Built-in player (libmpv) with auto-hiding controls, episode switching, fullscreen and playback speed, autoplay of the next episode and a one-press skip of the opening (AniSkip).
- Recent updates and Continue Watching on Home, resuming where you stopped.
- Following, watch history, and a schedule of the next seven days.
- Settings: theme, language (Traditional Chinese, English, Japanese), playback, proxy, data folder and update check.
- Mouse and keyboard both work; without a network the app still opens with its stored data.

## Run it

Download the ZIP from [Releases](https://github.com/loustack17/AnimePortable/releases), unzip it and run `animeportable.exe`. The ZIP includes `libmpv-2.dll`; the app checks its SHA-256 before loading it. Your data (following, history and progress, cached metadata, settings) lives in the `data/` folder next to the executable, so the whole folder can be moved or copied.

Windows 64-bit only for now. Linux and macOS come later.

## Build from source

Requirements: Go (version in `go.mod`), cgo, and an msvcrt MinGW gcc on `PATH` (MSYS2 MINGW64 or nuwen MinGW; a UCRT MinGW cannot link the FLTK libraries).

```
CGO_ENABLED=1 go build -trimpath -ldflags "-H windowsgui" -o animeportable.exe ./apps/desktop
CGO_ENABLED=1 go test ./...
```

Put the pinned `libmpv-2.dll` (see [docs/LIBMPV_RUNTIME_NOTICE.txt](docs/LIBMPV_RUNTIME_NOTICE.txt)) next to the executable to play video.

## Project status

v1.0.0 is the first full release, after a security and a resource pass. What comes next (version 2.0) is in the [roadmap](docs/04_ROADMAP.md).

- Detailed state: [docs/STATUS.md](docs/STATUS.md)
- Engineering docs: [docs/README.md](docs/README.md)
- Developers: [AGENTS.md](AGENTS.md)

## License

Mozilla Public License 2.0. See `LICENSE`, and `THIRD_PARTY_NOTICES.md` for dependencies, the bundled libmpv DLL and remote-service content.
