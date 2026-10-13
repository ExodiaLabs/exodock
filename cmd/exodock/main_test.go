package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
	"time"

	"github.com/exodialabs/exodock/internal/config"
)

func TestVersionAndHelpWithoutDocker(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--version"}, &out); err != nil || !strings.Contains(out.String(), version) {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := run([]string{"--help"}, &out); !errors.Is(err, flag.ErrHelp) || !strings.Contains(out.String(), "--") && !strings.Contains(out.String(), "-demo") {
		t.Fatal(out.String(), err)
	}
}

func TestFlagPreferencePrecedence(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	o, err := parseOptions([]string{"--theme", "light", "--ascii=false", "--refresh", "2s"}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	err = o.apply(config.Settings{Version: 1, Theme: "dark", ASCII: true, RefreshSeconds: 9})
	if err != nil || o.theme != "light" || o.ascii || !o.noColor || o.refresh != 2*time.Second {
		t.Fatal(o, err)
	}
}

func TestInvalidOptions(t *testing.T) {
	for _, args := range [][]string{{"--theme", "wrong"}, {"--refresh", "0s"}, {"--refresh", "1h"}, {"--host", "a", "--context", "b"}} {
		o, err := parseOptions(args, &bytes.Buffer{})
		if err != nil {
			t.Fatal(err)
		}
		if err = o.apply(config.Default()); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if _, err := parseOptions([]string{"positional"}, &bytes.Buffer{}); err == nil {
		t.Fatal("positional argument accepted")
	}
}
