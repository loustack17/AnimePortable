<!-- SPDX-License-Identifier: MPL-2.0 -->

# Third-Party Notices

AnimePortable source code, documentation, and build configuration are licensed under the Mozilla Public License 2.0. This file records the third-party components used by the native application. Exact versions are pinned in `go.mod` and `go.sum`.

## Go runtime dependencies

| Component | License |
| --- | --- |
| [Fyne](https://github.com/fyne-io/fyne) | BSD-3-Clause; full notice below |
| [go-winio](https://github.com/microsoft/go-winio) | MIT |
| [golang.org/x/net](https://cs.opensource.google/go/x/net) | BSD-3-Clause |
| [golang.org/x/sys](https://cs.opensource.google/go/x/sys) | BSD-3-Clause |
| [golang.org/x/text](https://cs.opensource.google/go/x/text) | BSD-3-Clause |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | BSD-3-Clause; SQLite notices are included upstream |

## Fyne license

BSD 3-Clause License

Copyright (C) 2018 Fyne.io developers (see AUTHORS)
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of Fyne.io nor the names of its contributors may be
      used to endorse or promote products derived from this software without
      specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## Remote services and content

The project license applies only to AnimePortable material that the project can license. It does not grant rights to Anime1, AniList, or Bangumi names, trademarks, logos, remote API responses, metadata, covers, subtitles, video files, or other provider and rights-holder content. Use of those services and content remains subject to their own terms and applicable law.

## Reviewed upstream projects

[oneAnime](https://github.com/Predidit/oneAnime) is a separate GPL-3.0 project that acknowledges code from [AnimeOne](https://github.com/HQAnime/AnimeOne), whose repository is MIT-licensed. AnimePortable does not copy or distribute source code from either project; its Anime1 adapter is an independent Go implementation of the public service protocol.

## Audit scope

The native dependency inventory was checked against the resolved Go module graph and upstream license files on 2026-09-24. The project does not vendor third-party source code. MPV is installed separately by the user and is not included in portable archives.
