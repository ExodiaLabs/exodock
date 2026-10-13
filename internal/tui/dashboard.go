package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

type inventoryHealth struct {
	total, running, stopped, other, healthy, unhealthy, starting, unchecked int
	attention                                                               []core.Resource
}

func (m *Model) inventoryHealth() inventoryHealth {
	s := inventoryHealth{}
	for _, r := range m.snapshot.Resources[core.Containers] {
		s.total++
		if r.State == "running" {
			s.running++
		} else if matchesStatus(r, "stopped") {
			s.stopped++
		} else {
			s.other++
		}
		switch r.Health {
		case "healthy":
			s.healthy++
		case "unhealthy":
			s.unhealthy++
		case "starting":
			s.starting++
		default:
			s.unchecked++
		}
		if r.Health == "unhealthy" || r.State == "dead" || r.State == "restarting" {
			s.attention = append(s.attention, r)
		}
	}
	// The daemon inventory counts may include resources excluded by an error.
	if m.snapshot.System.Containers > s.total {
		s.total = m.snapshot.System.Containers
		s.running = m.snapshot.System.Running
		s.stopped = m.snapshot.System.Stopped
		s.other = max(0, s.total-s.running-s.stopped)
	}
	return s
}

func (m *Model) overviewLines(w, h int) []string {
	if err := m.snapshot.Errors[core.Containers]; err != "" {
		return m.overviewUnavailable(w, h, err)
	}
	if !m.connected || m.snapshot.CollectedAt.IsZero() {
		reason := "Waiting for Docker's first inventory."
		if !m.connected && !m.snapshot.CollectedAt.IsZero() {
			reason = "Docker disconnected. Previous inventory is stale; health is unknown."
		}
		return m.overviewUnavailable(w, h, reason)
	}
	s := m.inventoryHealth()
	if h < 15 || w < 56 {
		lines := []string{m.style(" YOUR DOCKER WORKSPACE", "accent"), fmt.Sprintf(" %d containers · %d running · %d stopped", s.total, s.running, s.stopped)}
		if s.other > 0 {
			lines[1] += fmt.Sprintf(" · %d other", s.other)
		}
		lines = append(lines, m.style(" "+m.meter(float64(s.running)/float64(max(1, s.total)), max(8, w-23), "good")+fmt.Sprintf(" %d active / %d total", s.running, s.total), "text"))
		lines = append(lines, fmt.Sprintf(" Images %d · Volumes %d · Networks %d", len(m.snapshot.Resources[core.Images]), len(m.snapshot.Resources[core.Volumes]), len(m.snapshot.Resources[core.Networks])))
		if s.other > 0 {
			lines = append(lines, m.style(fmt.Sprintf(" Other states: %d paused or restarting", s.other), "warn"))
		}
		lines = append(lines, m.style(" ATTENTION", "warn"))
		if len(s.attention) == 0 {
			lines = append(lines, " No unhealthy or restarting containers detected.")
		} else {
			for _, r := range s.attention[:min(2, len(s.attention))] {
				lines = append(lines, m.style(" ! "+m.clip(safety.Inline(r.Name)+" · "+safety.Inline(r.State)+" / "+safety.Inline(r.Health), w-4), "warn"))
			}
		}
		lines = append(lines, m.style(" Enter containers · a attention · Ctrl+k find", "accent"))
		return lines
	}
	lines := m.dashboardHero(w)
	lines = append(lines, "")
	cardW := (w - 2) / 3
	stoppedNote := "inactive containers"
	if s.other > 0 {
		stoppedNote = fmt.Sprintf("%d other active", s.other)
	}
	cards := [][]string{m.countCard("CONTAINERS", s.total, "loaded inventory", cardW, h >= 23), m.countCard("RUNNING", s.running, "active containers", cardW, h >= 23), m.countCard("STOPPED", s.stopped, stoppedNote, w-2*cardW-2, h >= 23)}
	gap := make([]string, len(cards[0]))
	for i := range gap {
		gap[i] = " "
	}
	lines = append(lines, joinColumns(cards[0], gap, cards[1], gap, cards[2])...)
	lines = append(lines, "")
	kinds := fmt.Sprintf("%d images   %d volumes   %d networks   %d Compose projects", len(m.snapshot.Resources[core.Images]), len(m.snapshot.Resources[core.Volumes]), len(m.snapshot.Resources[core.Networks]), len(m.snapshot.Resources[core.Compose]))
	if s.other > 0 {
		kinds += fmt.Sprintf("   %d paused/restarting", s.other)
	}
	lines = append(lines, m.paint(" "+m.style(kinds, "muted"), "raised", w), "")
	health := []string{m.style("HEALTH DISTRIBUTION", "accent")}
	for _, x := range []struct {
		label string
		n     int
		role  string
	}{{"Healthy", s.healthy, "good"}, {"Unhealthy", s.unhealthy, "bad"}, {"Starting", s.starting, "warn"}, {"No healthcheck", s.unchecked, "muted"}} {
		health = append(health, fmt.Sprintf("%-14s", x.label)+m.meter(float64(x.n)/float64(max(1, s.healthy+s.unhealthy+s.starting+s.unchecked)), max(6, w/2-22), x.role)+fmt.Sprintf(" %d", x.n))
	}
	remaining := h - len(lines) - 2
	if remaining >= 6 && w >= 72 {
		half := (w - 1) / 2
		attention := m.attentionLines(s, half-2, max(3, remaining-1))
		lines = append(lines, joinColumns(m.card("HEALTH DISTRIBUTION", health[1:], half, 5), []string{" ", " ", " ", " ", " "}, m.card("ATTENTION", attention, w-half-1, 5))...)
	} else {
		lines = append(lines, health[:min(3, max(1, remaining))]...)
		if h-len(lines) > 2 {
			lines = append(lines, m.style(fmt.Sprintf(" ATTENTION  %d need review · a inspect", len(s.attention)), "warn"))
		}
	}
	if h-len(lines) >= 5 {
		activity := m.activityLines(w - 2)
		lines = append(lines, "")
		lines = append(lines, m.card("WORKSPACE ACTIVITY", activity, w, min(4, h-len(lines)-1))...)
	}
	if h-len(lines) >= 2 {
		lines = append(lines, "", m.style(" Enter browse containers   a review attention   Ctrl+k find", "accent"))
	}
	return lines
}

