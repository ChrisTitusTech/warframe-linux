# Go implementation architecture

A Wails v2 desktop shell, React/TypeScript views, a Go service layer, SQLite, and replaceable
providers. No hosted backend is required. This layout is proposed, not created:

```text
cmd/companion/             desktop entry point
frontend/                 React/TypeScript views and typed bindings
internal/domain/          Item, InventorySnapshot, MarketQuote, RewardCandidate
internal/services/        catalog, inventory, pricing, rewards, crafting
internal/providers/       market, catalog, import, capture, OCR, experimental
internal/storage/         SQLite repositories and versioned migrations
internal/platform/        XDG paths, capture capabilities, desktop integration
testdata/                 synthetic/redacted provider and image fixtures
oldhelp/                  preserved reference and original notices
```

## Data flow and boundaries

UI commands -> service validation -> provider/storage -> typed result and progress
event -> UI. Only backend providers make network calls or access local files.
Do not expose arbitrary shell commands, arbitrary URL fetches, process memory, or
raw credentials as frontend bindings. Load bundled UI assets; external links open
in the user's browser after scheme validation. No runtime third-party UI plugins.

Implement these contracts as small Go interfaces with context.Context for cancellable
operations. Bind only service-facing DTOs to the frontend:

| Contract | Inputs | Outputs / errors |
| --- | --- | --- |
| CatalogProvider.Sync | cancellation, source revision | item/recipe/drop batch + provenance; schema/network errors |
| InventoryProvider.Snapshot | cancellation, explicit provider configuration | versioned snapshot + timestamp; unavailable/permission/schema errors |
| MarketProvider.Quotes | canonical item IDs, rank, platform | timestamped quotes; rate-limit/stale/unavailable states |
| CaptureProvider.Frames | user-selected source, cancellation | bounded frame stream; denied/unsupported/source-closed states |
| RewardRecognizer.Recognize | image region, locale, catalog revision | slot-indexed candidates + confidence; unknown preserved |
| CapabilityService.Probe | desktop context | supported actions + user-readable limitations |

Events carry operation IDs, progress, timestamps, and structured error codes.
Cancellation stops subprocesses and network work; bound queues and response sizes.
Providers must be replaceable with fixtures. Start with compile-time adapters, not
a dynamic plugin loader. Experimental acquisition may later be a separate helper;
its IPC must be narrow and credentials remain within the helper.

## Storage

Use XDG data/config/cache locations under `warframe-linux`; do not overwrite
`~/.warframe-helper`. Explicitly import its inventory only through file selection.
Proposed normalized tables: items, aliases, recipes, recipe_components, drops,
inventory_snapshots, inventory_entries, market_quotes, sync_runs, schema_version.
Inventory entries retain canonical game ID, raw unknown ID, quantity, rank, and
snapshot ID. Market identity also includes platform, rank, price type, and time.
Mastery is separate from ownership. Stable game IDs are the join key; Market slugs
and localized names are aliases, not primary keys.

Apply migrations transactionally; back up before a schema upgrade. Ingestion
validates a staging batch, then atomically promotes it. Sync failure cannot mark a
partial catalog fresh. Cache quote requests, deduplicate concurrent lookups, and
honor provider retry guidance with a central limiter. Endpoint versions, terms,
headers, and actual rate limits must be verified before coding; do not assume the
old v1 statistics endpoint remains supported.

## Capture and Linux behavior

Prefer user-consented XDG ScreenCast portal + PipeWire for Wayland after proving
it works on the target desktops. Keep image import as a universal fallback and
an explicit X11/screenshot adapter where supported. A portal supplies capture;
it does not guarantee click-through, positioning, global shortcuts, or overlays.
Test each separately. No desktop framework selection is proof of an overlay
working over a fullscreen Proton game. Ship a normal reward window when necessary.

Source: [XDG ScreenCast interface](https://flatpak.github.io/xdg-desktop-portal/docs/doc-org.freedesktop.portal.ScreenCast.html).

## Personal data and experimental adapter

Do not run the app as root, change ptrace/sysctl settings, grant broad capabilities,
or silently retry with privilege escalation. Keep experimental session data only
for the acquisition operation; redact URLs and errors; never send it to Market,
the renderer, crash reports, or fixtures. Validate inventory content and save only
needed fields. Opt-in can be revoked; revocation cancels work and clears transient
state. Captures are processed locally and not retained by default.

Live access is not necessary for unit tests. Test expired credentials, permission
denial, logout, malformed responses, and cancellation through fixtures. A live
experiment requires explicit user authorization and a recorded environment. The
planning pass performs no such experiment.
