<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 34: Keyboard navigation everywhere

Status: PASS (owner accepted 2026-10-20). Commit c2ad61a, GitHub Windows CI 39496901879 green.

## Goal

The whole core workflow works with the keyboard only: launch, find, open, select an episode and play, with no mouse.

## Current behavior (inventory 2026-10-20)

Already works: Up/Down/Tab and Right on the navigation; arrows, Tab and Enter on Home (搜尋作品, 近期更新 rows, 繼續播放); `/` and Ctrl+K open 搜尋 with the input focused; Enter, Up/Down and Esc on search results; Enter or Space plays, Up/Down and Esc on episode rows; arrows and Enter on the 時間表 grid; the player's Tab ring, arrows, Space, Enter, K/M/F/J/L and Esc (closes the episode list, then the menu, then fullscreen). Every custom button draws an accent ring when focused. 追蹤, 歷史紀錄, 時間表 and the episode list keep focus across a reload; removing a history entry moves focus to its neighbour.

Gaps:

1. Esc outside 搜尋 is not handled by the app, so FLTK's default runs the window callback: on Home, 追蹤, 歷史紀錄 and 時間表 Esc closes the whole app.
2. The shared row list (追蹤, 歷史紀錄) binds only Enter. Up and Down fall to FLTK's default group navigation, which does not scroll the focused row into view, so focus can move off screen; there is no defined route to the navigation or the page button.
3. Home rebuilds its rows on every reload (entering Home, returning from the player, a new episode found in the background) and does not restore focus; the focused row is destroyed and focus is lost.
4. Returning from the player shows Home but sets no focus.
5. Search results are rebuilt when the stored library or the online results arrive, and focus on a result row is lost.
6. In the search input, Down does not reach the results, and Ctrl+K is passed to the input (FLTK deletes to the end of the line) instead of selecting the query.
7. 時間表's 重新整理 button has Enter only; Down does not reach the cards. Esc is unhandled (gap 1).
8. Detail: Up from the first episode and Left/Right have no route to 返回, 繼續觀看, 追蹤 or the page buttons.
9. Player: in a window Esc does nothing, so leaving playback needs the menu or the close button; `/` and Ctrl+K are ignored (Ctrl is rejected). The focus ring hides with the controls, but any key shows them again, so this is not a gap.
10. Key codes and the deferred-key pattern are repeated in six files.

設定 is a placeholder page; it gets no content (new screens are out of scope) but must not trap focus.

## Scope

In scope:

- One shared key policy in `fltkhome` (one file): the key codes, what Enter, Space, Esc, the arrows, Tab, `/` and Ctrl+K mean, and one window-level handler for Esc, `/` and Ctrl+K on every screen. Screens declare their neighbours; they do not copy the policy.
- Esc goes back (owner decision 2026-10-20): detail to where it was opened from (unchanged), 搜尋, 追蹤, 歷史紀錄, 時間表 and 設定 to Home with focus on 首頁; on Home Esc does nothing. Esc never closes the app (the close button and Alt+F4 still do).
- `/` and Ctrl+K on every screen open 搜尋 with the input focused; inside the input `/` types and Ctrl+K selects the query.
- Enter and Space activate the focused button.
- The shared row list: Up/Down between rows, scrolled into view; Left/Right between a row and its icon; Left from a row, or Up from the first row with no page button, goes to the navigation; Up from the first row goes to the page button when it shows.
- Focus is kept or moved to a defined place after every rebuild: Home rows by anime, search results by index, return from the player on the destination page.
- Search input Down to the first result; 重新整理 Down to the first card; detail Up from the first episode to the buttons above.
- Player (owner decision 2026-10-20): Esc in a window leaves playback like the close button (progress is saved); `/` and Ctrl+K leave playback and open 搜尋 with the input focused.
- Tests for each item. The mouse keeps working (existing click tests stay).

Out of scope: new screens or 設定 content, new shortcuts beyond the list, remapping player keys, screen-reader support, new dependencies, workflow or release changes.

## Tests

Unit tests (no window) for the key policy:

- Each of the three Enter key codes maps to "activate"; Space maps to "activate".
- Esc maps to "back"; the four arrows map to up, down, left and right; Tab and Shift+Tab map to next and previous.
- `/` and Ctrl+K (also with Shift) map to "search". Ctrl, Alt or Win with any other key maps to nothing.

UI tests (real FLTK windows, like the existing `*_windows_test.go`):

