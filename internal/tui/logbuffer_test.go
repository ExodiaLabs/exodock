package tui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/exodialabs/exodock/internal/core"
)

func TestLogBufferLineAndByteBudgets(t *testing.T) {
	var b logBuffer
	for n := 0; n < maxLogLines+50; n++ {
		b.add(core.LogEntry{Text: "entry", Stream: "stdout"})
	}
	if b.count != maxLogLines || b.dropped != 50 || b.bytes > maxLogBytes {
		t.Fatalf("line budget violated: %d lines %d bytes", b.count, b.bytes)
	}
	b = logBuffer{}
	for n := 0; n < 200; n++ {
		b.add(core.LogEntry{Text: strings.Repeat("x", maxLineBytes), Stream: "stderr"})
	}
	if b.bytes > maxLogBytes || b.count > maxLogLines || b.dropped == 0 {
		t.Fatal("byte budget violated")
	}
	for _, line := range b.lines("", "all") {
		if len(line) > maxLineBytes {
			t.Fatal("individual line limit violated")
		}
	}
}
func TestMultilineLogFramesCannotBypassLineBudget(t *testing.T) {
	var b logBuffer
	b.add(core.LogEntry{Text: strings.Repeat("\n", maxLogLines+100)})
	if b.count != maxLogLines || b.dropped == 0 {
		t.Fatal("newline-heavy frame bypassed line budget")
	}
}
func TestLogSearchAndStreamFilter(t *testing.T) {
	var b logBuffer
	b.add(core.LogEntry{Text: "INFO ready\nERROR one", Stream: "stdout"})
	b.add(core.LogEntry{Text: "error two", Stream: "stderr"})
	if got := b.lines("ErRoR", "all"); len(got) != 2 {
		t.Fatal("case-insensitive text filter failed")
	}
	if got := b.lines("error", "stderr"); len(got) != 1 || got[0] != "error two" {
		t.Fatal("stream filter failed")
	}
}
func TestLogTruncationAndControlSanitization(t *testing.T) {
	var b logBuffer
	b.add(core.LogEntry{Text: strings.Repeat("界", 30000) + "\x1b]52;c;secret\a", Stream: "stdout"})
	for _, line := range b.lines("", "all") {
		if len(line) > maxLineBytes || !utf8.ValidString(line) || strings.Contains(line, "\x1b") {
			t.Fatal("invalid or unsafe truncated log")
		}
	}
	if b.dropped == 0 {
		t.Fatal("truncation not reported")
	}
}
