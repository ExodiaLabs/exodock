package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Official default theme, ExodiaLabs/exodialabs-website tokens.css,
// c65f6dcd91bf7b94634f9ca289b4e8af1e37335d. State colors are ANSI roles.
type palette struct {
	canvas, header, surface, raised, selected, edge, text, muted, accent, glow string
	styles                                                                     map[string]lipgloss.Style
	backgrounds                                                                map[string]string
	foreground                                                                 string
}

func newPalette(light bool) palette {
	p := palette{canvas: "#0b0910", header: "#15111e", surface: "#1d1927", raised: "#27222f", selected: "#543680", edge: "#38323f", text: "#f5f2f9", muted: "#b4aebe", accent: "#bba9dc", glow: "#b274f2"}
	if light {
		p = palette{canvas: "#ece5f5", header: "#f5f2f9", surface: "#f5f2f9", raised: "#ece5f5", selected: "#decdf2", edge: "#bba9dc", text: "#15111e", muted: "#5e5868", accent: "#7960a6", glow: "#7041d1"}
	}
	p.styles = make(map[string]lipgloss.Style)
	p.backgrounds = make(map[string]string)
	p.foreground = ansi.NewStyle().ForegroundColor(lipgloss.Color(p.text)).String()
	for role, color := range map[string]string{"text": p.text, "muted": p.muted, "accent": p.accent, "glow": p.glow, "edge": p.edge, "good": "2", "warn": "3", "bad": "1"} {
		state := role == "good" || role == "warn" || role == "bad"
		if light && state {
			color = p.text
		}
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		if role == "accent" || role == "glow" || light && state {
			st = st.Bold(true)
		}
		p.styles[role] = st
	}
	for role, bg := range map[string]string{"canvas": p.canvas, "header": p.header, "surface": p.surface, "raised": p.raised, "selected": p.selected} {
		p.styles[role] = lipgloss.NewStyle().Background(lipgloss.Color(bg)).Foreground(lipgloss.Color(p.text))
		p.backgrounds[role] = ansi.NewStyle().BackgroundColor(lipgloss.Color(bg)).String()
	}
	return p
}

var darkPalette = newPalette(false)
var lightPalette = newPalette(true)

func (m *Model) isLight() bool {
	return m.opts.Theme == "light" || ((m.opts.Theme == "auto" || m.opts.Theme == "") && m.lightTheme)
}
func (m *Model) palette() *palette {
	if m.isLight() {
		return &lightPalette
	}
	return &darkPalette
}
func (m *Model) style(s, role string) string {
	if m.opts.NoColor {
		return s
	}
	if role == "" {
		role = "text"
	}
	return m.palette().styles[role].Render(s)
}
func (m *Model) paint(s, role string, w int) string {
	s = fit(s, w)
	if m.opts.NoColor {
		return s
	}
	p := m.palette()
	bg := p.backgrounds[role]
	return bg + p.foreground + restoreSurface(s, bg, p.foreground) + ansi.ResetStyle
}

// Nested foreground styles emit default-background and full SGR resets.
// Restore the enclosing surface only when colors return to defaults, preserving
// explicit child card backgrounds. Process SGR sequences, never terminal cells.
func restoreSurface(s, bg, fg string) string {
	var out strings.Builder
	out.Grow(len(s) + 64)
	for len(s) > 0 {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:start])
		s = s[start:]
		end := 2
		for end < len(s) && (s[end] < '@' || s[end] > '~') {
			end++
		}
		if end >= len(s) {
			out.WriteString(s)
			break
		}
		out.WriteString(s[:end+1])
		if s[end] == 'm' {
			restoreBG, restoreFG := sgrDefaults(s[2:end])
			if restoreBG {
				out.WriteString(bg)
			}
			if restoreFG {
				out.WriteString(fg)
			}
		}
		s = s[end+1:]
	}
	return out.String()
}

func sgrDefaults(params string) (bool, bool) {
	bg, fg := false, false
	if params == "" {
		return true, true
	}
	read := func(pos *int) int {
		v := 0
		for *pos < len(params) && params[*pos] >= '0' && params[*pos] <= '9' {
			v = v*10 + int(params[*pos]-'0')
			(*pos)++
		}
		if *pos < len(params) {
			(*pos)++
		}
		return v
	}
	for pos := 0; pos < len(params); {
		code := read(&pos)
		switch {
		case code == 0:
			bg, fg = true, true
		case code == 39:
			fg = true
		case code == 49:
			bg = true
		case code == 38 || code == 48 || code == 58:
			if code == 38 {
				fg = false
			} else if code == 48 {
				bg = false
			}
			mode := read(&pos)
			count := 0
			if mode == 2 {
				count = 3
			} else if mode == 5 {
				count = 1
			}
			for n := 0; n < count && pos < len(params); n++ {
				read(&pos)
			}
		case code >= 30 && code <= 37 || code >= 90 && code <= 97:
			fg = false
		case code >= 40 && code <= 47 || code >= 100 && code <= 107:
			bg = false
		}
	}
	return bg, fg
}
