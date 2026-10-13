package tui

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/exodialabs/exodock/internal/core"
)

type fakeBackend struct{ snapshots, disks int }

func (f *fakeBackend) Snapshot(context.Context) (core.Snapshot, error) {
	f.snapshots++
	return fixture(), nil
}
func (*fakeBackend) Inspect(_ context.Context, k core.Kind, id string) (core.Detail, error) {
	return core.Detail{Resource: core.Resource{ID: id, Kind: k}}, nil
}
func (*fakeBackend) Stats(context.Context, string) (core.Stats, error) {
	return core.Stats{Available: true, CPUPercent: 2}, nil
}
func (*fakeBackend) Logs(context.Context, string, core.LogOptions) (core.LogStream, error) {
	return &fakeStream{}, nil
}
func (f *fakeBackend) DiskUsage(context.Context) (core.DiskUsage, error) {
	f.disks++
	return core.DiskUsage{Images: 1024, CollectedAt: time.Now()}, nil
}
func (*fakeBackend) ContainerAction(context.Context, string, core.Action) error { return nil }
func (*fakeBackend) Close() error                                               { return nil }

type fakeStream struct{ closed bool }

func (*fakeStream) Next(context.Context) (core.LogEntry, error) { return core.LogEntry{}, io.EOF }
func (s *fakeStream) Close() error                              { s.closed = true; return nil }

type fakeExecutor struct {
	requests []core.ActionRequest
	err      error
}

func (e *fakeExecutor) Execute(_ context.Context, r core.ActionRequest) error {
	e.requests = append(e.requests, r)
	return e.err
}
func fixture() core.Snapshot {
	return core.Snapshot{Resources: map[core.Kind][]core.Resource{
		core.Containers: {{ID: strings.Repeat("a", 64), Name: "api-1", Kind: core.Containers, State: "running", Health: "healthy", Image: "demo/api", Project: "demo"}, {ID: "b", Name: "worker-1", Kind: core.Containers, State: "exited"}},
		core.Images:     {{ID: "img", Name: "alpine:latest", Image: "alpine:latest", Kind: core.Images}},
	}, CollectedAt: time.Now(), System: core.System{Containers: 2, Running: 1, Stopped: 1}}
}
func model(t *testing.T) (*Model, *fakeBackend, *fakeExecutor) {
	t.Helper()
	b, e := &fakeBackend{}, &fakeExecutor{}
	m := New(context.Background(), b, e, Options{Target: "demo (local)", NoColor: true, ASCII: true})
	m.snapshot = fixture()
	m.connected = true
	m.section = 1
	m.focus = 1
	m.selectedID = m.snapshot.Resources[core.Containers][0].ID
	t.Cleanup(m.cancel)
	return m, b, e
}
func press(m *Model, s string) tea.Cmd {
	code := rune(0)
	text := s
	switch s {
	case "enter":
		code = tea.KeyEnter
		text = ""
	case "esc":
		code = tea.KeyEscape
		text = ""
	case "tab":
		code = tea.KeyTab
		text = ""
	case "space":
		code = ' '
		text = " "
	default:
		code = []rune(s)[0]
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: code, Text: text})
	return cmd
}

