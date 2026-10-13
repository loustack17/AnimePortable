# Loop 30 — simplify-first requirements review

Date: 2026-10-13. Baseline: main `90a7fc9985bd5ab17b1198333567324fd69d2e8a`.

## Result

IN_PROGRESS, not PASS: owner feedback reopened rendered-control visibility on bca387a. Native C++ selected-focus bypass confirmed and corrected in two files; actual framebuffer hide/show regression added. Exact new Windows CI and owner acceptance pending. Video stutter with normal audio is unresolved without runtime measurement; no claim this fix resolves it. Earlier reviews/CI below are historical.

## Historical review order and verification before repair

1. Fresh simplify review completed first. Eight changed/new source/test files were inspected with their callers and tests. No proposed simplification justified changing behavior or adding indirection; no edits were made. Decisions and SHA-256 hashes are saved in `artifacts/evidence/loop30/simplify-first-review.md`.
2. A fresh independent reviewer then inspected the canonical requirements, final source, backend calls, and tests. The root independently corroborated both findings against the actual code.
3. All eight source/test hashes match `final-local-verification.json`. Its recorded pure-file tests/vet and desktop/backend/architecture/contract regression PASS remain applicable to this unchanged source. This turn reused those results rather than rerunning unchanged checks. Fresh `git diff --check` passed. Native tests were inspected, not executed.

## Historical confirmed findings — resolved in source

### R30-01 — distorted cover aspect ratio — P2/MUST_FIX

`apps/desktop/fltkhome/detail_cover_windows.go:39-52` samples every input into a fixed 240x320 pixel buffer. `search_view_windows.go:83-96` fits that buffer using its already-altered dimensions. A square source therefore displays at 3:4; the current 8x12 test source also changes from 2:3 to 3:4. Scaling the final native image does not recover the original ratio.

This violates correct cover presentation under FUNC-004 and the product contract's work-respecting metadata presentation. Required repair: preserve source aspect during bounded resizing; retain image byte/pixel/lifetime bounds and test square, portrait, and landscape inputs. Existing fixed-buffer-length and draw-fit assertions do not prove source aspect preservation.

### R30-02 — optional missing cover presented as a transient error — P2/MUST_FIX

`apps/desktop/backend/actions.go:508-533` returns an error when metadata is absent and passes an empty available metadata CoverURL to the loader. `adapters/metadata/cover/cover.go:108-112` rejects the empty URL. `detail_view_windows.go:22-48` handles all these errors as "封面暫時無法載入", sets `detailCoverErr`, and `search_view_windows.go:527-533` exposes Retry. Retrying cannot create an absent optional cover.

This violates `LOOP30_DETAIL.md`'s "Metadata absent is neutral" contract. `TestNativeDetailMetadataAndUnavailableCoverStayNeutral` manually prelabels a placeholder and does not exercise this backend-to-UI path. Required repair: distinguish absent optional cover from a genuine load failure through a typed boundary, without exposing URLs or weakening security validation; exercise missing metadata, empty cover, and actual load failure. Any necessary narrow backend change must be included in renewed repair scope before editing.

## Requirement mapping

Source-present means inspected implementation, not a completed native/human gate.

| Requirement | Implementation and evidence | Current result |
| --- | --- | --- |
| Phase24 / FUNC-004 cover | Decoder, secure typed GetCover, native drawing and cleanup tests | R30-01/R30-02 resolved in source; Native automated PASS; human presentation NOT_RUN |
| FUNC-004 title/native title/short synopsis/season/year/studio/count | `renderPreview`, `detailMetadataLabel`; metadata fallback test and bounded normalized excerpt test. Product section11 explicitly requests a short synopsis. | Source-present; CI assertions PASS; actual content/clipping human gate NOT_RUN |
| FUNC-005 ordered episodes | `detailModel.setEpisodes`, bounded page size8; `TestDetailModelKeepsEpisodeOrderAndBoundsPages` | Pure/native checks PASS in exact CI |
| Play/Continue/current/progress | Exact-ID `playRequest`/`continueRequest`, stored milliseconds, completed exclusion, row progress labels; identity/history pure tests | Pure/native checks PASS; actual playback human gate NOT_RUN |
| UX-001/002/003/004 | Enter/Space play the focused exact episode row, arrows start from focused index, async identity focus restoration and scroll visibility; added native key/focus tests | Native checks PASS; owner gate NOT_RUN |
| UX-005/006/007 | Escape/Back cancellation; existing slash and Ctrl+K Search handlers retained | Native regression PASS; owner gate NOT_RUN |
| UX-008/009/010 | Mouse callbacks, Scroll/paging, desktop controls, proportional toolbar bounds, calm existing theme; native layout tests | Native layout/focus checks PASS; mouse/visual owner gate NOT_RUN |
| QUAL-001..008/010/011, ARCH/SEC | Scoped model/cover/detail view, inward dependency direction, existing typed actions and bounded secure loader; no provider/core/schema/dependency/workflow changes | No additional concrete source finding; R30-02 resolved using the existing not-found sentinel |
| QUAL-009 | Legacy playback model/code remains unchanged baseline; startup tests now exercise actual selected Play after prefetched Episodes, preserving original assertions. Scoped reviewer did not identify a MUST_FIX requiring unrelated legacy removal. | Retention documented; no additional confirmed MUST_FIX |
| QUAL-012 | Fresh independent review after simplify | PASS_REVIEW after bounded repair; zero unresolved MUST_FIX |
| STATE / execution boundaries | Bounded handoff, hash-bound results, separate NOT_RUN gates, preserved user/data files; local cgo breaker remains closed | Exact CI/native PASS recorded; owner acceptance NOT_RUN |

