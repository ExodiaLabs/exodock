package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/exodialabs/exodock/internal/core"
	"github.com/moby/moby/api/types/container"
	sdk "github.com/moby/moby/client"
)

func fakeClient(t *testing.T, readonly bool, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_ping" {
			w.Header().Set("API-Version", "1.44")
			w.WriteHeader(200)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/v1.44/") {
			t.Errorf("API was not negotiated: %s", r.URL.Path)
		}
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/v1.44")
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	engine, err := sdk.New(sdk.WithHost(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{engine: engine, readonly: readonly}
	t.Cleanup(func() { _ = c.Close() })
	return c
}
func reply(w http.ResponseWriter, value any) { _ = json.NewEncoder(w).Encode(value) }
func inspectFixture(state string) map[string]any {
	return map[string]any{"Id": "id", "Name": "/app\x1b[31m", "State": map[string]any{"Status": state, "Running": state == "running"}, "Config": map[string]any{"Image": "alpine", "Env": []string{"SECRET=secret-marker"}, "Cmd": []string{"secret-marker"}, "Labels": map[string]string{"secret": "secret-marker", projectLabel: "project", serviceLabel: "web"}}}
}

func TestActionsCheckFreshStateAndPreserveVolumes(t *testing.T) {
	for _, action := range []core.Action{core.Start, core.Stop, core.Restart, core.Remove} {
		t.Run(string(action), func(t *testing.T) {
			called := false
			state := "running"
			if action == core.Start || action == core.Remove {
				state = "exited"
			}
			c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && r.URL.Path == "/containers/id/json" {
					reply(w, inspectFixture(state))
					return
				}
				called = true
				if action == core.Remove {
					if r.Method != "DELETE" || r.URL.Path != "/containers/id" || r.URL.Query().Get("force") != "" || r.URL.Query().Get("v") != "" {
						t.Errorf("unsafe removal: %s %s", r.Method, r.URL)
					}
				} else {
					if r.Method != "POST" || r.URL.Path != "/containers/id/"+string(action) {
						t.Errorf("wrong mutation: %s %s", r.Method, r.URL)
					}
					if action != core.Start && r.URL.Query().Get("t") != "10" {
						t.Errorf("missing grace: %s", r.URL)
					}
				}
				w.WriteHeader(http.StatusNoContent)
			})
			if err := c.ContainerAction(context.Background(), "id", action); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("mutation absent")
			}
		})
	}
}
func TestReadOnlyAndActiveRemovalNeverMutate(t *testing.T) {
	calls := 0
	c := fakeClient(t, true, func(w http.ResponseWriter, r *http.Request) { calls++ })
	for _, action := range []core.Action{core.Start, core.Stop, core.Restart, core.Remove} {
		if err := c.ContainerAction(context.Background(), "id", action); err == nil {
			t.Fatal("readonly action accepted")
		}
	}
	if calls != 0 {
		t.Fatal("readonly performed request")
	}
	c = fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatal("active container mutated")
		}
		reply(w, inspectFixture("running"))
	})
	if err := c.ContainerAction(context.Background(), "id", core.Remove); err == nil {
		t.Fatal("running removal accepted")
	}
}
func TestInspectAndErrorsDoNotExposePayloads(t *testing.T) {
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) { reply(w, inspectFixture("exited")) })
	detail, err := c.Inspect(context.Background(), core.Containers, "id")
	if err != nil {
		t.Fatal(err)
	}
	value, _ := json.Marshal(detail)
	if bytes.Contains(value, []byte("secret-marker")) || bytes.Contains(value, []byte("\\u001b")) {
		t.Fatalf("unsafe detail: %s", value)
	}
	c = fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		reply(w, map[string]string{"message": "secret-marker\x1b[2J"})
	})
	_, err = c.Inspect(context.Background(), core.Containers, "id")
	if err == nil || strings.Contains(err.Error(), "secret-marker") {
		t.Fatalf("unsafe error: %v", err)
	}
}
func TestSnapshotPartialFailuresComposeAndSanitization(t *testing.T) {
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/info":
			reply(w, map[string]any{"ServerVersion": "29", "OSType": "linux", "NCPU": 4})
		case "/version":
			reply(w, map[string]any{"ApiVersion": "1.44"})
		case "/containers/json":
			if r.URL.Query().Get("all") != "1" {
				t.Error("stopped containers excluded")
			}
			reply(w, []any{map[string]any{"Id": "id", "Names": []string{"/app\x1b[2J"}, "Image": "alpine", "State": "running", "Status": "Up (unhealthy)", "Labels": map[string]string{projectLabel: "project", serviceLabel: "web"}}})
		case "/images/json":
			w.WriteHeader(500)
			reply(w, map[string]string{"message": "secret-marker"})
		case "/volumes":
			reply(w, map[string]any{"Volumes": []any{map[string]any{"Name": "data", "Driver": "local"}}})
		case "/networks":
			reply(w, []any{map[string]any{"Id": "network", "Name": "bridge", "Driver": "bridge"}})
		default:
			t.Error(r.URL)
			w.WriteHeader(404)
		}
	})
	out, err := c.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Resources[core.Compose]) != 1 || out.Resources[core.Containers][0].Health != "unhealthy" || out.Resources[core.Containers][0].Name != "app" {
		t.Fatalf("bad containers: %+v", out.Resources)
	}
	if len(out.Errors) != 1 || out.Errors[core.Images] == "" || strings.Contains(out.Errors[core.Images], "secret-marker") {
		t.Fatalf("bad partial error: %+v", out.Errors)
	}
	if out.Resources[core.Volumes][0].Size != -1 {
		t.Fatal("unknown volume size must be marked unknown")
	}
}
func TestStatsRequestsPreviousSampleAndComputesDeltas(t *testing.T) {
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("stream") != "false" || r.URL.Query().Get("one-shot") != "" {
			t.Errorf("CPU previous sample missing: %s", r.URL)
		}
		reply(w, map[string]any{"read": "2026-10-13T12:00:01Z", "preread": "2026-10-13T12:00:00Z", "cpu_stats": map[string]any{"cpu_usage": map[string]any{"total_usage": 300}, "system_cpu_usage": 1200, "online_cpus": 4}, "precpu_stats": map[string]any{"cpu_usage": map[string]any{"total_usage": 100}, "system_cpu_usage": 1000}, "memory_stats": map[string]any{"usage": 1000, "limit": 2000, "stats": map[string]any{"inactive_file": 300}}, "networks": map[string]any{"eth0": map[string]any{"rx_bytes": 7, "tx_bytes": 9}}})
	})
	stats, err := c.Stats(context.Background(), "id")
	if err != nil {
		t.Fatal(err)
	}
	if stats.CPUPercent != 400 || stats.MemoryUsage != 700 || stats.NetworkRx != 7 || !stats.Available {
		t.Fatalf("incorrect stats: %+v", stats)
	}
	reset := calculateStats(container.StatsResponse{Read: time.Now(), CPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 1}}, PreCPUStats: container.CPUStats{CPUUsage: container.CPUUsage{TotalUsage: 100}}})
	if reset.CPUPercent != 0 {
		t.Fatal("counter reset underflow")
	}
}
func TestDiskUsageUsesLayerTotal(t *testing.T) {
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		reply(w, map[string]any{"LayersSize": 100, "Images": []any{map[string]any{"Size": 100}, map[string]any{"Size": 100}}, "Containers": []any{map[string]any{"SizeRw": 7}}, "Volumes": []any{map[string]any{"UsageData": map[string]any{"Size": 9}}}, "BuildCache": []any{map[string]any{"Size": 11}}})
	})
	usage, err := c.DiskUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if usage.Images != 100 || usage.Containers != 7 || usage.Volumes != 9 || usage.BuildCache != 11 {
		t.Fatalf("incorrect disk total: %+v", usage)
	}
}
func frame(stream byte, text string) []byte {
	value := make([]byte, 8)
	value[0] = stream
	binary.BigEndian.PutUint32(value[4:], uint32(len(text)))
	return append(value, text...)
}
func TestMultiplexedLogsPreserveSplitLinesAndBoundLongLine(t *testing.T) {
	data := append(frame(1, "split"), frame(2, "error\n")...)
	data = append(data, frame(1, " line\x1b[2J\n"+strings.Repeat("x", maxLogLine+100)+"\nlast")...)
	stream := newLogReader(io.NopCloser(bytes.NewReader(data)), false)
	defer stream.Close()
	expected := []core.LogEntry{{Text: "error", Stream: "stderr"}, {Text: "split line", Stream: "stdout"}, {Text: strings.Repeat("x", maxLogLine) + " [line truncated]", Stream: "stdout"}, {Text: "last", Stream: "stdout"}}
	for _, want := range expected {
		got, err := stream.Next(context.Background())
		if err != nil || got != want {
			t.Fatalf("entry differs: len=%d stream=%s err=%v", len(got.Text), got.Stream, err)
		}
	}
	if _, err := stream.Next(context.Background()); err != io.EOF {
		t.Fatal(err)
	}
}
func TestLogsRejectOversizedFrameAndCancelBlockedRead(t *testing.T) {
	oversized := make([]byte, 8)
	oversized[0] = 1
	binary.BigEndian.PutUint32(oversized[4:], maxLogFrame+1)
	stream := newLogReader(io.NopCloser(bytes.NewReader(oversized)), false)
	if _, err := stream.Next(context.Background()); err == nil {
		t.Fatal("oversized frame accepted")
	}
	reader, writer := io.Pipe()
	defer writer.Close()
	stream = newLogReader(reader, true)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := stream.Next(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked stream not canceled: %v", err)
	}
}
func TestLogOptionsAndTTY(t *testing.T) {
	since := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			v := inspectFixture("running")
			v["Config"].(map[string]any)["Tty"] = true
			reply(w, v)
			return
		}
		q := r.URL.Query()
		if q.Get("tail") != "10000" || q.Get("stdout") != "1" || q.Get("stderr") != "" || q.Get("since") == "" {
			t.Errorf("incorrect log options: %s", r.URL)
		}
		_, _ = io.WriteString(w, "tty line\n")
	})
	stream, err := c.Logs(context.Background(), "id", core.LogOptions{Tail: 10001, Stream: "stdout", Since: since})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	entry, err := stream.Next(context.Background())
	if err != nil || entry.Text != "tty line" {
		t.Fatalf("bad TTY logs: %+v %v", entry, err)
	}
	if _, err = c.Logs(context.Background(), "id", core.LogOptions{Stream: "stderr"}); err == nil {
		t.Fatal("TTY stderr silently accepted")
	}
}

