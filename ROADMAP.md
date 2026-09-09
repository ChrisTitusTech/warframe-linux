# Roadmap

Status: planning, 2026-09-09. Phases are dependency ordered; no calendar promises.
The current authorized milestone is planning documents and initial GitHub setup.
Stop there before application implementation.

| Phase | Outcome and scope | Dependencies | Exit / validation |
| --- | --- | --- | --- |
| P0 - Feasibility | Select Wails/Tauri; mock inventory desktop; capture/window/package spike; license and data-field decisions | Planning review; owner resolves license intent | Reproducible build on documented Linux matrix, capture denial/fallback evidence, measured budgets, written framework decision; no unverified overlay claims |
| P1 - Core foundation | Domain contracts, SQLite migrations, XDG paths, fixture providers, public catalog and pricing | P0 accepted | R1-R3, R6-R8 service portions; offline, failed-sync, 429, unknown-ID, migration/rollback tests; desktop smoke test |
| P2 - Inventory and sets | Explicit import, quantities/rank, set completion, manual corrections | P1 and validated import schema | R4, R6; duplicate/unknown/malformed imports; no data loss; keyboard table navigation and scale checks |
| P3 - Relic assistant | Screenshot import, OCR, live capture on proven desktops, reward panel | P1, P0 capture outcome; P2 for ownership hints | R5 and R8; labeled corpus, false-positive checks, real reward screen demonstration and normal-window fallback |
| P4 - Experimental inventory | Isolated memory/mobile adapter, redaction, consent/revoke, provider diagnostics | P2; explicit implementation/live-test authorization and reuse decision | R9; fixture failures first, then separately authorized live evidence; can remain unavailable without blocking beta |
| P5 - Planning features | Recipe trees, missing resources, collection/mastery; foundry only with proven fields | P2 plus complete recipe/mastery data coverage | R10; cycles/missing fields; snapshot age visible; hand-calculated crafting examples |
| P6 - Beta release readiness | Package, docs, dependency notices, upgrade/rollback, performance and accessibility review | P1-P3; P4/P5 optional and labeled | Clean-machine install, all claimed desktop configurations proven, required CI green, manual evidence attached; release publication needs explicit authorization |

## Validation and rollback

Application phases add actual Go tests/vet, frontend typecheck/lint/build, provider
contract tests, and a packaged smoke test once the relevant code exists. Do not
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
