<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 37b: Language

Status: BUILD. Scope approved 2026-10-22; the owner then chose a live switch over a restart (same day).

## Goal

The user chooses the UI language on the 設定 page: 繁體中文 (default), English or 日本語. Every UI string comes from one string table. Anime titles, synopses and other remote content are not translated. No Simplified Chinese anywhere.

## Current state (2026-10-22)

- `core.Settings.Language` exists (Unspecified, Traditional Chinese, English); the SQLite CHECK allows `0, 1, 2`. Japanese needs a value 3 and migration 0010 (table rebuild, like 0003). Migrations 0001-0009 stay unchanged.
- About 270 Chinese UI literals in Go: `fltkhome` (settings, Home model, search, Home window, detail, following, schedule, history, dialogs), the player (`fltkplayer`), `main_fltk_windows.go`, `restart.go`, and the 測試連線 reasons in `backend/network.go`. The Chinese strings in `core/metadata_*` and `adapters/metadata/bangumi` are matching rules for remote data, not UI text, and stay.
- The UI font is Microsoft JhengHei UI, which has kana; Japanese text must be checked on screen.
- Buttons have fixed widths (for example nav 177 px); English and Japanese labels can be longer.

## In scope

1. A string table package (`apps/desktop/uitext`) with one entry per UI string and its three translations; a test fails when any entry lacks a language or contains Simplified-only characters in the Chinese text.
2. Every UI literal above moves to the table, including formatted text (集數, dates, 「N 天前」, weekdays, 測試連線 results).
3. 設定 → 外觀: a 語言 segmented control 「繁體中文 | English | 日本語」, saved like the other settings.
4. The language switches at once, without a restart (owner, 2026-10-22). Fixed labels register with `labelled` and are set again on a switch; statuses and settings rows hold a `text` that is read when drawn; pages build their rows in the new language when they show next; the player's menu changes when it opens next. The stored language is applied when the settings load at startup, so 正在準備資料… and the missing-data-folder question before it stay in Traditional Chinese.
5. Layout: settings segments and action buttons grow to their label; the fixed-width buttons are checked by a window test in every language (English back buttons read Back).

## Out of scope

Translating remote content; Simplified Chinese; more languages; following the Windows display language automatically.

## Tests

- Table completeness, matching format arguments and the no-Simplified check; migration 0010 upgrade from a 0009 database; Language round trip; switching relabels the shown page and saves; the stored language applies at startup; fixed buttons fit (window tests, run locally after asking the owner).
- Full native suite and race; simplify; oracle review with no MUST_FIX.

## Owner check

1. 設定 → 語言 → English: the page changes at once; every screen (Home, 時間表, 追蹤, 歷史紀錄, 搜尋, 作品頁, player, 設定, dialogs) is in English, no text cut off.
2. Same for 日本語.
3. Back to 繁體中文: everything as before.

## Review (oracle, 2026-10-22)

- Round 1: MUST_FIX (1) the player's menu slice pointed into the Traditional Chinese table, so a later menu update wrote English into it; (2) the player overlay (C++) still drew 選集 and 正在載入…. SHOULD_FIX: 追蹤 and 時間表 rows built at load kept the old language; long Japanese and English status lines. All fixed: `SetMenuItems` copies, the overlay takes its labels from Go, loaded rows are rebuilt on a switch, texts shortened; separator and English episode label moved to the table.
- Round 2: no MUST_FIX. The menu test now reproduces the old aliasing; build tag matches the package. Left as is: English "Episode unknown" overflows the player's collapsed header, shown only for an episode without a number.

## Runs

- 2026-10-22, owner allowed GUI runs. First window run: the new fit test found the Japanese back labels too wide (now 戻る, like English Back), and two keyboard-order tests predated the 語言 row (expected order updated: 主題, then 語言, then the toggles). Then the full native suite passed. The race run failed once in `TestNativeSettingsDownloadPageOpensOnlyReleasePagesOfTheProject` (10 s wait for a key event); TypeSafe failure-class FLAKY_OR_NONDETERMINISTIC conf 1.00 (`artifacts/evidence/loop37b/failure-race-1.json`); that test alone and the whole `fltkhome` package then passed under race with no change.
- Test build 1: `artifacts/test-build/AnimePortable.exe` (EXE SHA-256 `B4E90F0C…7C36`; libmpv DLL matches the pin; data backed up to `artifacts/evidence/loop37b/data-backup-before-build-1/`; the first start applies migration 0010).
- Owner check 1 (2026-10-22, test build 1): "Owner check 1 passed：三種語言切換都正常。" TypeSafe owner-report HUMAN_PASS (conf 1.00, `owner-report-1.json`).
