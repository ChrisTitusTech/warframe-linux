# Contributing

This repository is in planning. Start with [SPEC.md](SPEC.md),
[ROADMAP.md](ROADMAP.md), and [TASKS.md](TASKS.md). Discuss the matching backlog
item before beginning application work. Framework and new-code license decisions
are still open.

Use small branches and ready-for-review pull requests. Describe the behavior,
requirement/task ID, relevant validation, and any remaining blocker using the PR
template. Preserve `oldhelp/` as reference; move code only with recorded provenance.
Never submit real inventory exports, session credentials, game memory dumps, or
unredacted account screenshots.

Planning checks: verify local Markdown links and staged whitespace with
`git diff --cached --check`. Application build/test commands will be pinned after
P0; oldhelp commands remain in its original README and are not new-app validation.
Do not run tests tagged `manual` without checking their live-game requirements.
