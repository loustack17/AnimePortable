# Loop 29 人工驗收

## 測試版本

- 直接執行：[AnimePortable.exe](../artifacts/loop29/AnimePortable.exe)，並保持同資料夾內的 `libmpv-2.dll`。
- 全新資料夾測試包：[AnimePortable-loop29-57782c8.zip](../artifacts/loop29/AnimePortable-loop29-57782c8.zip)。ZIP 不含個人資料。
- 版本：`57782c8c1459181bd27df312e6001235f81e2142`；[Windows CI 全部通過](https://github.com/loustack17/AnimePortable/actions/runs/37714489095)。exe SHA-256：`e7949ea7b7bfac3737300cff32b28eb34b78d1cfa6d52421f241847926112534`。ZIP SHA-256：`62d2250698118aee76f5b25fb4017e2e7058449f7a5d2476e3c488eeccdcc7e7`。
- 已保留原有 `data`；替換 exe 前後的資料檔雜湊相同。

## 請確認兩項

| 測試 | 操作 | 預期結果 |
| --- | --- | --- |
| 第二部作品的控制項 | 搜尋並播放作品 A，關閉播放器回首頁，再搜尋並播放作品 B。移動滑鼠或按 Tab，並試用進度列與選集。 | 作品 B 仍顯示可操作的控制列、文字、進度與選集；不會只剩影片。 |
| 最新播放位置 | 播放一集並前進或拖動進度，記下大致時間後關閉播放器。回首頁查看「繼續觀看」並開啟該集；再關閉、重啟程式後開啟同一筆。 | 首頁顯示剛才的進度；繼續觀看與重啟後都從最近關閉時的位置附近播放，不退回較舊時間。 |

請回覆每項通過或失敗；若失敗，提供作品名稱、集數、預期與實際時間，以及控制項是否在移動滑鼠或按 Tab 後恢復。完成這兩項前，Loop 29 保持 `NEEDS_HUMAN`。
