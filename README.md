<!-- SPDX-License-Identifier: MPL-2.0 -->

# AnimePortable

繁體中文 | [English](README.en.md)

輕量、以鍵盤操作為主的 Windows 動畫播放器，靈感來自 [OneAnime](https://github.com/Predidit/oneAnime)。搜尋動畫、選集數、在內建播放器觀看，下次從上次的位置繼續。解壓縮即可執行：不用安裝、不用帳號、不回傳任何使用資料。

## 功能

- 瀏覽與搜尋 Anime1 的作品。
- 作品詳情頁：封面與簡介來自 Bangumi（以 AniList 交叉確認），一律以繁體中文顯示。
- 集數依序排列，每一集顯示觀看進度。
- 內建播放器（libmpv）：控制列會自動隱藏，可切換集數與全螢幕。
- 首頁的「繼續觀看」，從上次停下的位置接著播放。
- 滑鼠與鍵盤都能操作。

追蹤、歷史紀錄、時間表、設定與自動播放下一集正在開發中，請見[開發計畫](docs/04_ROADMAP.md)。

## 執行

解壓縮後執行 `AnimePortable.exe`。壓縮檔內附 `libmpv-2.dll`，程式載入前會先核對它的 SHA-256。你的資料（歷史紀錄、觀看進度、作品資料快取）存放在執行檔旁的 `data/` 資料夾，整個資料夾可以直接搬移或複製。

目前只支援 Windows。Linux 與 macOS 會在 Windows 版發布之後進行。

## 從原始碼建置

需要：Go（版本見 `go.mod`）、cgo，以及 `PATH` 上的 msvcrt 版 MinGW gcc（MSYS2 MINGW64 或 nuwen MinGW；UCRT 版 MinGW 無法連結 FLTK 函式庫）。

```
CGO_ENABLED=1 go build -trimpath -o AnimePortable.exe ./apps/desktop
CGO_ENABLED=1 go test ./...
```

要播放影片，請把固定版本的 `libmpv-2.dll`（見 [docs/LIBMPV_RUNTIME_NOTICE.txt](docs/LIBMPV_RUNTIME_NOTICE.txt)）放在執行檔旁。

## 目前進度

開發中，尚未公開發布。瀏覽、作品詳情與播放已完成。第一個 Windows 版本發布前還有：追蹤、歷史紀錄、時間表、完整鍵盤操作、離線時用已存資料啟動、自動播放下一集、設定，以及安全與發布檢查。

- 詳細狀態：[docs/STATUS.md](docs/STATUS.md)
- 工程文件：[docs/README.md](docs/README.md)
- 貢獻者與 AI 代理：[AGENTS.md](AGENTS.md)

工程文件以英文撰寫。

## 授權

Mozilla Public License 2.0。相依套件、內附的 libmpv DLL 與遠端服務內容的授權說明，請見 `LICENSE` 與 `THIRD_PARTY_NOTICES.md`。
