<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable

繁體中文 | [English](README.en.md)

輕量、以鍵盤操作為主的 Windows 動畫播放器，靈感來自 [OneAnime](https://github.com/Predidit/oneAnime)。搜尋動畫、選集數、在內建播放器觀看，下次從上次的位置繼續。解壓縮即可執行：不用安裝、不用帳號、不回傳任何使用資料。

## 功能

- 瀏覽與搜尋 Anime1 的作品。
- 作品詳情頁：封面與簡介來自 Bangumi（以 AniList 交叉確認），一律以繁體中文顯示；先顯示上次存下的集數清單，再向 Anime1 更新。
- 集數依序排列，每一集顯示觀看進度。
- 內建播放器（libmpv）：控制列會自動隱藏，可切換集數、全螢幕與播放速度，播完自動下一集，片頭可一鍵跳過（AniSkip）。
- 首頁的「近期更新」與「繼續觀看」，從上次停下的位置接著播放。
- 追蹤作品、觀看紀錄，以及接下來 7 天的新番時間表。
- 「設定」：主題、語言（繁體中文、English、日本語）、播放、代理伺服器、資料位置與檢查更新。
- 滑鼠與鍵盤都能操作；沒有網路時仍可用已存的資料開啟。

## 執行

從 [Releases](https://github.com/loustack17/AnimePortable/releases) 下載 ZIP，解壓縮後執行 `animeportable.exe`。壓縮檔內附 `libmpv-2.dll`，程式載入前會先核對它的 SHA-256。你的資料（追蹤、觀看紀錄與進度、作品資料快取、設定）存放在執行檔旁的 `data/` 資料夾，整個資料夾可以直接搬移或複製。

目前只支援 Windows 64 位元。Linux 與 macOS 之後才會進行。

## 從原始碼建置

需要：Go（版本見 `go.mod`）、cgo，以及 `PATH` 上的 msvcrt 版 MinGW gcc（MSYS2 MINGW64 或 nuwen MinGW；UCRT 版 MinGW 無法連結 FLTK 函式庫）。

```
CGO_ENABLED=1 go build -trimpath -ldflags "-H windowsgui" -o animeportable.exe ./apps/desktop
CGO_ENABLED=1 go test ./...
```

要播放影片，請把固定版本的 `libmpv-2.dll`（見 [docs/LIBMPV_RUNTIME_NOTICE.txt](docs/LIBMPV_RUNTIME_NOTICE.txt)）放在執行檔旁。

## 目前進度

v1.0.0 是第一個正式版，已完成正式版前的安全與資源檢查。之後的計畫（2.0 版）見[開發計畫](docs/04_ROADMAP.md)。

- 詳細狀態：[docs/STATUS.md](docs/STATUS.md)
- 工程文件：[docs/README.md](docs/README.md)
- 開發者：[AGENTS.md](AGENTS.md)

工程文件以英文撰寫。

## 授權

Mozilla Public License 2.0。相依套件、內附的 libmpv DLL 與遠端服務內容的授權說明，請見 `LICENSE` 與 `THIRD_PARTY_NOTICES.md`。
