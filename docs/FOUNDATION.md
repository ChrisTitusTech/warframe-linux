# Foundation decision

Date: 2026-09-09. Status: proposed; no framework implementation is authorized in
this planning pass. The owner accepts considering a rewrite.

## Recommendation

Start the feasibility work with **Go services, Wails, TypeScript/React, and SQLite**.
Reuse reviewed domain behavior from oldhelp, replacing its terminal presentation
and hard-coded paths. This minimizes simultaneous changes while allowing a rich
inventory table, crafting detail panel, and reward view. React is a proposed
maintainability choice, not a requirement imposed by Wails.

Use Wails v2 as the stable baseline. Evaluate v3 specifically if its multi-window
and tray APIs make the reward panel substantially simpler. As checked on this
date, [Wails v3](https://v3.wails.io/) describes itself as beta with a stable desktop
API. Pin the selected version after the spike; do not use floating latest in CI.
Do not infer performance from framework marketing: measure the actual application.

## Options

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

## Decision gate

P0 compares a Wails mock-data desktop slice with Tauri if Wails hits a material
blocker or the owner prefers Rust. Run the same display/capture/package checks.
Accept Wails if the required Linux matrix works with a normal-window fallback,
installation is reproducible, and backend operations stay cancellable without
freezing the UI. If it fails, record the failing environment and choose Tauri or
Electron from measured evidence. Avoid maintaining two production backends.

A rewrite does not automatically erase inherited licensing obligations. Preserve
attribution for copied/adapted work, and resolve the intended distribution license
before moving reference code into the application. No root MIT license is granted
by this planning scaffold. See [provenance](../NOTICE.md).
