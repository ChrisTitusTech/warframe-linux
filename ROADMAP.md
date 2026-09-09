# Roadmap

Status: finalized Go implementation plan, 2026-09-09. This handoff finalizes and
merges planning only; begin implementation on the next machine with F01a.
Go + Wails v2 + React/TypeScript + SQLite is selected. Each child row in
[TASKS.md](TASKS.md) is a separate PR, following [CONTRIBUTING.md](CONTRIBUTING.md).
Phases group outcomes; they are not giant PRs or blanket approval checkpoints.

| Phase | Deliverable and PR units | Exit criteria |
| --- | --- | --- |
| P0 - Go desktop foundation | F01a-c: scaffold/CI/package; F02a-b: capture evidence; F03a-b: provenance/contracts | Pinned tooling works on implementation host and CI; package evidence, Linux capability matrix, license and field coverage recorded |
| P1 - Catalog and pricing | C01a-b: contracts/storage; C02a-d: ingestion/search/quotes/details | R1-R3 and foundational R6-R8 tests; cached/offline workflows usable and failed sync preserves data |
| P2 - Inventory and sets | I01a-d: import/table/sets/privacy controls | R4/R6, quantity/rank/provenance integrity, malformed import rollback and accessible table demonstrated |
| P3 - Relic assistant | R01a-b: recognition/image workflow; R02a-c: Wayland/X11/lifecycle | R5/R8, labeled corpus and live capture evidence for claimed desktops, usable ordinary-window fallback |
| P4 - Experimental inventory | E01a-b: fixture boundary then adapter | R9 including separately authorized live evidence; otherwise feature stays unavailable/pending |
| P5 - Crafting and collection | P01a-d: graph/planner/mastery/conditional timers | R10 with proven data fields, quantities and stale-state handling; unsupported timers deferred |
| P6 - Core beta readiness | B01a-c: package/upgrade/compatibility evidence | R1-R8 and all claimed environments proven; checks/review green; release publication separately authorized |

## Scheduling and merge boundaries

Critical path: F01a -> F01b plus F03a -> F03b -> C01a -> C01b, followed by the
catalog/quote tasks, inventory, and relic tasks listed in TASKS.md. P0 capture and
package evidence can be completed independently while core fixture work proceeds;
only explicit task dependencies block a PR. Merge dependencies before dependents.
P4 and P5 are optional branches after P2, not prerequisites to P6. The core beta
can ship import/OCR functionality with experimental sync and timers unavailable.
No calendar estimates imply completion before acceptance evidence passes.

Stop a PR at its child-task boundary; split unrelated work into the next branch.
Do not merge failed validation or claim unsupported desktop modes. Follow the
user's phase/approval instructions; documents do not independently grant permission
to publish, merge, run live game tests, or release software.

## Validation and rollback

Application phases add actual Go tests/vet, frontend typecheck/lint/build, provider
contract tests, and a packaged smoke test once the relevant code exists (F01a documents commands; F01b enables CI). Do not
create placeholder green application CI in the planning phase. Pin tooling and
record commands/versions. No game access or live account credentials in hosted CI.

Capture/OCR corpus must cover 1-4 rewards, negative screens, supported resolutions,
scales, and declared locales; split calibration examples from evaluation images.
Proposed target: at least 95 percent exact top-choice item recognition on the
held-out supported corpus and no confident match on its negative screens. Report
sample counts and unknown rate so small datasets cannot imply broad compatibility.

Rollback: retain prior package and migration backup, preserve original import,
restore last valid database on failed migration, and disable a failing provider
without disabling catalog/search. Never downgrade a database schema blindly.
Release evidence lists known unsupported desktop/game modes.
