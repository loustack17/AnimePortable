# Loop Engineering Runbook

This document tells an AI coding agent exactly how to execute the project incrementally.

## 1. Mandatory pre-loop reading

Before every loop, the agent must read:

- `README.md`
- `01_PRODUCT_CONTRACT.md`
- `02_ARCHITECTURE.md`
- `03_SECURITY.md`
- the relevant phase in `04_MVP_IMPLEMENTATION_PLAN.md`
- `06_ACCEPTANCE_CRITERIA.md`
- active ADRs

If the repository contains an implementation-status file, read that too.

## 2. One-loop rule

Each loop performs exactly one coherent phase or vertical slice.

Do not bundle unrelated work.

Examples of valid loops:

- secure redirect handling
- Anime1 episode parser
- MPV IPC lifecycle
- schedule UI

Examples of invalid loops:

- "implement playback, metadata, and redesign settings"
- "refactor architecture while adding a feature"

## 3. Loop start procedure

At the start of each loop:

1. inspect current repository state
2. inspect relevant code before modifying it
3. identify the exact phase/acceptance criteria being addressed
4. list current gaps
5. confirm no existing implementation already satisfies the requirement
6. create a short implementation plan for this loop only

## 4. Implementation rules

The agent must:

- preserve architecture boundaries
- minimize changes
- avoid unrelated refactors
- add or update tests
- reuse existing patterns when correct
- prefer standard library where practical
- keep dependencies minimal
- preserve security policies
- preserve keyboard accessibility
- preserve local-first behavior

The agent must not:

- add danmaku
- add VLC/IINA
- add mobile
- add social features
- add telemetry
- add account systems
- add ads
- add arbitrary remote fetch in frontend
- expose raw MPV commands
- expose arbitrary playback URLs
- bypass secure HTTP policy
- bypass playback proxy
- add provider-specific IDs to core domain
- over-engineer with unnecessary ports/layers
- change frameworks without ADR

## 5. Security gate in every loop

Before completing a loop, ask:

- Did this add a new external input?
- Did this add a new network path?
- Did this add a new persistence field?
- Did this add a new log field?
- Did this expose new data to frontend?
- Did this add a new process/socket/file?
- Does it need cleanup?
- Can an attacker control this value?
- Could it become SSRF, XSS, command injection, path traversal, secret leakage, or resource exhaustion?

If yes, add tests and safeguards in the same loop.

## 6. Resource-lifecycle gate

For any created resource, define ownership and cleanup.

Examples:

- HTTP response -> close body
- goroutine -> context cancellation path
- ticker -> Stop
- socket -> Close
- MPV process -> wait/reap
- IPC endpoint -> delete/close
- proxy session -> expire/invalidate
- temporary file -> remove

A loop is incomplete if cleanup is implicit or left to "later".

## 7. Testing requirements per loop

Run the smallest relevant tests plus broader regression tests when practical.

At minimum, report exactly what was executed.

Examples:

```text
go test ./...
go vet ./...
pnpm test
pnpm lint
pnpm check
```

Do not claim tests passed if they were not actually run.

If environment prevents a test:

- state exactly what could not run
- state why
- provide the closest substitute
- do not mark the criterion fully passed

## 8. Simplify gate

After relevant behavior passes its tests, review every changed code and configuration file with the `simplify` skill.

The review must:

- preserve behavior, errors, side effects, ordering, and security policy
- follow repository conventions
- make only genuine clarity or maintainability improvements
- avoid line-count optimization and unrelated refactors
- accept unchanged code when it is already clear
- rerun focused tests after every accepted simplification

A loop is incomplete until its changed code passes this review.

## 9. Loop output format

Every loop ends with a structured report:

### Implemented

Concise list of actual changes.

### Files changed

Exact paths.

### Tests executed

Exact commands and results.

### Acceptance criteria addressed

Reference criterion IDs if available.

### Security/resource review

State new trust boundaries/resources and how they are handled.

### Known issues

Only real remaining issues.

### Next loop

Name the next intended phase, but do not implement it.

## 10. Stop condition

After finishing the assigned loop, stop.

Do not continue because there is extra time/context.

The purpose of loop engineering is controlled convergence.

## 11. Scope drift rule

If a loop reveals that the documented architecture is materially wrong:

1. stop feature implementation
2. write a proposed ADR
3. explain:
   - current decision
   - new evidence
   - proposed change
   - migration impact
   - security impact
   - performance impact
4. wait for ADR acceptance before broad redesign

Minor implementation details do not require ADR.

## 12. Dependency rule

Before adding a dependency, the agent must answer:

- Can standard library solve it safely?
- Is the dependency actively maintained?
- Is it cross-platform if needed?
- Does it increase binary/runtime significantly?
- Does it add transitive dependencies?
- Does it have known vulnerabilities?
- Can the boundary be isolated if it must be replaced?

Avoid npm packages for trivial helpers.

## 13. Performance rule

Do not micro-optimize Go interface dispatch.

Measure or reason about the real bottlenecks:

- remote latency
- startup blocking
- SQLite queries
- cover decode
- WebView rendering
- MPV
- unbounded work
- repeated allocations in actual hot paths

Performance work must target observed or structurally credible bottlenecks.

## 14. AI anti-patterns prohibited

The agent must not:

- rewrite working modules "for cleanliness"
- introduce generic managers/factories without need
- build future source/plugin systems before MVP needs them
- create dozens of tiny packages
- make every function an interface
- put everything in `utils`
- duplicate security logic in adapters
- silently weaken fail-closed behavior to make tests pass
- replace typed errors with raw stack traces in UI
- put secrets in debug logs
- turn cached startup into network-blocking startup

## 15. Required loop sequence

Use this order unless an ADR changes it:

1. Repository/contracts
2. Core models/ports
3. Contract tests
4. Secure HTTP
5. Anime1 catalog/search
6. Anime1 episodes
7. Anime1 resolver
8. Anime1 schedule
9. Anime1 adapter acceptance
10. Secure playback proxy
11. MPV process lifecycle
12. MPV IPC
13. Same-session switching
14. SQLite
15. Progress/history
16. Following
17. AniList
18. Bangumi
19. Metadata matching
20. Metadata content security
21. Wails binding layer
22. Svelte shell
23. Home
24. Search
25. Anime detail
26. Schedule UI
27. Following UI
28. History UI
29. Keyboard navigation
30. Cache-first refresh
31. Autoplay next
32. Settings
33. Security hardening
34. Resource-leak testing
35. Cross-platform validation
36. CI/release
37. Full acceptance

## 16. Project-status file

Maintain `docs/IMPLEMENTATION_STATUS.md`.

Suggested format:

```markdown
# Implementation Status

## Current loop
Loop 12 — MPV IPC

## Completed
- [x] Loop 01 ...
- [x] Loop 02 ...

## In progress
- [ ] Loop 12 ...

## Blocked
None

## Known technical risks
- ...

## Last verified commands
- `go test ./...`
- ...
```

Update only after tests/validation.

## 17. Definition of done for each loop

A loop is done only if:

- code implemented
- relevant tests added
- tests actually run
- changed code passed simplify review
- security reviewed
- resource lifecycle reviewed
- documentation/status updated
- no unrelated scope added
- repository remains buildable at the expected level