## Historical TypeSafe and budget before repair

Necessary project uploads and memory-only BWS key retrieval were already authorized. `strict-review-judgments.json` retains raw scope1.93/confidence0.88, lifecycle1.98/0.98, verification1.66/0.49. The independent review adjudicated verification uncertainty: recorded pure PASS and native/owner NOT_RUN are accurately separated, but the two defects invalidate earlier zero-MUST_FIX text. Typed judgments did not detect these presentation/fallback defects and do not override source evidence.

Corrective rounds remain 5/5 used under `LOOP30_DETAIL.md`. This new review does not reset that limit. The owner subsequently instructed direct engineering completion strictly from docs; this authorizes one additional bounded pass for these two findings. CI permission must not be requested while the confirmed findings remain unresolved. After authorized repairs: simplify, focused verification, fresh independent review, and only then consider exact-source CI permission. Loop30 is not PASS.

The subsequent document-only TypeSafe batch is retained in strict-review-document-judgments.json: verification2/confidence1 and scope1.95/0.91 support accurate documentation; lifecycle1.65/0.47 required independent adjudication. The reviewer returned PASS_DOC: current status, findings and gates are consistent. This uncertainty establishes no additional lifecycle defect; generation/cancellation and bounded ownership remain source-present, while native lifecycle is still NOT_RUN. It does not change FAIL_REVIEW or resolve either cover finding.

## Final bounded repair result

R30-01 resolved: proportional content sampling into the existing bounded transparent240x320canvas; square/portrait/landscape tests check actual visible-content aspect. R30-02 resolved: backend emptyCoverURL uses existingcore.ErrNotFound; missingmetadata assertion remains unchanged. UI handles that sentinel neutrally and updates genuine failure Retry immediately. Invalid nonemptyURLs still go through the secure loader. Fresh scoped simplify and independent reviewer PASS_REVIEW, zero unresolved MUST_FIX; root corroborated code and five repair hashes.

Fresh CGO0 desktop/backend/architecture/contracts and backendvet PASS; formatting/diff PASS. Native tests/build/race and owner remain NOT_RUN. Correctivepass6completed under latestownerinstruction; noextra feature/scope or commit/push. Final TypeSafe scope1.87/confidence0.80 and verification1.89/0.83 support; lifecycle1.75/0.62 requires independent source adjudication. Unchanged generation/cancellation and UI-thread image ownership were corroborated; uncertainty is not native PASS. Evidence: cover-corrections.json, cover-repair-source-judgments.json, cover-repair-final-evidence.json. Existing Windows CI on exactsource and owner acceptance are the remaining gates.

## Current exact CI result

Main1158671026c429f1acc254e7376ea0e050803ffb matchesGitHub. CI38357613533 allstepsSUCCESS, includingfullnativeGo/20repeat/Anime1/race/vet/security/build/startup/artifact. The nativeonlytests are nowexecuted andPASS; historicalNOT_RUNentries above refer topre-CI checkpoints. Exactexe/ZIP ready, contenthashes/provenancechecked, nodata included. Ownergate remainsNOT_RUN, soLoop30notPASS. Currentpacketandhumansteps: docs/LOOP30_HUMAN_CHECK.md; evidence: artifacts/evidence/loop30/1158671-test-object.json. No source/workflowchangedaftergreenCI.

## Owner UX correction review

Six-file correction passed simplify-first independent review. Exact row callback uses existing deferred/generation guard and exact-ID play helper; Enter selects, Space plays. Continue visibility covers unwatched/foreign/completed/zero-position/resumable history. TypeSafe scope1.99/confidence0.98 and verification1.91/0.85 support review. Lifecycle1.29/0.40 required independent adjudication: patch-only input omitted unchanged cancellation/generation helpers; reviewer inspected them and found no defect. Raw state/probabilities are preserved in artifacts/evidence/loop30/owner-ux-{state,judgments}.json. CGO0 focused/regression and format/diff pass; exact native CI38361340591 PASS. Mouse-event dispatch is a human gate, not claimed automated PASS.

## Enter and idle-controls follow-up review

Simplify-first independent code/platform/lifecycle review PASS, no unresolved MUST_FIX. Enter/Space use the same exact-row generation-guarded playback path; Search Enter and working mouse callback retained. Existing player timer hides visible controls after3s active idle playback; passive hover no longer pins them. Any KEYDOWN/mouse movement reveals; pause/loading/state transitions reveal, progress-only updates do not reset the timer. Scrubbing/volume drag/pinned menu/episode list preserve interaction. Tests cover thresholds, pointer, key/modifier reveal, state transitions/progress and exact episode IDs. TypeSafe scope1.97/confidence0.97 and verification1.90/0.85 support review; lifecycle1.22/0.46 required independent adjudication against full unchanged generation/timer/close guards, which found no defect. Raw state/judgments and source hashes saved under artifacts/evidence/loop30/enter-idle-*.json. Format/diff and focused CGO0 regression PASS; exact native CI38435868704 PASS; human acceptance remains pending.

## Focused hidden-overlay correction

Simplify-first independent review PASS. The hidden/no-menu/no-episodes guard now returns regardless of remembered focus, before any GL state change. The native test disables inherited scissor before black clear, checks hidden/shown framebuffer pixels and retained focus in both reentry passes; existing assertions retained. TypeSafe scope/lifecycle/verification uncertainty independently adjudicated against this synchronous two-file patch: generic image/generation rubric omitted applicable renderer evidence, no new async/resource ownership work. Raw judgments saved in focused-overlay-judgments.json. Exact new native CI and owner acceptance required. Video-only stutter cause remains UNKNOWN; no unmeasured tuning.
