# Architecture

Applies to enver 0.0.1 and later.

## Goals

1. Make **vfox the single source of truth** for runtime installation and version state.
2. Compose multiple runtimes into reproducible, switchable environments.
3. Provide a pleasant local UI and a complete CLI from the same implementation.
4. Stay portable: one static Go binary, no runtime dependencies for the UI.

## Non-goals

- Reimplementing downloads, extraction, checksum validation or plugin hooks.
- Managing containers, language-level dependency managers, or system package managers.
- Running as a multi-tenant server. enver is a single-user local tool.

## Component map

```
cmd/enver             cobra CLI; also starts the HTTP server
internal/app          use cases: presets, venv, packages, storage, terminal env
internal/vfox         adapter over the vfox CLI and filesystem
internal/httpapi      REST, SSE job streams, WebSocket terminal
internal/jobs         in-process background jobs with bounded log buffers
internal/preset       preset validation and JSON store
internal/venv         virtualenv records, health checks, interpreter paths
internal/packages     npm/pip introspection and manifest export
internal/doctor       diagnostics
internal/fsutil       atomic JSON, tree copy, size helpers
internal/platform     disk-free implementations
internal/run          child-process runner with line-oriented output
internal/catalog      plugin catalog fetch/cache with embedded fallback
internal/webui        embedded `dist/` assets
web                   React + Vite UI
```

Dependency direction is one-way: `cmd` → `httpapi`/`app` → domain packages. Domain packages
never import the HTTP layer.

## vfox adapter contract

`internal/vfox` deliberately uses **stable surfaces** and avoids parsing human output where a
filesystem or JSON source exists.

### Detection

- `vfox --version` → semantic version, with `MinVersion = 1.0.13`.
- Home resolution: `VFOX_HOME` → `~/.vfox` → `~/.version-fox` (pre-1.0 compatibility).
- `vfox config -l` → `storage.sdkPath`, `registry.address`, etc.

### Reads

| Data | Source | Why |
| --- | --- | --- |
| Installed versions | scan `<sdkRoot>/<sdk>/v-*` and legacy `<sdkRoot>/cache/<sdk>` | `vfox list` has no JSON output |
| Current version | symlink `~/.vfox/sdks/<sdk>` → fallback `~/.vfox/.vfox.toml` | avoids ANSI parsing |
| Project versions | `<project>/.vfox.toml` | vfox-compatible file |
| Environment | `vfox env --json` | structured and stable |
| Available versions | `vfox search <sdk>` | only public source; tolerant parsing |
| Plugin catalog | registry `index.json` with 12h cache and embedded fallback | works offline |

Incomplete installs are ignored: a sibling `.v-<version>.installing` marker or an empty
payload directory is not reported as installed.

### Writes

All mutating operations are serialized with a process-wide mutex:

- `vfox install <sdk>@<version>` — streamed to a job
- `vfox uninstall <sdk>@<version>`
- `vfox use -g|-p <sdk>@<version>`
- `vfox config <key> <value>`
- `vfox add <sdk>` / `vfox remove <sdk>`

Two Windows-specific details matter:

1. `vfox use` normally spawns a new shell when it thinks it is not running under a shell
   hook. From a long-lived server this can recursively launch the parent process. enver sets
   `__VFOX_SHELL` for `use` so vfox takes the hook path and returns normally.
2. Child processes receive an empty stdin. Without this, a helper shell can wait forever on
   the server's inherited stdin.

### Compatibility policy

- Adapter behavior is covered by unit tests for version comparison, TOML parsing, ANSI
  stripping and path parsing.
- Unknown newer vfox versions are allowed but degrade to read-only behavior when a command
  fails.
- `env --json` is preferred; shell export output is not parsed.

## State ownership

| State | Owner | Location |
| --- | --- | --- |
| Installed SDKs, plugins, links | vfox | `VFOX_HOME` / `storage.sdkPath` |
| Global and project version selection | vfox | `~/.vfox/.vfox.toml`, `<project>/.vfox.toml` |
| Presets | enver | `<configDir>/enver/presets.json` |
| Virtualenv records | enver | `<configDir>/enver/venvs.json` |
| App preferences | enver | `<configDir>/enver/config.json` |
| Plugin catalog cache | enver | `<configDir>/enver/cache/catalog.json` |
| Job logs | enver | memory only, bounded |

`configDir` is `os.UserConfigDir()`; set `ENVER_HOME` to override. enver never copies vfox
version state into its own store.

## HTTP API

All routes are served from the same binary. JSON errors use:

