<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 36: Autoplay next episode

Status: owner check passed 2026-10-21 (owner approved the start the same day); waiting for commit, push and GitHub Windows CI.

## Goal

When an episode ends, the next one starts in the same player.

## Current behavior (inventory 2026-10-21)

- Switching episodes in the player (episode list, `SelectEpisode` in `apps/desktop/main_fltk_windows.go`) calls `service.Play`. The backend sees an open session and calls `core.App.SwitchEpisode`: it saves the current progress, resolves the new episode, then the libmpv adapter opens a new proxy session, sends `loadfile <url> replace` to the same libmpv instance and closes the old proxy session only after the load succeeded.
- The engine (`apps/desktop/fltkengine`) reads mpv's end-file reason: `eof` becomes `Ended`, `error` becomes `Failed`, `stop`, `quit` and `redirect` become `Stopped`. These events reach core progress tracking only; the UI hears about failures but never about a normal end. Nothing happens after an episode ends.
- Core marks an episode completed on `Ended` or at 90 % watched, and saves at once.
- After a normal end, libmpv unloads the file (idle). Its position and duration become unavailable, so the engine's next tick reports duration 0. A switch that runs before core has seen `Ended` would then not save the finished episode as completed.
- The player's episode list is the backend's ordered list (`orderEpisodes`, ascending number).
- The stored setting `AutoplayNext` already exists and defaults to enabled (`core.DefaultSettings`); no screen changes it yet.

## Scope

In scope:

- Engine: a normal end is mpv's `eof` for the current file within 30 s of its known duration (`endTolerance`). An `eof` farther from the end is a stream that stopped early: it is reported like any playback failure (此集暫時無法播放…), its real position is kept, and nothing advances. A file whose stated duration is more than 30 s longer than its content therefore never autoplays.
- Engine: report a normal end of the current file to the UI once, through a new end handler next to the existing failure handler. After a normal end, the engine keeps reporting the finished file's end (position = duration) until the next load, so a switch always saves it as completed.
- UI: a small `playerAutoplay` (`apps/desktop/player_autoplay.go`) decides what happens at a normal end:
  - If the stored `AutoplayNext` setting is `enabled` and a later episode exists in the ordered list, switch to it through the same path as choosing it in the episode list (same libmpv instance, same window, resume rules unchanged).
  - Otherwise (last episode, setting not enabled, or settings unreadable) stop cleanly: end the playback session (proxy session and progress tracking closed, progress saved) and leave the player showing the stopped episode; Play starts it again.
  - A switch that fails to resolve shows one message (無法播放下一集，請重試。) and then stops cleanly. It is never retried automatically; each end is acted on once.
- The one place for the on/off decision is the stored `AutoplayNext` setting, read when an episode ends. Loop 37 only adds the control that saves it.

Added after owner check 2 (owner decisions 2026-10-21):

- Home 繼續觀看 picks each anime's most recently played episode that was started or finished. A finished one shows 集數：12 · 已看完, and 繼續播放 plays it from the beginning. An episode only opened (position 0, not finished) still does not hide the series. Before, a finished latest episode fell back to an older unfinished one (owner saw 第 08 集 after finishing 第 12 集).
- Player episode list: opening it scrolls to and marks the playing episode, a scroll bar shows when there are more than 8 episodes, and rows read 第 12 集. After an autoplay switch the list shows the new episode. Before, it always opened at the top with 8 rows (00-07) and no sign of more, so the owner took 第 07 集 for the last one.

Out of scope: playlist editing, a settings screen or control, a countdown or "next episode" overlay, new backend actions, changes to resume rules.

## Tests

- Engine (real libmpv, `testdata/black-60s.mp4` started near its end): a normal end calls the end handler once with the load's generation, and a snapshot afterwards reports position = duration. Replacing the file while it plays (`stop`) and a failed load (`error`) do not call it. 30 load-to-end cycles in one engine: the goroutine count after cycle 30 is at most 2 above the count after cycle 10, and process private memory (Windows `PrivateUsage`) after cycle 30 is less than 32 MiB above the value after cycle 10. The measured values are printed with `-v` and saved as evidence.
- Engine unit (no mpv, `TestEndsNormallyOnlyForEOFOfTheCurrentFile`): end-file reasons `stop`, `quit`, `redirect` and `error` are not a normal end; neither is `eof` of a pending load. `eof` exactly 30 s before the end is normal; 30 s + 1 ns before, at 40 %, or with no duration, it is an early end (reported as a failure).
- `playerAutoplay` unit tests with fakes: a normal end advances to the next ordered episode; the last episode stops cleanly; setting `disabled` or unreadable stops cleanly; a second end for the same generation is ignored; an end whose generation is no longer playing (user switched or left) does nothing; a failed switch reports once, stops cleanly and does not switch again.
- Core: after `Ended`, a switch saves the finished episode with `Completed` true; a switch whose resolve fails calls the source once, leaves the current session open and does not load anything.
- Adapter with the real proxy (64-session limit) and a fake bridge: 200 switches all succeed (a leaked proxy session would hit the limit), exactly one proxy session stays open, and the goroutine count returns to its start after Close.
- Home (`model_test.go`): `TestSelectHomeRowsUsesLatestStartedOrFinishedEpisodePerAnime` (replaces `...UsesOnlyLatestResumableEpisodePerAnime` after the owner changed the rule; its earlier cases are kept, plus finished 第 12 集 newer than unfinished 第 08 集 shows 12) and `TestHomeShowsFinishedEpisodeAsWatchedAndReplaysItFromTheStart` (caption 集數：12 · 已看完, play request from 0).
- Player episode list (`fltkplayer/episode_list_windows_test.go`): with 13 episodes and 第 11 集 playing, opening shows rows 05-12 with the cursor on 11; clamping at both ends; scroll bar thumb geometry; click and bar mapping to an index.
- Existing playback, reentry and tracking tests still pass. Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 `A408D1ED…FAEE`, build 2 after owner check 2; libmpv DLL matches the pin; existing `data/` kept).

