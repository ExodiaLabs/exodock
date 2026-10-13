// Package tui contains the keyboard interface. Docker work runs only in commands.
package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
)

type Options struct {
	Target                   string
	ReadOnly, ASCII, NoColor bool
	Theme                    string
	RefreshInterval          time.Duration
}

var sections = []string{"Overview", "Containers", "Images", "Volumes", "Networks", "Compose", "Resources", "System"}
var actions = []core.Action{core.Start, core.Stop, core.Restart, core.Remove}

type Model struct {
	ctx                                     context.Context
	cancel                                  context.CancelFunc
	backend                                 core.Backend
	executor                                core.Executor
	opts                                    Options
	width, height, section, focus, selected int
	selectedID                              string
	snapshot                                core.Snapshot
	refreshing                              bool
	connected                               bool
	notice                                  string
	detail                                  core.Detail
	detailErr                               string
	detailPending                           bool
	lightTheme                              bool
	stats                                   core.Stats
	history                                 []float64
	statsPending                            bool
	generation                              uint64
	selectionCtx                            context.Context
	selectionCancel                         context.CancelFunc
	tab                                     string
	detailOpen                              bool
	detailOffset                            int
	filter, logQuery                        string
	overlay, input, inputMode               string
	menuIndex                               int
	confirm                                 bool
	frozen                                  core.ActionRequest
	frozenName, frozenTarget                string
	actionPending                           bool
	stream                                  core.LogStream
	logs                                    logBuffer
	logErr                                  string
	logPaused                               bool
	logOffset                               int
	logStream                               string
	sinceMinutes                            int
	disk                                    core.DiskUsage
	diskPending                             bool
	diskReady                               bool
	pollGeneration                          uint64
	quitting                                bool
}

type snapshotMsg struct {
	snapshot core.Snapshot
	err      error
}
type refreshMsg struct{ generation uint64 }
type detailMsg struct {
	gen    uint64
	detail core.Detail
	err    error
}
type statsMsg struct {
	gen   uint64
	stats core.Stats
	err   error
}
type logOpenMsg struct {
	gen    uint64
	stream core.LogStream
	err    error
}
type logMsg struct {
	gen   uint64
	entry core.LogEntry
	err   error
}
type actionMsg struct {
	request core.ActionRequest
	err     error
}
type diskMsg struct {
	disk core.DiskUsage
	err  error
}