- Esc: on Home nothing changes and the app stays open; on 搜尋, 追蹤, 歷史紀錄, 時間表 and 設定 Home shows with 首頁 focused; on the detail screen it returns to the page it was opened from.
- `/` and Ctrl+K: from every screen, 搜尋 shows and the input has focus. With the input focused, `/` is typed and Ctrl+K selects the query without changing it.
- Row list (追蹤 and 歷史紀錄): Down and Up move between rows, and the focused row is inside the visible scroll area; Right and Left move between a row and its icon; Left from a row focuses the page's navigation button.
- Focus after rebuilds: the focused Home row is focused again after a reload; returning from the player focuses the destination's navigation button; the focused search result is focused again after stored or online results arrive.
- Routes: Down in the search input focuses the first result; Down on 重新整理 focuses the first card; Up on the first episode focuses 返回.
- Mouse: the existing click tests keep passing (open a row, play, remove, follow).

Player tests (no window, like `player_keyboard_windows_test.go`):

- In a window with no list or menu open, Esc calls the leave callback once.
- In fullscreen, or with the episode list or the menu open, Esc keeps its current behavior.
- `/` and Ctrl+K call the player's Search callback, which ends playback and opens 搜尋. Ctrl+K is no longer in the list of modified keys the player ignores (owner decision above).

Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Launch the app, find an anime, open it and play an episode using only the keyboard.

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 `F31FC64B…BD91`; libmpv DLL matches the pin; existing `data/` kept).

1. 開啟程式，不要碰滑鼠。焦點應在左側「首頁」，有明顯的框線。
2. 按 `/`。應切到「搜尋」，游標在搜尋框。輸入作品名稱，按 Enter 搜尋。
3. 按 ↓ 到第一個結果，用 ↑↓ 選作品，按 Enter。應開啟作品詳情。
4. 按 ↓ 到集數清單，用 ↑↓ 選一集，按 Enter 或空白鍵。應開始播放。
5. 播放中按空白鍵暫停再繼續；按 Esc。應停止播放並回到首頁，焦點在「首頁」，進度已記住。
6. 在首頁、追蹤、歷史紀錄、時間表各按一次 Esc：首頁不應關閉程式，其他頁應回到首頁。任一頁按 Ctrl+K 應開啟搜尋並選取搜尋框文字。
7. 在歷史紀錄用 ↑↓ 移動，清單應自動捲動讓焦點那列可見；← 回到左側選單。最後用滑鼠點幾個按鈕，應照常運作。

## Status

- Implementation done 2026-10-20 (not committed): one key policy in `apps/desktop/fltkhome/keys_windows.go`; Home focus restore in `home_focus_windows.go`; player Leave and Search callbacks. UI tests send real key messages to the window (`SendMessageW`); Ctrl+K is tested through the handlers because modifier state cannot be sent that way. Gap 1 follows FLTK's documented default (an unhandled Esc runs the window callback); confirm it on the test build.
- Owner decisions 2026-10-20: player Esc leaves playback; `/` and Ctrl+K in the player leave and open 搜尋; Esc on 追蹤, 歷史紀錄, 時間表 and 設定 goes Home.
- TypeSafe doc-quality: Tests clarity 0.97 rewritten to 1.62/2; REVIEW-band answers adjudicated by reading the sections (concrete, no filler). Evidence in `artifacts/evidence/loop34/`.
- Local: gofmt and vet clean; full native suite and race green (nuwen msvcrt MinGW gcc 15.2), before and after the review fixes. Logs in `artifacts/evidence/loop34/`.
- Independent review: no MUST_FIX. SHOULD_FIX fixed: after `/` from a detail opened elsewhere, the next result's 返回 went to the old page (`showSection` now resets the origin; regression test added); Home arrow focus closures read rebuilt slices without bounds checks (pre-existing; bounds added). NITs fixed: unused key handlers before Home's own, duplicated go-Home step, double search render on return from the player, a comment on `/` for layouts that need Shift, this file's player test wording. Not changed: replacing every literal `4` with `SearchSection` and moving the window handler out of the search view (wider churn than this loop).
- Test changes: `TestPlayerRejectsModifiedShortcuts` no longer expects Ctrl+K to be ignored, because the owner made Ctrl+K quick search in the player; the new `TestPlayerSlashAndCtrlKOpenSearch` checks Ctrl+K searches and never plays, and Alt+K and Ctrl+Alt+K stay ignored. Detail episode key tests pass the same raw keys through `classifyKey`. Owner to confirm.
- TypeSafe scope-check: criteria_covered 1.70/2; in_scope 1.63/2 (conf 0.46, REVIEW) and verifier_weakened 0.67 (REVIEW). Adjudicated by reading the diff: the out-of-slice items are the two keyboard-path bug fixes above; the only changed assertion is the owner-decided Ctrl+K one, now covered by a stronger test. Evidence in `artifacts/evidence/loop34/`.
- Owner check (2026-10-20): PASS for steps 1-7 (TypeSafe owner-report HUMAN_PASS, conf 1.00; no new issue).
- Owner approval 2026-10-20: the Ctrl+K player test change, and commit and push to main.
