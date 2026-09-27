<!-- SPDX-License-Identifier: MPL-2.0 -->

# Third-Party Notices

AnimePortable source code, documentation, and build configuration are licensed under the Mozilla Public License 2.0. This file records the third-party components used by the native application. Exact versions are pinned in `go.mod` and `go.sum`.

## Go runtime dependencies

| Component | License |
| --- | --- |
| [go-fltk](https://github.com/pwiecz/go-fltk) | MIT; Windows FLTK path |
| [FLTK 1.4.5](https://github.com/fltk/fltk/releases/tag/release-1.4.5) | GNU Library General Public License with FLTK exceptions; statically linked by go-fltk with its [Windows patch](https://github.com/pwiecz/go-fltk/blob/3e944122e7b1/lib/fltk-1.4.patch) |
| [purego](https://github.com/ebitengine/purego) | Apache-2.0; Windows libmpv loader |
| [go-humanize](https://github.com/dustin/go-humanize) | MIT; linked through SQLite |
| [go-isatty](https://github.com/mattn/go-isatty) | MIT; linked through SQLite |
| [go-strftime](https://github.com/ncruces/go-strftime) | MIT; linked through SQLite |
| [bigfft](https://github.com/remyoudompheng/bigfft) | BSD-3-Clause; linked through SQLite |
| [golang.org/x/net](https://cs.opensource.google/go/x/net) | BSD-3-Clause |
| [golang.org/x/sys](https://cs.opensource.google/go/x/sys) | BSD-3-Clause |
| [golang.org/x/text](https://cs.opensource.google/go/x/text) | BSD-3-Clause |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause; SQLite notices are included upstream |
| [modernc.org/libc](https://pkg.go.dev/modernc.org/libc) | BSD-3-Clause; module also carries third-party notices |
| [modernc.org/mathutil](https://pkg.go.dev/modernc.org/mathutil) | BSD-3-Clause |
| [modernc.org/memory](https://pkg.go.dev/modernc.org/memory) | BSD-3-Clause; module also carries upstream notices |

AnimePortable is based in part on the work of the FLTK project (https://www.fltk.org).

### go-fltk MIT notice

Copyright (c) 2021 Piotr Wieczorek (p@wie.cz)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## Remote services and content

The project license applies only to AnimePortable material that the project can license. It does not grant rights to Anime1, AniList, or Bangumi names, trademarks, logos, remote API responses, metadata, covers, subtitles, video files, or other provider and rights-holder content. Use of those services and content remains subject to their own terms and applicable law.

## Reviewed upstream projects

[oneAnime](https://github.com/Predidit/oneAnime) is a separate GPL-3.0 project that acknowledges code from [AnimeOne](https://github.com/HQAnime/AnimeOne), whose repository is MIT-licensed. AnimePortable does not copy or distribute source code from either project; its Anime1 adapter is an independent Go implementation of the public service protocol.

## Audit scope

AnimePortable's own files remain MPL-2.0; the licenses above continue to govern third-party code. The project does not vendor third-party source code. The proposed portable archive bundles a pinned libmpv DLL and its build-specific license notice under `licenses/libmpv.txt`.

This inventory is not yet a complete distributable notice set. The go-fltk MIT text above matches the pinned module's `LICENSE`; other linked Go module license texts and nested notices still need to accompany the ZIP. Its local build recipe selects FLTK 1.4.5, applies a Windows patch to `src/Fl_win32.cxx`, and builds with bundled JPEG, PNG and zlib rather than system libraries. The pinned module includes that patch and FLTK's license, but its prebuilt Windows archives do not include a build-specific source manifest for the bundled image libraries. Confirm their versions and notices and make the modified FLTK source available as its license requires. FLTK's static-link exception permits the application to retain a separate license, but it does not remove obligations for modified FLTK or the bundled image libraries.

The pinned libmpv DLL's release archive has no complete build-specific license inventory or corresponding notices. Its build scripts disable GPL and nonfree options, but those flags alone do not prove the licenses of the exact mpv, FFmpeg and other linked components. Do not distribute that DLL or a Windows portable ZIP until the exact component inventory, notices, corresponding-source obligations and runtime dependencies are resolved. See `docs/19_ADR_FLTK_WINDOWS_DESKTOP.md`.

For a verified LGPL runtime release, users can run an interface-compatible replacement DLL without rebuilding AnimePortable by setting `ANIMEPORTABLE_LIBMPV_OVERRIDE` to its absolute path and `ANIMEPORTABLE_LIBMPV_OVERRIDE_SHA256` to the replacement DLL's SHA-256 before starting the application. Both values are required; the loader still verifies the file, its hash, and its absolute path. Place the replacement and any dependent DLLs in a directory you trust. Without these variables, AnimePortable accepts only the pinned DLL beside its executable.
