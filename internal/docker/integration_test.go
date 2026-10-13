package docker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/exodialabs/exodock/internal/core"
	"github.com/moby/moby/api/types/container"
	sdk "github.com/moby/moby/client"
)

// This opt-in test mutates only IDs returned by its own fixture creation.
// It never prunes, searches for old fixtures, or touches user containers.
func TestEngineIntegrationOwnContainerLifecycle(t *testing.T) {
	if os.Getenv("EXODOCK_INTEGRATION") != "1" {
		t.Skip("set EXODOCK_INTEGRATION=1 for the isolated Docker fixture test")
	}
	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adapter, err := New(host, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = adapter.Close() })
	image := os.Getenv("EXODOCK_TEST_IMAGE")
	if image == "" {
		image = "alpine:latest"
	}
	if _, err = adapter.engine.ImageInspect(ctx, image); err != nil {
		t.Fatalf("integration image unavailable; pull %s before running the test", image)
	}
	name := fmt.Sprintf("exodock-test-%d", time.Now().UnixNano())
	created, err := adapter.engine.ContainerCreate(ctx, sdk.ContainerCreateOptions{Name: name, Config: &container.Config{
		Image: image, Cmd: []string{"sh", "-c", "echo exodock-fixture; echo fixture-stderr >&2; trap 'exit 0' TERM; while :; do sleep 1; done"},
		Env: []string{"EXODOCK_SECRET=inspect-secret-marker"}, Labels: map[string]string{"dev.exodialabs.exodock.test": name, projectLabel: name, serviceLabel: "fixture"}, Volumes: map[string]struct{}{"/fixture-data": {}},
	}})
	if err != nil {
		t.Fatal(safeError(err))
	}
	id := created.ID
	volumes := []string{}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_, removeErr := adapter.engine.ContainerRemove(cleanupCtx, id, sdk.ContainerRemoveOptions{Force: true, RemoveVolumes: false})
		// The adapter might already have removed the fixture successfully.
		if removeErr != nil {
			if _, inspectErr := adapter.engine.ContainerInspect(cleanupCtx, id, sdk.ContainerInspectOptions{}); inspectErr == nil {
				t.Error("could not clean the owned integration container")
			}
		}
		for _, volume := range volumes {
			if _, err := adapter.engine.VolumeRemove(cleanupCtx, volume, sdk.VolumeRemoveOptions{}); err != nil {
				t.Error("could not clean the owned fixture volume")
			}
		}
	})
	fixture, err := adapter.engine.ContainerInspect(ctx, id, sdk.ContainerInspectOptions{})
	if err != nil {
		t.Fatal(safeError(err))
	}
	for _, mount := range fixture.Container.Mounts {
		if mount.Type == "volume" && mount.Destination == "/fixture-data" {
			volumes = append(volumes, mount.Name)
		}
	}
	if len(volumes) != 1 {
		t.Fatal("expected one owned anonymous fixture volume")
	}
	readonly, err := New(host, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = readonly.Close() })
	if err = readonly.ContainerAction(ctx, id, core.Start); err == nil {
		t.Fatal("readonly accepted a mutation")
	}
	if err = adapter.ContainerAction(ctx, id, core.Start); err != nil {
		t.Fatal(err)
	}
	snapshot, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, resource := range snapshot.Resources[core.Compose] {
		if resource.ID == name && resource.Project == name {
			found = true
		}
	}
	if !found {
		t.Fatal("owned Compose fixture absent from snapshot")
	}
	detail, err := adapter.Inspect(ctx, core.Containers, id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fmt.Sprintf("%+v", detail), "inspect-secret-marker") {
		t.Fatal("inspect exposed environment")
	}
	if err = adapter.ContainerAction(ctx, id, core.Remove); err == nil {
		t.Fatal("running removal accepted")
	}
	stats, err := adapter.Stats(ctx, id)
	if err != nil || !stats.Available {
		t.Fatalf("live stats unavailable: %v", err)
	}
	logCtx, logCancel := context.WithTimeout(ctx, 5*time.Second)
	stream, err := adapter.Logs(logCtx, id, core.LogOptions{Tail: 10})
	if err != nil {
		logCancel()
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for len(seen) < 2 {
		entry, err := stream.Next(logCtx)
		if err != nil {
			_ = stream.Close()
			logCancel()
			t.Fatal(err)
		}
		if entry.Text == "exodock-fixture" || entry.Text == "fixture-stderr" {
			seen[entry.Stream] = true
		}
	}
	_ = stream.Close()
	logCancel()
	if err = adapter.ContainerAction(ctx, id, core.Restart); err != nil {
		t.Fatal(err)
	}
	if err = adapter.ContainerAction(ctx, id, core.Stop); err != nil {
		t.Fatal(err)
	}
	if err = adapter.ContainerAction(ctx, id, core.Remove); err != nil {
		t.Fatal(err)
	}
	for _, volume := range volumes {
		if _, err = adapter.engine.VolumeInspect(ctx, volume, sdk.VolumeInspectOptions{}); err != nil {
			t.Fatal("container removal deleted its volume")
		}
	}
	if _, err = adapter.DiskUsage(ctx); err != nil {
		t.Fatal(err)
	}
}
