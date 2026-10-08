# Player keyboard follow-up

## Contract

### Owner regression report after dc2e361

The owner reported playback failure/stutter, ineffective Windows media keys, and Space failing to toggle playback while Play/Pause has focus. The delivered follow-up fails its owner gate and is under diagnosis. Preserve the accepted default-progress/Tab/contextual-arrow behavior. Scope is limited to these three regressions; planned Loop 30 is unchanged.

Before changing playback policy, identify the failing episode/action and distinguish load failure, seek buffering, control-queue delay and rendering stalls using source and reproducible evidence. Do not tune cache/decoder options or add retries based on the symptom alone. Acceptance requires successful initial/Continue/reentry playback and repeated seeks without a control-induced stall, supported Windows media Play/Pause input reaching the existing typed callback once, and Space/Enter toggling the focused Play/Pause control without navigating or selecting an unintended episode. Preserve actual open-list selection behavior. Hardware media keys and the Windows media control panel are distinct; confirm which input the owner expects before adding platform integration beyond focused-window input.

Verification requires focused behavioral/native integration regressions, independent code-quality/platform/lifecycle review, exact final Windows CI, exact executable/ZIP with unchanged portable data, and the owner reproducing these three actions. The local native cgo circuit breaker remains closed. Prior green CI does not establish that these reported runtime defects are fixed. Correction budget is 3/5 used, 2 remaining, including the earlier fixture correction and two independent native review repair rounds. Evidence-backed diagnosis precedes mutation.

The owner accepted Loop 29 and selected five-second arrow seeks plus ten-second J/L seeks on 2026-10-08. This bounded Windows player correction does not change the planned Loop 30 / Phase 24 detail and episode UI scope.

The owner's latest clarification supersedes the initial global-arrow proposal: default focus is the progress bar; Tab/Shift+Tab selects the control target; arrows operate that target without switching focus. New playback and reentry start on progress. Auto-hide, pointer movement, seek completion and ordinary playback state updates retain the selected target. A missing focus falls back to progress. A reload-like observation is not classified as an actual reload; production seek traces to MPV relative seek without `loadfile`.

| Input | Required behavior |
| --- | --- |
| Space / K | Play/pause outside an open menu or episode list; Space retains selection inside an open list. K remains a direct playback shortcut. |
| Left / Right | Progress focus: relative seek -5 / +5 seconds. Volume focus: volume -5 / +5 percent. Other buttons: consume without moving focus or invoking an action. |
| J / L | Relative seek by -10 / +10 seconds. |
| Up / Down | Volume focus: volume +5 / -5 percent, clamped to 0..100. Open lists: navigate the active list. Other controls: consume without moving focus or invoking an action. |
| M | Mute by setting volume to zero; unmute restores the last nonzero volume, defaulting to 100. |
| F | Toggle fullscreen. |
| Escape | Dismiss an open episode list/menu first; otherwise exit fullscreen. |
| Tab / Shift+Tab / Enter | Retain visible control focus and activation; open lists retain arrow navigation and selection. |
| Held keys | Seek, volume and list navigation may repeat. Pause, mute, fullscreen, Escape and Enter/Space selection act once per press. Key release and focus loss reset suppression. |
| Modified keys | Ctrl/Alt/Meta combinations must not invoke ordinary playback shortcuts. Shift+Tab retains reverse traversal; uppercase letters remain supported. |

Mouse controls, playback action serialization, saved progress, episode selection and lifecycle behavior must remain intact. Use existing typed player callbacks; no dependency, workflow, provider, schema or arbitrary MPV-command change.

## Verification and boundaries

Deterministic native tests cover default progress focus, contextual arrows after Tab, retained focus after auto-hide/pointer movement/state updates, exact seek increments, volume bounds/mute restoration, list priority, modifier rejection, repeat/release/focus-loss behavior, and existing Tab traversal. Independent code-quality and input/lifecycle review is required. Final-state Windows CI verifies the production build, native regression, race detector and existing player checks. The known local native cgo failure remains closed; no host run counts as PASS.

Deliver the exact green-CI executable and a versioned portable ZIP before requesting owner acceptance. The user checks visible play/pause, seek, volume/mute, fullscreen/Escape, Tab/Enter and episode-list arrows on actual playback. The follow-up remains `NEEDS_HUMAN` until that check passes. Loop 29 remains `PASS`.

## Owner playback check

State: `NEEDS_HUMAN`. Final Windows CI37730888867/job113159504236 passed all gates for `dc2e361`; artifact11530280438 is delivered at `artifacts/loop29/AnimePortable.exe` and `artifacts/loop29/AnimePortable-player-keyboard-dc2e361.zip`. Hashes/data preservation and historical failures are in `docs/IMPLEMENTATION_STATUS.md` and `artifacts/evidence/player-keyboard/dc2e361-test-object.json`. Independent final reviews found no MUST_FIX. Local native cgo remains blocked; Windows CI provided deterministic verification. Rollback uses the retained `AnimePortable-loop29-2d4bf48.zip`. One test-fixture correction was needed; original seek bounds remain unchanged.

1. Start an episode. Without clicking a control, Left/Right should seek on the progress bar. Wait for controls to hide, move the mouse, and seek again; the target should remain progress. Repeated seeks must not switch to Play or another button.
2. Press Tab once to select Play/Pause. Arrows should keep that selection and leave playback/progress unchanged; Enter and Space should each play/pause once per press. Shift+Tab returns to progress and Left/Right seeks again.
3. Use Tab to select volume. Arrows should change volume without seeking. Hide/reveal the controls and confirm volume stays selected. M restores the prior volume after mute; Space/K play/pause; J/L seek; F/Escape enter/leave fullscreen.
4. While the player is active, test keyboard hardware Play/Pause, volume up/down and mute. Repeat after fullscreen and Home/Continue. The Windows system media panel is outside this focused-window scope.
5. Open the episode list. Up/Down selects rows and Enter/Space plays the selected episode. The new episode starts with progress focus. Return Home and Continue; progress focus should again be the default, with saved history intact.

If a reload-like interruption remains, report whether the episode/time resets or the picture only pauses briefly, and whether the selected target changes. The visual cause remains unverified until actual playback confirms it.

The owner corrected the contract after commit `924b1e8`; that global-arrow build is superseded and must not be delivered. The clarified contract starts a new five-iteration causal correction budget; stop repeated identical failure without progress. Preserve unrelated `AGENTS.md`, `.codex/` and portable data. Prior repository-scoped main commit/push and exact executable/package delivery authorizations apply; no release or workflow mutation is authorized.
