// Exodock is an interactive Docker administration console.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/exodialabs/exodock/internal/app"
	"github.com/exodialabs/exodock/internal/config"
	"github.com/exodialabs/exodock/internal/connection"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/demo"
	"github.com/exodialabs/exodock/internal/docker"
	"github.com/exodialabs/exodock/internal/safety"
	"github.com/exodialabs/exodock/internal/tui"
)

var version = "0.1.0-dev"

type options struct {
	demo, readonly, ascii, noColor, version bool
	reducedMotion                           bool
	theme, context, host, configPath        string
	refresh                                 time.Duration
	visited                                 map[string]bool
}

func parseOptions(args []string, out io.Writer) (options, error) {
	o := options{visited: map[string]bool{}}
	fs := flag.NewFlagSet("exodock", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.BoolVar(&o.demo, "demo", false, "explore the sandbox without contacting Docker")
	fs.BoolVar(&o.readonly, "readonly", false, "disable all Docker changes")
	fs.BoolVar(&o.ascii, "ascii", false, "use ASCII symbols and borders")
	fs.BoolVar(&o.noColor, "no-color", false, "disable terminal colors")
	fs.BoolVar(&o.reducedMotion, "reduced-motion", false, "disable startup and activity animations")
	fs.BoolVar(&o.version, "version", false, "print the application version")
	fs.StringVar(&o.theme, "theme", "dark", "terminal theme: dark, light or auto")
	fs.StringVar(&o.context, "context", "", "select a local Docker context")
	fs.StringVar(&o.host, "host", "", "select a local Docker socket (exclusive with --context)")
	fs.StringVar(&o.configPath, "config", "", "optional preferences JSON file")
	fs.DurationVar(&o.refresh, "refresh", 5*time.Second, "inventory refresh interval, between 1s and 60s")
	fs.Usage = func() {
		fmt.Fprintln(out, "Exodock — Docker management, simplified.\n\nUsage: exodock [options]\n\nNavigate with arrows, Tab and Enter. Press ? for contextual help.")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if fs.NArg() != 0 {
		return o, errors.New("unexpected arguments; use exodock --help")
	}
	fs.Visit(func(f *flag.Flag) { o.visited[f.Name] = true })
	return o, nil
}

func (o *options) apply(s config.Settings) error {
	if !o.visited["ascii"] {
		o.ascii = s.ASCII
	}
	if !o.visited["no-color"] {
		o.noColor = s.NoColor
	}
	if !o.visited["reduced-motion"] {
		o.reducedMotion = s.ReducedMotion
	}
	if !o.visited["theme"] {
		o.theme = s.Theme
	}
	if !o.visited["refresh"] {
		o.refresh = time.Duration(s.RefreshSeconds) * time.Second
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		o.noColor = true
	}
	if o.context != "" && o.host != "" {
		return errors.New("choose either --context or --host")
	}
	if o.theme != "dark" && o.theme != "light" && o.theme != "auto" {
		return errors.New("theme must be dark, light or auto")
	}
	if o.refresh < time.Second || o.refresh > 60*time.Second {
		return errors.New("refresh must be between 1s and 60s")
	}
	return nil
}

func run(args []string, out io.Writer) error {
	o, err := parseOptions(args, out)
	if err != nil {
		return err
	}
	if o.version {
		_, err = fmt.Fprintln(out, "Exodock "+version)
		return err
	}
	path := o.configPath
	if path == "" {
		path, err = config.Path()
		if err != nil {
			return errors.New("cannot locate preferences; provide --config")
		}
	}
	settings, err := config.Load(path)
	if err != nil {
		return err
	}
	if err = o.apply(settings); err != nil {
		return err
	}
	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		return errors.New("Exodock needs an interactive terminal; run exodock --demo in a terminal or use --help")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var backend core.Backend
	targetName := "DEMO · isolated sandbox"
	if o.demo {
		backend = demo.New()
	} else {
		target, e := connection.Resolve(ctx, connection.Options{Context: o.context, Host: o.host})
		if e != nil {
			return e
		}
		targetName = target.Name + " · " + safety.Inline(target.Host)
		backend, err = docker.New(target.Host, o.readonly)
		if err != nil {
			return err
		}
	}
	defer backend.Close()
	model := tui.New(ctx, backend, app.New(backend, o.readonly), tui.Options{Target: targetName, ReadOnly: o.readonly, ASCII: o.ascii, NoColor: o.noColor, Theme: o.theme, RefreshInterval: o.refresh, ReducedMotion: o.reducedMotion})
	_, err = tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if ctx.Err() != nil && errors.Is(err, tea.ErrProgramKilled) {
		return nil
	}
	return err
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "Exodock:", safety.Inline(err.Error()))
		os.Exit(1)
	}
}
