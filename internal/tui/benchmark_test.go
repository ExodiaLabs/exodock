package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/exodialabs/exodock/internal/core"
)

// Measure the render cost with a full retained history, rather than an empty log view.
func BenchmarkLogViewFullHistory(b *testing.B) {
	m := New(context.Background(), &fakeBackend{}, &fakeExecutor{}, Options{NoColor: true, ASCII: true})
	defer m.cancel()
	m.snapshot = fixture()
	m.connected = true
	m.section = 1
	m.focus = 2
	m.tab = "Logs"
	m.detailOpen = true
	m.width = 120
	m.height = 32
	for n := 0; n < maxLogLines; n++ {
		m.logs.add(core.LogEntry{Text: strings.Repeat("x", 200), Stream: "stdout"})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		_ = m.View()
	}
}
