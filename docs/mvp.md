# MVP scope

This document records the intended Exodock v0.1 scope from
[the product conception](conception.md) and distinguishes code already present
from work and validation still outstanding. The current implementation is an
unreleased development build.

## Goal

Help a user understand a local Docker Engine and safely manage one container's
lifecycle from a keyboard-driven terminal interface. All resource categories
are available for inspection; higher-risk and more complex mutations are
deferred.

## Implemented in the current development build

- Overview of daemon connection, container counts, resource counts, and
  unhealthy or restarting containers.
- Container inventory, filtering, details, start, stop, restart, and confirmed
  removal of stopped containers.
- Image, volume, and network inventory and inspection.
- Compose groups inferred from Docker Compose labels.
- Selected-container logs with follow/pause, stream selection, time window,
  and text filtering controls.
- CPU and memory statistics for the selected container, with a short CPU
  history.
- On-demand Docker system information and disk usage.
- In-memory global search and a keyboard command palette/help overlay.
- Local Unix Docker targets, ASCII and no-color options, themes, read-only
  mode, and configurable refresh interval.

The UI displays the active target and connection state. Docker collection runs
off the render path, log buffers are bounded, and read-only mode blocks
mutations at the use-case boundary. Removal confirmation retains the resource
ID and target shown to the user. If the daemon is unavailable after startup,
the UI shows a disconnected state and a retry instruction.

## Validation still outstanding

The current implementation has unit and contract tests plus an opt-in Linux
integration test that creates one uniquely labeled container and anonymous
volume from a preloaded Alpine image. CI runs the integration test on every
push and pull request. Linux PTY journeys cover terminal behavior, restoration
on signals and small-terminal layouts. Docker Desktop on macOS and WSL still
need manual validation before claiming stable support. Windows is an experimental compile target;
named-pipe and terminal runtime support are not claimed.

## Deferred

Private image pulls, volume and network mutations, Compose lifecycle commands,
bulk actions, and applying Compose projects with `up -d` remain deferred. The
current release also excludes shell/exec, YAML editing, image builds, Swarm,
Kubernetes, plugins, multiple simultaneous daemons, global prune, and forced
removal.

TLS and SSH Docker transports are deferred. Windows remains experimental until
named pipes and the terminal experience have been tested. Cross-compilation by
itself is not platform validation.

## MVP exit checks

- Complete the four reference journeys by keyboard, including in a small
  terminal.
- Show a removal confirmation that freezes the selected resource ID and target.
- Enforce read-only mode in application logic, including direct use-case calls.
- Keep navigation responsive through slow daemon calls, busy streams, and
  reconnection.
- Keep buffers within defined limits and distinguish stale data from current
  data.
- Restore the terminal after normal exit, signals, and controlled errors.
- Keep fixture secrets out of diagnostics.
- Validate Linux and Docker Desktop on macOS; provide a manual WSL recipe.

Linux automation covers the keyboard, confirmation, read-only, cancellation,
buffer and terminal-restoration checks. macOS Desktop/WSL runtime validation
and full-workload performance measurements remain outstanding. See
[the validation evidence](validation.md) for executed commands and limits.

Initial performance budgets are goals to measure, not achieved results:
keyboard response p95 under 100 ms, idle memory under 64 MiB, and distributed
binary size under 25 MiB per target.
