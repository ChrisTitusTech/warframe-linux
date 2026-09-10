# Desktop development

F01a uses the Wails v2 React/TypeScript template with a fictional, offline catalog.
The supported Wails layout keeps `main.go` at the repository root so embedded
`frontend/dist` assets and generated bindings work without custom tooling.
`app.go` is the narrow desktop binding; `internal/services` has no Wails imports.
SQLite, provider contracts and real catalog data belong to later tasks.

## Pinned tools

| Tool | Version | Pin |
| --- | --- | --- |
| Go | 1.26.7 | `.go-version`, `go.mod` |
| Wails | 2.15.0 | `go.mod`, install command below |
| Node | 24.18.0 | `.node-version`, frontend engines |
| npm | 11.16.0 | frontend engines and packageManager |

Install Go 1.26.7 from [Go downloads](https://go.dev/dl/) and Node 24.18.0 from
[Node downloads](https://nodejs.org/en/download). Verify upstream checksums for
downloaded archives. Ensure these versions are on PATH, then install npm/Wails:

```sh
npm install --global npm@11.16.0
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
export PATH="$(go env GOPATH)/bin:$PATH"
go version
node --version
npm --version
wails version
```

On Fedora 44 x86_64, install native dependencies:

```sh
sudo dnf install gcc gcc-c++ pkgconf-pkg-config gtk3-devel webkit2gtk4.1-devel
pkg-config --modversion gtk+-3.0 webkit2gtk-4.1
wails doctor
```

Follow [Wails v2 installation guidance](https://v2.wails.io/docs/gettingstarted/installation/)
for other distributions. Fedora 44 uses WebKitGTK 4.1 and requires the
`webkit2_41` build tag. Wails 2.15.0 doctor checks older Fedora WebKit package
names and RPM-managed npm, so it reports missing dependencies for this setup
even with working pkg-config and a user-installed npm. Record the diagnostic;
verify the actual build rather than installing obsolete packages to silence it.

## Build and validate

Run from the repository root in this order on a fresh checkout:

```sh
go mod download
npm ci --prefix frontend
wails build -tags webkit2_41
npm run typecheck --prefix frontend
npm run lint --prefix frontend
npm test --prefix frontend
go test -race -tags webkit2_41 . ./internal/...
go vet -tags webkit2_41 . ./internal/...
git diff --check
./build/bin/warframe-linux
```

The Wails build generates ignored `frontend/wailsjs` bindings before compiling
the frontend. Do not run frontend checks before the first Wails build. The npm
lockfile pins dependency resolution; `npm ci` also runs inside the Wails build.
Scope Go checks to the app and `internal` so they exclude both `oldhelp` and Go
source embedded in frontend dependencies. Go assets require a built frontend.
For interactive development, use `wails dev -tags webkit2_41` after installation.

Manual acceptance: confirm the normal desktop window shows the mock-data notice
and all three examples, then close it with the window manager and confirm process
exit. Report blank content or rendering errors even if compilation succeeds.
No game, account, capture, personal database, or network catalog is used.
The frontend tests cover backend results, failure/retry and empty states; the Go
test verifies callers cannot mutate shared fixture data.

## Template provenance

Generated with `wails init -n warframe-linux -t react-ts` using Wails v2.15.0,
then adapted for this repository. Entry point, frontend bootstrap/configuration
and Wails configuration derive from that template. The template's MIT notice is
preserved in [Wails-MIT.txt](licenses/Wails-MIT.txt). Template logos and fonts are
excluded. No code or assets were copied from `oldhelp`.

## Additional implementation reference

At the user's request, inspected [WFHelper/WFHelper](https://github.com/WFHelper/WFHelper/tree/d32a3528012db36d020d0633fbc3ec40271e4629)
on 2026-09-10. Its Electron/Svelte implementation supplies reference cases for
later tasks, without changing the selected Go/Wails stack:

- F02a/F02b: [display backend fallback](https://github.com/WFHelper/WFHelper/blob/d32a3528012db36d020d0633fbc3ec40271e4629/services/linuxDisplayBackend.ts).
- R02a: [capture stream lifetime, decline cooldown and generation tracking](https://github.com/WFHelper/WFHelper/blob/d32a3528012db36d020d0633fbc3ec40271e4629/services/linuxStreamCapture.ts).
- I01a: [inventory envelope tests](https://github.com/WFHelper/WFHelper/blob/d32a3528012db36d020d0633fbc3ec40271e4629/tests/main/inventoryPayload.test.ts).
- R01a: [ambiguous OCR negative cases](https://github.com/WFHelper/WFHelper/blob/d32a3528012db36d020d0633fbc3ec40271e4629/tests/main/relicEraAmbiguity.test.ts).

Its root LICENSE identifies MIT, copyright 2026 WFHelper. No source or assets
were copied, and the app was not executed. F03a must check file/dependency/data
notices before reuse. These references are not evidence of Wails compatibility
or a verified provider contract for this project.
