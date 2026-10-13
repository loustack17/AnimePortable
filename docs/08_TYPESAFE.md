<!-- SPDX-License-Identifier: MPL-2.0 -->

# TypeSafe judgments

TypeSafe answers small, typed questions (Noul = yes/no probability, Choice = one option with probabilities, Score = position on ordered levels). Agents use it for every judgment step instead of long free-form reasoning. Code then applies fixed thresholds. This saves usage and makes decisions repeatable.

TypeSafe is a development tool only. The product never calls it.

## Rule: what goes where

| Use TypeSafe for | Use deterministic tools for |
| --- | --- |
| Classifying a failure, a CI log stage, a review finding, an owner reply | Running tests, builds, vet, format, scans |
| Checking scope, criteria coverage, verifier weakening | Exact lookups: Git state, file hashes, CI run status, link checks |
| Judging doc or requirement clarity, status, tool-neutrality | Counting, sizes, budgets, thresholds, aggregation |
| Deciding between named options when the evidence is text | Anything a test or command can prove |

If TypeSafe is unavailable (no key, `bws` failure, API outage), report `ENVIRONMENT_BLOCKED` for that judgment step and tell the owner. Do not silently replace it with free-form judgment.

A typed answer is not proof. It never replaces a required test, independent review or owner check. When TypeSafe and a deterministic check disagree, the deterministic check wins.

## How to run

```powershell
./tools/typesafe-judge/run.ps1 -In <items.json> -Questions tools/typesafe-judge/questions/<template>.json -Out artifacts/evidence/<loop>/<name>.json
```

- `run.ps1` fetches only the `TYPESAFE_API_KEY` secret (by its ID; override with `-SecretId` or `TYPESAFE_BWS_SECRET_ID`) from Bitwarden Secrets Manager (`bws`, needs `BWS_ACCESS_TOKEN`) into the child process only. It never lists the store. Never print, log or save the key.
- Items file: `{"items": [{"id": "...", "state": {...}}]}`. Put all evidence the question needs into `state` as named fields (the templates name the fields they expect, for example `failure`, `finding`, `contract`, `reply`).
- The tool sends one request per item (4 in parallel). It retries only HTTP 429 and 529 with backoff. Any other error, a timeout, or a response missing an answer, value or confidence fails that item and makes the command exit non-zero. A timed-out request is not resent because it may already have been processed.
- Raw answers, probabilities and confidence are saved to `-Out`. Keep them; do not summarize them away.
- Data boundary: send only repository text, code excerpts, CI log excerpts and de-identified owner replies. Never send secrets, personal data, raw diagnostic reports with hardware identity, or the portable database.

## Templates (tools/typesafe-judge/questions/)

| Template | Use at | Required `state` fields |
| --- | --- | --- |
| `failure-class.json` | Every failed check, before any fix (05_WORKFLOW step 5) | `failure`: command, exit status, smallest error, changed files |
| `ci-log.json` | Every failed CI run, before deciding the next action | `log_excerpt` |
| `review-triage.json` | Every reviewer finding | `finding`, `requirement` |
| `scope-check.json` | Before review and before final verification | `contract`, `change`, `evidence` |
| `owner-report.json` | Every owner test reply | `check`, `reply` |
| `doc-quality.json` | Any doc creation or major edit | `project_context`, `file`, `heading`, `section_text` |

Add a new template when a judgment repeats. Rules for questions: one narrow dimension per question; Score levels describe concrete situations; include a no-match option when nothing may fit; ask independent questions in one batch.

## Fixed thresholds

| Answer | Act on it | Needs independent adjudication |
| --- | --- | --- |
| Noul | `>= 0.7` yes, `<= 0.3` no | between 0.3 and 0.7 |
| Choice | `confidence >= 0.5` | `confidence < 0.5` |
| Score | `confidence >= 0.5` | `confidence < 0.5` |

The tool prints `REVIEW` next to answers in the adjudication band. Adjudicate by checking the cited evidence directly or asking the oracle; record the outcome next to the raw file. Gate thresholds for a specific use (for example doc clarity >= 0.75) are set in that loop's contract.

## Validating a template

Before relying on a new or changed template, run it on at least one clearly positive and one clearly negative example, plus one case with missing evidence (expect a no-match, `UNKNOWN_FAILURE` or low-confidence answer), and confirm the answers match. Keep those examples and results in the evidence folder. Example: `failure-class.json` was validated on 2026-10-13 against three past failures (Mesa version guard, undefined name, local cgo parse error); see `artifacts/evidence/docs-audit/failure-class-validation-2.json`.
