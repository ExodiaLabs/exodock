package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSystemShortcutsCannotTargetHiddenContainer(t *testing.T) {
	for _, key := range []string{"a", "l", "s", "i"} {
		m, _, e := model(t)
		m.section = 7
		oldGeneration := m.generation
		press(m, key)
		if m.overlay != "" || m.detailOpen || m.generation != oldGeneration || len(e.requests) != 0 {
			t.Fatalf("System shortcut %q opened a hidden resource operation", key)
		}
	}
}

func TestReadOnlyInventoryFootersShowApplicableShortcuts(t *testing.T) {
	m, _, _ := model(t)
	for _, width := range []int{60, 120} {
		for section := 2; section <= 5; section++ {
			m.section = section
			footer := ansi.Strip(m.footerLine(width))
			if !strings.Contains(footer, "i inspect") || strings.Contains(footer, "a actions") || strings.Contains(footer, "l logs") || strings.Contains(footer, "s stats") {
				t.Fatalf("section=%d width=%d invalid footer: %s", section, width, footer)
			}
		}
	}
}
