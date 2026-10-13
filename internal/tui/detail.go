package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

func shortID(s string) string {
	s = safety.Inline(s)
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func (m *Model) detailTitle() string {
	if m.section == 7 {
		return "DAEMON NOTES"
	}
	if m.section == 0 {
		return "GET STARTED"
	}
	if r, ok := m.resource(); ok {
		return m.clip(safety.Inline(r.Name), 27)
	}
	return "RESOURCE DETAILS"
}
func (m *Model) detailLines(w, h int) []string {
	if m.section == 0 {
		return []string{"", m.style("  Everything in one place.", "accent"), "", "  2  Browse containers", "  /  Filter the current list", "  Ctrl+k  Search all resources", "  Enter  Open resource details", "  a  See available actions", "", "  ?  Keyboard guide", "", "  Changes always require", "  confirmation on this target.", "", "  Exodialabs brand palette.", "  Exodock by Exodialabs."}
	}
	if m.section == 7 {
		return []string{"", "  Metrics describe the Docker", "  daemon, including its VM when", "  Docker Desktop is in use.", "", "  Disk usage is measured only", "  when you request it with d.", "  Shared image layers follow", "  Docker's disk accounting.", "", "  No host free-space estimate.", "  No prune or force removal.", "", "  r refreshes the inventory."}
	}
	r, ok := m.resource()
	if !ok {
		return []string{"", "  Select a resource in the list.", "  Ctrl+k searches all inventories."}
	}
	tabs := []string{"Summary", "Logs", "Stats", "Inspect"}
	if r.Kind != core.Containers {
		tabs = []string{"Summary", "Inspect"}
	}
	var tabLabels []string
	for _, tab := range tabs {
		label := " " + tab + " "
		if tab == m.tab {
			label = m.style("["+tab+"]", "accent")
		} else {
			label = m.style(label, "muted")
		}
		tabLabels = append(tabLabels, label)
	}
	lines := []string{m.clip(strings.Join(tabLabels, " "), w), m.separator(w)}
	switch m.tab {
	case "Logs":
		state := "LIVE"
		if m.logPaused {
			state = "PAUSED"
		}
		period := "all time"
		if m.sinceMinutes > 0 {
			period = fmt.Sprintf("last %dm", m.sinceMinutes)
		}
		role := "good"
		if m.logPaused {
			role = "warn"
		}
		lines = append(lines, m.paint(m.style(fmt.Sprintf(" %s · %s · %s", state, m.logStream, period), role), "raised", w), m.style(fmt.Sprintf(" %d lines / %s · buffer search only", m.logs.count, bytes(int64(m.logs.bytes))), "muted"))
		if m.logQuery != "" {
			lines = append(lines, m.style(" / "+safety.Inline(m.logQuery), "accent"))
		}
		if m.logs.dropped > 0 {
			lines = append(lines, m.style(fmt.Sprintf(" ! %d lines evicted or truncated", m.logs.dropped), "warn"))
		}
		if m.logErr != "" {
			lines = append(lines, m.style(" ! "+safety.Inline(m.logErr), "warn"))
		}
		all := m.logs.lines(m.logQuery, m.logStream)
		capacity := max(0, h-len(lines)-1)
		end := len(all)
		if m.logPaused {
			end = max(0, end-m.logOffset)
		}
		start := max(0, end-capacity)
		if len(all) == 0 {
			lines = append(lines, m.style(" Waiting for matching log entries…", "muted"))
		} else {
			for index, line := range all[start:end] {
				content := " " + m.clip(line, w-1)
				if index%2 == 0 {
					content = m.paint(content, "raised", w)
				}
				lines = append(lines, content)
			}
		}
	case "Stats":
		lines = append(lines, m.statsLines(w, h-len(lines))...)
	case "Inspect":
		var fields []string
		if m.detailErr != "" {
			fields = append(fields, m.style(" ! "+safety.Inline(m.detailErr)+" · i retry", "bad"))
		} else if len(m.detail.Fields) == 0 {
			fields = append(fields, m.style(" Loading safe inspection fields…", "muted"))
		} else {
			for _, f := range m.detail.Fields {
				fields = append(fields, m.style(" "+safety.Inline(f.Name), "muted"))
				for _, v := range strings.Split(ansi.Wrap(safety.Clean(f.Value), max(1, w-2), ""), "\n") {
					fields = append(fields, " "+v)
				}
			}
		}
		capacity := max(1, h-len(lines)-1)
		start := min(m.detailOffset, max(0, len(fields)-capacity))
		lines = append(lines, fields[start:min(len(fields), start+capacity)]...)
		lines = append(lines, m.style(" j/k scroll · sensitive fields masked", "muted"))
	default:
		state := safety.Inline(r.State)
		if state == "" {
			state = "available"
		}
		lines = append(lines, m.paint(" "+m.style(strings.ToUpper(state), stateRole(state))+"  "+m.style(string(r.Kind), "muted"), "raised", w))
		if r.Health != "" {
			lines = append(lines, " "+m.style("Health  ", "muted")+m.style(safety.Inline(r.Health), stateRole(r.Health)))
		}
		fields := []core.Field{{Name: "ID", Value: r.ID}, {Name: "Image", Value: r.Image}, {Name: "Project", Value: r.Project}, {Name: "Service", Value: r.Service}, {Name: "Ports", Value: r.Ports}, {Name: "Driver", Value: r.Driver}}
		for _, f := range fields {
			if f.Value != "" {
				if w >= 62 {
					lines = append(lines, " "+m.style(fit(f.Name, 10), "muted")+m.clip(safety.Inline(f.Value), w-11))
				} else {
					lines = append(lines, m.style(" "+strings.ToUpper(f.Name), "muted"), " "+m.clip(safety.Inline(f.Value), w-1))
				}
			}
		}
		if r.Kind == core.Images {
			lines = append(lines, m.style(" Size  "+bytes(r.Size), "muted"))
		}
		if m.section == 6 {
			lines = append(lines, "")
			lines = append(lines, m.statsLines(w, h-len(lines))...)
		} else if len(lines) < h-2 {
			lines = append(lines, "")
			actionHint := "i safe inspection"
			if r.Kind == core.Containers {
				actionHint = "a actions · l logs · s stats · i inspect"
			}
			lines = append(lines, m.paint(" "+m.style(m.clip(actionHint, w-2), "accent"), "raised", w))
		}
	}
	return lines
}
func (m *Model) statsLines(w, h int) []string {
	if !m.stats.Available {
		if m.statsPending {
			return []string{m.style(" Selected container only", "muted"), "", m.busySymbol() + " Loading container stats…"}
		}
		reason := "Docker did not return a usable sample."
		if r, ok := m.resource(); ok && matchesStatus(r, "stopped") {
			reason = "Stats unavailable for stopped containers."
		}
		return []string{"", " Selected container only.", " " + reason, " s retry · r refresh"}
	}
	s := m.stats
	memory := "unavailable"
	if s.MemoryLimit > 0 {
		memory = fmt.Sprintf("%s / %s", bytes(s.MemoryUsage), bytes(s.MemoryLimit))
	}
	memoryFraction := 0.0
	if s.MemoryLimit > 0 {
		memoryFraction = float64(s.MemoryUsage) / float64(s.MemoryLimit)
	}
	lines := []string{m.style(" Selected container only", "muted")}
	if w >= 65 && h >= 12 {
		half := (w - 1) / 2
		cpu := []string{m.style(fmt.Sprintf("%.2f%%", s.CPUPercent), "accent"), m.meter(s.CPUPercent/100, half-2, "accent"), m.style("100% = one CPU", "muted")}
		mem := []string{memory, m.meter(memoryFraction, w-half-3, "accent"), m.style("container memory / limit", "muted")}
		lines = append(lines, joinColumns(m.card("CPU", cpu, half, 4), []string{" ", " ", " ", " "}, m.card("Memory", mem, w-half-1, 4))...)
	} else {
		lines = append(lines, m.style(fmt.Sprintf(" CPU   %.2f%%", s.CPUPercent), "accent"), " "+m.meter(s.CPUPercent/100, w-2, "accent"), " Memory   "+memory)
		if s.MemoryLimit > 0 {
			lines = append(lines, " "+m.meter(memoryFraction, w-2, "accent"))
		}
	}
	lines = append(lines, m.style(" Network", "muted"), " RX "+bytes(s.NetworkRx)+"   TX "+bytes(s.NetworkTx))
	if h-len(lines) >= 4 {
		peak := 0.0
		for _, sample := range m.history {
			if !math.IsNaN(sample) && !math.IsInf(sample, 0) {
				peak = math.Max(peak, sample)
			}
		}
		lines = append(lines, "", m.style(fmt.Sprintf(" CPU history · %d samples · peak %.2f%%", len(m.history), peak), "muted"), " "+m.chart(max(1, w-2)))
	}
	if h-len(lines) >= 1 {
		lines = append(lines, m.style(" Sample "+s.CollectedAt.Format("15:04:05")+" · 100% is one CPU", "muted"))
	}

	return lines
}
func (m *Model) chart(w int) string {
	if len(m.history) == 0 {
		return "No samples yet"
	}
	samples := m.history[max(0, len(m.history)-w):]
	peak := 1.0
	for _, v := range samples {
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			peak = math.Max(peak, v)
		}
	}
	glyphs := []rune("▁▂▃▄▅▆▇█")
	if m.opts.ASCII {
		glyphs = []rune("._-=+*#@")
	}
	var b strings.Builder
	for _, v := range samples {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			b.WriteRune('?')
			continue
		}
		n := min(7, max(0, int(v/peak*7)))
		b.WriteRune(glyphs[n])
	}
	return m.style(b.String(), "accent")
}
func (m *Model) systemLines(w, h int) []string {
	s := m.snapshot.System
	if h < 16 {
		lines := []string{m.style(" DOCKER DAEMON", "accent"), " Engine " + safety.Inline(s.Version) + " · API " + safety.Inline(s.APIVersion), " Platform " + safety.Inline(s.OS) + " / " + safety.Inline(s.Architecture), fmt.Sprintf(" Capacity %d CPUs · %s", s.CPUs, bytes(s.Memory)), m.style(" DOCKER DISK USAGE", "accent")}
		if m.diskPending {
			return append(lines, " Measuring Docker usage…")
		}
		if !m.diskReady {
			return append(lines, " d  Measure Docker disk usage")
		}
		d := m.disk
		return append(lines, " Images "+bytes(d.Images)+" · Containers "+bytes(d.Containers), " Volumes "+bytes(d.Volumes)+" · Cache "+bytes(d.BuildCache), m.style(" Measured "+d.CollectedAt.Format("15:04:05"), "muted"))
	}
	platform := safety.Inline(s.OS) + " / " + safety.Inline(s.Architecture)
	engine := []string{"Engine     " + safety.Inline(s.Version), "API        " + safety.Inline(s.APIVersion), "Platform   " + platform, fmt.Sprintf("Capacity   %d CPUs · %s memory", s.CPUs, bytes(s.Memory))}
	lines := m.card("DOCKER DAEMON", engine, w, 6)
	lines = append(lines, "")
	var disk []string
	if m.diskPending {
		disk = []string{m.busySymbol() + " Measuring Docker usage…"}
	} else if m.diskReady {
		d := m.disk
		total := max(int64(1), d.Images+d.Containers+d.Volumes+d.BuildCache)
		for _, entry := range []struct {
			label string
			size  int64
		}{{"Images", d.Images}, {"Containers", d.Containers}, {"Volumes", d.Volumes}, {"Build cache", d.BuildCache}} {
			label := fit(entry.label, 12) + fit(bytes(entry.size), 12)
			if w >= 47 {
				label += m.meter(float64(entry.size)/float64(total), w-27, "accent")
			}
			disk = append(disk, label)
		}
		disk = append(disk, m.style("Measured "+d.CollectedAt.Format(time.RFC3339), "muted"))
	} else {
		disk = []string{"Measure Docker's images, layers, volumes and cache.", "", m.style("[ d  Measure Docker disk usage ]", "accent"), "Requested on demand; no background scan."}
	}
	lines = append(lines, m.card("DOCKER DISK USAGE", disk, w, len(disk)+2)...)
	if h-len(lines) >= 2 {
		lines = append(lines, "", m.style(" Daemon capacity; Docker Desktop may use a VM.", "muted"))
	}

	return lines
}
