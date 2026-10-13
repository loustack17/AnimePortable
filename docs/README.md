<!-- SPDX-License-Identifier: MPL-2.0 -->

# Docs index

AnimePortable is a lightweight, keyboard-first Windows anime player: find an anime, pick an episode, watch it in the built-in libmpv player, resume later. Go + FLTK + in-process libmpv, local SQLite, portable ZIP.

These docs are the source of truth. If code and docs disagree, the docs win unless a newer ADR says otherwise. Start with the root `AGENTS.md`.

## Which doc to read

| When you need to | Read |
| --- | --- |
| Resume work | [AGENT_HANDOFF.md](AGENT_HANDOFF.md), then [STATUS.md](STATUS.md) and the active loop file in [loops/](loops/) |
| Know what the product must do | [01_PRODUCT.md](01_PRODUCT.md) |
| Place code or add a dependency | [02_ARCHITECTURE.md](02_ARCHITECTURE.md) |
| Touch network, playback, storage, logs or the UI boundary | [03_SECURITY.md](03_SECURITY.md) |
| Start or plan a loop | [04_ROADMAP.md](04_ROADMAP.md) |
| Follow the work process | [05_WORKFLOW.md](05_WORKFLOW.md) |
| Look up a criterion and how it is verified | [06_ACCEPTANCE.md](06_ACCEPTANCE.md) |
| Make a judgment with TypeSafe | [08_TYPESAFE.md](08_TYPESAFE.md) |
| Check an architectural decision | [09_DECISIONS.md](09_DECISIONS.md) |
| Find old evidence, past loops or replaced designs | [archive/](archive/) (history only; never a current rule) |

## Layout rules

- Numbered docs hold current rules only. No history narration.
- `loops/` holds one file per active or recent loop: contract, state, evidence index, owner check.
- `STATUS.md` is one page. Detailed history goes to `archive/`.
- Every new or rewritten doc is checked with the `doc-quality` TypeSafe template.
