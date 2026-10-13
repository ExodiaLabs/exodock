package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

// Geometry takes full terminal dimensions. The two-cell clearance preserves
// the workspace context, and the confirmation guard uses this same geometry.
func modalGeometry(w, h int) (int, int) { return max(4, min(86, w-4)), max(3, min(24, h-8)) }
func (m *Model) confirmationFits() bool {
	w, h := modalGeometry(m.width, m.height)
	return m.width >= 60 && m.height >= 18 && len(m.confirmationLines(w)) <= h-2
}
func (m *Model) overlayOn(body string, w, h int) string {
	mw, mh := modalGeometry(w, h+6)
	modal := strings.Split(m.modal(mw, mh), "\n")
	backdrop := strings.Split(body, "\n")
	for len(backdrop) < h {
		backdrop = append(backdrop, "")
	}
	left, top := (w-mw)/2, (h-len(modal))/2
	for i := range backdrop {
		backdrop[i] = m.paint(m.style(ansi.Strip(fit(backdrop[i], w)), "muted"), "canvas", w)
	}
	for i, line := range modal {
		y := top + i
		if y < 0 || y >= len(backdrop) {
			continue
		}
		backdrop[y] = ansi.Cut(backdrop[y], 0, left) + line + ansi.Cut(backdrop[y], left+mw, w)
	}
	return strings.Join(backdrop[:h], "\n")
}

func (m *Model) modal(w, h int) string {
	title := ""
	var lines []string
	switch m.overlay {
	case "input":
		title = map[string]string{"filter": "FILTER THIS LIST", "logs": "SEARCH LOADED LOGS", "global": "FIND A RESOURCE", "palette": "COMMAND PALETTE"}[m.inputMode]
		lines = []string{m.paint(" "+m.style(m.glyph("›", ">")+" "+m.clip(safety.Inline(m.input), w-7)+"_", "accent"), "raised", w-2), m.separator(w - 2)}
		if m.inputMode == "global" || m.inputMode == "palette" {
			label := "SEARCH RESULTS"
			if m.inputMode == "palette" {
				label = "MATCHING COMMANDS"
			}
			lines = append(lines, m.style(" "+label, "muted"))
			lines = append(lines, m.searchResultLines(m.inputMode, w-2, max(1, h-7))...)
			lines = append(lines, "", m.style(" Up/Down select · Enter open · Esc cancel", "accent"))
		} else {
			lines = append(lines, "", " Enter apply · Esc cancel", " Text fields capture all characters.")
			if m.inputMode == "logs" {
				lines = append(lines, " Searches the loaded log buffer only.")
			}
		}
	case "global", "palette":
		title = "SEARCH RESULTS"
		if m.overlay == "palette" {
			title = "COMMAND PALETTE"
		}
		lines = []string{m.style(" / "+m.clip(safety.Inline(m.input), w-5), "accent"), m.separator(w - 2)}
		lines = append(lines, m.searchResultLines(m.overlay, w-2, max(1, h-6))...)
		lines = append(lines, "", m.style(" Enter open · / edit query · Esc close", "accent"))
	case "help":
		title = "KEYBOARD GUIDE · EXODIALABS"
		all := []string{" j/k or Up/Down    Move selection or section", " Tab / Shift+Tab   Cycle navigation, list, details", " [ / ] or 1-8      Switch workspace section", " Enter / Esc       Open details / return", " /                 Filter list or search loaded logs", " Ctrl+k            Find a resource as you type", " :                 Navigation and action palette", " a                 Container actions; Overview attention", " f (Containers)    Cycle all / running / stopped", " l / s / i         Logs / stats / safe inspection", " r                 Refresh inventory, retry connection", " d (System)        Measure Docker disk usage", " Space (Logs)      Pause/resume scrolling", " f (Logs)          Return to live following", " v / t (Logs)      Cycle stream / time range", " j/k (Logs/Inspect) Scroll loaded history or fields", " q / Ctrl+c        Quit (q closes open overlays)", " Esc               Close or cancel confirmation", "", " Read-only disables all mutations.", " Confirmations default to Cancel and freeze the ID.", " Logs: 10,000 entries / 8 MiB; lines capped at 64 KiB.", " CPU: 100% is one CPU; selected-container scope."}
		capacity := max(1, h-4)
		start := min(max(0, m.menuIndex), max(0, len(all)-capacity))
		lines = append(lines, all[start:min(len(all), start+capacity)]...)
		lines = append(lines, "", m.style(" Up/Down scroll guide · Esc close", "accent"))
	case "actions":
		title = "CONTAINER ACTIONS"
		if r, ok := m.resource(); ok {
			lines = append(lines, m.style(" "+m.clip(safety.Inline(r.Name), w-18)+"  "+shortID(r.ID), "muted"))
		}
		lines = append(lines, m.separator(w-2))
		for i, a := range actions {
			label := strings.ToUpper(string(a[:1])) + string(a[1:])
			reason := m.actionReason(a)
			role := "text"
			if reason != "" {
				role = "muted"
				label += " · " + reason
			}
			lines = append(lines, m.selectedRow(m.style(m.clip(label, w-5), role), w-2, i == m.menuIndex))
		}
		lines = append(lines, "", m.style(" Enter choose · Esc cancel", "accent"))
	case "confirm":
		title = "CONFIRM " + strings.ToUpper(string(m.frozen.Action))
		lines = m.confirmationLines(w)
		if len(lines) > h-2 {
			title = "ACTION BLOCKED"
			lines = []string{m.style(" The complete confirmation scope does not fit.", "warn"), "", " Resize to review the resource, target and full ID.", " No action can run while scope is hidden.", "", m.style(" Esc cancels", "accent")}
		}
	}
	// Small dialogs follow their content instead of becoming empty full panels.
	h = min(h, max(3, len(lines)+2))
	return m.box(title, lines, w, h, true)
}

