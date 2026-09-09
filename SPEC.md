# Product specification

Status: finalized implementation plan, 2026-09-09. Go is selected by the owner.
Implementation starts on the next machine with F01a; this handoff contains no app code.

## Product goal

Make a Linux-first companion that helps a player decide what to keep, craft, sell,
and select from relic rewards without requiring Overwolf. AlecaFrame is a feature
reference, not a UI/assets clone. The design should let contributors add a provider
or feature without changing unrelated screens.

## Target and scope

Initial target: Linux x86_64, Warframe through Steam/Proton. Proposed validation
matrix: KDE Wayland, GNOME Wayland, and X11; one wlroots compositor is an additional
capture target. Record exact distro, compositor, GPU, and Proton versions in P0.
Steam Deck desktop/gaming mode and Windows are later targets, not initial claims.

MVP workflow: open app, sync catalog, search an item, inspect cached prices, import
an inventory snapshot, see quantities/set gaps, then use a selected reward image
or supported live capture to compare reward values. Each feature reports missing
capabilities without preventing the rest of the app from opening.

## Requirements and acceptance

| ID | Requirement | Acceptance evidence |
| --- | --- | --- |
| R1 | No Overwolf runtime, SDK, service, or account dependency | Dependency audit and clean-machine launch without Overwolf |
| R2 | Searchable catalog with source/freshness | Offline cached search works; interrupted sync preserves last valid catalog; empty cache offers sync/retry |
| R3 | Read-only market lookup | Price type, platform, rank, timestamp, and volume visible; unavailable is not zero; 429/backoff, timeout, and stale-cache fixtures pass |
| R4 | Inventory quantities and provenance | Import validates schema/version and size; retains quantity/rank; unknown items remain visible; failed import preserves prior snapshot |
| R5 | Relic reward comparison | One to four rewards supported; confidence and corrections visible; prices/ducats/missing ownership shown separately; no automatic in-game selection |
| R6 | Local privacy | Private snapshot permissions; export is explicit; deleting personal data removes snapshots/history; logs contain no session data or raw capture |
| R7 | Modular framework | Fixture/import provider can replace experimental provider without UI changes; services have deadlines/cancellation and typed errors |
| R8 | Linux desktop usability | Keyboard navigation, readable focus, resizing and 100/150/200 percent scale checked; unsupported overlay falls back to a normal window |
| R9 | Optional experimental inventory sync | Disabled initially; opt-in and revocation tested; unsupported permissions fail without elevated app launch or host security changes |
| R10 | Crafting/collection extension | Later phase: quantities, prerequisite trees, missing components and mastery provenance; cycles/unknown requirements handled |

R1-R8 define the core beta. R9 is a separate experimental capability and is not
required to ship an import/OCR beta. R10 follows validation of catalog and inventory
field coverage. Catalog absence must never be interpreted as an item being owned,
unowned, mastered, or unmastered.

## UX and estimates

Navigation: Dashboard, Inventory, Relics, Crafting (later), Settings/Diagnostics.
Dashboard shows provider availability and last successful sync. Inventory supports
search, sorting, owned count, and set gaps. Reward view preserves the game slot
order, with explicit value indicators; the user can change uncertain matches.
Market estimates are estimates, not promised sale prices or completed trades.

Always show loading/cancel, empty, stale, unavailable, and retry states. Persist
last good data during failures. Keep account/session mechanics out of ordinary
product screens; experimental settings explain only meaningful permissions and
limitations. No required sign-in for public catalog/pricing use.

Proposed performance targets, to measure in P0: cached search p95 under 150 ms on
10,000 items; cached reward result p95 under 2 seconds after a frame arrives;
idle combined app processes under 250 MiB RSS; capture off while idle. Network
latency is reported separately. Targets may be revised from recorded measurements.

## Non-goals for first beta

Trade/order creation, automated play/input, game memory writes or injection,
riven valuation/sniping, cloud accounts, telemetry, a public plugin marketplace,
and complete AlecaFrame parity. Foundry timers and mastery require demonstrated
snapshot fields before scheduling implementation; no inferred live timers.

## Selected foundation and bounded follow-ups

Go + Wails v2 + React/TypeScript + SQLite is final for implementation. P0 validates
that selection; it is not a language comparison. Follow-ups have explicit owners
and deadlines: F01a pins toolchain versions; F03a records the maintainer's new-code
license/reuse decision before copying reference code; F03b verifies provider field
coverage; F02a/F02b record exact Linux versions; F01c records packaging behavior.
These do not prevent starting a fresh mock-data scaffold in F01a. Proposed initial packaging
is a native archive/package with declared system dependencies. Evaluate AppImage
in P0; defer Flatpak until capture and process-access constraints are understood.
Architecture is in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).
