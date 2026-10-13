# Loop 29 人工驗收

## 目前狀態

原始 Loop 29 已於 2026-10-08 通過。2026-10-12 使用者針對下方最新版本與四項檢查回覆「四項皆通過」；後續播放器修正的精確 Windows CI、獨立審查及人工驗收皆通過。Loop 29 與後續修正均為 `PASS`。

## 最新測試版本

- 直接執行：[AnimePortable.exe](../artifacts/loop29/AnimePortable.exe)，保留旁邊的 `libmpv-2.dll`。
- 全新資料夾測試包：[AnimePortable-player-input-90a7fc9.zip](../artifacts/loop29/AnimePortable-player-input-90a7fc9.zip)。ZIP 不含個人資料；若要確認既有 Continue 紀錄，使用上方直接執行版本。
- 版本：`90a7fc9985bd5ab17b1198333567324fd69d2e8a`；[Windows CI 全部通過](https://github.com/loustack17/AnimePortable/actions/runs/37735609430)。2026-10-12 已重新取得遠端 artifact，確認目前 exe 與其逐位元一致。
- exe SHA-256：`be4d9961c6d8c97d6d5fcc7aa470034ce6e433b9c949f13f3f1e43db9464e011`。
- ZIP SHA-256：`cd788512fc0220f55e8fe3801c1eba070a6a0cc6d3cb50c7ddf0908273ceee85`。
- 已確認資料庫符合先前替換前的雜湊；本次核對未替換 exe 或修改 `data`。

## 已通過的檢查

| 測試 | 操作與預期結果 |
| --- | --- |
| 播放與繼續觀看 | 從 Search 開始播放至少一分鐘，連續跳轉數次，再關閉播放器按 Continue。換另一部作品播放，重啟 app 後再 Continue。沒有無故返回首頁或控制失去反應；第二部作品控制項可見，集數與保存位置正確。 |
| 焦點與播放鍵 | 未點控制項時左右鍵跳轉五秒；控制項隱藏再出現後焦點仍在進度條。Tab 選 Play/Pause，Space 與 Enter 每按一次切換一次；左右鍵不改焦點或觸發播放。選音量後方向鍵只調音量，M 靜音／恢復、J/L 跳轉十秒。 |
| 實體媒體鍵與全螢幕 | 播放器在前景時使用鍵盤媒體 Play/Pause、音量及靜音鍵；進出全螢幕、返回首頁再 Continue 後重複。每次按鍵只執行一次。Windows 系統媒體面板屬另一項整合，未列入本次範圍。 |
| 換集與首頁 | 打開集數清單，以方向鍵選集並按 Enter／Space；新集預設焦點在進度條。首頁往下捲動後播放，再返回首頁兩次：繼續觀看標題與第一張卡片間沒有大片空白。 |

上述四項已獲使用者確認，保留作為回歸檢查。詳細鍵盤契約見 [PLAYER_KEYBOARD_FOLLOWUP.md](PLAYER_KEYBOARD_FOLLOWUP.md)。

## 原始 Loop 29 驗收紀錄

2026-10-08：使用者測試 `2d4bf48` 後回覆「perfect!!! loop 29 can be passed」。Search／鍵盤流程、第二部作品控制項、最新播放位置、集數顯示及返回首頁間距已獲接受；原始 Loop 29 為 `PASS`。此紀錄不代表後續播放器修正也已獲接受。

當時測試包：[AnimePortable-loop29-2d4bf48.zip](../artifacts/loop29/AnimePortable-loop29-2d4bf48.zip)；[Windows CI](https://github.com/loustack17/AnimePortable/actions/runs/37725390481)。exe SHA-256：`89c1f2bf39c02942ecfdd90ba357b42d00042e261068022d6a28317a3a5aae52`；ZIP SHA-256：`044ed1133a0b0f3058b283644790d5b39bbb54bb535d106f6d00cced190069a6`。保留此 ZIP 作回復版本。
