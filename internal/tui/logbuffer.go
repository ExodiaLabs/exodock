package tui

import (
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
	"strings"
)

const maxLogLines = 10000
const maxLogBytes = 8 * 1024 * 1024
const maxLineBytes = 64 * 1024

type logBuffer struct {
	entries                     [maxLogLines]core.LogEntry
	head, count, bytes, dropped int
}

func (b *logBuffer) add(e core.LogEntry) {
	if len(e.Text) > maxLineBytes {
		e.Text = e.Text[:maxLineBytes]
		b.dropped++
	}
	e.Text = safety.Clean(strings.ToValidUTF8(e.Text, "?"))
	if len(e.Text) > maxLineBytes {
		e.Text = strings.ToValidUTF8(e.Text[:maxLineBytes], "?")
	}
	// Store physical lines so even newline-heavy frames obey the line budget.
	for _, line := range strings.Split(e.Text, "\n") {
		b.addLine(core.LogEntry{Text: line, Stream: e.Stream})
	}
}
func (b *logBuffer) addLine(e core.LogEntry) {
	for b.count > 0 && (b.count == maxLogLines || b.bytes+len(e.Text) > maxLogBytes) {
		b.bytes -= len(b.entries[b.head].Text)
		b.entries[b.head] = core.LogEntry{}
		b.head = (b.head + 1) % maxLogLines
		b.count--
		b.dropped++
	}
	b.entries[(b.head+b.count)%maxLogLines] = e
	b.count++
	b.bytes += len(e.Text)
}
func (b *logBuffer) lines(query, stream string) []string {
	out := make([]string, 0, b.count)
	query = strings.ToLower(query)
	for n := 0; n < b.count; n++ {
		e := b.entries[(b.head+n)%maxLogLines]
		if stream != "all" && e.Stream != stream {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Text), query) {
			continue
		}
		out = append(out, e.Text)
	}
	return out
}
