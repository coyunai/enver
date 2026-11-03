# Contributing

Thanks for helping improve enver.

## Prerequisites

- Go 1.25+
- Node.js 22+ and npm
- vfox 1.0.13+ (`vfox --version`)
- On Windows: Developer Mode or an elevated shell if you want to test project-scoped
  switches (vfox creates symlinks).

## Setup

```bash
git clone https://github.com/enver-dev/enver.git
cd enver

cd web && npm ci && npm run build && cd ..
go build ./...
```

`internal/webui/dist/` is committed on purpose. Always rebuild it and include the complete
directory in a commit that touches `web/`; otherwise `go install` users get a broken UI.

## Development loop

```bash
# terminal 1: API + embedded UI, no auth
go run ./cmd/enver ui --no-auth --no-open --port 7788

# terminal 2: Vite dev server with /api proxy
cd web && npm run dev
```

The Vite server runs on `http://127.0.0.1:5173`.

## Before opening a PR

```bash
gofmt -w cmd internal
go vet ./...
go test ./... -count=1
cd web && npm run build
```

CI runs the same checks on Linux, Windows and macOS. A dependency audit can be run against
the official registry when your npm mirror does not implement the audit endpoint:

```bash
cd web && npm audit --omit=dev --registry=https://registry.npmjs.org
```

## Build artifacts and repository hygiene

Ignored locally (never commit):

- `dist/`, `.tools/`, `web/node_modules/`, `web/dist/`, `go.work`
- binaries, coverage output, logs, `.env*` files
- editor and OS state (`.idea/`, `.vscode/`, `.DS_Store`, `Thumbs.db`, …)

Committed intentionally:

- `internal/webui/dist/` — the embedded UI, so `go install` works without Node.js
- `web/package-lock.json` — required for reproducible `npm ci`

## Versioning

The version is maintained in three places and must stay in sync:

| File | Field |
| --- | --- |
| `internal/buildinfo/buildinfo.go` | `Version` |
| `Makefile` | `VERSION` |
| `web/package.json` + `web/package-lock.json` | `version` |

Release tags override the Go value through `-ldflags`. The UI reads the version from
`/api/overview`, so no frontend rebuild is needed solely for a version bump.

## Code guidelines

- Keep vfox as the single source of truth for runtime versions; do not duplicate its state.
- Do not parse human-oriented vfox output when a filesystem or `--json` source exists.
- All mutating vfox calls must go through the adapter and its write mutex.
- Execute commands with argument arrays, never by concatenating shell strings.
- Keep platform-specific code behind build tags or `internal/platform`.
- Add tests for new parsing, validation or state logic.
- UI text is Chinese today; keep it concise and consistent.

## Commit messages

Use conventional prefixes where possible: `feat:`, `fix:`, `docs:`, `test:`, `refactor:`,
`chore:`. Keep the subject under 70 characters and explain the "why" in the body.

## Reporting bugs

Include:

- enver version (`enver --version`)
- vfox version and `vfox config -l`
- OS and shell
- the output of `enver doctor --json`
- relevant job logs from the UI or `enver --json ...`

Redact usernames and tokens.

## Security

Please do not open a public issue for a vulnerability that could lead to arbitrary command
execution or token bypass. Contact the maintainers privately first.
