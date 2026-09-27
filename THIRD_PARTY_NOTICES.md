<!-- SPDX-License-Identifier: MPL-2.0 -->

# Third-Party Notices

AnimePortable source code, documentation, and build configuration are licensed under the Mozilla Public License 2.0. This file records the third-party components used by the native application. Exact versions are pinned in `go.mod` and `go.sum`.

## Go runtime dependencies

| Component | License |
| --- | --- |
| [go-fltk](https://github.com/pwiecz/go-fltk) | MIT; Windows FLTK path |
| [FLTK](https://www.fltk.org/COPYING.php) | GNU Library General Public License with FLTK exceptions; statically linked by go-fltk |
| [purego](https://github.com/ebitengine/purego) | Apache-2.0; Windows libmpv loader |
| [go-winio](https://github.com/microsoft/go-winio) | MIT |
| [golang.org/x/net](https://cs.opensource.google/go/x/net) | BSD-3-Clause |
| [golang.org/x/sys](https://cs.opensource.google/go/x/sys) | BSD-3-Clause |
| [golang.org/x/text](https://cs.opensource.google/go/x/text) | BSD-3-Clause |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause; SQLite notices are included upstream |

AnimePortable is based in part on the work of the FLTK project (https://www.fltk.org).

## Remote services and content

The project license applies only to AnimePortable material that the project can license. It does not grant rights to Anime1, AniList, or Bangumi names, trademarks, logos, remote API responses, metadata, covers, subtitles, video files, or other provider and rights-holder content. Use of those services and content remains subject to their own terms and applicable law.

## Reviewed upstream projects

[oneAnime](https://github.com/Predidit/oneAnime) is a separate GPL-3.0 project that acknowledges code from [AnimeOne](https://github.com/HQAnime/AnimeOne), whose repository is MIT-licensed. AnimePortable does not copy or distribute source code from either project; its Anime1 adapter is an independent Go implementation of the public service protocol.

## Audit scope

The Windows FLTK dependency inventory requires final upstream license review before release. The project does not vendor third-party source code. The proposed portable archive bundles a pinned libmpv DLL and its build-specific license notice under `licenses/libmpv.txt`; upstream mpv supports GPLv2+ by default and LGPLv2.1+ only for qualifying builds. The exact DLL and linked codec license terms must be verified before release.
