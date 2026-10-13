package tui

import (
	"context"
	"fmt"
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

// Include the cost of styled composition and a large inventory in addition to the
// bounded full log history. Keep the color-disabled case for baseline comparison.
func BenchmarkColorWorkspace(b *testing.B) {
	for _, theme := range []string{"dark", "light"} {
		for _, screen := range []string{"overview", "containers", "logs"} {
			b.Run(theme+"/"+screen, func(b *testing.B) {
				m := New(context.Background(), &fakeBackend{}, &fakeExecutor{}, Options{Theme: theme, Target: "demo (local)"})
				defer m.cancel()
				m.snapshot = fixture()
				m.snapshot.System = core.System{Containers: 1000, Running: 1000}
				m.snapshot.Resources[core.Containers] = nil
				for n := 0; n < 1000; n++ {
					m.snapshot.Resources[core.Containers] = append(m.snapshot.Resources[core.Containers], core.Resource{Kind: core.Containers, ID: fmt.Sprintf("%064x", n), Name: fmt.Sprintf("project-api-%d", n), State: "running", Health: "healthy", Image: "registry.example/project/api:stable", Project: "project"})
				}
				m.connected, m.width, m.height = true, 120, 32
				m.section, m.focus = 1, 1
				if screen == "overview" {
					m.section = 0
				} else if screen == "logs" {
					m.focus, m.tab, m.detailOpen = 2, "Logs", true
				}
				for n := 0; n < maxLogLines; n++ {
					m.logs.add(core.LogEntry{Text: "2026-10-13T15:04:05Z INFO request completed status=200 " + strings.Repeat("payload ", 20), Stream: "stdout"})
				}
				b.ReportAllocs()
				b.ResetTimer()
				for n := 0; n < b.N; n++ {
					_ = m.View()
				}
			})
		}
	}
}
