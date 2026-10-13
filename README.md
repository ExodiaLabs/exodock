# Exodock

**Exodock — Docker management, simplified.**

A keyboard-driven Docker administration console by **Exodialabs**. Explore
containers, images, volumes, networks and Compose projects from one terminal,
then manage a container's lifecycle with explicit confirmation.

Exodock v0.1 is a development MVP written in Go with Bubble Tea v2 and the
official Docker Engine SDK. It uses the established Exodialabs palette. It
does not require a browser, a separate server or elevated privileges.

The workspace uses the website's official plum, charcoal and lavender tokens:
layered panels, filled selections, faceted branding and an adaptive dashboard
with inventory cards, health meters and attention shortcuts. Search and the
command palette show results as you type; one Enter opens the selection.

![Exodock dashboard in an actual terminal, demo backend](docs/screenshots/overview-dark.png)

[Light theme](docs/screenshots/overview-light.png) ·
[Container workspace](docs/screenshots/containers-dark.png) ·
[Live search](docs/screenshots/search-dark.png)

## Install the command

Use Go 1.27.2 (the CI toolchain) and an interactive terminal of at least 60×18
cells. Docker is required for live mode; the sandbox works without it.

```sh
git clone https://github.com/exodialabs/exodock.git
cd exodock
make install
exodock --demo
exodock --readonly
exodock
```

`make install` builds and copies the executable to `~/.local/bin/exodock`.
Once that directory is in your `PATH`, run `exodock` from any working directory.
The installed command keeps working after you move the source tree or run
`make clean`; it requires no Go toolchain at runtime.

If your shell does not include `~/.local/bin`, add the following line to your
shell startup file, then open a new terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Use `make install PREFIX=/your/prefix` to choose another installation directory,
or `BINDIR=/your/bin` to select the binary directory directly. Run `make install`
again after updating the sources. No automatic privilege elevation occurs.
To build without installing, use `make build` and run `./dist/exodock`.

`--demo` never contacts Docker, and its actions affect only sample data.
`--readonly` blocks mutations in both the UI and application/Engine layers.
The header shows the target and access mode before you perform any action.

## What the MVP does

- Overview of the Docker daemon, inventories and unhealthy containers.
- List, filter, inspect, start, stop, restart and remove stopped containers.
- Browse and inspect images, volumes and networks.
- Discover Compose projects and their services through Engine labels.
- Follow container logs, pause scrolling, search the loaded buffer, filter
  stdout/stderr and select a recent time window.
- View selected-container CPU, memory and network counters with a short CPU
  history; request Docker disk accounting from the System section.
- Global resource search, command palette, contextual help, adaptive layouts,
  dark/light/automatic themes, ASCII and monochrome modes.

Every container action requires confirmation, with **Cancel selected by
default**. Removal preserves volumes and does not use force. Exodock refuses
confirmation when the terminal cannot display its complete scope. Docker can
still reject an operation if another client changes the resource state.

Images, volumes and networks are read-only in v0.1. Image pulls, their deletion,
network creation, Compose lifecycle commands and bulk actions follow the MVP.
There is no global prune, shell execution or automatic sudo.

## Keyboard guide

| Key | Action |
| --- | --- |
| `1`–`8`, `[` / `]` | Switch section |
| Arrows or `j` / `k` | Move in the focused section, list or detail |
| `Tab` / `Shift+Tab` | Change focus |
| `Enter` / `Esc` | Open details / return or cancel |
| `/`, `Ctrl+k`, `:` | Filter, search all resources, command palette |
| `a`, `l`, `s`, `i` | Actions, logs, statistics, safe inspection |
| `a` in Overview, `f` in Containers | Attention list, cycle status filters |
| `r`, `d` in System | Refresh, measure Docker disk usage |
| `Space`, `f`, `v`, `t` in Logs | Pause, follow, stream, time range |
| `?`, `q`, `Ctrl+c` | Help, quit, quit from any view |

Text fields capture characters without triggering shortcuts. `q` closes an
overlay; `Ctrl+c` quits from any view. Press `?` for the complete guide.

## Docker targets and configuration

```sh
exodock --context desktop-linux
exodock --host unix:///run/user/1000/docker.sock --readonly
exodock --ascii --no-color --refresh 3s
exodock --theme light
exodock --reduced-motion
exodock --help
```

`--context` and `--host` are mutually exclusive. Resolution follows explicit
flags, then `DOCKER_CONTEXT`, `DOCKER_HOST`, and the current Docker CLI context.
The CLI resolves context metadata; Engine requests use the SDK. When the CLI
is unavailable, an explicit socket works; default resolution uses the standard
local socket and displays that choice. Invalid contexts never silently fall
back to another daemon. Remote SSH/TLS transports are deferred.

Preferences are optional: `$XDG_CONFIG_HOME/exodock/config.json` (or the
platform's user-config directory). `--config FILE` selects another file.

```json
{
  "version": 1,
  "theme": "dark",
  "ascii": false,
  "no_color": false,
  "reduced_motion": false,
  "refresh_seconds": 5
}
```

Flags override preferences; `NO_COLOR` disables color. Exodock does not create
a preferences file or store Docker credentials. Inspection exposes an allowlist
of fields and masks environment variables. Container logs can contain
application secrets; Exodock displays them in the local bounded buffer,
never in a diagnostic file or telemetry upload.

`--reduced-motion` disables the brief startup and loading animations. At rest,
the UI schedules no animation ticks; data refreshes keep their configured rate.

## Development and validation

```sh
make check
EXODOCK_TEST_IMAGE=alpine:latest make integration
```

Integration tests require the chosen Alpine image to be present and create
uniquely labeled resources. They clean up only IDs they created, including
the fixture volume. They never prune or mutate your existing containers.
CI preloads Alpine 3.22 and runs integration tests on every push and PR.

The Linux PTY suite exercises keyboard input, resizing, cancellation, logs,
search, disconnected mode and restoration after SIGINT/SIGTERM:

```sh
python3 -m venv .venv
.venv/bin/pip install -r test/terminal/requirements.txt
make build
.venv/bin/python test/terminal/smoke.py dist/exodock
```

GitHub Actions checks formatting, vet, race tests, Docker integration,
vulnerabilities, terminal journeys, documentation links and compilation for
Linux/macOS amd64/arm64 and experimental Windows amd64. Linux has live Engine
and PTY tests. macOS Desktop and WSL need platform-specific runtime validation;
Windows is a compile-tested experimental target. See the
[development guide](docs/development.md).

## Project documentation

- [Implemented MVP and limits](docs/mvp.md)
- [Architecture and product conception](docs/conception.md)
- [Accepted stack decision](docs/adr/0001-stack.md)
- [Collection and cancellation decision](docs/adr/0002-collection.md)
- [Brand token provenance](docs/brand.md)
- [Visual experience and keyboard journeys](docs/ui-ux.md)
- [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)

Released under the [MIT license](LICENSE). Maintained by
[Exodialabs](https://github.com/exodialabs), [exodialabs.dev](https://exodialabs.dev).
