# GitHub setup

Target: https://github.com/ChrisTitusTech/warframe-linux

Inspected 2026-09-09: existing public repository, empty default branch, local unborn
`main`, origin already correct, administrator access available. Issues and Projects
enabled. Secret scanning and push protection already enabled; preserve them.
The owner explicitly authorized initial commit publication and repository setup.

## Initial setup scope (completed)

Publish planning documents, contribution/issue/PR templates, ignore rules, and the
unchanged oldhelp reference including its license. Add description/topics, use
squash merging and automatic branch deletion, enable private vulnerability reporting,
and create P0-P6 milestones plus phase labels and one issue per TASKS.md row.
No deployment, release, application scaffold, or live account integration.

Use issues/milestones as the initial project board; do not create an organization
Project requiring broader account scope. Main-branch protection is deferred until
actual CI check names exist and the maintainer chooses a solo-contributor review
policy. Do not invent a required check that prevents all merges. Wiki settings and
existing repository access/security settings remain as inspected.

## Finalized Go handoff

Go + Wails v2 + React/TypeScript + SQLite is selected. TASKS.md splits work into
31 PR-sized child tasks; the 11 original issues remain tracking epics with matching
child checklists. P0 is Go desktop foundation, not a Wails/Rust comparison. Update
those issues and milestone descriptions when publishing this handoff. Each future
PR references a child ID; parent issues close only after all children pass.

The handoff is published through a ready-for-review planning PR and squash merged
to main after local checks and independent Codex review. It leaves application
implementation for the next machine. An existing unrelated Dependabot PR is
outside this change. See [HANDOFF.md](HANDOFF.md).

## CI rollout

P0 pins framework/compiler/frontend dependencies and establishes real build,
format/lint/typecheck/test commands. P1 adds fixture-based unit/provider tests,
SQLite migration tests, and frontend tests where behavior warrants them. P3 adds
OCR fixtures. P6 adds reproducible packaging and dependency/license inventory.
Use read-only workflow permissions, pinned action revisions, and no game/account
secrets in CI. Live desktop/manual tests run outside hosted CI with explicit
consent and redacted evidence. Configure required checks only after they run.

## Publication checks

Check local links, whitespace, accidental sensitive files, oldhelp preservation,
and exact staged scope before committing. Verify remote `main` matches the pushed
commit, then read back description, topics, merge options, private reporting,
milestones, and issues. Record results in [VALIDATION.md](VALIDATION.md).
