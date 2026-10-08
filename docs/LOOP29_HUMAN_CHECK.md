# Loop 29 人工驗收

## 測試版本

- 直接執行：[AnimePortable.exe](../artifacts/loop29/AnimePortable.exe)，並保持同資料夾內的 `libmpv-2.dll`。
- 全新資料夾測試包：[AnimePortable-loop29-2d4bf48.zip](../artifacts/loop29/AnimePortable-loop29-2d4bf48.zip)。ZIP 不含個人資料。
- 版本：`2d4bf4892b7f2fe52887db8a8ba82a49d53bf20c`；[Windows CI 全部通過](https://github.com/loustack17/AnimePortable/actions/runs/37725390481)。exe SHA-256：`89c1f2bf39c02942ecfdd90ba357b42d00042e261068022d6a28317a3a5aae52`。ZIP SHA-256：`044ed1133a0b0f3058b283644790d5b39bbb54bb535d106f6d00cced190069a6`。
- 已保留原有 `data`；替換 exe 前後的資料檔雜湊相同。

## 已通過的項目

使用者已確認上一版的第二部作品控制項與最新播放位置問題均已修正，並接受集數顯示功能。

## 請確認返回首頁的間距

| 測試 | 操作 | 預期結果 |
| --- | --- | --- |
| 返回首頁的間距 | 在首頁往下捲動，再按一筆「繼續播放」。關閉播放器回首頁；再重複一次。 | 「繼續觀看」標題下方緊接第一筆卡片，沒有大片空白；集數、時間與播放按鈕保持對齊。 |

請回覆此項通過或失敗。依專案人工驗收規定，確認此項前 Loop 29 保持 `NEEDS_HUMAN`。
