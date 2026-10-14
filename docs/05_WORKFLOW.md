<!-- SPDX-License-Identifier: MPL-2.0 -->

# Workflow

How any AI agent or human works on this repository. It names roles, not products. Requirements are in [06_ACCEPTANCE.md](06_ACCEPTANCE.md); the loop list is in [04_ROADMAP.md](04_ROADMAP.md).

## 1. Start a session

1. Read `AGENTS.md`, then `docs/AGENT_HANDOFF.md`.
2. Run `git status --short --branch`, `git log -5 --oneline`, `git diff --stat`. Git wins over the handoff.
3. Read only what the next step needs. Never discard working-tree changes because the handoff does not mention them.

## 2. A loop

A loop is one coherent slice from the roadmap. Keep one slice per loop; no unrelated refactors.

1. **Scope.** Write `docs/loops/LOOPNN.md`: goal, in and out of scope, how it is tested, the owner check. Short.
2. **Build.** Implement with tests that encode the intended behavior.
3. **Check locally.** Run the focused test, then the full suite and race (section 5).
4. **Simplify.** Reread every changed file for clarity without changing behavior; rerun the tests.
5. **Review.** A fresh-context, read-only reviewer checks correctness, security, resources and code quality ([02_ARCHITECTURE.md](02_ARCHITECTURE.md)). Fix every must-fix finding, then rerun the tests.
6. **Owner check.** Put the build in `artifacts/test-build/` (section 4) and ask the owner to try it with plain steps.
7. **Commit and push** after owner approval. Confirm GitHub Windows CI is green for that commit.
8. **Close.** Mark the loop doc PASS, update `docs/STATUS.md` and the handoff.

The loop is done only when the tests, the review, the owner and CI agree. An agent saying "done" is not enough.

## 3. When something fails

- Read the error, form one hypothesis, gather evidence, make the smallest causal fix, rerun the failing check, then the wider suite.
- Retry only with new evidence or a changed fix. A rerun of the same state is not a fix, except at most two reruns to confirm a flaky test.
- After three failed attempts at the same problem, stop and report to the owner with the evidence.
- Never weaken, skip or special-case a test, check or security rule to get green. A test that looks wrong is reported, not edited to pass.
- A missing tool or a blocked environment is `ENVIRONMENT_BLOCKED`: report it; do not repair the host.

## 4. Owner checks

- The current test build lives only in `artifacts/test-build/` (exe, pinned libmpv DLL, notice, `data/`). Each delivery replaces it; delete older build folders, and keep reports or data backups under `artifacts/evidence/<loop>/`.
- Give numbered steps using visible actions only (click, type, press a key, watch) and the expected result of each. Never ask the owner to edit data, run internal tools or read logs they cannot interpret.
- Record what the owner reports, not "looks good".

## 5. Environment and tests

Local, before every push:

```
CGO_ENABLED=1 go test -count=1 ./...
CGO_ENABLED=1 go test -race -count=1 ./...
```

Use an msvcrt MinGW gcc (MSYS2 MINGW64 or nuwen MinGW) on PATH; go-fltk's prebuilt libraries cannot link with a UCRT MinGW. The git-ignored `artifacts/` folder has its own `go.mod` so `./...` skips it. GitHub Windows CI is the authoritative check for the pushed commit. A CI run on another OS cannot stand in for native Windows behavior, real playback or the owner's judgment.

## 6. Approvals and boundaries

Ask the owner first for: commit, push and any remote write (default target `main`); `.github/**` edits and CI reruns; releases and tags; new dependencies; changes to product, architecture, security policy, requirements or ADRs; weakening or removing a test or check.

Persistent writes only inside this repository. No global installs, admin rights, host configuration, credential stores or other repositories, and never widen permissions to get past a block. Never print or store secrets. The one approved secret is `TYPESAFE_API_KEY`, read from Bitwarden Secrets Manager into the TypeSafe process only ([08_TYPESAFE.md](08_TYPESAFE.md)).

Allowed without asking: read-only CI observation with `gh`.

## 7. Roles

Roles and delegation rules are in `AGENTS.md`. In short: one root agent owns the result, workers never share files or edit the handoff, and a reviewer never approves code it wrote.

## 8. TypeSafe

Use TypeSafe for judgment steps it can answer: failure classification, CI log reading, review triage, scope checks, reading owner replies, doc quality. Facts come from tests and commands. Templates and thresholds: [08_TYPESAFE.md](08_TYPESAFE.md). If TypeSafe is unavailable, report `ENVIRONMENT_BLOCKED` for that step instead of guessing.

## 9. Handoff

`docs/AGENT_HANDOFF.md` is a small recovery index (at most 8 KiB and 160 lines): instruction version, current loop and state, next actions, open problems, and short "do not repeat" notes. No logs, diffs, secrets or copied doc text. Lasting decisions go into an ADR, a doc, a test or a code comment. Update it when a slice is done, an owner decision changes the path, or before pausing.

## 10. Avoid

- Rewriting working code for taste; layers, factories or interfaces without a present need; `utils`, `helpers`, `common` or `misc` packages.
- Duplicating security or domain policy outside its owner.
- Showing stack traces in the UI, logging secrets, or making cached startup wait on the network.
- Optimizing without a measurement.