func (m *Model) searchResultLines(mode string, w, capacity int) []string {
	var entries []string
	if mode == "global" {
		for _, r := range m.globalResults() {
			entries = append(entries, safety.Inline(r.Name)+" · "+string(r.Kind)+" · "+shortID(r.ID))
		}
	} else {
		for _, entry := range m.paletteResults() {
			entries = append(entries, entry.name)
		}
	}
	if len(entries) == 0 {
		return []string{m.style(" No matching "+map[bool]string{true: "resources.", false: "commands."}[mode == "global"], "muted")}
	}
	selected := max(0, min(m.menuIndex, len(entries)-1))
	start := max(0, selected-capacity+1)
	lines := make([]string, 0, min(len(entries), capacity))
	for i := start; i < min(len(entries), start+capacity); i++ {
		lines = append(lines, m.selectedRow(m.clip(entries[i], w-3), w, i == selected))
	}
	return lines
}

func (m *Model) confirmationLines(w int) []string {
	var lines []string
	appendField := func(label, value string) {
		text := label + "  " + safety.Inline(value)
		for _, line := range strings.Split(ansi.Hardwrap(text, max(1, w-4), false), "\n") {
			lines = append(lines, " "+line)
		}
	}
	appendField("Resource", m.frozenName)
	appendField("Target  ", m.frozenTarget)
	// No abbreviations: the frozen opaque identity is always reviewable in full.
	appendField("ID      ", m.frozen.ID)
	consequences := map[core.Action]string{core.Start: "Start this existing container.", core.Stop: "Interrupt the service; allow Docker's grace period.", core.Restart: "Interrupt and restart this service.", core.Remove: "Delete the writable layer. Volumes are kept."}
	for _, line := range strings.Split(ansi.Wrap(consequences[m.frozen.Action], max(1, w-4), ""), "\n") {
		lines = append(lines, m.style(" "+line, "warn"))
	}
	cancel, accept := "[ CANCEL ]", "  Confirm  "
	if m.confirm {
		cancel, accept = "  Cancel  ", "[ CONFIRM ]"
	}
	lines = append(lines, m.style(" "+cancel+"    "+accept, "accent"), " Tab select · Enter apply · Esc cancel")
	return lines
}
