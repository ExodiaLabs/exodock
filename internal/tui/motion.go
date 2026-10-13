package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/exodialabs/exodock/internal/core"
)

// A single finite animation chain accompanies startup and actual pending work.
// An idle interface schedules no animation timers or render-only heartbeat.
func (m *Model) scheduleMotion() tea.Cmd {
	if m.opts.ReducedMotion || m.opts.NoColor || m.quitting || m.ctx.Err() != nil || m.motionActive {
		return nil
	}
	if m.startupFrames == 0 && !m.motionBusy() {
		return nil
	}
	m.motionActive = true
	m.motionGeneration++
	gen := m.motionGeneration
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return motionMsg{generation: gen} })
}

func (m *Model) motionBusy() bool {
	return m.refreshing || m.actionPending || m.diskPending || m.detailPending || m.statsPending
}

func needsAttention(r core.Resource) bool {
	return r.Health == "unhealthy" || r.State == "dead" || r.State == "restarting"
}

func matchesStatus(r core.Resource, status string) bool {
	switch status {
	case "running":
		return r.State == "running"
	case "stopped":
		return r.State != "running" && r.State != "restarting" && r.State != "paused"
	case "attention":
		return needsAttention(r)
	default:
		return true
	}
}
