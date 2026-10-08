# Player keyboard follow-up

## Contract

The owner accepted Loop 29 and selected five-second arrow seeks plus ten-second J/L seeks on 2026-10-08. This bounded Windows player correction does not change the planned Loop 30 / Phase 24 detail and episode UI scope.

The owner clarified that focus-independent seeking is the primary defect: arrow keys sometimes select controls instead of seeking, and after a reload-like interruption they default to the Play button and stop seeking. Outside open lists, seek keys must invoke only the existing seek callback and remain usable after auto-hide, focus resets and playback state updates. They must not activate Play, select an episode, navigate or move focus between controls. A reload-like observation is not yet classified as an actual reload; trace the production seek path before changing playback lifecycle code.

| Input | Required behavior |
| --- | --- |
| Space / K | Play/pause outside an open menu or episode list; Space retains selection inside an open list. K remains a direct playback shortcut. |
| Left / Right | Relative seek by -5 / +5 seconds outside open lists, regardless of focused playback control. |
| J / L | Relative seek by -10 / +10 seconds. |
| Up / Down | Volume +5 / -5 percent outside open lists, clamped to 0..100. |
| M | Mute by setting volume to zero; unmute restores the last nonzero volume, defaulting to 100. |
| F | Toggle fullscreen. |
| Escape | Dismiss an open episode list/menu first; otherwise exit fullscreen. |
| Tab / Shift+Tab / Enter | Retain visible control focus and activation; open lists retain arrow navigation and selection. |
| Held keys | Seek, volume and list navigation may repeat. Pause, mute, fullscreen, Escape and Enter/Space selection act once per press. Key release and focus loss reset suppression. |
| Modified keys | Ctrl/Alt/Meta combinations must not invoke ordinary playback shortcuts. Shift+Tab retains reverse traversal; uppercase letters remain supported. |

Mouse controls, playback action serialization, saved progress, episode selection and lifecycle behavior must remain intact. Use existing typed player callbacks; no dependency, workflow, provider, schema or arbitrary MPV-command change.

## Verification and boundaries

Deterministic native tests cover shortcut dispatch across focused controls, exact seek increments, volume bounds/mute restoration, list priority, modifier rejection, repeat/release/focus-loss behavior, and existing Tab traversal. Independent code-quality and input/lifecycle review is required. Final-state Windows CI verifies the production build, native regression, race detector and existing player checks. The known local native cgo failure remains closed; no host run counts as PASS.

Deliver the exact green-CI executable and a versioned portable ZIP before requesting owner acceptance. The user checks visible play/pause, seek, volume/mute, fullscreen/Escape, Tab/Enter and episode-list arrows on actual playback. The follow-up remains `NEEDS_HUMAN` until that check passes. Loop 29 remains `PASS`.

Limit correction to five causal iterations; stop repeated identical failure without progress. Preserve unrelated `AGENTS.md`, `.codex/` and portable data. Prior repository-scoped main commit/push and exact executable/package delivery authorizations apply; no release or workflow mutation is authorized.
