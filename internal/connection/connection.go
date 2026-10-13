// Package connection resolves one Docker target without changing the user's context.
package connection

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/exodialabs/exodock/internal/safety"
)

type Target struct{ Name, Host string }
type Runner interface {
	Run(context.Context, ...string) ([]byte, error)
}
type Options struct {
	Context   string
	Host      string
	LookupEnv func(string) (string, bool)
	Runner    Runner
	OS        string
}

type commandRunner struct{}

func (commandRunner) Run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var out limitedBuffer
	cmd.Stdout = &out
	// Daemon/CLI error payloads may contain credentials. Do not forward stderr.
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1024*1024 {
		return 0, errors.New("Docker context response exceeds limit")
	}
	return b.Buffer.Write(p)
}

func Resolve(ctx context.Context, opts Options) (Target, error) {
	if opts.Context != "" && opts.Host != "" {
		return Target{}, errors.New("choose either --context or --host")
	}
	lookup := opts.LookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	runner := opts.Runner
	if runner == nil {
		runner = commandRunner{}
	}
	osname := opts.OS
	if osname == "" {
		osname = runtime.GOOS
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	contextName := opts.Context
	host := opts.Host
	if contextName == "" && host == "" {
		contextName, _ = lookup("DOCKER_CONTEXT")
		if contextName == "" {
			host, _ = lookup("DOCKER_HOST")
		}
	}
	if host != "" {
		return localTarget("explicit host", host)
	}
	if contextName == "" {
		out, err := runner.Run(ctx, "context", "show")
		if err != nil {
			if errors.Is(err, exec.ErrNotFound) {
				host = "unix:///var/run/docker.sock"
				if osname == "windows" {
					host = "npipe:////./pipe/docker_engine"
				}
				return localTarget("local default (Docker CLI unavailable)", host)
			}
			return Target{}, errors.New("cannot resolve the current Docker context; use --host with a local socket or check the Docker CLI")
		}
		contextName = strings.TrimSpace(string(out))
	}
	if contextName == "" || strings.HasPrefix(contextName, "-") || strings.ContainsAny(contextName, "\x00\r\n") {
		return Target{}, errors.New("invalid Docker context name")
	}
	out, err := runner.Run(ctx, "context", "inspect", "--format", "{{json .Endpoints.docker}}", contextName)
	if err != nil {
		return Target{}, errors.New("cannot inspect the requested Docker context; check its name and Docker CLI availability")
	}
	var endpoint struct{ Host string }
	if err := json.Unmarshal(out, &endpoint); err != nil {
		return Target{}, errors.New("Docker context returned an invalid endpoint")
	}
	return localTarget(contextName, endpoint.Host)
}

func localTarget(name, host string) (Target, error) {
	u, err := url.Parse(host)
	if err != nil || strings.ContainsAny(host, "\x00\r\n") {
		return Target{}, errors.New("invalid Docker socket address")
	}
	switch u.Scheme {
	case "unix":
		if u.Host != "" || !strings.HasPrefix(u.Path, "/") || u.RawQuery != "" || u.Fragment != "" {
			return Target{}, errors.New("a local Docker Unix socket needs an absolute path")
		}
	case "npipe":
		if !strings.HasPrefix(u.Path, "//./pipe/") || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
			return Target{}, errors.New("invalid Docker named pipe")
		}
	default:
		return Target{}, fmt.Errorf("Docker transport %q is not supported in this MVP; choose a local Unix socket or named pipe", safety.Inline(u.Scheme))
	}
	return Target{Name: safety.Inline(name), Host: host}, nil
}
