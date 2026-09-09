# Foundation decision

Date: 2026-09-09. Status: accepted by the owner. Application implementation starts
on the next machine; this change finalizes the plan and handoff.

## Accepted stack

**Go backend + Wails v2 desktop shell + React/TypeScript frontend + SQLite.**
Keep the Go backend independent of Wails bindings. Reuse reviewed behavior from
oldhelp while replacing its terminal presentation and hard-coded paths. React is
the selected UI framework; this is not an all-Go UI.

Go is selected for direct reuse, straightforward maintenance, and the expected
HTTP/SQLite workload. A Rust rewrite is out of scope. Wails v3, Tauri, and Electron
are historical alternatives below, not competing implementation tracks. Revisit
the shell only if a documented blocker requires an owner decision; keep Go as the
backend. A normal companion window is the baseline, with overlays conditional on
actual desktop support.

F01a pins an exact supported Go version, Wails v2 release, Node LTS version, and
frontend lockfile on the implementation machine. The oldhelp Go version is a
reference constraint to assess, not an automatic pin for the new app. F01c verifies
packaging. Do not infer performance from framework marketing: measure the app.

## Historical options (decision closed)

| Foundation | Advantages for this project | Costs and uncertainties | Choose when |
| --- | --- | --- | --- |
| Go + Wails + React | Direct Go service reuse; web UI; one backend language | Linux WebKitGTK dependencies; window/capture behavior needs a real desktop spike | Default: shortest path to a maintainable companion |
| Rust + Tauri 2 + React | Typed native backend; opportunity to redesign data contracts | Reimplement Go behavior or ship a Go sidecar with versioned IPC; Linux still needs WebKitGTK | Rust is the preferred maintenance language or Wails fails the spike |
| Electron + React + Go sidecar | Chromium consistency; JavaScript desktop ecosystem | Bundled Chromium/Node, extra process/IPC/update lifecycle; measure resource impact | WebKit compatibility proves worse than shipping Chromium |
| Qt/QML + C++ or Python backend | Native desktop controls and Linux integration | New language/binding and deployment surface; audit module licenses; Go reuse needs a bridge | Native desktop UI is more important than web UI reuse |
| Go local service + browser UI | Simple UI development, reusable HTTP boundary | Desktop capture, global shortcuts, tray, and window placement still need native work; local server needs access protection | A second-screen dashboard is the whole product |
| Extend the existing Go TUI | Least initial work; useful diagnostic interface | Does not meet the intended graphical workflow | Developer utility, not the primary product |

These are engineering judgments, not benchmark results. Wails and Tauri both
have Linux system-webview dependencies. Electron bundles Chromium and Node.
No candidate automatically solves Wayland overlays.
Sources: [Wails installation](https://v2.wails.io/docs/gettingstarted/installation/),
[Tauri prerequisites](https://v2.tauri.app/start/prerequisites/),
[Electron introduction](https://www.electronjs.org/docs/latest/),
[Qt licensing](https://doc.qt.io/qt-6/licensing.html).

## What oldhelp actually provides

Inspected locally; no game process or live credentials were accessed.

| Area | Evidence | Reuse decision / gap |
| --- | --- | --- |
| Catalog and SQLite | `oldhelp/utils/database/` | Reuse after schema and transaction review; currently category tables, no enforced foreign keys |
| Prices and inventory parsing | `oldhelp/utils/pricechecker/priceChecker.go` | Retain mappings/tests where useful; inventory selection currently collapses ownership into item references, so preserve quantities in new model |
| Inventory acquisition | `oldhelp/utils/memoryScanner/` | Linux `/proc` reader finds account/session query data, then calls mobile inventory endpoint; not arbitrary live inventory extraction |
| OCR and reward detection | `oldhelp/utils/relicdetect/` | Useful fixtures, template detection, Tesseract integration; expand resolution/language coverage |
| Linux capture | `oldhelp/utils/relicdetect/capture_linux.go` | External screenshot commands; replace automatic binary selection with explicit capability probing and portal support |
| Terminal presentation | `oldhelp/views/`, `oldhelp/utils/viewBuilder/` | Reference workflows; do not couple new domain code to Bubble Tea |
| Tests | Tests beside packages, manual build tags | Existing coverage is unverified in this planning pass; manual tests are not evidence of real compatibility |

The module declares Go 1.26.2 and uses CGO SQLite. Catalog requests use Market v2,
while price statistics use a v1 endpoint. Both must be checked against current
provider documentation before reuse. Inventory saves currently use mode 0644;
new private snapshots should use 0600 under a 0700 private directory. HTTP error
wrapping can include request URLs, so credential-bearing requests need explicit
redaction before reuse. Do not persist or emit session material through UI events.

## Acquisition options

| Provider | What it can support | Boundary |
| --- | --- | --- |
| Public catalog/drop/market data | Search, crafting reference, market estimates | No personal ownership or mastery state |
| User-selected inventory import / manual edits | Owned quantities, set completion, planning | Snapshot only; validate schema; never claim freshness beyond import time |
| Screen capture + OCR | Visible relic reward names and prices | Cannot establish complete account inventory; user corrects uncertain matches |
| Log reader, if useful fields are demonstrated | Potential mission/event hints | Research only; do not assume logs contain full inventory or credentials |
| Experimental oldhelp memory/mobile adapter | Potential personal inventory snapshot | Session permissions, endpoint stability, field coverage, and account risk remain unverified |
| Official supported integration, if available later | Replace private acquisition path | Do not invent an available official inventory OAuth API |

The [Digital Extremes policy](https://support.warframe.com/hc/en-us/articles/360030014351-Third-Party-Software-and-You)
says third-party software is used at the player's risk. Read-only access and
removing Overwolf are not evidence of approval. Preserve the reference adapter's
explicit opt-in/revocation behavior if it is carried forward. This is a concrete
release constraint for that provider, not a reason to block the other features.

## Implementation gate

P0 proves the selected Wails v2 shell builds and runs on Linux, measures the mock
workflow, and records capture and packaging limitations. No Rust comparison spike
is scheduled. Capture failures produce explicit unsupported capabilities and a
normal-window/image-import fallback; they do not silently change the language.
Only demonstrated blockers warrant revisiting the shell with the owner.

A rewrite does not automatically erase inherited licensing obligations. Preserve
attribution for copied/adapted work, and resolve the intended distribution license
before moving reference code into the application. No root MIT license is granted
by this planning scaffold. See [provenance](../NOTICE.md).