1. 開啟一部至少有兩集的動畫，播放其中一集（不是最後一集）。點右上角的選集清單：清單應直接捲到目前這集並標示它；集數超過 8 集時右側有捲軸；每列寫「第 NN 集」。再點一次關閉清單。
2. 把進度條拖到接近結尾（約剩 20 秒），等這集播完：下一集應在同一個播放視窗自動開始，不會開新視窗；右上角顯示下一集的集數，打開選集清單時標示的也是下一集。
3. 播放該動畫選集清單中的最後一集（例如一拳超人 第三季的第 12 集），同樣拖到接近結尾。播完時播放應停止，播放視窗保持開啟，不會出現錯誤訊息，也不會開始播放其他集。
4. 按 Esc 回到首頁：「繼續觀看」中這部動畫應顯示「集數：12 · 已看完」。按「繼續播放」會從頭播放第 12 集；按 Esc 回首頁。
5. 播放另一部動畫的某一集到一半就按 Esc：首頁「繼續觀看」顯示那一集和上次播放位置。

## Status

- 2026-10-21: inventory done; scope written.
- Implementation done (not committed): engine end handler, normal-end vs early-end classification (`endTolerance` 30 s), Ended/Stopped events carry the end position; `playerAutoplay` (`apps/desktop/player_autoplay.go`) with the single on/off decision `autoplayEnabled` reading the stored `AutoplayNext`; main wiring (`selectEpisode`, `endPlayback`, `SetEndHandler`).
- Tests: as listed above; engine 30-cycle measurement (3 runs): goroutines 4/4/3 after cycle 10 and 3/3/3 after cycle 30; private memory 662.7/640.0/656.7 MB after cycle 10 and 610.8/591.6/612.8 MB after cycle 30. No existing test was weakened; one Home test was replaced because the owner changed the rule it encoded (see Tests).
- Local: gofmt and vet clean; full native suite and race green (nuwen msvcrt MinGW gcc 15.2), after the final review fixes. Evidence in `artifacts/evidence/loop36/`.
- Independent review: first pass one MUST_FIX (once-per-end guard keyed by a load generation that restarts in each playback session) and three SHOULD_FIX (stale settings read across sessions; Ended event carried position 0; a stream cut short counted as a full watch) plus a NIT (selection set after awaiting cleared). TypeSafe review-triage rated all five MUST_FIX; all fixed with tests. Re-review: no MUST_FIX; two SHOULD_FIX (a cut-short stream left the player frozen without a message; a failed manual switch lost the playing file's end and failures) fixed. Final check: no MUST_FIX or SHOULD_FIX; accepted NITs: an eof with no known duration is reported as a failure; a very fast double episode selection followed by a failed switch can still lose the old file's end; the main wiring has no automated test (owner check).
- TypeSafe: doc-quality on this file (Tests clarity raised by naming bounds); scope-check final in_scope 1.79/2, verifier_weakened 0.13, criteria_covered 1.61/2 (conf 0.41, REVIEW; adjudicated by reading: every listed test exists; only the main wiring is uncovered by automation and is in the owner check).
- Owner check 1 (2026-10-21): "1. 會自動播放 2. 會自動跳回重頭開始播放同一集 3. 我沒看到哪裡會顯示已完成". TypeSafe owner-report HUMAN_FAIL (conf 0.99). Diagnosis (`artifacts/evidence/loop36/owner-check-1-diagnosis.txt`): the database shows ep 07 saved completed and ep 08 starting 8 s later, so step 3 played ep 07, which is not the last episode (Anime1 lists 00-12); autoplay moved to ep 08, whose shared opening looked like a restart. A test copy on the same data shows 第 02 集 已完成 and 第 07 集 已完成 on the detail page; the step did not say how to reach it (Home and 歷史紀錄 cards play directly). No code defect found; steps 3 and 4 reworded for a second check.
- Owner check 2 (2026-10-21): "1. player上的dropdownlist沒有顯示完整 也沒有顯示drop bar, 容易誤會正確的集數 2. 我播放第12集, 不論是播一半 播完 首頁都停留在8 3. 我播完12集後 會停止, 但首頁沒有顯示已完成". TypeSafe owner-report HUMAN_FAIL (conf 0.99, new issue 0.82). Diagnosis (`owner-check-2-diagnosis.txt`): the player list opened at 00-07 with no scroll bar while 第 11 集 played; a half-watched episode does show on Home (checked on a test copy), but a finished newest episode fell back to 第 08 集 by the earlier Home rule. Owner decisions (2026-10-21): show the newest episode as 已看完 on Home; fix the episode list in this loop. Both done (scope bullets above).
- Review of the follow-up: one MUST_FIX (an autoplay switch did not update the player's shown episode; `selectEpisode` now sets it) and one SHOULD_FIX (player labels duplicated Home's episode label rule; now `fltkhome.EpisodeLabel`) fixed; accepted NITs: the C overlay repeats the list geometry literals (comment ties them to `episode_list_windows.go`); a wheel scroll does not move the keyboard cursor; the ellipsis may run up to two character widths over its budget.
- Owner check 3 (2026-10-21, test build 2): "Owner check 3 passed: all 5 steps behaved as described. The 30 s end rule is OK." TypeSafe owner-report HUMAN_PASS (conf 1.00). The 30 s normal-end rule is accepted.
- Local after the last fixes: gofmt and vet clean; full native suite and race green (`full-suite-4.txt`, `full-suite-race-4.txt`). Next: commit and push after owner approval, then GitHub Windows CI.