func TestNewKeepsExplicitHostAndRejectsRemote(t *testing.T) {
	t.Setenv("DOCKER_HOST", "tcp://unintended:2375")
	t.Setenv("DOCKER_API_VERSION", "1.10")
	c, err := New("unix:///tmp/exodock-explicit.sock", true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.engine.DaemonHost() != "unix:///tmp/exodock-explicit.sock" {
		t.Fatal("ambient environment replaced resolved target")
	}
	for _, host := range []string{"tcp://localhost:2375", "http://localhost:2375", "ssh://localhost", ""} {
		if c, err := New(host, false); err == nil {
			c.Close()
			t.Fatalf("unsupported transport accepted: %s", host)
		}
	}
}

func TestCanceledRequestDoesNotMutate(t *testing.T) {
	calls := 0
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) { calls++; reply(w, inspectFixture("exited")) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.ContainerAction(ctx, "id", core.Start); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if calls != 0 {
		t.Fatal("canceled action reached daemon")
	}
}

func TestTruncatedLogFrameSignalsFailure(t *testing.T) {
	data := frame(1, "hello\n")
	stream := newLogReader(io.NopCloser(bytes.NewReader(data[:len(data)-3])), false)
	if _, err := stream.Next(context.Background()); err == nil || err == io.EOF {
		t.Fatalf("truncated frame treated as clean EOF: %v", err)
	}
}

func TestContainerHealthUsesStructuredSummaryAndLegacyStarting(t *testing.T) {
	structured := containerResource(container.Summary{Status: "Up", Health: &container.HealthSummary{Status: container.Starting}})
	legacy := containerResource(container.Summary{Status: "Up (health: starting)"})
	if structured.Health != "starting" || legacy.Health != "starting" {
		t.Fatalf("starting health missing: structured=%q legacy=%q", structured.Health, legacy.Health)
	}
}

func TestComposeAggregatesProjectsAndInspectsServices(t *testing.T) {
	fixtures := []container.Summary{
		{ID: "id1", Names: []string{"/web1"}, State: container.StateRunning, Labels: map[string]string{projectLabel: "project", serviceLabel: "web"}},
		{ID: "id2", Names: []string{"/web2"}, State: container.StateExited, Labels: map[string]string{projectLabel: "project", serviceLabel: "web"}},
		{ID: "id3", Names: []string{"/db"}, State: container.StateRunning, Labels: map[string]string{projectLabel: "project", serviceLabel: "db"}},
	}
	projects := composeProjects(fixtures)
	if len(projects) != 1 || projects[0].ID != "project" || projects[0].State != "2/3 running" || projects[0].Service != "db, web" {
		t.Fatalf("incorrect project aggregation: %+v", projects)
	}
	c := fakeClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" || !strings.Contains(r.URL.Query().Get("filters"), projectLabel+"=project") {
			t.Errorf("wrong project filter: %s", r.URL)
		}
		reply(w, fixtures)
	})
	detail, err := c.Inspect(context.Background(), core.Compose, "project")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Resource.ID != "project" || len(detail.Fields) != 7 {
		t.Fatalf("incorrect project details: %+v", detail)
	}
}

