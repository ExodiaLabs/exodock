package connection

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls         [][]string
	current, host string
	err           error
}

func (r *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	r.calls = append(r.calls, args)
	if r.err != nil {
		return nil, r.err
	}
	if args[1] == "show" {
		return []byte(r.current), nil
	}
	return []byte(`{"Host":"` + r.host + `"}`), nil
}
func env(values map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := values[k]; return v, ok }
}

func TestPrecedence(t *testing.T) {
	for _, tt := range []struct {
		name               string
		opts               Options
		wantName, wantHost string
		calls              int
	}{
		{"explicit host", Options{Host: "unix:///explicit", LookupEnv: env(map[string]string{"DOCKER_CONTEXT": "production"})}, "explicit host", "unix:///explicit", 0},
		{"explicit context", Options{Context: "desktop", LookupEnv: env(map[string]string{"DOCKER_HOST": "unix:///ignored"})}, "desktop", "unix:///context", 1},
		{"env context wins", Options{LookupEnv: env(map[string]string{"DOCKER_CONTEXT": "local", "DOCKER_HOST": "unix:///ignored"})}, "local", "unix:///context", 1},
		{"env host", Options{LookupEnv: env(map[string]string{"DOCKER_HOST": "unix:///env"})}, "explicit host", "unix:///env", 0},
		{"current", Options{LookupEnv: env(nil)}, "current", "unix:///context", 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeRunner{current: "current", host: "unix:///context"}
			tt.opts.Runner = r
			got, err := Resolve(context.Background(), tt.opts)
			if err != nil || got.Name != tt.wantName || got.Host != tt.wantHost || len(r.calls) != tt.calls {
				t.Fatalf("%+v %v calls %v", got, err, r.calls)
			}
		})
	}
}

func TestNeverFallsBackFromInvalidContext(t *testing.T) {
	_, err := Resolve(context.Background(), Options{Context: "missing", Runner: &fakeRunner{err: errors.New("SECRET")}})
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Fatal(err)
	}
}

func TestMissingCLIFallsBackOnlyForDefault(t *testing.T) {
	r := &fakeRunner{err: exec.ErrNotFound}
	got, err := Resolve(context.Background(), Options{Runner: r, LookupEnv: env(nil), OS: "linux"})
	if err != nil || got.Host != "unix:///var/run/docker.sock" {
		t.Fatal(got, err)
	}
	if _, err = Resolve(context.Background(), Options{Context: "desktop", Runner: r}); err == nil {
		t.Fatal("explicit context silently fell back")
	}
}

func TestValidateTarget(t *testing.T) {
	for _, host := range []string{"tcp://localhost:2375", "ssh://host", "unix://relative", "unix:///tmp/socket?secret=x", "npipe:///wrong"} {
		if _, err := localTarget("test", host); err == nil {
			t.Errorf("accepted %s", host)
		}
	}
	if _, err := localTarget("windows", "npipe:////./pipe/docker_engine"); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(context.Background(), Options{Context: "a", Host: "b"}); err == nil {
		t.Fatal("ambiguous options accepted")
	}
}