func TestConfirmationDefaultsToCancel(t *testing.T) {
	m, _, e := model(t)
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	if m.overlay != "confirm" || m.confirm {
		t.Fatal("confirmation should open with Cancel selected")
	}
	press(m, "enter")
	if len(e.requests) != 0 || m.overlay != "" {
		t.Fatal("default Enter must cancel")
	}
}
func TestConfirmationFreezesIDAndTarget(t *testing.T) {
	m, _, e := model(t)
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	original := m.frozen.ID
	m.selected = 1
	m.opts.Target = "other target"
	if m.frozen.ID != original || m.frozenTarget != "demo (local)" {
		t.Fatal("confirmation target changed")
	}
	press(m, "tab")
	cmd := press(m, "enter")
	if cmd == nil {
		t.Fatal("expected async action")
	}
	msg := cmd()
	m.Update(msg)
	if len(e.requests) != 1 || e.requests[0].ID != original || !e.requests[0].Confirmed || e.requests[0].Action != core.Stop {
		t.Fatalf("wrong execution: %#v", e.requests)
	}
}
func TestReadonlyCannotExecuteEvenThroughConfirmation(t *testing.T) {
	m, _, e := model(t)
	m.opts.ReadOnly = true
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	if m.overlay == "confirm" {
		t.Fatal("readonly action offered")
	}
	m.overlay = "confirm"
	m.confirm = true
	m.frozen = core.ActionRequest{ID: "b", Action: core.Remove}
	if cmd := press(m, "enter"); cmd != nil || len(e.requests) > 0 {
		t.Fatal("readonly bypass")
	}
}
func TestRemoveRunningContainerIsUnavailable(t *testing.T) {
	m, _, _ := model(t)
	if m.actionReason(core.Remove) == "" {
		t.Fatal("running removal offered")
	}
	m.selected = 1
	if m.actionReason(core.Remove) != "" {
		t.Fatal("stopped container cannot be removed")
	}
}
func TestStaleSelectionMessagesAreIgnored(t *testing.T) {
	m, _, _ := model(t)
	m.generation = 4
	m.statsPending = true
	m.detail = core.Detail{Resource: core.Resource{Name: "current"}}
	m.Update(detailMsg{gen: 3, detail: core.Detail{Resource: core.Resource{Name: "stale"}}})
	m.Update(statsMsg{gen: 3, stats: core.Stats{Available: true}})
	m.Update(logMsg{gen: 3, entry: core.LogEntry{Text: "stale"}})
	stream := &fakeStream{}
	_, cmd := m.Update(logOpenMsg{gen: 3, stream: stream})
	if cmd == nil {
		t.Fatal("stale stream must close")
	}
	cmd()
	if !stream.closed || m.detail.Resource.Name != "current" || !m.statsPending || m.logs.count != 0 {
		t.Fatal("stale message affected current selection")
	}
}
func TestSelectionChangeCancelsReaders(t *testing.T) {
	m, _, _ := model(t)
	old := m.selectionCtx
	press(m, "j")
	select {
	case <-old.Done():
	default:
		t.Fatal("selection did not cancel request context")
	}
	if m.selectedID != "b" || m.generation == 0 {
		t.Fatal("new selection not established")
	}
	press(m, "q")
	if m.ctx.Err() == nil {
		t.Fatal("quit did not cancel all work")
	}
}
func TestRefreshIsSerializedAndStaleTicksIgnored(t *testing.T) {
	m, b, _ := model(t)
	cmd := m.Init()
	if cmd == nil || m.refresh() != nil {
		t.Fatal("overlapping refresh")
	}
	old := m.pollGeneration
	m.Update(cmd())
	manual := m.refresh()
	m.Update(refreshMsg{generation: old})
	if b.snapshots != 1 || !m.refreshing {
		t.Fatal("tick changed pending fetch")
	}
	m.Update(manual())
	if b.snapshots != 2 {
		t.Fatal("manual fetch failed")
	}
	m.Update(refreshMsg{generation: old})
	if m.refreshing {
		t.Fatal("old tick started new fetch")
	}
}
func TestSelectedIDSurvivesInventoryReorder(t *testing.T) {
	m, _, _ := model(t)
	s := fixture()
	s.Resources[core.Containers][0], s.Resources[core.Containers][1] = s.Resources[core.Containers][1], s.Resources[core.Containers][0]
	m.Update(snapshotMsg{snapshot: s})
	if m.selected != 1 || m.selectedID != strings.Repeat("a", 64) {
		t.Fatal("selection followed position instead of ID")
	}
}
func TestSearchUsesNamesIDsTagsProjects(t *testing.T) {
	m, _, _ := model(t)
	for _, query := range []string{"API", "demo", "aaaaaaaa", "alpine:latest"} {
		m.input = query
		if len(m.globalResults()) == 0 {
			t.Fatalf("query %q has no results", query)
		}
	}
	m.filter = "worker"
	if got := m.resources(); len(got) != 1 || got[0].ID != "b" {
		t.Fatal("per-list filter failed")
	}
}
func TestTextEntryCapturesActionAndQuitKeys(t *testing.T) {
	m, _, e := model(t)
	press(m, "/")
	for _, key := range []string{"q", "a", "l"} {
		press(m, key)
	}
	if m.input != "qal" || m.quitting || len(e.requests) > 0 || m.tab != "Summary" {
		t.Fatal("text field leaked shortcuts")
	}
	press(m, "enter")
	if m.filter != "qal" {
		t.Fatal("filter not committed")
	}
	m.overlay = "input"
	m.inputMode = "filter"
	m.input = ""
	m.Update(tea.PasteMsg{Content: strings.Repeat("x", 1000)})
	if len(m.input) > 256 {
		t.Fatal("unbounded paste")
	}
}
func TestDiskUsageRunsOnDemand(t *testing.T) {
	m, b, _ := model(t)
	press(m, "8")
	if b.disks != 0 {
		t.Fatal("disk measurement ran implicitly")
	}
	cmd := press(m, "d")
	if cmd == nil || press(m, "d") != nil {
		t.Fatal("disk work not serialized")
	}
	m.Update(cmd())
	if b.disks != 1 || !m.diskReady {
		t.Fatal("disk measurement failed")
	}
}
func TestStatsHistoryBounded(t *testing.T) {
	m, _, _ := model(t)
	for n := 0; n < 100; n++ {
		m.Update(statsMsg{gen: m.generation, stats: core.Stats{Available: true, CPUPercent: float64(n)}})
	}
	if len(m.history) != 60 || m.history[0] != 40 {
		t.Fatal("stats history unbounded")
	}
}
func TestConnectionFailurePreservesSnapshotAndProvidesRetry(t *testing.T) {
	m, _, _ := model(t)
	m.Update(snapshotMsg{err: errors.New("permission denied\x1b[2J")})
	if m.connected || len(m.resources()) != 2 || !strings.Contains(m.notice, "r retry") || strings.Contains(m.notice, "\x1b") {
		t.Fatal("failure removed inventory or rendered unsafe error")
	}
}
func TestLogPauseAnchorsHistoryWhileNewEntriesArrive(t *testing.T) {
	m, _, _ := model(t)
	m.tab = "Logs"
	m.logPaused = true
	m.logs.add(core.LogEntry{Text: "first", Stream: "stdout"})
	m.Update(logMsg{gen: m.generation, entry: core.LogEntry{Text: "new", Stream: "stdout"}})
	if m.logOffset != 1 {
		t.Fatal("paused viewport follows new entries")
	}
	press(m, "f")
	if m.logPaused || m.logOffset != 0 {
		t.Fatal("follow did not return to tail")
	}
}
func TestModalKeepsFocus(t *testing.T) {
	m, _, _ := model(t)
	press(m, "a")
	section, selected := m.section, m.selected
	press(m, "j")
	press(m, "2")
	if m.section != section || m.selected != selected || m.menuIndex != 1 {
		t.Fatal("modal leaked navigation")
	}
	press(m, "esc")
	if m.overlay != "" {
		t.Fatal("Esc did not close modal")
	}
}
func TestRenderFitsAllLayouts(t *testing.T) {
	for _, size := range [][2]int{{150, 40}, {120, 30}, {100, 28}, {80, 24}, {60, 18}, {50, 12}, {10, 3}} {
		t.Run(strings.ReplaceAll(time.Duration(size[0]).String(), " ", ""), func(t *testing.T) {
			m, _, _ := model(t)
			m.width, m.height = size[0], size[1]
			for section := 0; section < 8; section++ {
				m.section = section
				for _, overlay := range []string{"", "help", "actions", "confirm", "input", "global", "palette"} {
					m.overlay = overlay
					m.frozen = core.ActionRequest{ID: strings.Repeat("a", 64), Action: core.Remove}
					m.frozenName = "worker"
					m.frozenTarget = "demo (local)"
					view := m.View().Content
					lines := strings.Split(view, "\n")
					if len(lines) > m.height {
						t.Fatalf("section %d overlay %q too tall", section, overlay)
					}
					for _, line := range lines {
						if ansi.StringWidth(line) > m.width {
							t.Fatalf("line too wide: %q", line)
						}
					}
					if strings.Contains(view, "\x1b") {
						t.Fatal("NO_COLOR emitted escapes")
					}
					for _, r := range view {
						if r > 127 {
							t.Fatalf("ASCII emitted %q", r)
						}
					}
				}
			}
		})
	}
}
func TestSmallConfirmationShowsFrozenIDAndCancel(t *testing.T) {
	m, _, _ := model(t)
	m.width, m.height = 60, 18
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	view := m.View().Content
	if !strings.Contains(view, "CANCEL") || !strings.Contains(view, "demo (local)") {
		t.Fatal("confirmation controls or target hidden")
	}
	if strings.Count(view, "a") < 64 {
		t.Fatal("frozen ID is truncated")
	}
}
func TestUntrustedDataCannotInjectTerminalCommands(t *testing.T) {
	m, _, _ := model(t)
	attack := "name\x1b]52;c;secret\a\x1b[2J"
	m.opts.Target = attack
	m.snapshot.Resources[core.Containers][0].Name = attack
	m.detail.Fields = []core.Field{{Name: attack, Value: attack}}
	m.tab = "Inspect"
	m.width, m.height = 150, 40
	view := m.View().Content
	if strings.Contains(view, "\x1b") || strings.Contains(view, "secret") {
		t.Fatal("unsafe terminal content rendered")
	}
}

