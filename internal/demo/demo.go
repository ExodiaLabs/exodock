// Package demo provides a deterministic sandbox that never connects to Docker.
package demo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/exodialabs/exodock/internal/core"
)

type Backend struct {
	mu        sync.Mutex
	resources map[core.Kind][]core.Resource
}

func New() *Backend {
	return &Backend{resources: map[core.Kind][]core.Resource{
		core.Containers: {
			{ID: "demo-api-001", Name: "exolan-api-1", Kind: core.Containers, State: "running", Health: "healthy", Image: "exolan/api:dev", Project: "exolan", Service: "api", Ports: "8080 -> 80"},
			{ID: "demo-db-002", Name: "exolan-db-1", Kind: core.Containers, State: "running", Health: "healthy", Image: "postgres:17-alpine", Project: "exolan", Service: "db", Ports: "5432/tcp"},
			{ID: "demo-worker-003", Name: "exolan-worker-1", Kind: core.Containers, State: "exited", Image: "exolan/api:dev", Project: "exolan", Service: "worker"},
			{ID: "demo-auth-004", Name: "signinforge-auth-1", Kind: core.Containers, State: "running", Health: "unhealthy", Image: "signinforge:dev", Project: "signinforge", Service: "auth", Ports: "8081 -> 80"},
		},
		core.Images:   {{ID: "demo-image-001", Name: "exolan/api:dev", Kind: core.Images, Size: 185 * 1024 * 1024}, {ID: "demo-image-002", Name: "postgres:17-alpine", Kind: core.Images, Size: 95 * 1024 * 1024}},
		core.Volumes:  {{ID: "exolan_db", Name: "exolan_db", Kind: core.Volumes, Driver: "local", Size: -1}, {ID: "signinforge_data", Name: "signinforge_data", Kind: core.Volumes, Driver: "local", Size: -1}},
		core.Networks: {{ID: "demo-net-001", Name: "exolan_default", Kind: core.Networks, Driver: "bridge"}, {ID: "demo-net-002", Name: "signinforge_default", Kind: core.Networks, Driver: "bridge"}},
	}}
}

func (b *Backend) Snapshot(ctx context.Context) (core.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return core.Snapshot{}, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	s := core.Snapshot{Resources: make(map[core.Kind][]core.Resource), CollectedAt: time.Now(), Errors: make(map[core.Kind]string), System: core.System{Version: "demo", APIVersion: "demo", OS: "Linux (sandbox)", Architecture: "amd64", CPUs: 4, Memory: 8 * 1024 * 1024 * 1024}}
	for k, v := range b.resources {
		s.Resources[k] = append([]core.Resource(nil), v...)
	}
	for _, r := range s.Resources[core.Containers] {
		s.System.Containers++
		if r.State == "running" {
			s.System.Running++
		} else {
			s.System.Stopped++
		}
	}
	s.Resources[core.Compose] = projects(s.Resources[core.Containers])
	return s, nil
}

