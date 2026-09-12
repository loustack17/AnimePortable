<!-- SPDX-License-Identifier: MPL-2.0 -->

# Agent Handoff

> Bounded recovery checkpoint. Keep <= 4 KiB target, <= 8 KiB / 160 lines hard cap. Git/verifiers/source-of-truth docs override this file.

```yaml
schema_version: 1
protocol_version: v2.3
instruction_version: v2.3
updated_at_utc: null
status: NEEDS_RECONCILIATION

repository:
  branch: null
  observed_head: null
  working_tree: unknown

active_loop:
  number: 23
  name: Home
  state: IN_PROGRESS_PRE_V2_2_RECONCILIATION
  goal: "Continue the existing Loop 23 implementation without replaying Loops 01-22."
  acceptance_ids: []

progress:
  completed: []
  in_progress:
    - "Existing Loop 23 work predates this durable-state file and must be reconciled with current Git/status before mutation."
  unresolved_failures: []

next_actions:
  - "Reconcile branch/HEAD/working-tree with current IMPLEMENTATION_STATUS.md and Loop 23 diff."
  - "Populate Loop 23 acceptance IDs and current verification state."
  - "Continue existing Loop 23 under Protocol v2.3 after reconciliation; preserve valid pre-migration work and do not reimplement Loops 01-22 without demonstrated failure."

touched_paths: []

verification:
  last_focused: null
  last_regression: null
  ci_run: null

pending:
  human_gate: null
  adr: null
  environment_blocker: null

budget:
  corrective_iterations_used: 0
  same_failure_repairs: {}
  replans_used: 0

do_not_repeat: []

references:
  - "05_LOOP_ENGINEERING_RUNBOOK.md"
  - "06_ACCEPTANCE_CRITERIA.md"
  - "08_VERIFICATION_MATRIX.md"
  - "09_CODEX_EXECUTION_PROFILE.md"
  - "10_DURABLE_AGENT_STATE.md"
  - "IMPLEMENTATION_STATUS.md"
```

## Recovery notes

This initial file intentionally does **not** invent Loop 23 implementation details. The first v2.3 root session must reconcile it against the real repository and then replace the placeholders with concise current truth.
