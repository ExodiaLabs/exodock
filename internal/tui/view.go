package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
	"strings"
)

func (m *Model) View() tea.View {
	w, h := max(1, m.width), max(1, m.height)
	var lines []string
	if w < 60 || h < 18 {
		lines = []string{m.style(" EXODOCK", "accent"), fmt.Sprintf(" Terminal too small: %d x %d", w, h), " Resize to at least 60 x 18.", " q / Ctrl+c quit"}
	} else {
		lines = m.headerLines(w)
		bodyH := h - 6
		body := m.workspace(w, bodyH)
		if m.overlay != "" {
			body = m.overlayOn(body, w, bodyH)
		}
		lines = append(lines, strings.Split(body, "\n")...)
		lines = append(lines, m.separator(w), m.noticeLine(w), m.footerLine(w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i := range lines {
		if m.opts.ASCII {
			lines[i] = strings.NewReplacer("·", "|", "…", "...", "→", "->", "–", "-", "—", "-").Replace(lines[i])
		}
		lines[i] = m.paint(lines[i], "canvas", w)
	}
	v := tea.NewView(strings.Join(lines, "\n"))
	v.AltScreen = true
	v.WindowTitle = "Exodock"
	if !m.opts.NoColor {
		p := m.palette()
		v.BackgroundColor = lipgloss.Color(p.canvas)
		v.ForegroundColor = lipgloss.Color(p.text)
	}
	return v
}

func (m *Model) headerLines(w int) []string {
	connection, role := "CONNECTING", "warn"
	if m.connected {
		connection, role = "CONNECTED", "good"
	} else if !m.refreshing {
		connection, role = "DISCONNECTED", "bad"
	}
	access := "READ / WRITE"
	if m.opts.ReadOnly {
		access = "READ ONLY"
	}
	mark := m.glyph("◈", "<>")
	left := " " + m.style(mark, m.facetRole()) + " " + m.style("EXODOCK", "accent")
	if w >= 85 {
		left += "  " + m.style("Docker management, simplified.", "muted")
	}
	right := m.badge(connection, role) + " " + m.badge(access, "muted") + " "
	head := fit(left, max(0, w-ansi.StringWidth(right))) + right
	target := " " + m.style("TARGET", "muted") + "  " + safety.Inline(m.opts.Target)
	status := "inventory ready"
	if m.actionPending {
		status = "action in progress"
	} else if m.refreshing {
		status = "refreshing inventory"
	} else if m.diskPending {
		status = "measuring Docker usage"
	} else if m.statsPending {
		status = "sampling container stats"
	} else if m.detailPending {
		status = "loading safe inspection"
	}
	if m.actionPending || m.refreshing || m.diskPending || m.statsPending || m.detailPending {
		status = m.busySymbol() + " " + status
	}
	meta := " " + m.style(status, "muted") + " "
	target = fit(target, max(0, w-ansi.StringWidth(meta))) + meta
	breadcrumb := " " + m.style("WORKSPACE", "muted") + "  " + m.style(m.glyph("›", ">"), "edge") + "  " + m.style(sections[m.section], "accent")
	if m.detailOpen && m.section > 0 && m.section < 7 {
		breadcrumb += "  " + m.style(m.glyph("›", ">"), "edge") + "  " + m.style(m.tab, "text")
	}
	tip := " Ctrl+k find  : commands "
	return []string{m.paint(head, "header", w), m.paint(target, "header", w), m.paint(fit(breadcrumb, max(0, w-len(tip)))+m.style(tip, "muted"), "canvas", w)}
}

func (m *Model) busySymbol() string {
	// Motion is limited to work in progress; idle frames never animate.
	if m.opts.ReducedMotion {
		return m.style(m.glyph("◈", "*"), "glow")
	}
	frames := []string{"◜", "◝", "◞", "◟"}
	if m.opts.ASCII {
		frames = []string{"|", "/", "-", "\\"}
	}
	return m.style(frames[m.animationFrame%len(frames)], "glow")
}

func (m *Model) facetRole() string {
	if m.startupFrames > 0 && !m.opts.ReducedMotion && !m.opts.NoColor && m.animationFrame%2 == 1 {
		return "accent"
	}
	return "glow"
}

func (m *Model) workspace(w, h int) string {
	if w >= 80 && h >= 18 {
		navW := 20
		mainW := w - navW - 1
		nav := m.box("NAVIGATION", m.navLinesWidth(navW-2), navW, h, m.focus == 0)
		if m.section != 0 && w >= 120 && h >= 24 && !(m.detailOpen && m.tab != "Summary") {
			detailW := min(38, w/3)
			mainW -= detailW + 1
			return lipgloss.JoinHorizontal(lipgloss.Top, nav, " ", m.box(strings.ToUpper(sections[m.section]), m.mainLines(mainW-2, h-2), mainW, h, m.focus == 1), " ", m.box(m.detailTitle(), m.detailLines(detailW-2, h-2), detailW, h, m.focus == 2))
		}
		title := strings.ToUpper(sections[m.section])
		content := m.mainLines(mainW-2, h-2)
		if m.detailOpen {
			title = m.detailTitle()
			content = m.detailLines(mainW-2, h-2)
		}
		return lipgloss.JoinHorizontal(lipgloss.Top, nav, " ", m.box(title, content, mainW, h, m.focus != 0))
	}
	nav := fmt.Sprintf(" %d/8  %s   [ / ] sections · Tab focus", m.section+1, sections[m.section])
	title := strings.ToUpper(sections[m.section])
	content := m.mainLines(w-2, h-3)
	if m.detailOpen {
		title = m.detailTitle()
		content = m.detailLines(w-2, h-3)
	}
	return m.paint(m.style(nav, "accent"), "header", w) + "\n" + m.box(title, content, w, h-1, true)
}

func (m *Model) noticeLine(w int) string {
	tone := m.noticeTone
	if tone == "" {
		tone = "muted"
	}
	symbol := m.glyph("●", "*")
	if tone == "bad" || tone == "warn" {
		symbol = "!"
	}
	return m.paint(" "+m.style(symbol, tone)+" "+m.style(safety.Inline(m.notice), tone), "header", w)
}
func (m *Model) footerLine(w int) string {
	keys := " Enter containers  a attention  Ctrl+k find  ? help  q quit"
	if m.tab == "Logs" && m.detailOpen {
		keys = " / search  Space pause  f follow  v stream  t time  Esc back"
	} else if m.section >= 1 && m.section <= 6 {
		keys = " j/k move  Enter details  / filter  i inspect  ? help"
		if m.section == 1 || m.section == 6 {
			keys = " j/k move  Enter details  / filter  a actions  l logs  s stats"
		}
		if m.section == 1 && !m.detailOpen {
			keys = " j/k move  Enter details  / filter  f state  a actions  ? help"
		}
	} else if m.section == 7 {
		keys = " d disk usage  r refresh  [/] sections  ? help  q quit"
	}
	if w < 80 {
		switch {
		case m.tab == "Logs" && m.detailOpen:
			keys = " / search  Space pause  f follow  v stream  Esc back"
		case m.section >= 1 && m.section <= 6:
			keys = " j/k move  Enter details  / filter  i inspect  ? help"
			if m.section == 1 || m.section == 6 {
				keys = " j/k move  Enter details  a actions  / filter  ? help"
			}
		case m.section == 0:
			keys = " Enter browse  a attention  Ctrl+k find  ? help  q quit"
		}
	}
	return m.paint(m.style(keys, "accent"), "header", w)
}
func (m *Model) navLines() []string { return m.navLinesWidth(18) }
func (m *Model) navLinesWidth(w int) []string {
	lines := []string{m.style("  CONTROL CENTER", "muted")}
	for i, s := range sections {
		if i == 1 {
			lines = append(lines, "", m.style("  INVENTORY", "muted"))
		}
		if i == 6 {
			lines = append(lines, "", m.style("  OBSERVABILITY", "muted"))
		}
		count := ""
		if i >= 1 && i <= 5 {
			kind := []core.Kind{core.Containers, core.Images, core.Volumes, core.Networks, core.Compose}[i-1]
			count = fmt.Sprint(len(m.snapshot.Resources[kind]))
		}
		label := fmt.Sprintf("%d %s", i+1, s)
		if count != "" {
			label = fit(label, max(0, w-3-len(count))) + m.style(count, "muted")
		}
		lines = append(lines, m.selectedRow(label, w, i == m.section))
	}
	lines = append(lines, "", m.style("  [ / ] sections", "muted"), m.style("  Tab focus", "muted"))
	return lines
}
func (m *Model) mainLines(w, h int) []string {
	switch m.section {
	case 0:
		return m.overviewLines(w, h)
	case 7:
		return m.systemLines(w, h)
	case 6:
		return append([]string{m.style(" Selected container only", "muted")}, m.resourceTable(w, h-1)...)
	}
	return m.resourceTable(w, h)
}
