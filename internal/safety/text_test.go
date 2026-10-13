package safety

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNeutralizesTerminalSequences(t *testing.T) {
	cases := map[string]string{
		"\x1b[31mred\x1b[0m":                           "red",
		"before\x1b]52;c;secret\aafter":                "beforeafter",
		"\x1b]8;;https://evil\x1b\\link\x1b]8;;\x1b\\": "link",
		"\x1bPpayload\x1b\\safe":                       "safe",
		"a\u009b31mb\u009b0m":                          "ab",
		"a\u202eb\x00\r\x7f\tc\n":                      "ab  c\n",
		"\x1b]unterminated":                            "",
		"étiquette ●":                                  "étiquette ●",
	}
	for input, want := range cases {
		if got := Clean(input); got != want {
			t.Errorf("Clean(%q)=%q want %q", input, got, want)
		}
	}
	if got := Inline("a\nb"); got != "a b" {
		t.Fatal(got)
	}
}

func TestBoundedValidUTF8(t *testing.T) {
	for _, input := range []string{strings.Repeat("é", MaxTextBytes), strings.Repeat("\t", MaxTextBytes), strings.Repeat("x", MaxTextBytes*2)} {
		got := Clean(input)
		if len(got) > MaxTextBytes || !utf8.ValidString(got) {
			t.Fatal("invalid or unbounded output")
		}
	}
}

func FuzzClean(f *testing.F) {
	f.Add("\x1b]52;c;secret\ahello")
	f.Fuzz(func(t *testing.T, input string) {
		got := Clean(input)
		if !utf8.ValidString(got) || strings.ContainsRune(got, 27) || strings.ContainsRune(got, 0x9b) {
			t.Fatalf("unsafe text %q", got)
		}
	})
}