func projects(containers []core.Resource) []core.Resource {
	groups := map[string][]core.Resource{}
	for _, r := range containers {
		groups[r.Project] = append(groups[r.Project], r)
	}
	var out []core.Resource
	for project, rows := range groups {
		if project == "" {
			continue
		}
		running := 0
		var services []string
		for _, r := range rows {
			if r.State == "running" {
				running++
			}
			services = append(services, r.Service)
		}
		sort.Strings(services)
		out = append(out, core.Resource{ID: project, Name: project, Kind: core.Compose, Project: project, State: fmt.Sprintf("%d/%d running", running, len(rows)), Service: strings.Join(services, ", ")})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (b *Backend) Inspect(ctx context.Context, kind core.Kind, id string) (core.Detail, error) {
	if err := ctx.Err(); err != nil {
		return core.Detail{}, err
	}
	if kind == core.Compose {
		s, err := b.Snapshot(ctx)
		if err != nil {
			return core.Detail{}, err
		}
		for _, r := range s.Resources[core.Compose] {
			if r.ID == id {
				d := core.Detail{Resource: r, Fields: []core.Field{{Name: "Project", Value: r.Project}, {Name: "Containers", Value: r.State}, {Name: "Services", Value: r.Service}}}
				for _, c := range s.Resources[core.Containers] {
					if c.Project == id {
						d.Fields = append(d.Fields, core.Field{Name: c.Service, Value: c.Name + ": " + c.State})
					}
				}
				return d, nil
			}
		}
		return core.Detail{}, errors.New("demo project no longer exists")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.resources[kind] {
		if r.ID == id {
			return core.Detail{Resource: r, Fields: []core.Field{{Name: "Name", Value: r.Name}, {Name: "ID", Value: r.ID}, {Name: "State", Value: r.State}, {Name: "Health", Value: r.Health}, {Name: "Image", Value: r.Image}, {Name: "Project", Value: r.Project}, {Name: "Environment", Value: "[hidden]"}}}, nil
		}
	}
	return core.Detail{}, errors.New("demo resource no longer exists")
}
func (b *Backend) Stats(ctx context.Context, id string) (core.Stats, error) {
	d, err := b.Inspect(ctx, core.Containers, id)
	if err != nil {
		return core.Stats{}, err
	}
	if d.Resource.State != "running" {
		return core.Stats{CollectedAt: time.Now()}, nil
	}
	return core.Stats{CPUPercent: 1.2, MemoryUsage: 184 * 1024 * 1024, MemoryLimit: 1024 * 1024 * 1024, NetworkRx: 2400000, NetworkTx: 160000, CollectedAt: time.Now(), Available: true}, nil
}
func (b *Backend) Logs(ctx context.Context, id string, opts core.LogOptions) (core.LogStream, error) {
	if _, err := b.Inspect(ctx, core.Containers, id); err != nil {
		return nil, err
	}
	return &logStream{done: make(chan struct{}), stream: opts.Stream}, nil
}
func (b *Backend) DiskUsage(ctx context.Context) (core.DiskUsage, error) {
	if err := ctx.Err(); err != nil {
		return core.DiskUsage{}, err
	}
	return core.DiskUsage{Images: 280 * 1024 * 1024, Containers: 12 * 1024 * 1024, Volumes: 460 * 1024 * 1024, BuildCache: 64 * 1024 * 1024, CollectedAt: time.Now()}, nil
}
func (b *Backend) ContainerAction(ctx context.Context, id string, action core.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	rows := b.resources[core.Containers]
	for i, r := range rows {
		if r.ID != id {
			continue
		}
		switch action {
		case core.Start, core.Restart:
			rows[i].State = "running"
		case core.Stop:
			rows[i].State = "exited"
		case core.Remove:
			if r.State == "running" {
				return errors.New("stop the demo container before removal")
			}
			b.resources[core.Containers] = append(rows[:i], rows[i+1:]...)
		default:
			return errors.New("invalid demo action")
		}
		return nil
	}
	return errors.New("demo container no longer exists")
}
func (b *Backend) Close() error { return nil }

type logStream struct {
	done   chan struct{}
	once   sync.Once
	count  int
	stream string
}

func (s *logStream) Next(ctx context.Context) (core.LogEntry, error) {
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return core.LogEntry{}, ctx.Err()
	case <-s.done:
		return core.LogEntry{}, io.EOF
	case <-timer.C:
	}
	s.count++
	stream := "stdout"
	text := "request completed status=200 duration=12ms"
	if s.stream == "stderr" || (s.stream != "stdout" && s.count%5 == 0) {
		stream = "stderr"
		text = "upstream retry scheduled after timeout"
	}
	return core.LogEntry{Text: fmt.Sprintf("%s %s", time.Now().UTC().Format(time.RFC3339), text), Stream: stream}, nil
}
func (s *logStream) Close() error { s.once.Do(func() { close(s.done) }); return nil }