func TestInspectionRecoveryClearsPriorError(t *testing.T) {
	m, _, _ := model(t)
	m.Update(detailMsg{gen: m.generation, err: errors.New("temporary inspect failure")})
	m.Update(detailMsg{gen: m.generation, detail: core.Detail{Fields: []core.Field{{Name: "State", Value: "running"}}}})
	m.tab = "Inspect"
	if m.detailErr != "" || strings.Contains(strings.Join(m.detailLines(70, 18), "\n"), "temporary inspect failure") {
		t.Fatal("successful inspection retained prior error")
	}
}

func TestNewLogSelectionStartsInLiveFollow(t *testing.T) {
	m, _, _ := model(t)
	m.tab = "Logs"
	m.logPaused = true
	press(m, "j")
	if m.logPaused {
		t.Fatal("new log selection retained a pause anchored to the old stream")
	}
}

func TestConfirmCannotExecuteWhenTooSmall(t *testing.T) {
	m, _, e := model(t)
	press(m, "a")
	m.menuIndex = 3
	m.selected = 1
	press(m, "enter")
	press(m, "tab")
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 12})
	cmd := press(m, "enter")
	if cmd != nil {
		cmd()
	}
	if len(e.requests) != 0 || m.confirm || m.overlay == "confirm" {
		t.Fatal("hidden confirmation remained executable after resize")
	}
	press(m, "q")
	if !m.quitting {
		t.Fatal("undersized terminal must still allow quit")
	}
}

