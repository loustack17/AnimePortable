# Loop 29 人工驗收

## 測試版本

- 直接執行：[AnimePortable.exe](../artifacts/loop29/AnimePortable.exe)，並保持同資料夾內的 `libmpv-2.dll`。
- 全新資料夾測試包：[AnimePortable-loop29-ae8d2d2.zip](../artifacts/loop29/AnimePortable-loop29-ae8d2d2.zip)。ZIP 不含個人資料。
- 版本：`ae8d2d22e93e0823b28c962ed6569c0a974c74a7`；[Windows CI 全部通過](https://github.com/loustack17/AnimePortable/actions/runs/37718984683)。exe SHA-256：`3ec74671ce88d751221cba83d09874e2b04453927d856d8cbd79cf82e1cbd86a`。ZIP SHA-256：`9732a678de059285837ead2141f35f757399bc4fe2db612ab976cf8d3edc7be6`。
- 已保留原有 `data`；替換 exe 前後的資料檔雜湊相同。

## 已通過的項目

使用者已確認上一版的第二部作品控制項與最新播放位置問題均已修正。

## 請確認集數顯示

| 測試 | 操作 | 預期結果 |
| --- | --- | --- |
| 最新集數與位置 | 播放同一作品第 1 集，再切換至第 4 集，播放一段時間後關閉播放器。查看首頁「繼續觀看」，再按「繼續播放」。 | 卡片顯示第 4 集的集數與剛才的位置，例如「集數：04 · 上次播放位置 2:05」；繼續播放開啟第 4 集並從該位置附近開始。 |

集數會在背景載入；若來源無法連線或找不到對應集數，顯示「集數：未知」，仍可繼續播放。請回覆此項通過或失敗；若失敗，提供作品名稱與卡片顯示的集數、時間。依專案人工驗收規定，確認此項前 Loop 29 保持 `NEEDS_HUMAN`。
