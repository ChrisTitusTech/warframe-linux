# Repository conventions

- The Go implementation plan is finalized. Start with docs/HANDOFF.md and respect
  the user's current phase boundary.
- Read SPEC.md, ROADMAP.md, and TASKS.md before changing product scope.
- Preserve oldhelp as a reference snapshot and retain its license and attribution.
- Use Go + Wails v2 + React/TypeScript + SQLite; do not restart language selection.
- New application code belongs outside oldhelp.
- One TASKS.md child task per PR. Follow CONTRIBUTING.md size and merge guidance.
- Keep provider/domain logic independent of desktop UI bindings.
- Do not run live memory, account, or desktop capture tests without authorization.
- Never commit credentials, personal inventory, memory dumps, or local databases.
- Record real validation evidence; documentation checks do not prove app behavior.
- Use ASCII punctuation and preserve unrelated work.
