# Loop 29 接手紀錄

更新：2026-10-07 05:08 UTC。本文件保留接手前的失敗紀錄與診斷起點；目前狀態以 `docs/AGENT_HANDOFF.md`、`docs/IMPLEMENTATION_STATUS.md` 及 Git／CI 為準。

## 現行結果（2026-10-08）

`57782c8` 的 [Windows CI](https://github.com/loustack17/AnimePortable/actions/runs/37714489095) 已全部通過；新版 exe 與 ZIP 已交付，原有資料雜湊未變。使用者仍需依 `docs/LOOP29_HUMAN_CHECK.md` 確認兩項實際播放情境，Loop 29 維持 `NEEDS_HUMAN`。

## 接手前紀錄（2026-10-07，歷史）

以下「目前」、「下一步」及舊 exe 敘述只代表接手前狀態，已由上方現行結果取代。

### 現況

| 問題 | 進度 | 尚待完成 |
| --- | --- | --- |
| 首次播放後，所有控制項消失 | 已套用修正；attempt1普通／race通過，但attempt2 race首播字形再次失敗，修正尚未穩定 | 必須定位目前字形失敗；新 exe 未交付，使用者 GPU 未驗收 |
| 關閉後 Continue 顯示／使用舊時間 | 即時 Snapshot、跳轉完成等待、首頁刷新及舊卡片失效已修；native／SQLite／DB 重開與 Continue 測試通過 | 實際播放、關閉、Continue、重啟未驗收 |
| CI／exe | full native、20-repeat、Anime1 通過；attempt1 race播放器／首頁通過，attempt2播放器失敗 | attempt1 core30ms斷言失敗；attempt2播放器字形失敗；沒有修正版 exe |
| 效能 | CPU 字形快取上限128筆；首頁一次實體讀取與一個待刷新意圖 | 未量測新增效能基準，不可宣稱提升數字 |

Loop29 **不是 PASS**，不要開始 Loop30。鍵盤、搜尋結果、首次播放、下拉換集及其他先前修正已獲使用者正面回饋。亮暗色持久化、首頁／前段搜尋、自動初始化、Continue間距已有先前修正；勿擴大功能。「追蹤」完整頁面仍為Loop32／Phase26，完整詳細資料／選集 UI 為後續切片。

### Git 與修正

`main`／`origin/main` HEAD：`c0f27135c039f10c51c6b4d5b0e2a1eab9b3dfe0`。修正均已 commit/push。保留使用者原有 `AGENTS.md` 修改及未追蹤 `.codex/`，未納入提交。

| Commit | 內容 |
| --- | --- |
| `63e4f1f` | 控制項繪字、live Snapshot、首頁刷新、保存錯誤提示、native tests |
| `b4afd2f` | 等待非同步 seek 完成再 Snapshot；nil-context 契約回歸 |
| `c0f2713` | 相容的三秒 H264 fixture；Home 測試通知 latch |

主要改動：

- `apps/desktop/fltkplayer/text_windows.cc`：GDI UTF-16／1-bit 點陣與 glBitmap；只快取 CPU bytes，不保存 GL texture IDs；GDI／GL pixel-store 清理與還原。
- `fltkplayer/overlay_windows.cc`、`overlay_windows.go`：標籤／圖示改用上述繪字，連結系統 gdi32。
- `fltkengine/engine_windows.go`：在 FLTK 執行緒讀 time-pos／duration／pause；數值驗證；seek/restart 事件追蹤。Snapshot 在 UI 外每10ms等待，context／3秒上限；load/end重設。原 seek 模式保留。
- `fltkengine/internal/mpvwin/types_windows.go`：事件20／21常數。
- `fltkhome/window_windows.go`：返回首頁 reload；合併刷新；generation／取消／closing 檢查；隱藏停用舊卡片；焦點避開失效項目。
- `apps/desktop/main_fltk_windows.go`：進度確認失敗顯示提示，避免無聲回到舊紀錄。

沒有 DB schema、provider、核心架構、依賴版本或產品 TypeSafe runtime 變更；直接修正階段沒有 workflow 變更。

### CI 與驗證

[CI37574002579](https://github.com/loustack17/AnimePortable/actions/runs/37574002579)，精確 HEAD `c0f2713`。

**Attempt1／job112638711703：FAIL。**

- `go test -count=1 ./...`：PASS，含真實 H264／libmpv 兩次播放字形、即時 seek Snapshot、首頁刷新／錯誤／合併舊讀取。
- 20次 desktop/backend/Home/libmpv：PASS；Anime1：PASS。
- `go test -race ./...`：desktop／fltkengine／mpvwin／fltkhome／fltkplayer PASS；core 的 `TestTrackedCloseWaitsForOwnedRunAfterCheckpointTimeout` FAIL。
- `core/playback_tracking_test.go:694`：`raw close count before blocked checkpoint release = 0`。測試等待30ms後假設 raw.Close已執行；沒有 DATA RACE 報告。排程敏感是推論，未證明產品缺陷。
- vet／漏洞／build／upload 因前項失敗未執行，無新 exe。

原封不動的該 core test 本機 `CGO_ENABLED=0` count20 PASS。只要求一次同 HEAD／同檢查 rerun，未修改 core 或放寬斷言。

**Attempt2／job112639950602：FAIL。** race階段 `TestPlayerControlsRenderAfterEngineReentry` 在 `player_reentry_windows_test.go:144` 首次播放失敗：episode glyph ROI `(815,570)-(905,602)` 只有0 bright pixels，framebuffer1000x618、drawCounts4、context0x1、GLerrors0。Home、overlay、core race本次PASS；不等於完整播放器通過。兩次同HEAD結果不穩定，原因尚未分類，不能宣稱控制項已解決，也不再盲目重跑。後續build/upload仍未執行。接手先比對兩次log，定位產品GL狀態、前buffer採樣／視窗時序是否相關，保留原字形oracle：

```powershell
gh api repos/loustack17/AnimePortable/actions/runs/37574002579
gh api repos/loustack17/AnimePortable/actions/runs/37574002579/jobs
gh api --allow-escape-sequences repos/loustack17/AnimePortable/actions/jobs/112639950602/logs
```

其他檢查：C++11語法、gofmt、diff check、pure core/libmpv/backend/architecture PASS。Local native cgo 穩定受限：`cannot parse _cgo_.o as ELF, Mach-O, PE or XCOFF`／unlink Access denied，等價重試已停止。不得以 host native／FullAccess／全域安裝代替 PASS，沿用既有隔離 Windows CI。

### 已查明的原因與錯誤方向

1. FLTK global glyph texture cache 沒有 context identity，Hide銷毀context。改為CPU點陣；保留字形像素斷言，不能只看背景色。
2. 首頁不在返回後reload、closure抓舊history；native Snapshot只讀0.5秒tick快取。均已修。
3. absolute+exact候選被否決：absolute原本就預設exact，不能解釋失敗；候選未提交。
4. 固定libmpv把舊GIF視為duration1秒，seek2被clamp到1秒；ffprobe顯示3秒不代表固定runtime支援動畫。新fixture為2448-byte、H264320×180／10fps／3秒。測試先要求2.9..3.1秒／180p，兩次字形與即時2秒斷言未放寬。
5. Home ready predicate消耗channel，helper查兩次造成false failure；已latch。顯示native測試視窗也保留。
6. 相同pinnedlibmpv headless診斷確認MP4 duration3秒、seek/restart位置2秒；這是診斷，不是GL verifier PASS。使用既有ffmpeg/ffprobe，未安裝或改OS設定。暫存probe已移出產品樹。
7. reset-on-Hide候選未採用：failed SwitchEpisode可能仍有live engine，不能任意刪texture。此錯誤路徑尚未使用者重現，不能當作已修。

相關測試：`fltkengine/player_reentry_windows_test.go`、`engine_windows_test.go`、`testdata/black-3s.mp4`、`fltkhome/home_refresh_windows_test.go`、`backend/resume_checkpoint_test.go`、`fltkplayer/overlay_windows_test.go`。

### 審查、證據與使用量問題

先前確有流程失誤：擴大到CI環境、錯誤MSYS2 cache／Go temp假設、碎片化審查/API呼叫、錯誤fixture與通知predicate。使用者非常不滿，要求只處理專案bug及有證據的效能問題，禁止無關設定。沒有帳務資料能核實使用量百分比。

先前明確批准的Mesa workflow已提交，不要重做：固定MSYS2 Mesa26.2.4-1、簽章／archiveSHA檢查、暫存test.exe兩個DLL／llvmpipe、產品PE依賴拒絕Mesa/Gallium/LLVM。直接修正階段未再改它。

最終獨立platform/quality及test-integrity審查PASS_REVIEW，不能當成人工PASS。TypeSafe skill已讀，BWS key／必要片段上傳已批准，未輸出key、未加入runtime AI。最新原始結果 `artifacts/evidence/loop29/direct-fix-followup-judgments.json`：lifecycle1.93/conf.90、scope1.94/conf.91，機率另存。`direct-fix-judgments.json` 被follow-up覆寫，不能當成初輪原始證據。

本機證據在 Git ignored `artifacts/evidence/loop29/`：

- `direct-fix-ci-37572285446.log`、`direct-fix-ci-37573212734.log`：glyph PASS，舊fixture／predicate FAIL。
- `direct-fix-ci-37574002579.log`：attempt1完整native／repeat PASS，core30ms race斷言FAIL。
- `direct-fix-ci-37574002579-attempt2.log`：同HEAD race首播episode字形0 pixels；Home／overlay／core race PASS。
- `seek-mp4-host-diagnostic.txt`／`seek-host-diagnostic.go`：pinned runtime診斷。
- `mesa-ci-37570998564.log`：實際字形失敗基線。

遠端接手若無本機ignored證據，讀對應GitHub job，不要將證據缺失當產品缺陷。

### 測試物件、資料、授權與下一步

`artifacts/loop29/AnimePortable.exe` **仍為舊2013fce，沒有修正版交付**。SHA256：`7bfb7e9e894ce3a415e7da079be31395a46140281c5142274dc123e5f7262b17`。`docs/LOOP29_HUMAN_CHECK.md`仍指向舊版，不能要求當成新版重測。

目前attempt2失敗，沒有可下載的修正版artifact。下一個AI先定位字形失敗；未來同精確HEAD全部綠燈且產出artifact後，才可使用相應run／commit下載（以下為原本預期命名，不能假定已存在）：

```powershell
gh run download 37574002579 --name windows-desktop-c0f27135c039f10c51c6b4d5b0e2a1eab9b3dfe0 --dir artifacts/downloads/c0f2713
```

確認app未執行，保存 `artifacts/loop29/data/` hashes，只替換exe，再核對data。不可覆蓋真實DB。沿用批准DLL／授權文件，DLL SHA256 `24e848f59c047c9442501fdbe619ad39b98be7d4dd402691f79931c852c0070a`。ZIP用現有portable-package／community-runtime，以commit命名，不含data。

原legacy清理已記在IMPLEMENTATION_STATUS：204個舊驗證檔封存於 `artifacts/evidence/legacy-verification.zip`，manifest核對後移除原路徑；不要重做全專案清理。

既有授權：BWS／必要TypeSafe片段、main commit/push、exe upload、精確Mesa提案；沒有release、全域設定、其他repo或無上限修正授權。新workflow範圍仍需明確批准與獨立審查。

原5+追加3次已用完；3次為 `f687e11`／`56c875f`／`81810be`，不能合併計數。之後使用者再次明確要求直接修兩bug，允許本次恢復，未指定新數字上限。最新要求是交接，本代理不再新增修正。

接手順序：讀AGENTS／handoff／本文件，核對Git與attempt2；目前第一優先是字形race失敗的因果診斷，禁止盲重跑或弱化測試；精確版本綠燈後才取新exe保留資料並更新人工驗收。使用者只測 A-close-B控制項，以及 advance/seek-close-Continue/restart位置。人工確認前不結束Loop29、不擴大功能。
