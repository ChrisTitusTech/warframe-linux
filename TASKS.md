# PR-sized task backlog

Status: finalized for Go implementation on the next machine, 2026-09-09.
F01a and F01b acceptance are **complete**. Other application
tasks are **not started**.
Each row is one PR-sized outcome;
GitHub issues #1-#11 are parent tracking epics, not individual PR scopes. Use the
child ID in the branch and PR title. See [CONTRIBUTING.md](CONTRIBUTING.md) for size
limits, dependency merges, and validation rules.

Requirement IDs refer to [SPEC.md](SPEC.md). Dependencies name merged child tasks,
not entire phases. The license decision blocks reuse, not the fresh mock scaffold.
Manual acceptance requires actual evidence; unavailable environments stay pending.

## P0 - Go desktop foundation

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| F01a ([#1](https://github.com/ChrisTitusTech/warframe-linux/issues/1)) | Pinned Go/Wails mock desktop scaffold | None | R1/R7: pin Go, Wails v2, Node and npm lockfile; fresh install runs real build, lint/typecheck and test commands; mock list opens and closes on target Linux. |
| F01b ([#1](https://github.com/ChrisTitusTech/warframe-linux/issues/1)) | Build and validation CI | F01a | Run the documented commands on a fresh runner with read-only permissions and pinned actions; prove an intentional failure is reported before final passing run. |
| F01c ([#1](https://github.com/ChrisTitusTech/warframe-linux/issues/1)) | First Linux package and measurements | F01b | Build a native archive/package with declared dependencies; install on a clean target; record startup/RSS/search timings and uninstall steps. AppImage is an optional comparison. |
| F02a ([#2](https://github.com/ChrisTitusTech/warframe-linux/issues/2)) | Wayland capture feasibility evidence | F01a | Document KDE/GNOME versions; user-approved portal capture of a test window, deny/revoke/source-close behavior; record unsupported cases without claiming game compatibility. |
| F02b ([#2](https://github.com/ChrisTitusTech/warframe-linux/issues/2)) | X11 and companion-window feasibility | F01a | Record X11 capture, focus, scale and ordinary-window behavior; document fullscreen/shortcut/overlay limitations. Actual game checks need separate authorization. |
| F03a ([#3](https://github.com/ChrisTitusTech/warframe-linux/issues/3)) | License and reuse inventory | None | Maintainer records new-code license and copied-code notices; list reusable modules and source/data attribution; no blanket MIT claim or code migration before this decision. |
| F03b ([#3](https://github.com/ChrisTitusTech/warframe-linux/issues/3)) | Provider schema and endpoint contract notes | F03a | Document supported Market endpoints/rank/platform semantics and synthetic inventory schema, quantities, unknowns, mastery/foundry field availability; list unavailable data explicitly. |

## P1 - Catalog and pricing

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| C01a ([#4](https://github.com/ChrisTitusTech/warframe-linux/issues/4)) | Domain interfaces and fixture services | F01b, F03b | R7: typed Go DTOs, canonical IDs, fixture adapter substitution, deadlines/cancellation and structured errors verified without UI or network. |
| C01b ([#4](https://github.com/ChrisTitusTech/warframe-linux/issues/4)) | Private storage and first migration | C01a | R6: XDG paths, 0700/0600 personal storage, transactional migration and failed-upgrade restore tests; oldhelp data is untouched. |
| C02a ([#5](https://github.com/ChrisTitusTech/warframe-linux/issues/5)) | Catalog ingestion and atomic sync | C01b | R2: catalog adapter stages/validates/promotes; malformed/partial/offline sync retains last valid revision; source and freshness persisted. |
| C02b ([#5](https://github.com/ChrisTitusTech/warframe-linux/issues/5)) | Cached catalog search screen | C02a | R2/R8: search, loading/empty/stale/retry states and keyboard focus; measure 10,000-item cached search against budget. |
| C02c ([#5](https://github.com/ChrisTitusTech/warframe-linux/issues/5)) | Market quote service | C01b, F03b | R3: platform/rank/price-type keys, request deduplication, central rate limiting and stale cache; timeout/429/malformed fixture tests. |
| C02d ([#5](https://github.com/ChrisTitusTech/warframe-linux/issues/5)) | Price details in catalog | C02b, C02c | R3/R8: timestamp/type/volume shown, unavailable distinct from zero; offline UI demonstration and readable scale checks. |

## P2 - Inventory and sets

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| I01a ([#6](https://github.com/ChrisTitusTech/warframe-linux/issues/6)) | Validated inventory snapshot importer | C02a | R4: bounded schema-validated import; quantity/rank and unknown IDs preserved; malformed/duplicate input tests and atomic replacement. |
| I01b ([#6](https://github.com/ChrisTitusTech/warframe-linux/issues/6)) | Inventory table and corrections | I01a, C02d | R4/R8: quantity/rank/provenance, sort/filter and manual correction overlay separate from imported truth; empty/error and keyboard workflow demonstrated. |
| I01c ([#6](https://github.com/ChrisTitusTech/warframe-linux/issues/6)) | Set completion calculations and view | I01b | R4: set recipes plus owned quantities give correct gaps for hand-calculated fixtures; unknown coverage visible; no mastery inference. |
| I01d ([#6](https://github.com/ChrisTitusTech/warframe-linux/issues/6)) | Personal data export and deletion | I01a | R6: explicit export; delete snapshots/corrections/history with confirmation and transaction; fixtures and manual verification of remaining public catalog/cache. |

## P3 - Relic assistant

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| R01a ([#7](https://github.com/ChrisTitusTech/warframe-linux/issues/7)) | Reward recognition from image fixtures | C02a, F03a | R5: bounded Tesseract invocation, slot-indexed candidates, confidence/unknowns; held-out 1-4 reward and negative-image corpus with sample counts and recognition metrics. |
| R01b ([#7](https://github.com/ChrisTitusTech/warframe-linux/issues/7)) | Reward image import and comparison UI | R01a, I01c | R5/R8: user-selected image, correct slot order, prices/ducats/ownership hints and uncertain-match correction; cached latency and scale demonstration. |
| R02a ([#8](https://github.com/ChrisTitusTech/warframe-linux/issues/8)) | Wayland live capture adapter | R01b, F02a | R5/R6: selected portal source, bounded frames, stop/revoke/cancel cleanup and no retained images; real desktop evidence on claimed Wayland targets. |
| R02b ([#8](https://github.com/ChrisTitusTech/warframe-linux/issues/8)) | X11 capture adapter and fallback | R01b, F02b | R5/R8: explicit supported X11 capture; unavailable capability yields image-import/normal-window fallback; manual evidence and cleanup tests. |
| R02c ([#8](https://github.com/ChrisTitusTech/warframe-linux/issues/8)) | Capture controls and reward lifecycle | R02a, R02b | R5/R8: user starts/stops scan; no duplicate/stale reward sessions; memory/latency and focus evidence; real game verification only when separately authorized. |

## P4 - Experimental inventory (optional)

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| E01a ([#9](https://github.com/ChrisTitusTech/warframe-linux/issues/9)) | Experimental acquisition boundary with fixtures | I01a, F03b | R9: disabled-by-default provider and consent/revoke states, redacted errors, expired-session/permission/cancel fixtures; no live acquisition yet. |
| E01b ([#9](https://github.com/ChrisTitusTech/warframe-linux/issues/9)) | Experimental memory/mobile adapter | E01a | R9: preserve notices; bounded cancellable acquisition, private DTO conversion, no credential logs or privilege escalation; synthetic failures first, separately authorized live evidence recorded as pending if absent. |

## P5 - Crafting and collection (optional)

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| P01a ([#10](https://github.com/ChrisTitusTech/warframe-linux/issues/10)) | Recipe graph and missing-resource service | I01c, F03b | R10: known recipe/component data, quantity arithmetic, cycle/unknown handling; hand-calculated multi-level fixtures. |
| P01b ([#10](https://github.com/ChrisTitusTech/warframe-linux/issues/10)) | Crafting planner UI | P01a | R10/R8: prerequisite tree and missing quantities, unknown requirements and snapshot age; keyboard and scale checks. |
| P01c ([#10](https://github.com/ChrisTitusTech/warframe-linux/issues/10)) | Collection and mastery tracking | I01b, F03b | R10: only verified mastery fields or explicit user input; ownership distinct from mastery; unavailable status and provenance tested. |
| P01d ([#10](https://github.com/ChrisTitusTech/warframe-linux/issues/10)) | Foundry timers if fields are proven | P01b, F03b | R10: only schedule with verified timing/state data; stale snapshots and clock changes tested. Otherwise record as deferred, not complete. |

## P6 - Core beta readiness

| PR task | Outcome | Merged prerequisites | Acceptance / validation |
| --- | --- | --- | --- |
| B01a ([#11](https://github.com/ChrisTitusTech/warframe-linux/issues/11)) | Distributable package and notices | F01c, I01d, R02c | R1/R6: reproducible package, dependency/license inventory, clean-machine install and launch; experimental/extension features excluded unless independently complete. |
| B01b ([#11](https://github.com/ChrisTitusTech/warframe-linux/issues/11)) | Upgrade and rollback validation | B01a | Back up before migration, interrupted-upgrade and restore tests; demonstrate package/data rollback on target Linux without blindly downgrading schema. |
| B01c ([#11](https://github.com/ChrisTitusTech/warframe-linux/issues/11)) | Beta compatibility and release evidence | B01b | R1-R8: real claimed Linux matrix, accessibility/performance/privacy evidence and usage docs; all current-commit checks/review resolved. Publication remains a separate authorized action. |

## Completion and issue tracking

After each PR, record its URL/merge commit, exact validation, manual environment,
and date here, then tick the same child ID in its GitHub parent. F01a acceptance
is complete; merge status is available through its PR below. Never use `Closes #<parent>` until every child of that epic passes.
P01d can stay deferred without blocking the core beta; its parent remains open or
is explicitly rescoped by the maintainer. E01b stays pending live verification
until that evidence exists; it is not required for the import/OCR beta.

Planning documents, initial repository setup, and finalized Go/PR breakdown are
the handoff deliverables. Historical setup evidence is in
[docs/VALIDATION.md](docs/VALIDATION.md); next-machine steps are in
[docs/HANDOFF.md](docs/HANDOFF.md).

F01a acceptance completed on 2026-09-10 in
[PR #14](https://github.com/ChrisTitusTech/warframe-linux/pull/14). See
[its validation record](docs/F01a-VALIDATION.md) for the Fedora 44 X11 environment,
clean-directory build, lint/typecheck/tests, independent reviews and
user-confirmed display/normal-close evidence. The PR records the merge commit;
GitHub issue #1 tracks the child checklist.

F01b acceptance completed on 2026-09-10 in
[PR #16](https://github.com/ChrisTitusTech/warframe-linux/pull/16).
[Its evidence record](docs/F01b-VALIDATION.md) links the intentional-failure
and passing fresh-Ubuntu-runner runs; the PR provides final checks and merge commit.
