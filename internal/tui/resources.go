package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

type tableColumn struct {
	label string
	width int
	value func(core.Resource) string
}

func (m *Model) resourceTable(w, h int) []string {
	list := m.resources()
	count := len(list)
	filter := "all resources"
	if m.statusFilter != "" {
		filter = "state: " + m.statusFilter
	}
	if m.filter != "" {
		if filter == "all resources" {
			filter = "filter: " + safety.Inline(m.filter)
		} else {
			filter += " · filter: " + safety.Inline(m.filter)
		}
	}
	metadata := fmt.Sprintf(" %d items · %s", count, filter)
	lines := []string{m.style(metadata, "muted"), ""}
	if m.filter != "" && m.statusFilter != "" && ansi.StringWidth(metadata) > w {
		lines = []string{m.style(fmt.Sprintf(" %d items · state: %s", count, m.statusFilter), "muted"), m.style(" filter: "+m.clip(safety.Inline(m.filter), w-10), "muted"), ""}
	}
	if err := m.snapshot.Errors[m.kind()]; err != "" {
		lines = append(lines, m.style(" ! "+safety.Inline(err)+" · r retry", "bad"))
	}
	cols := m.tableColumns(w - 2)
	var header strings.Builder
	for _, c := range cols {
		header.WriteString(fit(c.label, c.width))
	}
	lines = append(lines, m.paint("  "+m.style(header.String(), "muted"), "raised", w), m.separator(w))
	if count == 0 {
		reason := "No resources in this section."
		if m.filter != "" || m.statusFilter != "" {
			reason = "No matches. Esc clears the filter."
		} else if m.refreshing {
			reason = "Loading inventory…"
		} else if !m.connected {
			reason = "Waiting for Docker. r retries the connection."
		}
		return append(lines, "", m.style(" "+reason, "muted"))
	}
	capacity := max(1, h-len(lines)-2)
	start := max(0, m.selected-capacity+1)
	for i := start; i < min(count, start+capacity); i++ {
		r := list[i]
		var row strings.Builder
		for _, c := range cols {
			value := safety.Inline(c.value(r))
			text := m.clip(value, max(1, c.width-1))
			if c.label == "STATE" || c.label == "HEALTH" {
				text = m.style(text, stateRole(value))
			}
			row.WriteString(fit(text, c.width))
		}
		lines = append(lines, m.selectedRow(row.String(), w, i == m.selected))
	}
	lines = append(lines, "", m.style(fmt.Sprintf(" %d / %d · / filter · Enter details", m.selected+1, count), "muted"))
	return lines
}
func stateRole(state string) string {
	switch state {
	case "running", "healthy", "available":
		return "good"
	case "unhealthy", "dead":
		return "bad"
	case "restarting", "starting", "paused":
		return "warn"
	default:
		return "muted"
	}
}
func (m *Model) tableColumns(w int) []tableColumn {
	name := func(r core.Resource) string {
		if r.Name != "" {
			return r.Name
		}
		return shortID(r.ID)
	}
	state := func(r core.Resource) string {
		if r.State != "" {
			return r.State
		}
		return "-"
	}
	cols := []tableColumn{}
	switch m.kind() {
	case core.Containers:
		if w >= 76 {
			nameW := max(17, w-59)
			cols = []tableColumn{{"NAME", nameW, name}, {"STATE", 12, state}, {"HEALTH", 13, func(r core.Resource) string {
				if r.Health == "" {
					return "no check"
				}
				return r.Health
			}}, {"PROJECT / SERVICE", 19, func(r core.Resource) string {
				if r.Service != "" {
					return r.Project + " / " + r.Service
				}
				return r.Project
			}}, {"PORTS", 15, func(r core.Resource) string { return r.Ports }}}
		} else {
			nameW := max(12, w-26)
			cols = []tableColumn{{"NAME", nameW, name}, {"STATE", 12, state}, {"HEALTH", max(8, w-nameW-12), func(r core.Resource) string {
				if r.Health == "" {
					return "no check"
				}
				return r.Health
			}}}
		}
	case core.Images:
		cols = []tableColumn{{"IMAGE / TAG", max(12, w-13), name}, {"SIZE", 13, func(r core.Resource) string { return bytes(r.Size) }}}
	case core.Compose:
		if w >= 65 {
			cols = []tableColumn{{"PROJECT", max(16, w-38), name}, {"STATE", 13, state}, {"SERVICES", 25, func(r core.Resource) string { return r.Service }}}
		} else {
			cols = []tableColumn{{"PROJECT", max(12, w-14), name}, {"STATE", 14, state}}
		}
	default:
		cols = []tableColumn{{"NAME", max(12, w-16), name}, {"DRIVER", 16, func(r core.Resource) string { return r.Driver }}}
	}
	return cols
}
