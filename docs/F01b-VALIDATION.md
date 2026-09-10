# F01b validation - 2026-09-10

Prerequisite: F01a merged in PR #14 (`ffad543`).
Branch: `codex/f01b-build-validation-ci`.

## Scope and local evidence

The desktop validation workflow runs the F01a build, typecheck, lint, frontend
tests, Go race tests and vet on Ubuntu 24.04. It also checks Go formatting and
tracked-file drift. All three actions use verified upstream commit pins, token
permissions are contents read only, persisted checkout credentials and caches
are disabled. No secrets, game access or desktop capture are used.

- `actionlint .github/workflows/validate.yml`: passed.
- `wails build -tags webkit2_41`: passed on the F01a Fedora environment.
- Frontend typecheck, lint and two behavior tests: passed.
- Go race test and vet (`. ./internal/...`, `webkit2_41`): passed.
- `git diff --check`: passed.
- Independent `codex review --uncommitted`: no actionable defects; verified pins.
- CodeRabbit CLI review: zero findings across all three implementation/doc files.

## Hosted acceptance

The branch first includes a temporary `TestIntentionalCIFailureProbe` that calls
`t.Fatal` with an explicit acceptance-probe message. It must fail at the actual
Go test step, then be removed before the final passing run and PR publication.
This is an expected test failure, not a workflow command that ignores errors.
The failing commit remains in branch history for audit and is excluded from
the final squash-merged diff.

- [Deliberate failure run](https://github.com/ChrisTitusTech/warframe-linux/actions/runs/34543606233)
  at `2ef75c4`: failed exactly at `Test Go with race detector`, exit 1, with
  `TestIntentionalCIFailureProbe` and its acceptance-probe message. The build,
  typecheck, lint and frontend tests passed; vet/drift steps were correctly
  skipped after the failure. The temporary test has been removed.
- [Restored passing run](https://github.com/ChrisTitusTech/warframe-linux/actions/runs/34543796389)
  at `5f8d937`: every build/typecheck/lint/test/vet/format/drift step passed on
  a fresh Ubuntu 24.04 GitHub runner with caches disabled.
- Independent review of the restored branch (`codex review --base main`):
  no actionable defects. The PR supplies final head checks and merge evidence.
- Hosted review identified concurrency collisions between fork PRs and a missing
  patch whitespace check. PR numbers now isolate PR concurrency groups; push
  groups use full refs. Checkout fetches history so `git diff --check` can cover
  the PR base or push-before range, with the main merge base for a first push,
  manual dispatch or unreachable pre-force-push commit. Independent review
  caught that last case; unavailable-base fallback was checked locally.
  Worktree whitespace and tracked-file drift are also checked.

The workflow proves build/test automation, not window or package compatibility.
F01a's confirmed native window acceptance is reused because application code
is unchanged. F01c still requires clean-target package and measurement evidence.
