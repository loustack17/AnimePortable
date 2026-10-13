<!-- SPDX-License-Identifier: MPL-2.0 -->

# Docs restructure (D1)

This was a documentation-only loop. It has no product loop number. Loop 30 was paused while it ran.

## Why

On 2026-10-13 TypeSafe scored all 38 old docs (546,000 characters). Only 23% of the text was needed for normal feature work. 27% was hard to understand, 25% described history or removed designs, and 16% required one specific AI tool. Agents spent usage rereading it and reopening settled questions.

## What changed

- `AGENTS.md` and `docs/` were rewritten to the layout in [docs/README.md](../README.md). The current docs total about 149,000 characters.
- The verification matrix was merged into `docs/06_ACCEPTANCE.md`, so each criterion shows its requirement, verifier and evidence in one place.
- All 38 old docs were moved unchanged to `docs/archive/`.
- `tools/typesafe-judge/` was added: a Go command, its tests, six question templates and a launcher script.
- No product code, product test, CI workflow or dependency changed.

## Owner decisions applied (2026-10-13)

1. The player has on-video controls that hide when idle, as in OneAnime.
2. Criteria written for an external MPV process now describe in-process libmpv. Their IDs are unchanged.
3. Shipping the pinned libmpv DLL with a disclosure notice is settled. Licensing is not reopened.
4. Docs name roles, not specific AI tools or models.
5. Every judgment step that TypeSafe can answer uses TypeSafe.
6. The player ignores the user's `mpv.conf` (PLAY-009). Settings has no MPV path. Ctrl+K stays a planned binding.

## Results

| Check | Result |
| --- | --- |
| All 207 criterion IDs kept | PASS: a script found 0 missing |
| All relative links open | PASS: a script found 0 broken |
| Old docs unchanged in `docs/archive/` | PASS: 28 of 38 are byte-identical to commit e8f0640, 9 differ only in line endings, and `AGENT_HANDOFF.md` keeps the owner's uncommitted edit |
| Clarity of 0.75 or more per current doc | PASS: every doc scores between 0.75 and 0.95 |
| No section requires a specific AI coding tool | PASS after review: only sections naming TypeSafe or Bitwarden are flagged, and the owner requires both |
| Independent review | PASS: 18 `MUST_FIX` findings across two reviews were fixed |
| `tools/typesafe-judge` vet and tests | PASS |
| Owner approves the commit | Approved on 2026-10-13, after the fixes |

Raw evidence is in `artifacts/evidence/docs-audit/`: scores, gate script, review triage and archive hashes.

## Next

Legacy cleanup is a separate code change. It removes:

- the unused `go-winio` dependency;
- empty placeholder files and unused icons;
- dead build variables and the unused `-tags production` flag (the owner approved the CI workflow edit);
- the unused strict packaging mode;
- the SQLite `mpv_path` column, through a new migration.
