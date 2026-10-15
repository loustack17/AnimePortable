<!-- SPDX-License-Identifier: MPL-2.0 -->

# Loop 31: Following

Status: OWNER PASS (2026-10-14); waiting for commit, push and CI.

## Goal

The user can follow and unfollow anime and see which have new episodes.

## Scope

In scope:

- Follow and unfollow from the anime detail screen (a 追蹤 / 取消追蹤 button).
- The 追蹤 section: one row per followed anime with title, latest episode, watched episode and a 有新集數 indicator. A row opens the detail screen; 取消追蹤 removes it.
- Home 追蹤中 shows the same state in short form.
- The list is built only from local data (local anime IDs, stored episodes, history), so it loads at once and still shows when Anime1 is unreachable. Opening 追蹤 refreshes episode lists in the background through the existing Episodes action, one anime at a time with a 15 s limit each; it is skipped for 10 minutes after a fully successful refresh, and 更新集數 forces one. On failure the last stored state stays, with a note that says how many series could not be updated. No new backend action is added.
- Episode numbers are stored with episode mappings (SQLite migration 0004), so "latest" is the highest episode number, not Anime1's list order (Anime1 lists some series newest first).

Out of scope: notifications, Schedule, History page, new dependencies, workflow or release changes.

Indicator rules (core `ListFollowing`):

- Latest episode: the highest numbered stored episode. Episodes without a plain number (specials) are not "latest".
- Watched episode: the highest numbered episode with any playback history; rewatching an older one does not lower it.
- 有新集數: the user has watched at least one episode and no watched episode is numbered at least the latest one (numbers, not IDs, because one number can have several local IDs). Never-watched series show 尚未觀看 instead.
- Stored episodes are never deleted (their IDs are history keys), so an episode removed from Anime1 can stay the latest until a higher number appears.

## Tests

- SQLite: migration 0004 upgrade keeps rows; mapping number is saved, returned and filled in for older rows; an empty number never overwrites a stored one.
- Core: `ListFollowing` uses only the store; latest by number; watched does not regress; indicator cases (watched latest, watched older, never watched, unnumbered only); local errors and cancellation propagate.
- Backend: Following survives reopening the store; still returns stored state when the source fails; loading Episodes updates the latest episode; Detail reports the follow state; Follow/Unfollow round trip.
- UI: 追蹤 page shows stored rows (new episodes first), empty state, refresh failure and partial-failure notes, no repeat refresh right after success, unfollow, detail 追蹤 button, Back returns to 追蹤.
- UI models: row text and indicator for each state; Home summary.
- Full native suite and race locally; GitHub Windows CI after the approved push.

## Owner check

Follow an anime, restart the app, and see it still followed with the correct episode state.

Test build: `artifacts/test-build/AnimePortable.exe` (uncommitted worktree build, EXE SHA-256 see the latest owner check entry; libmpv DLL matches the pin). Data backup before migration 0004: `artifacts/evidence/loop31/data-backup-before-0004/`.

## Status

- Local: full native suite green; race green except a pre-existing intermittent failure in `apps/desktop/backend/failurelog_test.go` (one log line dropped when a Windows file replace fails; also fails on clean HEAD 8f61bff; TypeSafe FLAKY, evidence in `artifacts/evidence/loop31/`). Not changed in this loop.
- Independent review: no MUST_FIX after fixes (number-based 有新集數, refresh interval, failure wording, scroll and focus). Removed core source-based tests judged NOT_WEAKENED.
- Owner check 1 (2026-10-14): HUMAN_FAIL. The owner wants follow as an icon (heart, pin or star) at the far right of the detail button row, aligned with the right edge of the episode list, not a text button in the middle. Fix: a star icon button (outline = not followed, filled accent = followed, tooltip 追蹤/取消追蹤) placed flush with the episode list. Build EXE SHA-256 `357B2062…43AC`.
- Owner check 2 (2026-10-14): HUMAN_FAIL. The star stuck out past the episode rows (it used the scroll area's edge, not the rows' edge, which are inset 19 px), and 取消追蹤 on the 追蹤 page was still text. Fix: one shared episode-row span for rows and the icon; the 追蹤 page unfollow is a filled star aligned with 更新集數. Tests now check both right edges at three window sizes. Build EXE SHA-256 `123A38E5…1E93`.
- Owner check 3 (2026-10-14): PASS for the icon layout: stars aligned, follow and unfollow by star work, alignment holds when resizing (owner: all 4 steps passed). Restart persistence not yet confirmed by the owner.
- Owner check 4 (2026-10-14): PASS. After a restart the anime is still followed with the same episode state.
- Pending: commit and push approval, GitHub Windows CI.
