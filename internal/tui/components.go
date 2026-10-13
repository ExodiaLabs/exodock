package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"math"
	"strings"
)

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
func (m *Model) glyph(unicode, ascii string) string {
	if m.opts.ASCII {
		return ascii
	}
	return unicode
}
func (m *Model) separator(w int) string {
	return m.style(strings.Repeat(m.glyph("─", "-"), max(0, w)), "edge")
}

// A panel paints its entire surface, including padding and its focus rail.
func (m *Model) box(title string, lines []string, w, h int, active bool) string {
	if w < 4 || h < 3 {
		return ""
	}
	tl, tr, bl, br, hz, vt := "╭", "╮", "╰", "╯", "─", "│"
	if m.opts.ASCII {
		tl, tr, bl, br, hz, vt = "+", "+", "+", "+", "-", "|"
	}
	role := "edge"
	if active {
		role = "accent"
	}
	if active {
		title = m.glyph("▰", "*") + " " + title
	}
	heading := m.clip(" "+title+" ", w-4)
	top := tl + heading + strings.Repeat(hz, max(0, w-2-ansi.StringWidth(heading))) + tr
	out := []string{m.paint(m.style(top, role), "surface", w)}
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, m.paint(m.style(vt, role)+fit(line, w-2)+m.style(vt, role), "surface", w))
	}
	out = append(out, m.paint(m.style(bl+strings.Repeat(hz, w-2)+br, role), "surface", w))
	return strings.Join(out, "\n")
}
func (m *Model) selectedRow(s string, w int, selected bool) string {
	if selected {
		return m.paint(m.style(m.glyph("▎", ">"), "glow")+" "+s, "selected", w)
	}
	return m.paint("  "+s, "surface", w)
}
func (m *Model) badge(s, role string) string { return m.style(" "+s+" ", role) }
func (m *Model) card(title string, lines []string, w, h int) []string {
	out := []string{m.paint(" "+m.style(title, "accent"), "raised", w)}
	for i := 0; i < h-1; i++ {
		s := ""
		if i < len(lines) {
			s = lines[i]
		}
		out = append(out, m.paint(" "+s, "raised", w))
	}
	return out
}
func joinColumns(columns ...[]string) []string {
	blocks := make([]string, len(columns))
	for i, c := range columns {
		blocks[i] = strings.Join(c, "\n")
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, blocks...), "\n")
}
func (m *Model) meter(value float64, w int, role string) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	value = math.Max(0, math.Min(1, value))
	w = max(1, w)
	n := int(math.Round(value * float64(w)))
	return m.style(strings.Repeat(m.glyph("━", "#"), n), role) + m.style(strings.Repeat(m.glyph("─", "."), w-n), "edge")
}
