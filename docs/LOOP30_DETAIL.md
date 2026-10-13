# Loop 30 — Windows anime detail and episode UI

## Contract

- Scope: Phase 24 under ADR-023: cover, title/native title, synopsis, season/year, studio, episode count when metadata is available; ordered episodes, Play/Continue, Space playback and Enter selection, visible current/progress state. Existing Search preview becomes the detail screen. No Schedule/Following/new settings or platform migration.
- Baseline: main 90a7fc9985bd5ab17b1198333567324fd69d2e8a plus retained uncommitted Loop29 closure docs. Preserve AGENTS.md, .codex/, all prior documentation and portable data.
- Harness: Codex App; OMO configured explorer/worker/reviewer; one bounded implementation worker, root integration/docs/verifier. No nested agents or overlapping writers.
- Boundary: workspace-write repository only; sandbox disposable cache allowed; GitHub CI reads only. Owner subsequently authorized scoped commit/push after verification; workflow/release mutation remains outside scope. BWS key read and necessary TypeSafe state/code uploads explicitly authorized by user; key stays in memory.
- Architecture: UI uses existing typed Detail/Episodes/History/GetCover actions. Only the necessary existing GetCover empty-URL absence classification changes in backend; no core/provider/schema/dependency policy changes. Metadata absent is neutral, cover loads lazily through existing secure backend (not directly by URL). No new cover cache scope.
- Acceptance: FUNC-004/005, UX-001..005/008/010, relevant ARCH/SEC/PRIV/resource/LOOP and QUAL-001..012. Preserve Loop29 behavior. Detail open never autoplays. Selected episode/Continue use exact anime/episode identity and stored milliseconds. Completed/missing history is not resumable. Rows are bounded/paged and keep source order. Late responses after navigation/close are rejected; load failures have safe retry/error states. Image memory and native object lifetime are bounded.
- Deterministic verification: focused pure-Go model tests/vet, backend/contracts/architecture regression, formatting and diff check; native Windows tests/build/race remain required via capable isolated CI. Closed local cgo fingerprint is not retried. Native/user presentation gates must not be claimed from pure tests.
- Independent review: code quality/lifecycle/keyboard/cover review required. TypeSafe batches scope/lifecycle/verification rubrics separately; raw answers/probabilities/confidence retained. Semantic output does not replace tests or human gates.
- Human gate: prepared exact green-CI executable/ZIP then owner metadata/episode selection/Space/Enter/Continue/back/mouse checks. No Loop30 PASS before that evidence.
- Budgets: initial max corrective iterations5; owner latest docs-only completion instruction authorizes one additional bounded pass for R30-01/02 (maximum6), max repairs per identical fingerprint 2, max replans 1, unchanged flaky reruns 0, agent turns 12. Usage at start: 18% of five-hour window, 65% of weekly window remaining; token budget unspecified. Stop broadening; checkpoint partial verified work before exhaustion. No runtime TypeSafe feature is added.

## Verification state

LOCAL_VERIFIED / NEEDS_HUMAN. R30-01/02 are resolved in one owner-authorized bounded pass: proportional transparent cover letterbox; existing not-found sentinel for absent cover, neutral UI, genuine failure retry. Fresh simplify and scoped independent PASS_REVIEW; no unresolved MUST_FIX. Fresh CGO0 desktop/backend/architecture/contract regression, backend vet, formatting and diff checks PASS. Native Windows/build/race and owner gates remain NOT_RUN. Owner authorized scoped commit/push after final verification; exact Windows CI and owner gates still required. Loop30 is not PASS.

## 2026-10-13 recovery checkpoint

- State: IN_PROGRESS, not Loop30 PASS. Implementation is saved locally in detail_model.go/test, detail_cover_windows.go, detail_view_windows.go plus Search/window/theme integration and native Search tests. Worker/reviewer were interrupted by usage exhaustion before final sign-off.
- Verified: final pure Home/Search/appearance/play-entry/detail model tests and vet passed after model changes; backend/architecture/contracts passed. Existing source order, bounded pages, exact-ID Play/Continue, completed-history exclusion and milliseconds have tests.
- Review fixes already present: row Space selects the focused exact row before play; explicit selection restores replacement row focus; failed cover ends loading with unavailable text; placeholder invokes default drawing. New helpers were split out of Search view.
- Open confirmed keyboard MUST_FIX: handleDetailEpisodeKey arrows still move from stale model.selected rather than focused index (Tab row4 then Down should select row5); asynchronous Episodes/History rendering destroys row widgets and loses their focus. Fix and add native regressions, then re-review. Do not mark existing native tests PASS: they have not run.
- TypeSafe plan evaluation and raw probabilities are saved; required final source-state evaluation is pending. Necessary project state/code upload was explicitly approved; BWS key remains memory-only.
- Required next gates: final source simplify/independent code review, focused rerun, native Windows CI/build/race and exact acceptance package, then owner UI/playback acceptance. No commit/push is authorized. The old90a7 CI is baseline only, not Loop30 evidence.
- Preserve all Loop29 closure docs, AGENTS/.codex user edits, all existing packages and real portable data. Temporary Go verification cache remains available in artifacts/evidence/loop30/verification and should be removed only after final checks, preserving evidence.

### 2026-10-13 keyboard correction saved

The current source now starts arrows from the focused row index and restores focused episode identity after asynchronous row replacement. Cover decode work is moved off the UI thread, while native image creation/destruction remains on it. These changes are saved but await final worker simplify, independent re-review and final-state checks; the earlier open-findings list is a historical checkpoint, not a new PASS claim. Evidence: `artifacts/evidence/loop30/recovery-20261013.json`.

