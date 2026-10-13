package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPreferences(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	got, err := Load(p)
	if err != nil || got != Default() {
		t.Fatal(got, err)
	}
	for _, tt := range []struct {
		input string
		valid bool
	}{
		{`{"version":1,"theme":"light","refresh_seconds":3}`, true},
		{`{"theme":"dark"}`, true},
		{`{"theme":"pink"}`, false}, {`{"version":2}`, false}, {`{"refresh_seconds":0}`, false}, {`{"unknown":"SECRET"}`, false}, {`{} {}`, false}, {`null`, false}, {`{}` + strings.Repeat(" ", 64*1024), false},
	} {
		if err = os.WriteFile(p, []byte(tt.input), 0600); err != nil {
			t.Fatal(err)
		}
		_, err = Load(p)
		if (err == nil) != tt.valid {
			t.Errorf("%s: %v", tt.input, err)
		}
	}
}
