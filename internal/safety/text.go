// Package safety neutralizes untrusted text before it reaches a terminal.
package safety

import (
	"strings"
	"unicode"
)

const MaxTextBytes = 64 * 1024

// Clean strips ANSI/OSC/DCS sequences, control and bidi formatting characters.
// Newlines are retained for logs. Output and the amount of input scanned are bounded.
func Clean(s string) string {
	if len(s) > MaxTextBytes {
		s = s[:MaxTextBytes]
	}
	var b strings.Builder
	b.Grow(len(s))
	state := byte(0)
	for _, r := range s {
		switch state {
		case 1: // ESC
			switch r {
			case '[':
				state = 2
			case ']', 'P', '^', '_', 'X':
				state = 3
			default:
				state = 0
			}
			continue
		case 2: // CSI, ending at its final byte
			if r >= 0x40 && r <= 0x7e {
				state = 0
			}
			continue
		case 3: // OSC or another terminal control string
			if r == 7 || r == 0x9c {
				state = 0
			} else if r == 27 {
				state = 4
			}
			continue
		case 4:
			if r == '\\' || r == 7 || r == 0x9c {
				state = 0
			} else {
				state = 3
			}
			continue
		}
		switch r {
		case 27:
			state = 1
		case 0x9b:
			state = 2
		case 0x9d, 0x90, 0x98, 0x9e, 0x9f:
			state = 3
		case '\n':
			b.WriteRune(r)
		case '\t':
			b.WriteString("  ")
		default:
			if !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) && r != unicode.ReplacementChar {
				b.WriteRune(r)
			}
		}
		if b.Len() >= MaxTextBytes {
			break
		}
	}
	result := b.String()
	if len(result) > MaxTextBytes {
		result = strings.ToValidUTF8(result[:MaxTextBytes], "")
	}
	return result
}

// Inline additionally prevents untrusted names from introducing table rows.
func Inline(s string) string { return strings.ReplaceAll(Clean(s), "\n", " ") }