func (m *Model) dashboardHero(w int) []string {
	mark := []string{"    /\\", "   / /\\", "   \\/_/"}
	if !m.opts.ASCII {
		mark = []string{"    ╱╲", "   ╱╱ ╲", "   ╲╲╱╱"}
	}
	return []string{
		m.style(mark[0], m.facetRole()) + "   " + m.style("YOUR DOCKER WORKSPACE", "accent"),
		m.style(mark[1], "accent") + "   Containers, composed.",
		m.style(mark[2], "edge") + "   " + m.style(m.clip("A clear view of "+safety.Inline(m.opts.Target), w-10), "muted"),
	}
}

func (m *Model) overviewUnavailable(w, h int, err string) []string {
	lines := []string{m.style(" YOUR DOCKER WORKSPACE", "accent")}
	if h >= 15 && w >= 56 {
		lines = m.dashboardHero(w)
		lines = append(lines, "")
	}
	message := []string{m.style("Container inventory unavailable", "warn"), "Health unknown; no reliable distribution to display."}
	for _, line := range strings.Split(ansi.Hardwrap(safety.Inline(err), max(1, w-3), false), "\n") {
		message = append(message, m.style(line, "bad"))
	}
	message = append(message, m.style("r retry container inventory", "accent"))
	if h >= 15 {
		lines = append(lines, m.card("HEALTH UNAVAILABLE", message, w, min(len(message)+1, h-len(lines)))...)
	} else {
		lines = append(lines, m.style(" HEALTH UNAVAILABLE · state unknown", "warn"))
		lines = append(lines, message[1:]...)
	}
	return lines
}

var displayDigits = [10][3]string{
	{"╭─╮", "│ │", "╰─╯"}, {" ╷ ", " │ ", " ╵ "}, {"╭─╮", "╭─╯", "╰──"}, {"──╮", " ─┤", "──╯"}, {"╷ ╷", "╰─┤", "  ╵"}, {"╭──", "╰─╮", "──╯"}, {"╭──", "├─╮", "╰─╯"}, {"──╮", "  │", "  ╵"}, {"╭─╮", "├─┤", "╰─╯"}, {"╭─╮", "╰─┤", "──╯"},
}

func (m *Model) countCard(label string, n int, note string, w int, large bool) []string {
	number := fmt.Sprint(n)
	content := []string{m.style(number, "accent"), m.style(note, "muted")}
	if large && !m.opts.ASCII && len(number)*4 < w-2 {
		content = nil
		for row := 0; row < 3; row++ {
			var b strings.Builder
			for _, digit := range number {
				b.WriteString(displayDigits[digit-'0'][row])
				b.WriteByte(' ')
			}
			content = append(content, m.style(b.String(), "accent"))
		}
		content = append(content, m.style(note, "muted"))
	}
	return m.card(label, content, w, len(content)+1)
}
func (m *Model) attentionLines(s inventoryHealth, w, h int) []string {
	if len(s.attention) == 0 {
		return []string{m.style("No anomalies detected", "good"), "No unhealthy or restarting", "containers in loaded inventory.", m.style("a review containers", "muted")}
	}
	lines := []string{m.style(fmt.Sprintf("%d containers need review", len(s.attention)), "warn")}
	for _, r := range s.attention[:min(len(s.attention), max(1, h-2))] {
		state := r.State
		if r.Health == "unhealthy" {
			state = "unhealthy"
		}
		lines = append(lines, m.style("! "+m.clip(safety.Inline(r.Name), max(1, w-14)), "text")+" "+m.style(safety.Inline(state), "warn"))
	}
	return append(lines, m.style("a open attention list", "accent"))
}
func (m *Model) activityLines(w int) []string {
	collected := "Awaiting first inventory"
	if !m.snapshot.CollectedAt.IsZero() {
		collected = "Inventory refreshed at " + m.snapshot.CollectedAt.Format("15:04:05")
	}
	activity := m.lastActivity
	if activity == "" {
		activity = "No container action in this session"
	}
	return []string{m.style(collected, "muted"), m.clip(safety.Inline(activity), w), m.style("r refresh inventory  ·  8 inspect daemon", "accent")}
}
