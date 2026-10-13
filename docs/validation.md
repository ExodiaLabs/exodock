# MVP validation evidence

Date: 2026-10-13. Local environment: Linux amd64, Go 1.27.2, Docker Engine
29.9.0 (API 1.56), AMD Ryzen AI 5 PRO 340.

## Automated checks

The development gate runs formatting, `go vet`, race tests with coverage,
local Markdown links, pinned `govulncheck` and `actionlint`. The live Engine
test creates its own uniquely labeled Alpine container and anonymous volume;
it exercises lifecycle, logs, statistics, safe inspection, Compose discovery
and volume preservation. Cleanup targets only IDs returned by fixture creation.

The Python PTY suite drives the built executable. It checks default Cancel,
confirmed stop/removal in the sandbox, logs and filters, search, inspection,
statistics, disk usage, resize, read-only mode and missing-daemon behavior.
It verifies canonical terminal input modes and alternate-screen restoration
on normal exit, SIGINT and SIGTERM. These tests never mutate live Docker.

An independent reviewer reproduced and rechecked five regressions: inspection
failure followed by success, hidden confirmation after resize, actual daemon
identity, opaque Compose project IDs and repeated pause/resume. Each has a
repository regression test. Review found no remaining blocking defect in its
examined paths; this is not a claim of exhaustive formal verification.

## Initial performance measurement

`BenchmarkLogViewFullHistory`: 120×32 cells, ASCII/monochrome, 10,000 retained
log entries of 200 bytes each, focused logs view. The initial local run measured
**857,541 ns/op** (about **0.86 ms/render**), **212,043 B/op**, **280 allocs/op**.
This isolates rendering; it is not an end-to-end keyboard latency benchmark,
a Docker network benchmark or the original 100-container reference workload.

The optimized development binaries use `-trimpath -ldflags '-s -w'`; Linux
distribution builds disable CGo. Raw target binaries are about 9 MiB, below
the provisional 25 MiB size budget. Cross-compilation checks Linux/macOS
amd64/arm64 and experimental Windows amd64.

An additional read-only PTY session against the local Docker daemon successfully
opened the live inventories and measured disk usage. Its observed RSS was
18,284 kB (about 18 MiB). This single inventory session is not a sustained-load
or 100-container memory benchmark.

## Terminal redesign validation

The October 13 redesign adds a full dark/light/ASCII/monochrome layout matrix,
live one-Enter search and palette journeys, combined status/text filters,
unknown health on collection failure, frozen complete confirmations, and bounded
motion scheduling. Actual PTY checks verify entire header, card and selected-row
backgrounds, including padding; a nested SGR reset regression was reproduced
before correction. An independent review reproduced four display defects and
verified their corrections, plus RGB payload/reset handling.

After redesign, the same monochrome log benchmark measured **1.41 ms/render**,
**264,220 B/op**, **913 allocs/op**. The colored workspace benchmark at 120×32,
with 1,000 containers and 10,000 retained log entries, measured:

| Screen | Dark | Light |
| --- | --- | --- |
| Overview | 2.66 ms | 2.76 ms |
| Containers | 2.75 ms | 3.21 ms |
| Logs | 2.84 ms | 2.80 ms |

These local `-benchtime=150ms` samples measure render work, not interaction p95.
The richer composition increases the baseline rendering cost. Animation ticks
run only for six startup frames or pending work, at most ten per second; no
animation timer continues at rest. Docker polling retains its configured rate.
The [screenshots](ui-ux.md) were captured from actual demo PTY cells, rather than
designed mockups; the chosen monospace font illustrates a terminal setting.

## Platform limits

Docker Desktop macOS, WSL and native Windows still require runtime recipes.
Full-workload p95 keyboard latency and memory budgets remain measurements to
perform before a stable cross-platform release. Inventory refresh uses polling
rather than Engine event subscriptions; selected metrics are not host-wide totals.
No published stable release or automatic package-manager installation is implied.

GitHub Actions is the authoritative remote check. A successful local run does
not establish remote CI status; the delivery links the actual run for its commit.
