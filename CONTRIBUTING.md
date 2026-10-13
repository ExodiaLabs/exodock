# Contributing to Exodock

Thanks for taking the time to contribute. Exodock is an open source terminal
interface for Docker, with a focus on clear actions, safe defaults, and a
responsive keyboard-driven workflow.

## Before you start

- Check existing issues and pull requests for related work.
- For a substantial change, open an issue or discussion first so the scope can
  be agreed before implementation.
- Keep changes focused. Explain user-visible behavior and document new limits.
- Do not include credentials, private registry data, real container logs, or
  other sensitive Docker data in issues, tests, or screenshots.

## Development setup

Use Go 1.27.2, the version pinned in CI; `go.mod` declares a Go 1.26 language
version. A local Docker Engine is optional for unit and contract tests. Follow
[the development guide](docs/development.md) for CLI commands and integration
test requirements.

The current CLI options include `--demo`, `--readonly`, `--ascii`,
`--no-color`, `--theme`, `--context`, `--host`, `--config`, `--refresh`, and
`--version`. The development guide lists their defaults and constraints.

## Pull requests

Pull requests should explain the problem, the resulting behavior, and how the
change was checked. Add or update tests when behavior changes. Run `make check`
before opening a pull request when the relevant code is available. Changes to
Docker operations should describe their confirmation, read-only, timeout, and
cleanup behavior.

The CI workflow runs without secrets on pull requests, including those from
forks. Its Docker integration job uses the hosted Linux runner's local daemon
and cleans up only its own fixture IDs. Do not add workflows that expose
secrets to untrusted pull-request code or run privileged `pull_request_target`
jobs on that code.

## Project conduct and brand

Participation follows the [Code of Conduct](CODE_OF_CONDUCT.md). The Exodock
and Exodialabs names and logos identify the project; a code contribution does
not grant permission to present a fork or product as an official Exodialabs
release.

By submitting a contribution, you agree that it may be distributed under the
project's [MIT License](LICENSE).
