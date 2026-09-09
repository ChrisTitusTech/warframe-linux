# Start implementation on another machine

The accepted stack is Go + Wails v2 + React/TypeScript + SQLite. Language selection
is complete. The repository contains a finalized plan and the preserved oldhelp
reference, not a new application. Start with **F01a**, tracked under
[issue #1](https://github.com/ChrisTitusTech/warframe-linux/issues/1).

## Checkout

For a new checkout:

```sh
git clone https://github.com/ChrisTitusTech/warframe-linux.git
cd warframe-linux
git status --short
git switch -c codex/f01a-go-desktop-scaffold
```

For an existing checkout, inspect and preserve local changes first, then update
main with `git pull --ff-only` before creating the task branch. Do not reset or
clean an existing worktree to follow this guide.

Read AGENTS.md, SPEC.md, ROADMAP.md, TASKS.md, CONTRIBUTING.md, and
[the architecture](ARCHITECTURE.md). GitHub issues #1-#11 track the child tasks;
keep their checklists synchronized as PRs merge. No completed application work
needs to be transferred outside this repository.

## F01a scope and exit

1. Inspect the machine's distro, architecture, desktop session, compiler and
   package manager. Install the required Go, Node/npm, C build tools, GTK and
   WebKitGTK development dependencies appropriate to that machine. Verify against
   [Wails v2 installation guidance](https://v2.wails.io/docs/gettingstarted/installation/).
2. Pin exact supported Go, Wails v2 and Node versions plus the frontend lockfile.
   Record them and repeatable install/build/lint/test commands in a development
   guide committed with the scaffold. Do not blindly copy oldhelp's version pins.
3. Create a fresh Wails v2 React/TypeScript shell with module identity
   `github.com/ChrisTitusTech/warframe-linux`, Go services behind narrow bindings,
   and a mock catalog. Start from the framework's supported layout; if its entry
   point differs from the proposed tree, record the small layout decision.
4. Prove actual build, lint/typecheck and relevant tests on this machine. Record
   `wails doctor` diagnostics and manually open/close the mock desktop window.
   Tests should verify behavior, not repeat generated boilerplate. A headless build
   cannot replace the required window evidence.
5. Open one ready PR for F01a when authorized, including its environment and results.
   Stop at this task's boundary; F01b adds CI after the commands work locally.

No catalog networking, live capture, memory scan, migration of oldhelp code,
installer framework, or inventory feature belongs in this first scaffold PR.
Preserve oldhelp and its license. F03a resolves the new-code license and reuse
notices before copying it; that decision can proceed independently of F01a.

## Pending evidence and decisions

Exact Linux versions and package behavior are measured in P0. Source/schema
coverage is F03b. Game access and private inventory acquisition require explicit
live-test authorization, and no credentials or personal snapshots are needed for
F01a. There is no installed toolchain or passing app build to inherit from the
planning machine. Record new evidence in docs/VALIDATION.md or linked per-task
reports instead of treating the planning checks as runtime validation.

An unrelated Dependabot PR may target oldhelp; review it separately. Preserving
reference code does not certify its dependencies for release. Do not merge it
as part of the planning handoff or silently include its changes in F01a.
