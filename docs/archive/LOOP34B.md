<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 34b: Follow from 歷史紀錄 and the player

Status: PASS (owner check passed 2026-10-21). Commit 18b6c6a, pushed to main with Loop 35 and the resize hotfix in ef97b6c; GitHub Windows CI 39532702396 green. Owner approved the contract on 2026-10-20: history star between title and ✕, player star in the top bar left of 選集, numbered Loop 34b.

## Goal

The user can follow (追蹤) or unfollow an anime from the 歷史紀錄 page and from the player while watching, not only from the detail screen. Owner's reason: people try an episode first and then decide whether to follow.

## Current behavior

- Following is only possible on the detail screen (star at the right of the button row, `toggleDetailFollow`). The 追蹤 page can only unfollow.
- 歷史紀錄 rows are a title/status button plus one remove (X) icon (shared row list, one icon per row). The page does not load the follow list.
- The player's on-video controls (auto-hide) have no follow control and no link to follow state.

## UI proposal

歷史紀錄 row: `[title / status] [☆] [✕]`

- A star icon sits between the title and the remove icon, same size and style as the detail and 追蹤 stars (`styleFollowIcon`): outline when not followed, filled in the accent color when followed.
- Click, Enter or Space toggles 追蹤 / 取消追蹤. The tooltip names the action (追蹤 or 取消追蹤).
- Keyboard: Left/Right move title → star → ✕; Up/Down keep the column, as today.
- A failure shows the existing message 「無法更新追蹤，請重試。」; the star keeps its old state.

Player top bar: `[≡]  …  [☆] [選集 ▾] 1080p`

- A star sits in the top bar just left of the episode selector, the same height as the selector. It hides and shows with the other controls.
- Outline star (Segoe MDL2 FavoriteStar) when not followed, filled accent star when followed. Hidden until the follow state is known.
- Click, or Enter/Space when focused, toggles. It joins the Tab ring just before the episode selector. No new letter key.
- A failure shows a message box with the same text as above, like the player's other error boxes.

Also: the 追蹤 page's empty text becomes 「尚未追蹤任何作品。在作品頁、歷史紀錄或播放中按星號即可追蹤。」

## Scope

In scope:

- The shared row list takes one or more icons per row (追蹤 keeps one, 歷史紀錄 gets two). Keyboard routes generalize from two columns to N.
- 歷史紀錄 loads the follow list with the existing `Following` action and toggles with `Follow` / `Unfollow`.
- Player: a follow control (`Callbacks.ToggleFollow`, `State.Following`), drawn in the overlay; the app wires it to `Following`, `Follow`, `Unfollow` for the anime being played.
- Reuse `styleFollowIcon` and the follow error message (exported as `FollowErrorMessage` for the player). Detail, 追蹤 and 歷史紀錄 share one `followPending` guard; the player, a separate window, keeps its own one-change-at-a-time guard (Home is hidden while it plays). Home and 追蹤 show the new state the next time they load.

Out of scope:

- New backend actions (the `main_test.go` allowlist stays as it is).
- Follow buttons on Home rows, 時間表 cards or search result rows.
- Any change to what following means (recent updates, 更新集數).

## Acceptance

1. 歷史紀錄: each row shows a star whose fill matches whether the anime is followed; toggling it follows or unfollows, and 追蹤 shows the change.
2. 歷史紀錄 keyboard: Right from the title reaches the star, Right again the ✕; Up/Down stay in the column; focus survives the reload after a toggle or removal.
3. Player: the star shows the follow state of the anime being played, toggles it by click and by Enter/Space, is in the Tab ring, and auto-hides with the controls.
4. Errors: a failed toggle shows the message, leaves the old state, and a second click while one is pending does nothing.
5. No new backend action; the allowlist test is unchanged.
6. Full native suite and `-race` pass; independent review has no MUST_FIX; TypeSafe scope check passes.

## Tests (written first)

- Row list: N-icon placement and keyboard routes (title ↔ star ↔ ✕, Up/Down per column).
- 歷史紀錄: star state from `Following`; toggle calls `Follow` / `Unfollow` with the row's anime ID; failure message; pending guard.
- Player: hit test and Tab order for the new control; `activate` calls `ToggleFollow`; hidden while the state is unknown.
- App wiring: follow state loaded for the played anime; toggle calls the right action.

## Owner check

With the build in `artifacts/test-build/` (your `data/` kept):

1. Open 歷史紀錄. Each row has a star between the title and ✕. Click the star of an anime you do not follow: it fills. Open 追蹤: the anime is listed.
2. Back in 歷史紀錄, click the filled star: it empties; 追蹤 no longer lists it.
3. Play any episode. Move the mouse: a star shows left of 選集. Click it: it fills. Stop playback and open 追蹤: the anime is listed.
4. Play again, press Tab until the star is ringed, press Enter: it empties.

## Results (2026-10-20)

- Tests: `CGO_ENABLED=1 go test -count=1 ./...` and `-race` green, on this change alone and combined with the uncommitted resize fix.
- Independent review: first pass 1 MUST_FIX (Space on the player star) and 3 SHOULD_FIX (stale follow-state loads, a test that did not catch them, star left hidden after a dropped load); all fixed with tests that fail without the fix. Re-review: no MUST_FIX.
- TypeSafe scope check (artifacts/evidence/loop34b/scope-check.json): in_scope 1.75/2 (conf 0.62), criteria_covered 1.87/2 (conf 0.79), verifier_weakened 0.11.
- Owner check: the combined build (resize fix + this loop) in `artifacts/test-build/`. Owner reported 「RWD 和追蹤星號都正常。」
- Merge note: the resize fix's `window_resize_windows_test.go` expects two buttons per history row; when this loop lands after it, that test uses `historyList().button(0, 1)` (star, also clicked) and `button(0, 2)` (remove).

## Files (expected)

`apps/desktop/fltkhome/row_list_windows.go`, `history_view_windows.go`, `following_view_windows.go` (row list call and empty text only), their tests; `apps/desktop/fltkplayer/player_windows.go`, `overlay_windows.go`, `overlay_windows.cc`, tests; `apps/desktop/main_fltk_windows.go`. In `window_windows.go` only one new field next to `historyItems` in the `view` struct (the resize fix and Loop 35 edit other hunks of that file). Not `search_view_windows.go`, `schedule_view_windows.go` or `model.go`.