func TestLogDaemonRefusalDoesNotBecomeTimeout(t *testing.T) {
	c := fakeClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/json") {
			reply(w, inspectFixture("running"))
			return
		}
		w.WriteHeader(500)
		reply(w, map[string]string{"message": "driver-secret-marker"})
	})
	_, err := c.Logs(context.Background(), "id", core.LogOptions{})
	if err == nil || strings.Contains(err.Error(), "timed out") || strings.Contains(err.Error(), "driver-secret-marker") {
		t.Fatalf("incorrect safe daemon error: %v", err)
	}
}

func TestComposeOpaqueIdentitySurvivesDisplaySanitizationAndLookup(t *testing.T) {
	rawProject := "demo\x1b[31m"
	fixture := container.Summary{ID: "fixture\x1b[2J", Names: []string{"/web"}, State: container.StateRunning, Labels: map[string]string{projectLabel: rawProject, serviceLabel: "web"}}
	projects := composeProjects([]container.Summary{fixture, {ID: "plain", Labels: map[string]string{projectLabel: "demo"}}})
	if len(projects) != 2 {
		t.Fatal("different raw projects were merged")
	}
	rawFound := false
	for _, project := range projects {
		if project.ID == rawProject {
			rawFound = true
			if project.Name != "demo" || project.Project != "demo" {
				t.Fatal("project display metadata retains terminal escape")
			}
		}
	}
	if !rawFound {
		t.Fatal("opaque project identity was rewritten")
	}
	c := fakeClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		var filters sdk.Filters
		if err := json.Unmarshal([]byte(r.URL.Query().Get("filters")), &filters); err != nil {
			t.Fatal(err)
		}
		if !filters["label"][projectLabel+"="+rawProject] {
			t.Fatal("project lookup did not preserve opaque label")
		}
		reply(w, []container.Summary{fixture})
	})
	detail, err := c.Inspect(context.Background(), core.Compose, rawProject)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Resource.ID != rawProject {
		t.Fatal("project detail identity was rewritten")
	}
	for _, field := range detail.Fields {
		if strings.ContainsAny(field.Name+field.Value, "\x1b\n") {
			t.Fatal("terminal controls in rendered project field")
		}
	}
}

func TestContainerInspectReturnsDaemonIdentityRatherThanAlias(t *testing.T) {
	c := fakeClient(t, true, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/alias/json" {
			t.Error("wrong inspect alias path")
		}
		fixture := inspectFixture("exited")
		fixture["Id"] = "actual-container-id"
		reply(w, fixture)
	})
	detail, err := c.Inspect(context.Background(), core.Containers, "alias")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Resource.ID != "actual-container-id" {
		t.Fatalf("inspect echoed request identity: %q", detail.Resource.ID)
	}
}

func TestContainerActionRefusesDaemonIdentityMismatch(t *testing.T) {
	c := fakeClient(t, false, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatal("mismatched identity was mutated")
		}
		fixture := inspectFixture("exited")
		fixture["Id"] = "replacement-id"
		reply(w, fixture)
	})
	if err := c.ContainerAction(context.Background(), "id", core.Remove); err == nil {
		t.Fatal("mismatched identity accepted")
	}
}