func New(ctx context.Context, b core.Backend, e core.Executor, o Options) *Model {
	ctx, cancel := context.WithCancel(ctx)
	if o.RefreshInterval <= 0 {
		o.RefreshInterval = 5 * time.Second
	}
	m := &Model{ctx: ctx, cancel: cancel, backend: b, executor: e, opts: o, width: 100, height: 28, tab: "Summary", logStream: "all", notice: "Connecting to Docker…"}
	m.selectionCtx, m.selectionCancel = context.WithCancel(ctx)
	return m
}
func (m *Model) Init() tea.Cmd {
	cmd := m.refresh()
	if (m.opts.Theme == "auto" || m.opts.Theme == "") && !m.opts.NoColor {
		return tea.Batch(cmd, tea.RequestBackgroundColor)
	}
	return cmd
}
func (m *Model) refresh() tea.Cmd {
	if m.refreshing || m.quitting {
		return nil
	}
	m.refreshing = true
	m.pollGeneration++
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 12*time.Second)
		defer cancel()
		s, e := m.backend.Snapshot(ctx)
		return snapshotMsg{s, e}
	}
}
func (m *Model) poll() tea.Cmd {
	gen := m.pollGeneration
	return tea.Tick(m.opts.RefreshInterval, func(time.Time) tea.Msg { return refreshMsg{gen} })
}
func (m *Model) resources() []core.Resource {
	kind := m.kind()
	list := m.snapshot.Resources[kind]
	out := make([]core.Resource, 0, len(list))
	for _, r := range list {
		if matches(r, m.filter) {
			out = append(out, r)
		}
	}
	return out
}
func matches(r core.Resource, q string) bool {
	return strings.Contains(strings.ToLower(r.Name+" "+r.ID+" "+r.Image+" "+r.State+" "+r.Health+" "+r.Project+" "+r.Service), strings.ToLower(q))
}
func (m *Model) kind() core.Kind {
	switch m.section {
	case 2:
		return core.Images
	case 3:
		return core.Volumes
	case 4:
		return core.Networks
	case 5:
		return core.Compose
	default:
		return core.Containers
	}
}
func (m *Model) resource() (core.Resource, bool) {
	list := m.resources()
	if m.selected < 0 || m.selected >= len(list) {
		return core.Resource{}, false
	}
	return list[m.selected], true
}
func closeStream(s core.LogStream) tea.Cmd {
	if s == nil {
		return nil
	}
	return func() tea.Msg { _ = s.Close(); return nil }
}
func (m *Model) resetSelection() tea.Cmd {
	m.selectionCancel()
	closeCmd := closeStream(m.stream)
	m.stream = nil
	m.selectionCtx, m.selectionCancel = context.WithCancel(m.ctx)
	m.generation++
	m.detail = core.Detail{}
	m.detailErr = ""
	m.detailPending = false
	m.detailOffset = 0
	m.stats = core.Stats{}
	m.history = nil
	m.statsPending = false
	m.logs = logBuffer{}
	m.logErr = ""
	m.logOffset = 0
	m.logPaused = false
	r, ok := m.resource()
	if !ok {
		m.selectedID = ""
		return closeCmd
	}
	m.selectedID = r.ID
	cmds := []tea.Cmd{closeCmd, m.fetchDetail()}
	if r.Kind == core.Containers && (m.tab == "Stats" || m.section == 6) {
		cmds = append(cmds, m.fetchStats())
	}
	if r.Kind == core.Containers && m.tab == "Logs" {
		cmds = append(cmds, m.openLogs())
	}
	return tea.Batch(cmds...)
}
func (m *Model) fetchDetail() tea.Cmd {
	r, ok := m.resource()
	if !ok || m.detailPending {
		return nil
	}
	m.detailPending = true
	ctx, gen := m.selectionCtx, m.generation
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		d, e := m.backend.Inspect(c, r.Kind, r.ID)
		return detailMsg{gen, d, e}
	}
}
func (m *Model) fetchStats() tea.Cmd {
	r, ok := m.resource()
	if !ok || r.Kind != core.Containers || m.statsPending {
		return nil
	}
	m.statsPending = true
	ctx, gen := m.selectionCtx, m.generation
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		s, e := m.backend.Stats(c, r.ID)
		return statsMsg{gen, s, e}
	}
}
func (m *Model) openLogs() tea.Cmd {
	r, ok := m.resource()
	if !ok || r.Kind != core.Containers {
		return nil
	}
	ctx, gen := m.selectionCtx, m.generation
	o := core.LogOptions{Tail: 1000, Stream: m.logStream}
	if m.sinceMinutes > 0 {
		o.Since = time.Now().Add(-time.Duration(m.sinceMinutes) * time.Minute)
	}
	return func() tea.Msg { s, e := m.backend.Logs(ctx, r.ID, o); return logOpenMsg{gen, s, e} }
}
func (m *Model) nextLog() tea.Cmd {
	ctx, gen, s := m.selectionCtx, m.generation, m.stream
	if s == nil {
		return nil
	}
	return func() tea.Msg { e, err := s.Next(ctx); return logMsg{gen, e, err} }
}
func (m *Model) loadDisk() tea.Cmd {
	if m.diskPending {
		return nil
	}
	m.diskPending = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(m.ctx, 20*time.Second)
		defer cancel()
		d, e := m.backend.DiskUsage(ctx)
		return diskMsg{d, e}
	}
}
func (m *Model) switchSection(n int) tea.Cmd {
	m.section = (n + len(sections)) % len(sections)
	m.focus = 1
	m.selected = 0
	m.filter = ""
	m.detailOpen = false
	m.tab = "Summary"
	if m.section == 6 {
		m.tab = "Stats"
		m.detailOpen = true
		m.focus = 2
	}
	return m.resetSelection()
}
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if m.opts.Theme == "auto" || m.opts.Theme == "" {
			m.lightTheme = !msg.IsDark()
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if (m.width < 60 || m.height < 18) && m.overlay == "confirm" {
			m.overlay = ""
			m.confirm = false
			m.notice = "Confirmation canceled: terminal too small"
		}
	case refreshMsg:
		if msg.generation != m.pollGeneration {
			return m, nil
		}
		return m, m.refresh()
	case snapshotMsg:
		m.refreshing = false
		if msg.err != nil {
			m.connected = false
			m.notice = "Docker unavailable: " + safety.Clean(msg.err.Error()) + " · r retry; verify the daemon and target"
			return m, m.poll()
		}
		m.connected = true
		m.snapshot = msg.snapshot
		if strings.HasPrefix(m.notice, "Connecting") || strings.HasPrefix(m.notice, "Inventory updated") || strings.HasPrefix(m.notice, "Docker unavailable") {
			m.notice = "Inventory updated " + msg.snapshot.CollectedAt.Format("15:04:05")
		}
		list := m.resources()
		m.selected = 0
		for i, r := range list {
			if r.ID == m.selectedID {
				m.selected = i
				break
			}
		}
		r, ok := m.resource()
		var cmd tea.Cmd
		if (ok && r.ID != m.selectedID) || (!ok && m.selectedID != "") {
			cmd = m.resetSelection()
		} else if m.tab == "Stats" || m.section == 6 {
			cmd = m.fetchStats()
		}
		return m, tea.Batch(m.poll(), cmd, m.fetchDetail())
	case detailMsg:
		if msg.gen != m.generation {
			return m, nil
		}
		m.detailPending = false
		if msg.err != nil {
			m.detailErr = safety.Clean(msg.err.Error())
		} else {
			m.detailErr = ""
			m.detail = msg.detail
		}
	case statsMsg:
		if msg.gen != m.generation {
			return m, nil
		}
		m.statsPending = false
		if msg.err != nil {
			m.notice = "Stats unavailable: " + safety.Clean(msg.err.Error()) + " · r retry"
		} else {
			m.stats = msg.stats
			if msg.stats.Available {
				m.history = append(m.history, msg.stats.CPUPercent)
				if len(m.history) > 60 {
					m.history = m.history[len(m.history)-60:]
				}
			}
		}
	case logOpenMsg:
		if msg.gen != m.generation {
			return m, closeStream(msg.stream)
		}
		if msg.err != nil {
			m.logErr = safety.Clean(msg.err.Error()) + " · l retry"
		} else {
			m.stream = msg.stream
			return m, m.nextLog()
		}
	case logMsg:
		if msg.gen != m.generation {
			return m, nil
		}
		if msg.err != nil {
			if msg.err == io.EOF {
				m.logErr = "Log stream ended · l reconnect"
			} else if m.selectionCtx.Err() == nil {
				m.logErr = safety.Clean(msg.err.Error()) + " · l reconnect"
			}
			s := m.stream
			m.stream = nil
			return m, closeStream(s)
		}
		if m.logPaused {
			entry := msg.entry
			if len(entry.Text) > maxLineBytes {
				entry.Text = entry.Text[:maxLineBytes]
			}
			for _, line := range strings.Split(safety.Clean(entry.Text), "\n") {
				if (m.logStream == "all" || entry.Stream == m.logStream) && strings.Contains(strings.ToLower(line), strings.ToLower(m.logQuery)) {
					m.logOffset = min(maxLogLines, m.logOffset+1)
				}
			}
		}
		m.logs.add(msg.entry)
		return m, m.nextLog()
	case actionMsg:
		m.actionPending = false
		if msg.err != nil {
			m.notice = string(msg.request.Action) + " failed: " + safety.Clean(msg.err.Error()) + " · verify the state, then r refresh"
		} else {
			m.notice = string(msg.request.Action) + " completed · " + safety.Clean(msg.request.ID)
		}
		return m, m.refresh()
	case diskMsg:
		m.diskPending = false
		if msg.err != nil {
			m.notice = "Disk usage unavailable: " + safety.Clean(msg.err.Error()) + " · d retry"
		} else {
			m.disk = msg.disk
			m.diskReady = true
		}
	case tea.PasteMsg:
		if m.overlay == "input" {
			m.input = boundedInput(m.input + msg.String())
		}
	case tea.KeyPressMsg:
		return m, m.key(msg)
	}
	return m, nil
}
func boundedInput(s string) string {
	r := []rune(safety.Clean(s))
	if len(r) > 256 {
		r = r[:256]
	}
	return string(r)
}
func (m *Model) key(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		m.quitting = true
		m.cancel()
		m.selectionCancel()
		return tea.Batch(closeStream(m.stream), tea.Quit)
	}
	if m.width < 60 || m.height < 18 {
		if key == "q" {
			m.quitting = true
			m.cancel()
			m.selectionCancel()
			return tea.Batch(closeStream(m.stream), tea.Quit)
		}
		return nil
	}
	if m.overlay == "input" {
		switch key {
		case "esc":
			m.overlay = ""
			m.input = ""
		case "enter":
			m.overlay = ""
			switch m.inputMode {
			case "filter":
				m.filter = m.input
				m.selected = 0
				return m.resetSelection()
			case "logs":
				m.logQuery = m.input
			case "global":
				m.overlay = "global"
				m.menuIndex = 0
			case "palette":
				m.overlay = "palette"
				m.menuIndex = 0
			}
		case "backspace":
			r := []rune(m.input)
			if len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		default:
			if msg.Text != "" {
				m.input = boundedInput(m.input + msg.Text)
			}
		}
		return nil
	}
	if m.overlay != "" {
		return m.overlayKey(key)
	}
	switch key {
	case "q":
		m.quitting = true
		m.cancel()
		m.selectionCancel()
		return tea.Batch(closeStream(m.stream), tea.Quit)
	case "?":
		m.overlay = "help"
		m.menuIndex = 0
	case "ctrl+k":
		m.overlay = "input"
		m.inputMode = "global"
		m.input = ""
	case ":":
		m.overlay = "input"
		m.inputMode = "palette"
		m.input = ""
	case "/":
		m.overlay = "input"
		m.inputMode = "filter"
		m.input = m.filter
		if m.tab == "Logs" {
			m.inputMode = "logs"
			m.input = m.logQuery
		}
	case "tab":
		m.focus = (m.focus + 1) % 3
	case "shift+tab":
		m.focus = (m.focus + 2) % 3
	case "[":
		return m.switchSection(m.section - 1)
	case "]":
		return m.switchSection(m.section + 1)
	case "1", "2", "3", "4", "5", "6", "7", "8":
		return m.switchSection(int(key[0] - '1'))
	case "up", "k", "down", "j":
		delta := 1
		if key == "up" || key == "k" {
			delta = -1
		}
		if m.focus == 0 {
			cmd := m.switchSection(m.section + delta)
			m.focus = 0
			return cmd
		}
		if m.tab == "Logs" && m.detailOpen && m.focus == 2 {
			m.logPaused = true
			m.logOffset = max(0, m.logOffset-delta)
			return nil
		}
		if m.tab == "Inspect" && m.detailOpen && m.focus == 2 {
			m.detailOffset = max(0, min(10000, m.detailOffset+delta))
			return nil
		}
		list := m.resources()
		next := max(0, min(len(list)-1, m.selected+delta))
		if next != m.selected {
			m.selected = next
			return m.resetSelection()
		}
	case "enter":
		if m.section == 0 {
			return m.switchSection(1)
		}
		m.detailOpen = true
		m.focus = 2
	case "esc":
		if m.detailOpen {
			m.detailOpen = false
			m.tab = "Summary"
			m.focus = 1
			return m.resetSelection()
		}
		if m.filter != "" {
			m.filter = ""
			m.selected = 0
			return m.resetSelection()
		}
	case "r":
		return m.refresh()
	case "d":
		if m.section == 7 {
			return m.loadDisk()
		}
	case "a":
		m.overlay = "actions"
		m.menuIndex = 0
	case "l":
		r, ok := m.resource()
		if ok && r.Kind == core.Containers {
			m.tab = "Logs"
			m.detailOpen = true
			m.focus = 2
			return m.resetSelection()
		}
		m.notice = "Logs are available for containers; open Containers (2)"
	case "i":
		m.tab = "Inspect"
		m.detailOpen = true
		m.focus = 2
		return m.resetSelection()
	case "s":
		r, ok := m.resource()
		if ok && r.Kind == core.Containers {
			m.tab = "Stats"
			m.detailOpen = true
			m.focus = 2
			return m.resetSelection()
		}
	case "space":
		if m.tab == "Logs" {
			m.logPaused = !m.logPaused
			if !m.logPaused {
				m.logOffset = 0
			}
		}
	case "f":
		if m.tab == "Logs" {
			m.logPaused = false
			m.logOffset = 0
		}
	case "v":
		if m.tab == "Logs" {
			switch m.logStream {
			case "all":
				m.logStream = "stdout"
			case "stdout":
				m.logStream = "stderr"
			default:
				m.logStream = "all"
			}
			return m.resetSelection()
		}
	case "t":
		if m.tab == "Logs" {
			switch m.sinceMinutes {
			case 0:
				m.sinceMinutes = 15
			case 15:
				m.sinceMinutes = 60
			default:
				m.sinceMinutes = 0
			}
			return m.resetSelection()
		}
	}
	return nil
}
func (m *Model) actionReason(a core.Action) string {
	r, ok := m.resource()
	if !ok {
		return "Select a container first"
	}
	if r.Kind != core.Containers {
		return "Container actions only in this release"
	}
	if m.opts.ReadOnly {
		return "Read-only mode"
	}
	if m.actionPending {
		return "An action is already running"
	}
	if m.executor == nil {
		return "Actions unavailable"
	}
	running := r.State == "running" || r.State == "restarting" || r.State == "paused"
	if a == core.Start && running {
		return "Container is already active"
	}
	if (a == core.Stop || a == core.Restart) && !running {
		return "Container is stopped"
	}
	if a == core.Remove && running {
		return "Stop the container before removing it"
	}
	return ""
}
func (m *Model) overlayKey(key string) tea.Cmd {
	if key == "esc" || key == "q" {
		m.overlay = ""
		m.confirm = false
		return nil
	}
	if m.overlay == "help" {
		if key == "down" || key == "j" {
			m.menuIndex++
		}
		if key == "up" || key == "k" {
			m.menuIndex = max(0, m.menuIndex-1)
		}
		return nil
	}
	if m.overlay == "confirm" {
		if key == "tab" || key == "shift+tab" || key == "left" || key == "right" {
			m.confirm = !m.confirm
			return nil
		}
		if key == "enter" {
			if !m.confirm {
				m.overlay = ""
				return nil
			}
			if m.opts.ReadOnly || m.actionPending || m.executor == nil {
				m.overlay = ""
				m.notice = "Action unavailable in read-only mode or while busy"
				return nil
			}
			if len(m.confirmationLines(m.width)) > m.height-9 {
				m.notice = "Resize terminal to show the complete confirmation; Esc cancels"
				return nil
			}
			req := m.frozen
			req.Confirmed = true
			m.overlay = ""
			m.actionPending = true
			m.notice = string(req.Action) + " in progress…"
			ctx := m.ctx
			return func() tea.Msg {
				c, cancel := context.WithTimeout(ctx, 45*time.Second)
				defer cancel()
				return actionMsg{req, m.executor.Execute(c, req)}
			}
		}
		return nil
	}
	count := 0
	switch m.overlay {
	case "actions":
		count = len(actions)
	case "global":
		count = len(m.globalResults())
	case "palette":
		count = len(m.paletteResults())
	}
	switch key {
	case "up", "k":
		m.menuIndex = max(0, m.menuIndex-1)
	case "down", "j":
		m.menuIndex = min(max(0, count-1), m.menuIndex+1)
	case "/":
		m.inputMode = m.overlay
		m.overlay = "input"
	case "enter":
		if count == 0 {
			return nil
		}
		m.menuIndex = min(m.menuIndex, count-1)
		switch m.overlay {
		case "actions":
			a := actions[m.menuIndex]
			if reason := m.actionReason(a); reason != "" {
				m.notice = reason
				return nil
			}
			r, _ := m.resource()
			m.frozen = core.ActionRequest{ID: r.ID, Action: a}
			m.frozenName = r.Name
			m.frozenTarget = m.opts.Target
			m.confirm = false
			m.overlay = "confirm"
		case "global":
			r := m.globalResults()[m.menuIndex]
			m.overlay = ""
			for i, k := range []core.Kind{core.Containers, core.Images, core.Volumes, core.Networks, core.Compose} {
				if k == r.Kind {
					m.section = i + 1
					break
				}
			}
			m.filter = ""
			for i, item := range m.resources() {
				if item.ID == r.ID {
					m.selected = i
					break
				}
			}
			m.detailOpen = true
			m.focus = 2
			m.tab = "Summary"
			return m.resetSelection()
		case "palette":
			entry := m.paletteResults()[m.menuIndex]
			m.overlay = ""
			if entry.section >= 0 {
				return m.switchSection(entry.section)
			}
			return m.key(tea.KeyPressMsg{Code: rune(entry.key[0]), Text: entry.key})
		}
	}
	return nil
}
func (m *Model) globalResults() []core.Resource {
	var out []core.Resource
	for _, kind := range []core.Kind{core.Containers, core.Images, core.Volumes, core.Networks, core.Compose} {
		for _, r := range m.snapshot.Resources[kind] {
			if matches(r, m.input) {
				out = append(out, r)
			}
		}
	}
	return out
}

type paletteEntry struct {
	name, key string
	section   int
}

func (m *Model) paletteResults() []paletteEntry {
	var out []paletteEntry
	for i, s := range sections {
		if strings.Contains(strings.ToLower(s), strings.ToLower(m.input)) {
			out = append(out, paletteEntry{"Go to " + s, "", i})
		}
	}
	for _, e := range []paletteEntry{{"Refresh inventory", "r", -1}, {"Container actions", "a", -1}, {"View logs", "l", -1}, {"View stats", "s", -1}, {"Inspect selection", "i", -1}, {"Help", "?", -1}} {
		if strings.Contains(strings.ToLower(e.name), strings.ToLower(m.input)) {
			out = append(out, e)
		}
	}
	return out
}
func bytes(v int64) string {
	if v < 0 {
		return "unavailable"
	}
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}
	d, e := int64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		d *= unit
		e++
	}
	return fmt.Sprintf("%.1f %ciB", float64(v)/float64(d), "KMGTPE"[e])
}
