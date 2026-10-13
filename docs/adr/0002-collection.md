# 0002 : Poll inventories and cancel selected-resource streams

- **Status:** accepted
- **Date:** 2026-10-13
- **Scope:** v0.1 MVP

## Context

The original conception proposed Engine events plus periodic reconciliation.
The MVP needs responsive keyboard input and current local inventories; event
subscription adds another reconnecting stream before it brings a measured
benefit. Logs already require explicit ownership and cancellation.

## Decision

Poll inventories asynchronously every five seconds by default, configurable
from one to sixty seconds. Serialize refreshes and invalidate stale timer
generations. Fetch selected details with bounded deadlines, reject responses
from previous selections and cancel log/stat requests when selection changes.
Collect statistics only for the selected resource and state that scope in UI.

Logs use a single consumer and no background producer queue. Bound decoder
lines at 16 KiB, frames at 16 MiB, UI entries at 64 KiB and displayed history
at 10,000 entries/8 MiB. Closing a stream cancels a blocked read. Inspection and
inventory calls have deadlines. The root context cancels on exit/signals.

## Consequences

An external resource change can take up to the configured interval to appear.
Users can refresh on demand. Exodock retains the last inventory after a
connection failure and marks it stale/disconnected. Write timeouts do not
trigger an automatic replay.

Add Engine events later if measurements or feedback justify faster updates.
Keep polling as reconciliation. No database, persistent metric history or
monitoring daemon accompanies this MVP.
