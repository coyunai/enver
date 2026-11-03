# enver

[**English**](README.md) | [简体中文](README.zh-CN.md)

> Environment orchestration for full-stack developers, built on [vfox](https://vfox.dev).

**Current version: 0.0.1** · MIT · Linux / macOS / Windows

vfox answers **"which version of Node / Python / Java is active?"**
enver answers **"what does this project need, all together, and can I reproduce it in one click?"**

One Go binary, a local web UI, and a complete CLI. No account, no cloud, no telemetry.

```
Browser (React + xterm.js)
   │  HTTP + SSE + WebSocket (127.0.0.1, session token)
enver-core (Go)  ──  enver CLI
   │  subprocess + environment injection
vfox CLI  ──  vfox plugins  ──  installed runtimes
```

## Why enver

vfox is an excellent version manager, but it manages one SDK at a time. A real full-stack
project needs a **combination**: Node 20 + Python 3.12 + Java 17, a virtualenv, a few
environment variables, and a fixed set of ports. enver owns that combination layer and
delegates every download, extraction and plugin concern back to vfox.

- **Presets** — a named runtime combination, applied globally or per project.
- **Python isolation** — create, inspect, rebuild and activate `venv` / `uv` environments
  on top of vfox-managed interpreters.
- **Global package visibility** — see what is installed per runtime version, export a
  manifest, and understand what disappears when you switch Node versions.
- **Storage control** — change the SDK root, migrate existing runtimes with verification,
  and keep the old directory until you are confident.
- **Diagnostics** — PATH conflicts, broken links, virtualenv drift, disk space, and the
  classic "another version manager is shadowing vfox" problem.
- **A local web UI that is actually local** — only loopback, short-lived session token,
  no outbound calls except the vfox plugin catalog and the package managers you invoke.

## Features

| Area | What you get |
| --- | --- |
| Runtimes | SDK catalog, version search, install queue with live logs, uninstall, global/project switch |
| Presets | Multi-runtime presets, env vars, optional venv, apply globally or to a project |
| Python | `uv` or stdlib `venv`, drift detection, rebuild, terminal activation, external interpreter compatibility mode |
| Packages | Per-version global package list (`npm -g`, `pip`), filtering, export |
| Storage | `storage.sdkPath`, `VFOX_HOME`, migration with copy → verify → relink → optional delete |
| Diagnostics | vfox health, PATH shadowing, broken symlinks, venv health, disk space, shell integration |
| Terminal | Embedded PTY terminal with preset / venv environment injection |
| CLI | Full functionality, JSON output, usable without the UI |

## Requirements

- [vfox](https://vfox.dev) **>= 1.0.13** on `PATH` (or set `ENVER_VFOX_BIN`).
- Go **1.25+** only when building from source.
- Node **22+** only when building the web UI from source.
- Windows 10 1809+ for the embedded terminal (ConPTY); Linux and macOS are supported.

## Install

### From source

```bash
git clone https://github.com/coyunai/enver.git
cd enver
make build          # builds the UI, then embeds it into the binary
./dist/enver --version
```

On Windows without `make`:

```powershell
cd web; npm ci; npm run build; cd ..
go build -trimpath -o dist\enver.exe .\cmd\enver
```

### Go install

The web UI is embedded in the binary, so `go install` works:

```bash
go install github.com/coyunai/enver/cmd/enver@latest
```

### Release binaries

Tagged releases publish `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`,
`windows/amd64` and `windows/arm64` archives plus `checksums.txt`.

## Quick start

```bash
enver
```

This starts the local UI (default `127.0.0.1:7788`), prints a URL containing a one-time
session token, and opens your browser. After the token exchange the UI uses an HttpOnly
cookie; the token is never stored in JavaScript.

```bash
enver ui --no-open --port 7788       # do not open a browser
enver ui --no-auth                   # local development only
enver ui --no-open --token ci-token  # fixed token for automation
```

The fixed `--token` form exists for scripts and CI. Normal use should rely on the printed
one-time URL instead.

### CLI

```bash
enver sdk list
enver sdk search nodejs
enver sdk add python
enver install nodejs@22.23.4
enver use nodejs@22.23.4 -g
enver use nodejs@22.23.4 -p --dir D:\work\myapp

enver preset ls
enver preset apply fullstack --scope global
enver preset apply fullstack --scope project --dir D:\work\myapp

enver venv create --python 3.12.4 --tool uv --project D:\work\myapp
enver venv ls
enver venv rebuild <id>
enver venv rm <id>

enver pkg ls nodejs@22.23.4
enver pkg export nodejs@22.23.4

enver doctor
enver storage show
enver storage migrate D:\vfox-sdks --delete-old
enver env --preset fullstack --json
```

Every list-style command supports `--json`.

## Python isolation

- The base interpreter comes from vfox by default. enver never lets `uv` download its own
  Python: it passes an explicit interpreter and sets `UV_PYTHON_DOWNLOADS=never`.
- `uv` is used when available; otherwise enver falls back to `python -m venv`.
- Environments record their base interpreter, version, tool, project and preset. enver
  compares `pyvenv.cfg` with the record to detect drift after a Python upgrade.
- **Compatibility mode:** if your vfox Python plugin cannot install (see known issues), you
  can point enver at an existing interpreter path. This is explicit, labeled in the UI, and
  still gives you full virtualenv isolation. The default remains vfox.

## Storage

Two different knobs, often confused:

| Setting | Scope | Use when |
| --- | --- | --- |
| `vfox config storage.sdkPath <dir>` | SDK installs only; config/plugins/cache stay in `~/.vfox` | You want runtimes on another disk |
| `VFOX_HOME` environment variable | Entire vfox home, including plugins | Portable setup or multi-user sharing |

The migration wizard is transactional in spirit: preflight → copy → verify → write vfox
config → recreate global links → optional delete. Both the current layout
(`<sdkPath>/<sdk>/v-<version>`) and the legacy `1.0.2`–`1.0.7` layout
(`<sdkPath>/cache/<sdk>`) are detected. enver also understands the pre-1.0
`~/.version-fox` home.

## Security model

- Binds `127.0.0.1` only.
- A random token is generated per launch and exchanged at `/auth` for an HttpOnly,
  SameSite=Strict cookie.
- Token comparison is constant-time; unsafe requests with a foreign `Origin` are rejected.
- The terminal WebSocket validates the `Origin` header.
- Commands are executed with argument arrays, never through a shell string.
- No telemetry. The only network calls are the vfox plugin catalog, vfox itself, and the
  package managers you explicitly invoke.

## Known issues

- **Windows + vfox Python plugin.** `vfox 1.0.13` with `vfox-python 0.6.2` can fail during
  `PostInstall` with `attempt to call a non-function object` (the plugin calls
  `listing:lines()` on a `io.popen` handle). This is upstream. enver surfaces the failure
  and offers an external interpreter in the virtualenv page as a workaround.
- **Project scope on Windows** uses symlinks. Enable Developer Mode or run elevated, or use
  global scope. enver reports the underlying vfox error instead of hiding it.
- **`vfox use -g` on Windows** updates the user environment in the registry. This is vfox
  behavior; switching versions can affect newly opened terminals outside enver.

## Development

```bash
make web      # build the React UI into internal/webui/dist
make build    # build ./dist/enver with version metadata
make test     # go test ./...
make vet      # go vet ./...
```

Hot-reload workflow:

```bash
enver ui --no-auth --no-open --port 7788   # terminal 1
cd web && npm run dev                      # terminal 2, proxies /api to 7788
```

### Build artifacts and repository hygiene

- `dist/`, `.tools/`, `web/node_modules/`, `web/dist/` and `go.work` are local only.
- `internal/webui/dist/` **is committed on purpose**, so `go install` produces a working
  binary without Node.js. Run `make web` after changing anything under `web/` and include the
  rebuilt assets in the same commit.
- Binaries, coverage files, logs, local env files and editor state are ignored. Do not commit
  them.
- The UI is a Vite build with vendor chunks for React and xterm; the committed `index.html`
  references hashed assets, so always commit the whole `internal/webui/dist` directory as a
  unit.

Repository layout:

```
cmd/enver              CLI entrypoint
internal/app           use cases shared by CLI and HTTP; storage migration
internal/vfox          vfox adapter, compatibility and directory scanning
internal/httpapi       REST, SSE jobs, WebSocket terminal
internal/jobs          background jobs and bounded log streams
internal/preset        presets
internal/venv          virtualenv metadata and health
internal/packages      npm/pip introspection and export
internal/doctor        diagnostics
internal/fsutil        atomic JSON, tree copy, size helpers
internal/platform      disk-space helpers
internal/run           child-process runner
internal/catalog       plugin catalog cache with embedded fallback
internal/config        application configuration
internal/buildinfo     version metadata
internal/webui         embedded web assets
web                    React + Vite UI
docs                   architecture and operations notes
```

See [docs/architecture.md](docs/architecture.md) for the adapter contract, API surface and
design decisions.

## Roadmap

- **P1**: package diff/migration across runtime versions, snapshots and rollback, mirror
  and proxy center, port manager, `env.lock` reproduction, project import, IDE interpreter
  wiring, task runner.
- **P2**: team preset sharing, offline runtime bundles, process panel, plugin debugging UI,
  WSL management, portable mode, EOL/CVE notices, multi-machine sync.

## Contributing

Issues and pull requests are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and run
`make test` plus `make vet` before opening a PR.

## License

MIT. See [LICENSE](LICENSE).

---

## 中文

完整中文文档见 [README.zh-CN.md](README.zh-CN.md)。
