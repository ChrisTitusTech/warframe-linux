# Contributing

The Go implementation plan is finalized. Read [SPEC.md](SPEC.md),
[ROADMAP.md](ROADMAP.md), [TASKS.md](TASKS.md), and [handoff](docs/HANDOFF.md).
Use Go + Wails v2 + React/TypeScript + SQLite. Original contributions are accepted
under the project's [MIT + Commons Clause v1.0](LICENSE) terms, unless separately
agreed. Preserve third-party licenses and attribution. F03a still requires the
reuse/data attribution inventory before copying reference code.

## Small PR workflow

- One child task ID and one reviewable outcome per PR. Parent issues are tracking
  epics; do not implement an entire epic or phase in one PR.
- Aim for 100-400 changed handwritten lines and at most 10 relevant files. Above
  600 handwritten lines, split first; if the change is inseparable, explain why
  in the PR. These are review targets, not reasons to omit tests or error handling.
- Generated scaffold, lockfiles, and fixture data may exceed those targets; identify
  them and keep that PR free of unrelated feature work. Separate mechanical moves
  from behavior changes when each remains buildable.
- Branch from current main as `codex/<task-id>-<short-description>`. Merge dependency
  PRs before starting dependents; avoid stacked PRs by default.
- Include implementation, relevant tests, task status, and docs for that outcome.
  Keep the app buildable after every merge. Use an internal fixture/disabled route
  for incomplete features rather than presenting a broken user workflow.
- Open ready-for-review PRs only after relevant checks pass. Review the full diff,
  verify independent review and remote checks for the current commit, resolve
  actionable comments, then squash merge when authorized. No automatic permission
  to publish or merge future work is conferred by this document.
- Mark a task complete only after its acceptance evidence passes. Close the parent
  issue only after every child is complete; attach manual environment evidence
  where required. Update TASKS.md and the GitHub parent checklist together.

Preserve `oldhelp/` as reference; move code only with recorded provenance. Never
submit real inventory exports, session credentials, game memory dumps, or
unredacted account screenshots.

Planning checks: local Markdown links, task dependency consistency, and
`git diff --check`. Run the automated commands in [the development guide](docs/DEVELOPMENT.md)
before publication. F01b's `Validate desktop / Build, lint and test` workflow
enforces build, frontend typecheck/lint/tests, Go race tests/vet and clean tracked
outputs. Required manual evidence remains separate. Oldhelp commands remain in
its original README and are not new-app validation. Do not run tests tagged
`manual` without checking their live-game requirements.