```json
{ "error": { "code": "venv_failed", "message": "..." } }
```

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/health` | liveness, version, auth flag (no auth) |
| GET | `/api/overview` | dashboard aggregate |
| GET/PUT | `/api/vfox` | adapter info / custom binary |
| GET | `/api/catalog` | plugin catalog with installed flags |
| GET | `/api/sdks` | installed runtimes |
| GET | `/api/sdks/{sdk}/versions` | available versions |
| POST | `/api/sdks/{sdk}/plugins` | add plugin (job) |
| POST | `/api/install`, `/api/uninstall` | runtime jobs |
| POST | `/api/use` | switch version |
| GET/POST/PUT/DELETE | `/api/presets...` | preset CRUD and apply |
| GET/POST/DELETE | `/api/venvs...` | virtualenv lifecycle |
| GET | `/api/packages`, `/api/packages/export` | global packages / manifest |
| GET | `/api/doctor` | diagnostics |
| GET/PUT | `/api/storage` | storage info / set path |
| POST | `/api/storage/migrate` | migration job |
| GET | `/api/jobs/{id}` | job snapshot |
| GET | `/api/jobs/{id}/events` | SSE log stream |
| DELETE | `/api/jobs/{id}` | cancel |
| GET | `/api/terminal` | WebSocket PTY |

## Jobs and streaming

Long operations (install, uninstall, migration) run in `internal/jobs` so that closing the
browser does not cancel them. A job keeps a bounded ring buffer of normalized lines and
fans out events to subscribers. `GET /api/jobs/{id}/events` replays history and then streams
live events as `text/event-stream`, ending with a `done` event.

The CLI subscribes to the same job abstraction, which keeps UI and CLI behavior aligned.
Finished jobs are pruned when a new job starts once they are older than 24 hours, so a
long-running local server does not accumulate log buffers indefinitely.

## Terminal

`GET /api/terminal` upgrades to a WebSocket, then starts a PTY via
`github.com/aymanbagabas/go-pty` (ConPTY on Windows, Unix PTY elsewhere).

- Binary frames: terminal output.
- Text frames from the client: `{"type":"input","data":"..."}` or
  `{"type":"resize","cols":120,"rows":30}`.
- Environment is injected directly, so the shell does not need a vfox hook. A preset
  prepends the selected runtime `bin` directories; a virtualenv prepends its `Scripts`/`bin`
  directory and sets `VIRTUAL_ENV`.
- The WebSocket `Origin` must match the request host.

## Security model

- Listener defaults to `127.0.0.1`.
- Per-launch token, constant-time comparison, exchanged for an HttpOnly SameSite=Strict
  cookie at `/auth`.
- Cross-origin unsafe requests are rejected when the `Origin` host differs.
- Execution uses `exec.Command` with argument arrays; no shell interpolation.
- The UI is served with a restrictive Content-Security-Policy.
- No telemetry or outbound calls beyond the plugin catalog, vfox and user-invoked package
  managers.

## Storage migration

`internal/app.migrate`:

1. Validate target: absolute, not equal to source, not nested inside it (in either
   direction), writable, enough free space.
2. Snapshot current version selections.
3. Copy the tree, preserving modes, symlinks and mtimes, with progress throttling.
4. Write `storage.sdkPath` through vfox.
5. Recreate global links for each snapshotted selection with `vfox use -g`.
6. Verify every migrated `v-<version>` directory.
7. Only then optionally delete the old root. The old root is never deleted automatically.

Failure before step 4 leaves vfox configuration untouched. Failure after step 4 preserves
both trees and reports the verification error.

The entire migration holds an exclusive operation lock (`App.opMu`). Installs, uninstalls,
version switches, plugin additions and preset applications take a shared lock, so they queue
until the copy and relink finish. This prevents a half-written SDK tree from being copied or
used as the active version.

## Virtualenv lifecycle

- Base interpreter: vfox-managed by default, explicit external path in compatibility mode.
- Creation: `uv venv --python <explicit> <path>` or `<python> -m venv <path>`.
- `UV_PYTHON_DOWNLOADS=never` prevents uv from introducing a second Python source of truth.
- Health: directory exists, `pyvenv.cfg` exists, base interpreter exists, version still
  matches, environment interpreter exists.
- Rebuild is delete-and-create; virtualenvs are not relocatable.

## Packages

Node: `npm ls -g --json --depth=0` executed with the selected version's directory prefixed
to `PATH`, so npm reports that version's global prefix.

Python: `<interpreter> -m pip list --format=json`.

Export formats: `global-packages.json` for Node, `requirements.txt` for Python.

## Cross-platform notes

| Concern | Handling |
| --- | --- |
| PATH separator / case | `os.PathListSeparator`, case-insensitive env merge |
| Shell | `pwsh` → `powershell` → `COMSPEC` on Windows; `$SHELL` on Unix |
| PTY | ConPTY on Windows 10 1809+; Unix PTY otherwise |
| Symlinks | project scope requires Developer Mode/elevation on Windows; errors are surfaced |
| Disk free | `GetDiskFreeSpaceExW` / `statfs` behind `internal/platform` |
| Long paths | copied with native APIs; path length limits are still an OS concern |

## Build artifacts and repository hygiene

- Local only, never committed: `dist/`, `.tools/`, `web/node_modules/`, `web/dist/`,
  `go.work`, binaries, coverage output, logs, `.env*` files, editor and OS state.
- `internal/webui/dist/` is committed intentionally. It contains the hashed Vite output
  (`index.html` plus vendor chunks for React and xterm) so `go install` produces a complete
  UI without Node.js. Always rebuild and commit the whole directory after changing `web/`.
- `web/package-lock.json` is committed to keep `npm ci` reproducible in CI and release builds.

## Testing strategy

- Unit tests for version comparison, TOML parsing, preset validation, jobs, tree copy,
  venv health, export formats.
- `go vet` and `gofmt` in CI on Linux, Windows and macOS.
- The web UI is built in CI before `go build`, which also validates `go:embed`.
- Manual E2E checklist lives in issues; install/network paths are intentionally not part of
  unit tests.

## Limitations

- vfox human-readable output is not a stable API; the adapter avoids it where possible.
- Only Node.js and Python are supported for global package introspection today.
- No multi-user server mode; use `VFOX_HOME` sharing at the vfox layer instead.
- Preset application is sequential. If a later runtime fails, earlier runtimes stay
  switched; fix the failure and apply again. vfox has no cross-SDK transaction.
- On Windows, `vfox use -g` updates the user environment in the registry. This is vfox
  behavior and affects terminals opened outside enver.
