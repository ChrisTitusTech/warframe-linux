# F01a validation - 2026-09-10

Base: `fb72200`; branch: `codex/f01a-go-desktop-scaffold`.
Local implementation, automated checks and manual acceptance passed.
PR and merge evidence will be recorded with publication.

## Environment

- Fedora Linux 44, x86_64, X11 (`DISPLAY=:0`). No Wayland claim.
- AMD Ryzen 7 7800X3D, 31 GiB reported by Wails doctor; NVIDIA RTX 4070 SUPER.
- Fedora Go `go1.26.7-X:nodwarf5`, Node v24.18.0, npm 11.16.0, Wails v2.15.0.
- GCC 16.0.1, pkg-config 2.5.1, GTK 3.24.52, WebKitGTK 2.52.5 (4.1 API).
- Installed GTK/WebKit development packages with dnf. Existing Go/Node were used.

## Results

- Clean directory populated from the Git index (no node_modules, dist or bindings):
  `go mod download`, `npm ci --prefix frontend`, native Wails build, frontend
  typecheck/lint/tests, `go test -race -tags webkit2_41 . ./internal/...` and
  `go vet -tags webkit2_41 . ./internal/...` all passed. This reused installed
  system tools and the Go module cache; it was not a fresh operating system.
- `git diff --cached --check`: passed after removing a template trailing blank line.
- `wails build -tags webkit2_41`: passed, Linux amd64 executable produced.
- `npm run typecheck --prefix frontend`: passed.
- `npm run lint --prefix frontend`: passed.
- `npm test --prefix frontend`: passed, two behavior tests.
- `go test -race -tags webkit2_41 ./...`: passed, one service test; also traversed
  a dependency's Go sources under node_modules. Documented commands now scope
  checks to `. ./internal/...` to avoid this unrelated package.
- `go vet -tags webkit2_41 ./...`: passed.
- `wails doctor`: completed with a fatal diagnosis of missing libwebkit/npm
  despite exit status 0. Its Fedora package table only recognizes WebKit 4.0/3
  names and RPM npm packages. Installed WebKit 4.1 is verified by pkg-config and
  successful linking; user-local npm is verified by frontend installation/build.
- Initial build caught and corrected a malformed generated Go version pin
  (`1.26.7.0`); subsequent build passed with `1.26.7`.
- Native executable launched on X11; window manager reports the titled window.
  Startup emitted WebKit's signal-10 handler notice; no crash observed.
- User confirmed the display and normal close on 2026-09-10. The original
  executable process exited with status 0. No screenshot or capture was taken.
- `codex review --uncommitted`: completed, no actionable defects. Reviewer also
  reproduced the clean staged-checkout build and checks. The user subsequently
  supplied the outstanding manual acceptance above.
- `coderabbit review --agent -t uncommitted` (CLI 0.7.6): completed, zero findings
  across all 29 changed files.
- Local Markdown links: passed. `oldhelp` has no changes.
- Remote PR checks, reviews and threads: to be verified after publication.
  Application CI is introduced in F01b; no application CI gate exists for F01a.
  Generated bindings, build outputs and node_modules are ignored.
- Scope: one child task. The bulk of the diff is generated npm/Go lock data
  and Wails template configuration; handwritten work is the mock shell,
  behavior tests and development/evidence documentation. `oldhelp` is untouched.

Only this host is exercised. No live game/account/memory/capture tests were run.
Packaging measurements, CI and broader desktop compatibility remain later tasks.
