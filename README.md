# Warframe Linux

Planning an Overwolf-independent, Linux-first Warframe companion for inventory,
market prices, relic rewards, crafting, and collection progress.

**Status: planning. There is no new desktop application or release yet.**
The existing [oldhelp](oldhelp/README.md) directory is a preserved Go reference
implementation, not a supported release of this project.

## Start here

- [Foundation options and recommendation](docs/FOUNDATION.md)
- [Product specification](SPEC.md)
- [Architecture and provider contracts](docs/ARCHITECTURE.md)
- [Phased roadmap](ROADMAP.md)
- [Implementation backlog](TASKS.md)
- [GitHub setup](docs/GITHUB_SETUP.md)
- [Validation evidence](docs/VALIDATION.md)
- [Contribution guidance](CONTRIBUTING.md)
- [Source provenance and licensing](NOTICE.md)

The proposed foundation is Go + Wails + TypeScript/React + SQLite. Tauri/Rust is
an alternative to evaluate in the first prototype; a rewrite is acceptable.
Framework selection is provisional until Linux desktop and packaging checks pass.
No Overwolf runtime, SDK, account, or service is part of the proposed design.

The reference inventory reader finds session credentials in game memory and
requests a snapshot from the mobile inventory endpoint. That is not a supported,
general-purpose live game API. The new design keeps this experimental acquisition
path separate from catalog, pricing, import, and OCR functionality.

This project is unofficial and is not affiliated with Digital Extremes,
AlecaFrame, Overwolf, or Warframe Market. Warframe is a Digital Extremes trademark.
Inherited code has MIT + Commons Clause terms; this repository must not be
represented as plain MIT. See [NOTICE.md](NOTICE.md).
