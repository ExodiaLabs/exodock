package demo

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/exodialabs/exodock/internal/app"
	"github.com/exodialabs/exodock/internal/core"
)

func TestSandboxActionsAndSnapshotIsolation(t *testing.T) {
	b := New()
	ctx := context.Background()
	s, err := b.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.Resources[core.Containers][0].State = "corrupted"
	d, err := b.Inspect(ctx, core.Containers, "demo-api-001")
	if err != nil || d.Resource.State != "running" {
		t.Fatal(d, err)
	}
	service := app.New(b, false)
	if err = service.Execute(ctx, core.ActionRequest{ID: "demo-api-001", Action: core.Remove, Confirmed: true}); err == nil {
		t.Fatal("removed running sandbox container")
	}
	for _, a := range []core.Action{core.Stop, core.Start, core.Restart, core.Stop, core.Remove} {
		if err = service.Execute(ctx, core.ActionRequest{ID: "demo-api-001", Action: a, Confirmed: true}); err != nil {
			t.Fatal(a, err)
		}
	}
	if _, err = b.Inspect(ctx, core.Containers, "demo-api-001"); err == nil {
		t.Fatal("removed container remains")
	}
	s, err = b.Snapshot(ctx)
	if err != nil || len(s.Resources[core.Volumes]) != 2 {
		t.Fatal("sandbox removal deleted volumes")
	}
	d, err = b.Inspect(ctx, core.Compose, "exolan")
	if err != nil || d.Resource.State != "1/2 running" {
		t.Fatal("demo project did not reflect container action", d, err)
	}
}

func TestDemoDiagnosticsAndCancellation(t *testing.T) {
	b := New()
	ctx := context.Background()
	s, err := b.Stats(ctx, "demo-api-001")
	if err != nil || !s.Available || s.MemoryLimit <= 0 {
		t.Fatal(s, err)
	}
	s, err = b.Stats(ctx, "demo-worker-003")
	if err != nil || s.Available {
		t.Fatal(s, err)
	}
	if _, err = b.Stats(ctx, "missing"); err == nil {
		t.Fatal("unknown container stats accepted")
	}
	disk, err := b.DiskUsage(ctx)
	if err != nil || disk.Images <= 0 {
		t.Fatal(disk, err)
	}
	stream, err := b.Logs(ctx, "demo-api-001", core.LogOptions{Stream: "stderr"})
	if err != nil {
		t.Fatal(err)
	}
	entry, err := stream.Next(ctx)
	if err != nil || entry.Stream != "stderr" {
		t.Fatal(entry, err)
	}
	if err = stream.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Next(ctx); !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = b.Snapshot(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = b.DiskUsage(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err = b.ContainerAction(cancelled, "demo-api-001", core.Stop); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = b.Inspect(ctx, core.Compose, "missing"); err == nil {
		t.Fatal("unknown project accepted")
	}
}
