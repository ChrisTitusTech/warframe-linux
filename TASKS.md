# Task backlog

Owner: repository maintainer. Status: proposed until implementation is authorized.
Acceptance requirements refer to [SPEC.md](SPEC.md); phase gates to
[ROADMAP.md](ROADMAP.md).

| ID | Phase / task | Acceptance and validation | Depends on | Status |
| --- | --- | --- | --- | --- |
| F01 | P0: desktop foundation and packaging spike | Mock catalog opens; record Wails baseline, Tauri comparison if needed; clean-machine package test; launch/RSS/search measurements | Planning review | Not started |
| F02 | P0: Linux capture/window matrix | KDE/GNOME Wayland and X11 capture/deny/revoke; fullscreen, focus, scaling, normal-window fallback evidence | Planning review | Not started |
| F03 | P0: provenance and field contracts | Decide reuse/license; map sanitized inventory fields for quantities/mastery/foundry; verify Market endpoints and data attribution | Planning review | Not started |
| C01 | P1: service/storage foundation | Typed fixture providers; transactional migration/rollback; private XDG paths; cancellation tests | F01, F03 | Not started |
| C02 | P1: catalog and price adapters | Offline/stale/429/partial sync contract tests; canonical IDs; rank/platform quote semantics visible | C01 | Not started |
| I01 | P2: inventory import and sets | Atomic import; quantity/rank preservation; unknown entries; set arithmetic fixtures and manual table workflow | C02 | Not started |
| R01 | P3: image reward recognition | Held-out corpus and negative-screen results; correction UX; slot order and stale quote display | C02, F02 | Not started |
| R02 | P3: live capture and reward panel | User selects source; stop/revoke work; live desktop evidence and normal-window fallback | R01 | Not started |
| E01 | P4: experimental inventory provider | Opt-in/revoke, credential redaction, no escalation; failure fixtures; separately authorized live test | I01, F03 | Not started |
| P01 | P5: crafting and collection | Recipe graph fixtures, missing quantities and cycles; mastery separate from ownership; timers only with proven data | I01, F03 | Not started |
| B01 | P6: beta readiness | Clean install/upgrade/rollback; notices; required automated and manual checks; documented unsupported modes | I01, R02 | Not started |

## Planning/setup evidence

- [x] Inspect oldhelp and existing GitHub repository without running the game adapter.
- [x] Compare foundations using current primary documentation.
- [x] Write requirements, architecture, roadmap, and acceptance backlog.
- [x] Validate and publish initial scaffold; record commit and repository configuration.

The final setup status is recorded in [validation evidence](docs/VALIDATION.md).
No application build, test, or real-game compatibility claim is made by these checks.

## Published GitHub backlog

- F01: [#1](https://github.com/ChrisTitusTech/warframe-linux/issues/1)
- F02: [#2](https://github.com/ChrisTitusTech/warframe-linux/issues/2)
- F03: [#3](https://github.com/ChrisTitusTech/warframe-linux/issues/3)
- C01: [#4](https://github.com/ChrisTitusTech/warframe-linux/issues/4)
- C02: [#5](https://github.com/ChrisTitusTech/warframe-linux/issues/5)
- I01: [#6](https://github.com/ChrisTitusTech/warframe-linux/issues/6)
- R01: [#7](https://github.com/ChrisTitusTech/warframe-linux/issues/7)
- R02: [#8](https://github.com/ChrisTitusTech/warframe-linux/issues/8)
- E01: [#9](https://github.com/ChrisTitusTech/warframe-linux/issues/9)
- P01: [#10](https://github.com/ChrisTitusTech/warframe-linux/issues/10)
- B01: [#11](https://github.com/ChrisTitusTech/warframe-linux/issues/11)
