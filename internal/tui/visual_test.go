package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
)

func visualModel(t *testing.T, theme string, ascii, noColor bool) *Model {
	t.Helper()
	m := New(context.Background(), &fakeBackend{}, &fakeExecutor{}, Options{Target: "demo (local)", Theme: theme, ASCII: ascii, NoColor: noColor})
	m.snapshot = fixture()
	m.connected = true
	m.section, m.focus = 1, 1
	m.selectedID = m.snapshot.Resources[core.Containers][0].ID
	t.Cleanup(m.cancel)
	return m
}

func TestVisualLayoutsFitAcrossThemesAndFallbacks(t *testing.T) {
	for _, mode := range []struct {
		name, theme    string
		ascii, noColor bool
	}{{"dark", "dark", false, false}, {"light", "light", false, false}, {"ascii-color", "dark", true, false}, {"no-color", "dark", false, true}, {"ascii-no-color", "light", true, true}} {
		t.Run(mode.name, func(t *testing.T) {
			m := visualModel(t, mode.theme, mode.ascii, mode.noColor)
			for _, size := range [][2]int{{160, 44}, {120, 32}, {100, 28}, {80, 24}, {60, 18}, {42, 12}, {10, 3}} {
				m.width, m.height = size[0], size[1]
				for section := range sections {
					m.section = section
					for _, overlay := range []string{"", "help", "actions", "confirm", "input", "global", "palette"} {
						m.overlay = overlay
						m.inputMode = "global"
						m.frozen = core.ActionRequest{ID: strings.Repeat("abcdef0123456789", 4), Action: core.Remove}
						m.frozenName, m.frozenTarget = "worker-1", "demo (local)"
						view := m.View().Content
						lines := strings.Split(view, "\n")
						context := fmt.Sprintf("%dx%d section=%s overlay=%q", m.width, m.height, sections[section], overlay)
						if len(lines) > m.height {
							t.Fatalf("%s: %d rows exceed terminal", context, len(lines))
						}
						for i, line := range lines {
							if width := ansi.StringWidth(line); width > m.width {
								t.Fatalf("%s row=%d width=%d exceeds terminal", context, i, width)
							}
						}
						if mode.noColor && strings.Contains(view, "\x1b") {
							t.Fatalf("%s: no-color emitted terminal escapes", context)
						}
						if mode.ascii {
							for _, r := range ansi.Strip(view) {
								if r > 127 {
									t.Fatalf("%s: ASCII emitted %q", context, r)
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestMinimumConfirmationShowsCompleteFrozenScopeAcrossModes(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		for _, ascii := range []bool{false, true} {
			for _, noColor := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/ascii=%t/no-color=%t", theme, ascii, noColor), func(t *testing.T) {
					m := visualModel(t, theme, ascii, noColor)
					m.width, m.height = 60, 18
					m.snapshot.Resources[core.Containers][0].ID = strings.Repeat("9bcde012345678af6", 4)
					press(m, "a")
					m.menuIndex = 1
					press(m, "enter")
					originalID := m.frozen.ID
					m.opts.Target = "different daemon"
					m.snapshot.Resources[core.Containers][0].ID = "replacement"
					view := ansi.Strip(m.View().Content)
					visibleID := strings.Join(regexp.MustCompile(`[a-f0-9]{8,}`).FindAllString(view, -1), "")
					// IDs can wrap at the minimum size, but every character must remain visible in order.
					if visibleID != originalID || !strings.Contains(view, "demo (local)") || !strings.Contains(view, "CANCEL") {
						t.Fatalf("frozen scope or safe default hidden:\n%s", view)
					}
					if m.frozenTarget != "demo (local)" || m.frozen.ID != originalID {
						t.Fatal("confirmation changed the scope of its frozen request")
					}
				})
			}
		}
	}
}

func TestLiveSearchOpensTypedResourceWithOneEnter(t *testing.T) {
	m, _, _ := model(t)
	m.statusFilter = "running"
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	press(m, "worker")
	if !strings.Contains(ansi.Strip(m.View().Content), "worker-1") {
		t.Fatal("typed query does not display matching resources before Enter")
	}
	press(m, "enter")
	if m.overlay != "" || m.selectedID != "b" || !m.detailOpen || m.focus != 2 {
		t.Fatal("one Enter did not open the typed resource directly")
	}
}

func TestContainerStatusCycleCombinesWithTextAndReturnsToAll(t *testing.T) {
	m, _, _ := model(t)
	m.snapshot.Resources[core.Containers] = append(m.snapshot.Resources[core.Containers], core.Resource{ID: "bad", Name: "api-unhealthy", Kind: core.Containers, State: "running", Health: "unhealthy"})
	m.filter = "api"
	for _, expected := range []struct {
		status string
		count  int
	}{{"running", 2}, {"stopped", 0}, {"attention", 1}, {"", 2}} {
		press(m, "f")
		if m.statusFilter != expected.status || len(m.resources()) != expected.count || m.filter != "api" {
			t.Fatalf("status %q did not combine with the existing text filter", expected.status)
		}
	}
}

func TestCombinedContainerFiltersRemainVisible(t *testing.T) {
	m, _, _ := model(t)
	m.filter = "api"
	press(m, "f")
	view := strings.ToLower(m.View().Content)
	if !strings.Contains(view, "filter: api") || !strings.Contains(view, "running") {
		t.Fatal("container inventory hides one of its active filters")
	}
}

func TestStoppedDashboardCountExcludesPausedAndRestarting(t *testing.T) {
	m, _, _ := model(t)
	m.snapshot.Resources[core.Containers] = append(m.snapshot.Resources[core.Containers],
		core.Resource{Kind: core.Containers, ID: "paused", State: "paused"},
		core.Resource{Kind: core.Containers, ID: "restarting", State: "restarting"})
	counts := m.inventoryHealth()
	if counts.total != 4 || counts.running != 1 || counts.stopped != 1 {
		t.Fatalf("paused or restarting containers were counted as stopped: %#v", counts)
	}
}

func TestOverviewAttentionOpensOnlyContainersNeedingAction(t *testing.T) {
	m, _, _ := model(t)
	m.snapshot.Resources[core.Containers] = append(m.snapshot.Resources[core.Containers],
		core.Resource{ID: "bad", Name: "api-unhealthy", Kind: core.Containers, State: "running", Health: "unhealthy"},
		core.Resource{ID: "retry", Name: "api-restarting", Kind: core.Containers, State: "restarting"},
		core.Resource{ID: "dead", Name: "api-dead", Kind: core.Containers, State: "dead"})
	m.section, m.filter = 0, "not-present"
	press(m, "a")
	if m.section != 1 || m.statusFilter != "attention" || len(m.resources()) != 3 || m.overlay != "" {
		t.Fatal("dashboard attention shortcut did not open the containers needing action")
	}
}

func TestLivePaletteCapturesShortcutLettersAndOpensWithOneEnter(t *testing.T) {
	m, _, e := model(t)
	press(m, ":")
	press(m, "Images")
	if m.input != "Images" || m.quitting || len(e.requests) != 0 {
		t.Fatal("palette query leaked a shortcut")
	}
	if !strings.Contains(ansi.Strip(m.View().Content), "Images") {
		t.Fatal("palette does not show matches while typing")
	}
	press(m, "enter")
	if m.overlay != "" || m.section != 2 {
		t.Fatal("palette requires an extra Enter to open a section")
	}
}

func TestLiveSearchPasteIsBoundedAndEscapeRestoresSelection(t *testing.T) {
	m, _, e := model(t)
	original := m.selectedID
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	m.Update(tea.PasteMsg{Content: strings.Repeat("q", 1000) + "\x1b[2J"})
	if len([]rune(m.input)) > 256 || strings.Contains(m.input, "\x1b") || m.quitting || len(e.requests) != 0 {
		t.Fatal("live search paste is unbounded, unsafe or leaked shortcuts")
	}
	press(m, "esc")
	if m.overlay != "" || m.selectedID != original {
		t.Fatal("cancelling search changed the selected resource")
	}
}

func TestLiveSearchArrowSelectionOpensMatchingResult(t *testing.T) {
	m, _, _ := model(t)
	m.snapshot.Resources[core.Containers] = append(m.snapshot.Resources[core.Containers], core.Resource{ID: "api-two", Kind: core.Containers, Name: "api-2", State: "running"})
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	press(m, "api")
	m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.menuIndex != 1 || m.input != "api" {
		t.Fatal("arrow selection modified the live query")
	}
	press(m, "enter")
	if m.selectedID != "api-two" || m.overlay != "" {
		t.Fatal("Enter did not open the arrow-selected search result")
	}
}

func TestLiveSearchKeepsResultsVisibleWhenInventoryShrinks(t *testing.T) {
	m, _, _ := model(t)
	items := make([]core.Resource, 25)
	for n := range items {
		items[n] = core.Resource{Kind: core.Containers, ID: fmt.Sprintf("live-id-%02d", n), Name: fmt.Sprintf("matching-resource-%02d", n), State: "running"}
	}
	m.snapshot.Resources[core.Containers] = items
	m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	press(m, "matching")
	for n := 0; n < 20; n++ {
		m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	if m.menuIndex != 20 {
		t.Fatal("test did not establish selection deep in the result list")
	}
	snapshot := fixture()
	snapshot.Resources[core.Containers] = items[:3]
	m.Update(snapshotMsg{snapshot: snapshot})
	if !strings.Contains(ansi.Strip(m.View().Content), "matching-resource-02") {
		t.Fatal("refresh hid live search results after its result list shrank")
	}
	press(m, "enter")
	if m.overlay != "" || m.selectedID != "live-id-02" {
		t.Fatal("live search did not retain a valid selection after inventory shrink")
	}
}

func TestOverviewCollectionErrorDoesNotClaimKnownHealth(t *testing.T) {
	for _, size := range [][2]int{{60, 18}, {100, 28}, {150, 40}} {
		m := visualModel(t, "dark", true, true)
		m.section, m.width, m.height = 0, size[0], size[1]
		m.snapshot.Errors = map[core.Kind]string{core.Containers: "permission denied"}
		view := strings.ToLower(m.View().Content)
		if !strings.Contains(view, "unknown") && !strings.Contains(view, "unavailable") {
			t.Fatalf("%dx%d dashboard claims known health when container collection failed", size[0], size[1])
		}
		if strings.Contains(view, "no anomalies detected") || strings.Contains(view, "no unhealthy or restarting containers") {
			t.Fatal("failed collection reports an all-clear health state")
		}
	}
}

func TestHiddenScopeExplainsBlockAndCannotRun(t *testing.T) {
	m, _, e := model(t)
	m.width, m.height = 60, 18
	m.opts.Target = strings.Repeat("long-target-", 30)
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	view := m.View().Content
	if !strings.Contains(view, "ACTION BLOCKED") || !strings.Contains(view, "Resize") || strings.Contains(view, "[ CONFIRM ]") {
		t.Fatal("clipped scope does not explain that the action is blocked")
	}
	press(m, "tab")
	if cmd := press(m, "enter"); cmd != nil {
		cmd()
	}
	if len(e.requests) != 0 {
		t.Fatal("action executed with hidden target scope")
	}
}

var trueColorForeground = regexp.MustCompile(`\x1b\[[0-9;:]*38[;:]2[;:][0-9;:]+m`)
var trueColorBackground = regexp.MustCompile(`\x1b\[[0-9;:]*48[;:]2[;:][0-9;:]+m`)

func TestColorCompositionsCarryForegroundAndBackground(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			m := visualModel(t, theme, false, false)
			m.width, m.height = 120, 32
			for _, section := range []int{0, 1} {
				m.section = section
				view := m.View()
				if view.BackgroundColor == nil || view.ForegroundColor == nil || !trueColorForeground.MatchString(view.Content) || !trueColorBackground.MatchString(view.Content) {
					t.Fatal("color composition lacks explicit truecolor foreground/background roles")
				}
				if !strings.Contains(ansi.Strip(view.Content), "CONNECTED") {
					t.Fatal("connection state relies on color alone")
				}
			}
		})
	}
}

func TestSelectionHasDistinctFilledColorAndMonochromeMarker(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		m := visualModel(t, theme, true, false)
		selected := m.selectedRow("api-1", 40, true)
		unselected := m.selectedRow("worker-1", 40, false)
		selectedBG := trueColorBackground.FindString(selected)
		unselectedBG := trueColorBackground.FindString(unselected)
		if selectedBG == "" || unselectedBG == "" || selectedBG == unselectedBG {
			t.Fatal("selected and ordinary rows share the same background")
		}
		m.opts.NoColor = true
		if !strings.HasPrefix(m.selectedRow("api-1", 40, true), "> ") || strings.HasPrefix(m.selectedRow("worker-1", 40, false), "> ") {
			t.Fatal("no-color selection has no distinct text marker")
		}
	}
}

func TestNoColorFocusAndStatesRemainReadable(t *testing.T) {
	m := visualModel(t, "dark", true, true)
	m.width, m.height = 120, 32
	view := m.View().Content
	for _, label := range []string{"* CONTAINERS", "CONNECTED", "running", "exited"} {
		if !strings.Contains(view, label) {
			t.Fatalf("no-color view hides %q", label)
		}
	}
	m.focus = 0
	view = m.View().Content
	if !strings.Contains(view, "* NAVIGATION") || strings.Contains(view, "* CONTAINERS") {
		t.Fatal("no-color focus marker did not follow the active panel")
	}
}

func TestColorRenderingRejectsUntrustedTerminalCommands(t *testing.T) {
	m := visualModel(t, "dark", false, false)
	attack := "name\x1b]52;c;secret\a\x1b[2J"
	m.opts.Target = attack
	m.snapshot.Resources[core.Containers][0].Name = attack
	m.detail.Fields = []core.Field{{Name: attack, Value: attack}}
	m.tab, m.detailOpen, m.focus = "Inspect", true, 2
	m.width, m.height = 150, 40
	view := m.View().Content
	if strings.Contains(view, "\x1b]") || strings.Contains(view, "\x1b[2J") || strings.Contains(view, "secret") {
		t.Fatal("color rendering permitted an untrusted terminal command")
	}
	if !trueColorBackground.MatchString(view) {
		t.Fatal("test did not exercise the colored composition")
	}
}