### 2026-10-13 playback-failure recovery finding

Independent review confirmed that selected Play/Continue cleared detail identity before invoking the existing playback callback. A synchronous backend Play failure returns the existing Home view without navigation, leaving Detail Retry/Play unusable. This finding is now assigned to the sole worker: cancel in-flight requests without clearing usable detail identity during playback entry; retain full cleanup on Back/navigation/close, and add a native regression. Do not claim this finding resolved until stable source is re-reviewed.

### 2026-10-13 final local verification and layout follow-up

Final pure tests/vet, desktop/backend/architecture/contract regression, formatting and diff checks passed; evidence and exact hashes are in `final-local-verification.json`. Simplify split detail view responsibilities and removed redundant state without behavior change. Independent code/lifecycle review found the prior keyboard, cover and playback-recovery defects resolved. Refreshed TypeSafe source judgments support scope1.88/confidence0.80, lifecycle1.98/0.98 and verification1.94/0.91; raw probabilities are retained in `final-source-judgments.json`.

A subsequent layout recheck confirmed the episode pane fix but found the four-button toolbar extends 9px beyond the detail group at supported minimum760x480. The sole worker is making the smallest width fix and extending the native bounds assertion to all toolbar controls. Corrections used: four bounded review/fix rounds (initial input/cover, focus/playback recovery, episode pane, toolbar). Native execution remains NOT_RUN, so no Loop30 PASS is claimed. Source/evidence snapshots must be reconciled after the toolbar correction.
## 2026-10-13 historical final local checkpoint (superseded by strict review)

- Implementation: Phase24 fields, lazy secure cover, ordered/paged episodes, selected Play/Continue, exact history milliseconds, completed-state/progress labels, Enter/Space/Tab, cancellation/stale-result protection and safe retry are implemented in eight fltkhome code/test files.
- Source review: PASS_REVIEW, zero unresolved MUST_FIX. Final fixes include focused-index keys, asynchronous focus identity, retained detail after playback failure, terminal cover errors, min-size toolbar/pane bounds, partial-scroll visibility and correctly scaled cover drawings. Simplify applied; no unrelated refactor.
- Local verification: final pure tests/vet and desktop/backend/architecture/contracts PASS. Pure file hashes still match tested versions after native-only layout changes. Formatting/diff PASS. Native tests added for bounds, partial scrolling, focus, exact playback and cover redraw/cleanup, but NOT_RUN locally under the closed cgo environment fingerprint.
- TypeSafe: final-reviewed-judgments.json preserves current raw answers/probabilities/confidence. Lifecycle1.99/confidence0.98 supports review; scope1.84/0.74 and verification1.86/0.79 required independent adjudication. Reviewer confirmed no scope violation or false-completion claim: only fltkhome production changes, existing typed backend, native/user gates explicitly open. This does not replace CI or owner acceptance.
- Correction rounds: 5/5 used. Do not make another failure-driven repair under this contract without renewed scope/budget. Do not retry the closed native capability fingerprint or blindly rerun flaky CI.
- Next: obtain explicit commit/push permission for the scoped source/tests and Loop29/Loop30 closure/contract/handoff docs, excluding AGENTS/.codex and artifacts/data. Existing Windows CI must verify the exact committed source. On green CI prepare exact exe/ZIP and update LOOP30_HUMAN_CHECK; then obtain actual owner acceptance before PASS.
- Evidence: artifacts/evidence/loop30/final-local-verification.json, final-independent-review.json, final-source-state.json, final-reviewed-judgments.json. Earlier checkpoint notes above are historical, superseded by this current state.

## 2026-10-13 simplify-first strict review checkpoint

Fresh simplify-first review made no edits; all8hashes match the saved pure verification. Fresh independent review plus root corroboration found R30-01 cover aspect distortion and R30-02 missing optional cover shown as transient failure. Current FAIL_REVIEW, exact requirement evidence and repair scope are in LOOP30_REQUIREMENTS_REVIEW.md. Earlier PASS_REVIEW/zero-MUST_FIX/CI-next text is historical and superseded. Corrective rounds remain5/5; obtain renewed bounded repair scope before another failure-driven edit. Do not ask CI permission while these findings remain unresolved.

## 2026-10-13 bounded cover repair completed

Owner latest instruction authorized direct docs-only engineering completion. One corrective pass (6 total) resolved R30-01/02 without new UI design or public API. Five repair files changed; total Loop30 source/test scope is10files. Backend adds only emptyCoverURL -> existingcore.ErrNotFound; missingmetadata and invalidURL security contracts remain unchanged. Fresh scoped simplify and independent PASS_REVIEW confirm no unresolved MUST_FIX. Fresh CGO0 regression/backendvet/format/diff PASS; added nativeaspect/absence/error tests NOT_RUN. Evidence: cover-corrections.json, cover-repair-source-judgments.json and cover-repair-final-evidence.json under artifacts/evidence/loop30. Exact CI/build and owner gates still required; no commit/push occurred.

## 2026-10-13 commit/push authorization

Owner explicitly authorized repeated verification/review then push if no issues. Scope:10reviewed source/test files and7related Loop29/30 docs; exclude AGENTS.md,.codex/,artifacts/data. Existing CI only, no workflow/release changes. Pre-push checks and exact committed-source evidence are recorded under artifacts/evidence/loop30; native/user gates remain open until actual evidence.