func TestConfirmationCannotExecuteWithClippedScope(t *testing.T) {
	m, _, e := model(t)
	m.width, m.height = 60, 18
	m.opts.Target = strings.Repeat("long-target-", 30)
	press(m, "a")
	m.menuIndex = 1
	press(m, "enter")
	press(m, "tab")
	cmd := press(m, "enter")
	if cmd != nil {
		cmd()
	}
	if len(e.requests) != 0 {
		t.Fatal("confirmation executed while its target was clipped")
	}
}

func TestResumingLogsReturnsToTailBeforePausingAgain(t *testing.T) {
	m, _, _ := model(t)
	m.tab = "Logs"
	m.logPaused = true
	m.logOffset = 8
	press(m, "space")
	press(m, "space")
	if !m.logPaused || m.logOffset != 0 {
		t.Fatal("resuming logs retained old historical offset")
	}
}
func TestFocusedLogDetailUsesAvailableWidth(t *testing.T) {
	m, _, _ := model(t)
	m.width, m.height = 120, 32
	m.tab = "Logs"
	m.detailOpen = true
	m.focus = 2
	m.logs.add(core.LogEntry{Text: "2026-10-13T15:04:05Z request completed", Stream: "stdout"})
	if !strings.Contains(m.View().Content, "request completed") {
		t.Fatal("focused log message clipped despite wide terminal")
	}
}
