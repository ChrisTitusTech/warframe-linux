# Warframe Linux

**License: [MIT + Commons Clause v1.0](LICENSE) (source-available).**

Planning an Overwolf-independent, Linux-first Warframe companion for inventory,
market prices, relic rewards, crafting, and collection progress.

**Status: Go implementation plan finalized; development starts on the next machine.**
There is no new desktop application or release yet.
The existing [oldhelp](oldhelp/README.md) directory is a preserved Go reference
implementation, not a supported release of this project.

## Start here

- [Accepted foundation and alternatives](docs/FOUNDATION.md)
- [Product specification](SPEC.md)
- [Architecture and provider contracts](docs/ARCHITECTURE.md)
- [Phased roadmap](ROADMAP.md)
- [Implementation backlog](TASKS.md)
- [GitHub setup](docs/GITHUB_SETUP.md)
- [Start on another machine](docs/HANDOFF.md)
- [Validation evidence](docs/VALIDATION.md)
- [Contribution guidance](CONTRIBUTING.md)
- [Source provenance and licensing](NOTICE.md)

The selected foundation is Go + Wails v2 + React/TypeScript + SQLite. Work is split
into PR-sized tasks with dependencies and acceptance checks. Begin with F01a in
[TASKS.md](TASKS.md); exact toolchain versions are pinned and verified there.
No Overwolf runtime, SDK, account, or service is part of the proposed design.

The reference inventory reader finds session credentials in game memory and
requests a snapshot from the mobile inventory endpoint. That is not a supported,
general-purpose live game API. The new design keeps this experimental acquisition
path separate from catalog, pricing, import, and OCR functionality.

This project is unofficial and is not affiliated with Digital Extremes,
AlecaFrame, Overwolf, or Warframe Market. Warframe is a Digital Extremes trademark.
Project-authored material is licensed under MIT + Commons Clause v1.0.
Third-party material retains its own terms. This is not plain MIT or an
OSI-approved open-source license. See [LICENSE](LICENSE) and [NOTICE.md](NOTICE.md).
