# GitHub setup

Target: https://github.com/ChrisTitusTech/warframe-linux

Inspected 2026-09-09: existing public repository, empty default branch, local unborn
`main`, origin already correct, administrator access available. Issues and Projects
enabled. Secret scanning and push protection already enabled; preserve them.
The owner explicitly authorized initial commit publication and repository setup.

## Initial setup scope

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
