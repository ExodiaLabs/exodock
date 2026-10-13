package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

// Brand primitives are sourced from ExodiaLabs/exodialabs-website,
// assets/styles/settings/tokens.css at c65f6dcd91bf7b94634f9ca289b4e8af1e37335d.
// Status colors use terminal ANSI colors and always retain textual labels.
func (m *Model) isLight() bool {
	return m.opts.Theme == "light" || ((m.opts.Theme == "auto" || m.opts.Theme == "") && m.lightTheme)
}
func (m *Model) style(s, role string) string {
	if m.opts.NoColor {
		return s
	}
	light := m.isLight()
	color := "#f5f2f9"
	if light {
		color = "#15111e"
	}
	switch role {
	case "accent":
		color = "#bba9dc"
		if light {
			color = "#7960a6"
		}
	case "muted":
		color = "#8c8697"
		if light {
			color = "#38323f"
		}
	case "good":
		color = "2"
	case "warn":
		color = "3"
	case "bad":
		color = "1"
	}
	st := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
	if role == "accent" {
		st = st.Bold(true)
	}
	return st.Render(s)
}
func fit(s string, w int) string {
	if w < 1 {
		return ""
	}
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
func (m *Model) clip(s string, w int) string {
	tail := "…"
	if m.opts.ASCII {
		tail = "~"
	}
	return ansi.Truncate(s, max(0, w), tail)
}
func (m *Model) separator(w int) string {
	c := "─"
	if m.opts.ASCII {
		c = "-"
	}
	return m.style(strings.Repeat(c, max(0, w)), "muted")
}
func (m *Model) box(title string, lines []string, w, h int, active bool) string {
	if w < 4 || h < 3 {
		return ""
	}
	tl, tr, bl, br, hz, vt := "╭", "╮", "╰", "╯", "─", "│"
	if m.opts.ASCII {
		tl, tr, bl, br, hz, vt = "+", "+", "+", "+", "-", "|"
	}
	role := "muted"
	if active {
		role = "accent"
	}
	heading := " " + title + " "
	heading = m.clip(heading, w-4)
	top := tl + heading + strings.Repeat(hz, max(0, w-2-ansi.StringWidth(heading))) + tr
	out := []string{m.style(top, role)}
	for i := 0; i < h-2; i++ {
		s := ""
		if i < len(lines) {
			s = lines[i]
		}
		out = append(out, m.style(vt, role)+fit(s, w-2)+m.style(vt, role))
	}
	out = append(out, m.style(bl+strings.Repeat(hz, w-2)+br, role))
	return strings.Join(out, "\n")
}
func (m *Model) View() tea.View {
	w, h := max(1, m.width), max(1, m.height)
	var output string
	if w < 60 || h < 18 {
		output = fmt.Sprintf("EXODOCK\nTerminal too small: %d x %d\nResize to at least 60 x 18.\nq / Ctrl+c quit", w, h)
	} else {
		connection := "CONNECTING"
		role := "warn"
		if m.connected {
			connection = "CONNECTED"
			role = "good"
		} else if !m.refreshing {
			connection = "DISCONNECTED"
			role = "bad"
		}
		access := "READ / WRITE"
		if m.opts.ReadOnly {
			access = "READ ONLY"
		}
		header := " " + m.style("EXODOCK", "accent") + "  " + m.style("Docker management, simplified.", "muted")
		if w < 85 {
			header = " " + m.style("EXODOCK", "accent") + "  " + m.style("Docker, simplified.", "muted")
		}
		right := m.style(connection, ""+role) + " · " + m.style(access, "warn") + " "
		header = fit(header, max(0, w-ansi.StringWidth(right))) + right
		target := " Target  " + safety.Inline(m.opts.Target)
		if m.actionPending {
			target += "  · action in progress"
		} else if m.refreshing {
			target += "  · refreshing"
		}
		bodyH := h - 7
		var body string
		if m.overlay != "" {
			body = m.modal(w, bodyH)
		} else if w >= 120 && h >= 30 && m.detailOpen && m.tab != "Summary" {
			navW := 20
			detailW := w - navW - 1
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.box("WORKSPACE", m.navLines(), navW, bodyH, m.focus == 0), " ", m.box(m.detailTitle(), m.detailLines(detailW-2, bodyH-2), detailW, bodyH, m.focus != 0))
		} else if w >= 120 && h >= 30 {
			navW := 20
			detailW := min(40, w/3)
			mainW := w - navW - detailW - 2
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.box("WORKSPACE", m.navLines(), navW, bodyH, m.focus == 0), " ", m.box(strings.ToUpper(sections[m.section]), m.mainLines(mainW-2, bodyH-2), mainW, bodyH, m.focus == 1), " ", m.box(m.detailTitle(), m.detailLines(detailW-2, bodyH-2), detailW, bodyH, m.focus == 2))
		} else if w >= 80 && h >= 24 {
			navW := 18
			mainW := w - navW - 1
			title := strings.ToUpper(sections[m.section])
			lines := m.mainLines(mainW-2, bodyH-2)
			if m.detailOpen {
				title = m.detailTitle()
				lines = m.detailLines(mainW-2, bodyH-2)
			}
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.box("WORKSPACE", m.navLines(), navW, bodyH, m.focus == 0), " ", m.box(title, lines, mainW, bodyH, m.focus != 0))
		} else {
			nav := fmt.Sprintf(" [%d/8] %s   [ / ] sections · Tab focus", m.section+1, sections[m.section])
			title := strings.ToUpper(sections[m.section])
			lines := m.mainLines(w-2, bodyH-3)
			if m.detailOpen {
				title = m.detailTitle()
				lines = m.detailLines(w-2, bodyH-3)
			}
			body = fit(m.style(nav, "accent"), w) + "\n" + m.box(title, lines, w, bodyH-1, true)
		}
		footer := " / filter  Ctrl+k search  : palette  ? help  q quit"
		if m.tab == "Logs" && m.detailOpen {
			footer = " / search  Space pause  f follow  v stream  t time  Esc back"
		} else if m.section >= 1 && m.section <= 6 {
			footer = " j/k move  Enter details  a actions  l logs  s stats  ? help"
		} else if m.section == 7 {
			footer = " d disk usage  r refresh  [/] sections  ? help  q quit"
		}
		output = strings.Join([]string{fit(header, w), fit(m.style(target, "muted"), w), m.separator(w), body, m.separator(w), fit(m.style(" "+safety.Inline(m.notice), "muted"), w), fit(m.style(footer, "accent"), w)}, "\n")
	}
	// Clamp every rendered line; terminals smaller than the minimum remain usable.
	if m.opts.ASCII {
		output = strings.NewReplacer("·", "|", "…", "...", "→", "->", "–", "-", "—", "-").Replace(output)
	}
	lines := strings.Split(output, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], w, "")
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	if !m.opts.NoColor {
		bg, fg := "#15111e", "#f5f2f9"
		if m.isLight() {
			bg, fg = "#f5f2f9", "#15111e"
		}
		v.BackgroundColor = lipgloss.Color(bg)
		v.ForegroundColor = lipgloss.Color(fg)
	}
	v.WindowTitle = "Exodock"
	return v
}
func (m *Model) navLines() []string {
	lines := []string{"", m.style("  LOCAL DOCKER", "muted"), ""}
	for i, s := range sections {
		prefix := "  "
		role := ""
		if i == m.section {
			prefix = "> "
			role = "accent"
		}
		lines = append(lines, m.style(fmt.Sprintf(" %s%d  %s", prefix, i+1, s), role))
	}
	lines = append(lines, "", m.style("  [ / ] sections", "muted"), m.style("  Tab focus", "muted"), "", m.style("  by Exodialabs", "muted"))
	if m.opts.ReadOnly {
		lines = append(lines, m.style("  READ ONLY", "warn"))
	}
	return lines
}
func (m *Model) mainLines(w, h int) []string {
	switch m.section {
	case 0:
		return m.overviewLines(w, h)
	case 7:
		return m.systemLines(w, h)
	}
	if m.section == 6 {
		lines := []string{m.style(" Selected container only", "muted"), ""}
		return append(lines, m.resourceTable(w, h-2)...)
	}
	return m.resourceTable(w, h)
}
func (m *Model) overviewLines(w, h int) []string {
	s := m.snapshot.System
	running, stopped, total := s.Running, s.Stopped, s.Containers
	if total == 0 {
		for _, r := range m.snapshot.Resources[core.Containers] {
			total++
			if r.State == "running" {
				running++
			} else {
				stopped++
			}
		}
	}
	if h < 12 {
		lines := []string{m.style(" YOUR DOCKER WORKSPACE", "accent"), fmt.Sprintf(" %d containers · %d running · %d stopped", total, running, stopped), fmt.Sprintf(" Images %d · Volumes %d", len(m.snapshot.Resources[core.Images]), len(m.snapshot.Resources[core.Volumes])), fmt.Sprintf(" Networks %d · Compose %d", len(m.snapshot.Resources[core.Networks]), len(m.snapshot.Resources[core.Compose]))}
		anomalies := 0
		for _, r := range m.snapshot.Resources[core.Containers] {
			if r.Health == "unhealthy" || r.State == "dead" || r.State == "restarting" {
				anomalies++
				if len(lines) < h-2 {
					lines = append(lines, m.style(" ! "+safety.Inline(r.Name)+" · "+safety.Inline(r.Health+" "+r.State), "warn"))
				}
			}
		}
		if anomalies == 0 {
			lines = append(lines, m.style(" No unhealthy containers detected.", "muted"))
		}
		return append(lines, m.style(" 2 containers · Ctrl+k find · 8 system", "accent"))
	}
	lines := []string{"", m.style("  YOUR DOCKER WORKSPACE", "accent"), "", fmt.Sprintf("  %d containers   %d running   %d stopped", total, running, stopped), ""}
	if w >= 43 {
		segments := []struct {
			name string
			kind core.Kind
		}{{"IMAGES", core.Images}, {"VOLUMES", core.Volumes}, {"NETWORKS", core.Networks}, {"COMPOSE", core.Compose}}
		for _, x := range segments {
			lines = append(lines, fmt.Sprintf("  %-12s %d", x.name, len(m.snapshot.Resources[x.kind])))
		}
	} else {
		lines = append(lines, fmt.Sprintf("  Images %d · Volumes %d", len(m.snapshot.Resources[core.Images]), len(m.snapshot.Resources[core.Volumes])), fmt.Sprintf("  Networks %d · Compose %d", len(m.snapshot.Resources[core.Networks]), len(m.snapshot.Resources[core.Compose])))
	}
	lines = append(lines, "", m.style("  ATTENTION", "warn"))
	anomalies := 0
	for _, r := range m.snapshot.Resources[core.Containers] {
		if r.Health == "unhealthy" || r.State == "dead" || r.State == "restarting" {
			anomalies++
			if len(lines) < h-3 {
				lines = append(lines, m.style("  ! "+safety.Inline(r.Name)+" · "+safety.Inline(r.State)+" / "+safety.Inline(r.Health), "warn"))
			}
		}
	}
	if anomalies == 0 {
		lines = append(lines, "  No unhealthy or restarting containers detected.")
	}
	if !m.connected {
		lines = append(lines, m.style("  Docker unavailable · r retry", "bad"))
	}
	if len(lines) < h-3 {
		lines = append(lines, "", m.style("  2 containers · Ctrl+k find a resource", "muted"), m.style("  System (8) → d measures Docker disk usage", "muted"))
	}
	return lines
}
func (m *Model) resourceTable(w, h int) []string {
	list := m.resources()
	count := len(list)
	filter := "all resources"
	if m.filter != "" {
		filter = "filter: " + safety.Inline(m.filter)
	}
	lines := []string{m.style(fmt.Sprintf(" %d items · %s", count, filter), "muted"), ""}
	if err := m.snapshot.Errors[m.kind()]; err != "" {
		lines = append(lines, m.style(" ! "+safety.Inline(err)+" · r retry", "bad"))
	}
	nameW := max(12, w*45/100)
	extraW := max(8, w-nameW-20)
	header := fit("   NAME", nameW) + fit("STATE", 13)
	if m.kind() == core.Containers {
		header += fit("HEALTH", extraW)
	} else if m.kind() == core.Images {
		header += fit("SIZE", extraW)
	} else {
		header += fit("DRIVER", extraW)
	}
	lines = append(lines, m.style(header, "muted"), m.separator(w))
	if count == 0 {
		reason := "No resources in this section."
		if m.filter != "" {
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
		prefix := "   "
		role := ""
		if i == m.selected {
			prefix = " > "
			role = "accent"
		}
		name := safety.Inline(r.Name)
		if name == "" {
			name = shortID(r.ID)
		}
		state := safety.Inline(r.State)
		if state == "" {
			state = "—"
			if m.opts.ASCII {
				state = "-"
			}
		}
		extra := safety.Inline(r.Driver)
		switch m.kind() {
		case core.Containers:
			extra = safety.Inline(r.Health)
			if extra == "" {
				extra = "not configured"
			}
		case core.Images:
			extra = bytes(r.Size)
		case core.Compose:
			extra = safety.Inline(r.Project)
		}
		line := fit(prefix+m.clip(name, nameW-4), nameW) + fit(m.clip(state, 12), 13) + m.clip(extra, extraW)
		lines = append(lines, m.style(line, role))
	}
	lines = append(lines, "", m.style(fmt.Sprintf(" %d / %d · / filter · Enter details", m.selected+1, count), "muted"))
	return lines
}
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
	tabs := " Summary  l Logs  s Stats  i Inspect"
	if r.Kind != core.Containers {
		tabs = " Summary  i Inspect"
	}
	lines := []string{m.style(m.clip(tabs, w), "muted"), m.style(" "+strings.ToUpper(m.tab), "accent"), m.separator(w)}
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
		lines = append(lines, m.style(fmt.Sprintf(" %s · %s · %s", state, m.logStream, period), "good"), m.style(fmt.Sprintf(" %d lines / %s · buffer search only", m.logs.count, bytes(int64(m.logs.bytes))), "muted"))
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
			for _, line := range all[start:end] {
				lines = append(lines, " "+m.clip(line, w-1))
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
		lines = append(lines, "", m.style(" "+state, "good"))
		if r.Health != "" {
			role := "good"
			if r.Health == "unhealthy" {
				role = "warn"
			}
			lines = append(lines, m.style(" Health: "+safety.Inline(r.Health), role))
		}
		fields := []core.Field{{Name: "ID", Value: r.ID}, {Name: "Image", Value: r.Image}, {Name: "Project", Value: r.Project}, {Name: "Service", Value: r.Service}, {Name: "Ports", Value: r.Ports}, {Name: "Driver", Value: r.Driver}}
		for _, f := range fields {
			if f.Value != "" {
				lines = append(lines, "", m.style(" "+f.Name, "muted"), " "+m.clip(safety.Inline(f.Value), w-1))
			}
		}
		if r.Kind == core.Images {
			lines = append(lines, "", m.style(" Size  "+bytes(r.Size), "muted"))
		}
		if m.section == 6 {
			lines = append(lines, "")
			lines = append(lines, m.statsLines(w, h-len(lines))...)
		} else if len(lines) < h-2 {
			lines = append(lines, "", m.style(" a actions · i safe inspection", "accent"))
		}
	}
	return lines
}
func (m *Model) statsLines(w, h int) []string {
	if !m.stats.Available {
		return []string{"", " Selected container only.", " Stats unavailable for stopped containers.", " s retry · r refresh"}
	}
	s := m.stats
	memory := "unavailable"
	if s.MemoryLimit > 0 {
		memory = fmt.Sprintf("%s / %s", bytes(s.MemoryUsage), bytes(s.MemoryLimit))
	}
	lines := []string{m.style(" Selected container only", "muted"), m.style(fmt.Sprintf(" CPU   %.2f%%", s.CPUPercent), "accent"), " Memory   " + memory, " Network received   " + bytes(s.NetworkRx), " Network sent       " + bytes(s.NetworkTx), "", m.style(" CPU history · up to 60 samples", "muted"), " " + m.chart(max(1, w-2)), "", m.style(" Selected container only · "+s.CollectedAt.Format("15:04:05"), "muted"), m.style(" 100% represents one CPU; multicore may exceed it.", "muted")}
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
	lines := []string{"", m.style("  DOCKER DAEMON", "accent"), "", "  Engine     " + safety.Inline(s.Version), "  API        " + safety.Inline(s.APIVersion), "  Platform   " + safety.Inline(s.OS) + " / " + safety.Inline(s.Architecture), fmt.Sprintf("  CPUs       %d", s.CPUs), "  Memory     " + bytes(s.Memory), "", m.style("  DOCKER DISK USAGE", "accent")}
	if m.diskPending {
		lines = append(lines, "  Measuring usage…")
	} else if m.diskReady {
		d := m.disk
		lines = append(lines, "  Images       "+bytes(d.Images), "  Containers   "+bytes(d.Containers), "  Volumes      "+bytes(d.Volumes), "  Build cache  "+bytes(d.BuildCache), m.style("  Measured "+d.CollectedAt.Format(time.RFC3339), "muted"))
	} else {
		lines = append(lines, "  d  Measure Docker disk usage")
	}
	if len(lines) < h-2 {
		lines = append(lines, "", m.style("  Daemon capacity; Docker Desktop may use a VM.", "muted"))
	}
	return lines
}
func (m *Model) modal(w, h int) string {
	title := ""
	var lines []string
	switch m.overlay {
	case "input":
		title = map[string]string{"filter": "FILTER THIS LIST", "logs": "SEARCH LOADED LOGS", "global": "FIND A RESOURCE", "palette": "COMMAND PALETTE"}[m.inputMode]
		lines = []string{"", m.style(" > "+safety.Inline(m.input)+"_", "accent"), "", " Enter apply · Esc cancel", " Text fields capture all characters."}
		if m.inputMode == "global" {
			lines = append(lines, "", " Names, IDs, images and Compose projects.", " Searches the current target's loaded inventory.")
		}
		if m.inputMode == "logs" {
			lines = append(lines, "", " Searches only the bounded in-memory buffer.")
		}
	case "help":
		title = "KEYBOARD GUIDE · EXODIALABS"
		all := []string{" j/k or Up/Down    Move selection or section", " Tab / Shift+Tab   Cycle navigation, list, details", " [ / ]  or 1–8     Switch workspace section", " Enter / Esc       Open details / return", " /                 Filter list or search loaded logs", " Ctrl+k            Global resource search", " :                 Navigation and action palette", " a                 Contextual container actions", " l / s / i         Logs / stats / safe inspection", " r                 Refresh inventory, retry connection", " d (System)        Measure Docker disk usage", " Space (Logs)      Pause/resume scrolling", " f (Logs)          Return to live following", " v / t (Logs)      Cycle stream / time range", " j/k (Logs/Inspect) Scroll loaded history or fields", " q / Ctrl+c        Quit (q closes open overlays)", " Esc               Close overlay or cancel confirmation", "", " Read-only disables all mutations.", " Confirmations default to Cancel and freeze the ID.", " Logs: 10,000 entries / 8 MiB; lines capped at 64 KiB.", " CPU: 100% is one CPU; selected-container scope.", " Palette uses verified Exodialabs brand tokens."}
		capacity := max(1, h-5)
		start := min(max(0, m.menuIndex), max(0, len(all)-capacity))
		lines = append(lines, all[start:min(len(all), start+capacity)]...)
		lines = append(lines, "", m.style(" Up/Down scroll guide · Esc close", "accent"))
	case "actions":
		title = "CONTAINER ACTIONS"
		if r, ok := m.resource(); ok {
			lines = append(lines, " "+safety.Inline(r.Name), " "+shortID(r.ID), "")
		}
		for i, a := range actions {
			prefix := "   "
			role := ""
			if i == m.menuIndex {
				prefix = " > "
				role = "accent"
			}
			label := strings.ToUpper(string(a[:1])) + string(a[1:])
			if reason := m.actionReason(a); reason != "" {
				label += " · " + reason
				role = "muted"
			}
			lines = append(lines, m.style(prefix+label, role))
		}
		lines = append(lines, "", " Enter choose · Esc cancel")
	case "confirm":
		title = "CONFIRM " + strings.ToUpper(string(m.frozen.Action))
		lines = m.confirmationLines(w)
		if len(lines) > h-2 {
			lines = append(lines[:max(0, h-3)], m.style(" Resize to review full scope · Esc cancels", "warn"))
		}
	case "global":
		title = "SEARCH RESULTS · " + safety.Inline(m.input)
		results := m.globalResults()
		cap := max(1, h-5)
		start := max(0, m.menuIndex-cap+1)
		for i := start; i < min(len(results), start+cap); i++ {
			r := results[i]
			prefix := "   "
			role := ""
			if i == m.menuIndex {
				prefix = " > "
				role = "accent"
			}
			lines = append(lines, m.style(prefix+safety.Inline(r.Name)+" · "+string(r.Kind)+" · "+shortID(r.ID), role))
		}
		if len(results) == 0 {
			lines = append(lines, " No matching resources.")
		}
		lines = append(lines, "", " Enter open · / edit query · Esc close")
	case "palette":
		title = "COMMAND PALETTE"
		entries := m.paletteResults()
		cap := max(1, h-5)
		start := max(0, m.menuIndex-cap+1)
		for i := start; i < min(len(entries), start+cap); i++ {
			prefix := "   "
			role := ""
			if i == m.menuIndex {
				prefix = " > "
				role = "accent"
			}
			lines = append(lines, m.style(prefix+entries[i].name, role))
		}
		if len(entries) == 0 {
			lines = append(lines, " No matching commands.")
		}
		lines = append(lines, "", " Enter choose · / edit query · Esc close")
	}
	return m.box(title, lines, w, h, true)
}

func (m *Model) confirmationLines(w int) []string {
	lines := []string{" Resource  " + safety.Inline(m.frozenName)}
	for _, line := range strings.Split(ansi.Wrap("Target    "+safety.Inline(m.frozenTarget), max(1, w-4), ""), "\n") {
		lines = append(lines, " "+line)
	}
	id := safety.Inline(m.frozen.ID)
	for id != "" {
		size := min(len(id), max(1, w-7))
		lines = append(lines, " ID "+id[:size])
		id = id[size:]
	}
	consequences := map[core.Action]string{core.Start: "Start this existing container.", core.Stop: "Interrupt the service; allow Docker's grace period.", core.Restart: "Interrupt and restart this service.", core.Remove: "Delete the writable layer. Volumes are kept."}
	for _, line := range strings.Split(ansi.Wrap(consequences[m.frozen.Action], w-4, ""), "\n") {
		lines = append(lines, m.style(" "+line, "warn"))
	}
	cancel, accept := "[ CANCEL ]", "  Confirm  "
	if m.confirm {
		cancel, accept = "  Cancel  ", "[ CONFIRM ]"
	}
	lines = append(lines, m.style(" "+cancel+"    "+accept, "accent"), " Tab select · Enter apply · Esc cancel")
	return lines
}
