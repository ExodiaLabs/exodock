# Development guide

Exodock is an early v0.1 development build. The repository includes a working
CLI, Docker Engine adapter, interactive terminal UI, and a deterministic demo
backend. A published release and stable platform support are not available
yet.

## Requirements

- Go 1.27.2, the version used by CI. The `go.mod` language version is 1.26.
- Make, for the convenience targets in the root `Makefile`.
- Docker Engine only for real-daemon integration tests and interactive use.

The CI build uses Go's cross-compilation support for Linux and macOS on amd64
and arm64. Windows amd64 is a compile-only experimental target; it is not yet a
supported runtime platform.

## Run the application

Install the command for your user account from the repository root:

```sh
make install
cd /tmp
exodock --demo
```

The target copies a standalone executable into `~/.local/bin`, which must be
in `PATH`. You can override `PREFIX`, `BINDIR` and the packaging staging prefix
`DESTDIR`. Installing never changes shell files or requests elevated privileges.
The command does not depend on the source directory or the `dist` directory
after installation. Reinstall after rebuilding to update the installed copy.

From the repository root, start the live Docker UI with:

```sh
make run
```

The CLI options currently implemented are:

| Option | Purpose |
| --- | --- |
| `--demo` | Use sample data without contacting Docker |
| `--readonly` | Disable all mutations |
| `--ascii` | Use ASCII symbols and borders |
| `--no-color` | Disable color output |
| `--reduced-motion` | Disable startup and loading animations |
| `--theme {dark,light,auto}` | Choose a terminal theme; defaults to `dark` |
| `--context NAME` | Select a local Docker context |
| `--host ADDRESS` | Select a local Docker socket; exclusive with `--context` |
| `--config FILE` | Load preferences from a JSON file |
| `--refresh DURATION` | Set inventory refresh interval from 1s to 60s; defaults to 5s |
| `--version` | Print the application version |

`NO_COLOR` disables colors. Preferences default to the operating system's user
configuration directory under `exodock/config.json`; the file can set theme,
ASCII mode, no-color mode, reduced motion (`reduced_motion`), and refresh interval. Explicit CLI values override
the corresponding preferences. When the selected daemon is unavailable after
startup, the UI shows a disconnected state and allows retrying. If standard
input or output is not a terminal, Exodock exits with a message to use an
interactive terminal or `--help`.

There is no published release archive yet. Build the executable without
installing it with:

```sh
make build
```

## Tests and checks

Unit and contract tests should not need a running Docker daemon:

```sh
make test
```

Run formatting, static checks, and tests together with:

```sh
make check
```

Integration tests that contact a real local Docker Engine are opt-in locally.
The Linux CI job runs them on every pull request and push, after preparing the
Alpine 3.22 fixture image. Run the local integration test only against a daemon
where creating and removing one isolated fixture container and anonymous
volume is acceptable:

```sh
make integration
```

This sets `EXODOCK_INTEGRATION=1`. CI also sets
`EXODOCK_TEST_IMAGE=alpine:3.22` and preloads that image. Locally the test
defaults to `alpine:latest`; set `EXODOCK_TEST_IMAGE` to another image that is
already present to select it. The integration fixture uses a unique name and
labels, and cleanup is limited to resources it created. It does not run a
global prune or pull images implicitly. CI pull-request jobs do not receive
secrets.

## Contribution checks

Before opening a pull request, review the project's contribution and security
policies. CI checks Go formatting, `go vet`, tests with the race detector and
coverage, a pinned `govulncheck` release, and cross-compilation. Run
`actionlint` locally when available to validate GitHub Actions workflow syntax.
Check relative links in Markdown with `python3 scripts/check-doc-links.py`.

Compilation for a target does not establish that Docker connections, terminal
behavior, or interactive controls work on that operating system. Linux and
macOS Docker environments need runtime validation; WSL and Windows terminal
support need separate manual recipes before they are announced as supported.

## Terminal journeys

Linux CI builds the static executable and drives it through a real
pseudo-terminal. The test dependencies are version- and hash-pinned:

```sh
python3 -m venv .venv
.venv/bin/pip install -r test/terminal/requirements.txt
make build
.venv/bin/python test/terminal/smoke.py dist/exodock
```

The journeys cover demo actions and default cancellation, logs and filters,
safe inspection, statistics, live search and palette with one Enter, disk accounting, small terminals,
dark/light truecolor surfaces,
read-only controls, missing-daemon errors and terminal restoration after normal
exit/SIGINT/SIGTERM. They never mutate Docker.

Capture a rendered terminal scene as SVG with the same pinned `pyte`
environment. The capture script uses Python's standard library plus `pyte`;
it reads actual PTY cell text, foregrounds and backgrounds:

```sh
.venv/bin/python scripts/capture-terminal.py dist/exodock /tmp/exodock-overview.svg --theme dark --scene overview --cols 120 --rows 36
```

Available scenes are `overview`, `containers`, `logs` and `search`; themes are
`dark` and `light`. The script uses demo data and reduced motion. It produces
SVG; converting a capture to PNG requires a separate SVG renderer.

Performance measurements belong in [validation evidence](validation.md).
The full-history render benchmark runs with:

```sh
go test ./internal/tui -run '^$' -bench 'Benchmark(LogViewFullHistory|ColorWorkspace)' -benchmem
```

The color workspace benchmark renders dark/light overview, container inventory
and log screens with 1,000 containers and 10,000 retained log lines. It reports
allocations alongside render time; the no-color log benchmark preserves the
earlier baseline. UI behavior and palette sources are documented in
[the interface guide](ui-ux.md) and [brand tokens](brand.md).
