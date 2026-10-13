// Package docker adapts the official Engine SDK to Exodock's display-safe models.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	errdefs "github.com/containerd/errdefs"
	"github.com/exodialabs/exodock/internal/core"
	"github.com/exodialabs/exodock/internal/safety"
	"github.com/moby/moby/api/types/container"
	sdk "github.com/moby/moby/client"
)

const requestTimeout = 10 * time.Second
const projectLabel = "com.docker.compose.project"
const serviceLabel = "com.docker.compose.service"

// Client uses only the explicitly resolved target; ambient Docker variables cannot override it.
type Client struct {
	engine   *sdk.Client
	readonly bool
}

var _ core.Backend = (*Client)(nil)

func New(host string, readonly bool) (*Client, error) {
	if !strings.HasPrefix(host, "unix:///") && !(runtime.GOOS == "windows" && strings.HasPrefix(host, "npipe://")) {
		return nil, errors.New("Docker target must be a local unix:/// socket; remote transports are not supported")
	}
	engine, err := sdk.New(sdk.WithHost(host), sdk.WithUserAgent("Exodock")) // Negotiation is enabled by default in SDK v0.6.
	if err != nil {
		return nil, errors.New("Cannot configure Docker connection; check the local socket target")
	}
	return &Client{engine: engine, readonly: readonly}, nil
}
func (c *Client) Close() error { return c.engine.Close() }

// safeError deliberately drops daemon payloads, which can contain secrets.
func safeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		return errors.New("Docker request timed out; refresh the resource to verify its current state")
	case errdefs.IsNotFound(err):
		return errors.New("Docker resource no longer exists; refresh the inventory")
	case errdefs.IsConflict(err):
		return errors.New("Docker refused this operation because the resource state changed; refresh and try again")
	case errdefs.IsPermissionDenied(err):
		return errors.New("Docker permission denied; check access to the selected socket")
	default:
		return errors.New("Docker request failed; check daemon availability and socket permissions, then retry")
	}
}
func cleanResource(r core.Resource) core.Resource {
	// IDs remain opaque; sanitize them only when rendering a display field.
	r.Name = safety.Inline(r.Name)
	r.State = safety.Inline(r.State)
	r.Health = safety.Inline(r.Health)
	r.Image = safety.Inline(r.Image)
	r.Project = safety.Inline(r.Project)
	r.Service = safety.Inline(r.Service)
	r.Ports = safety.Inline(r.Ports)
	r.Driver = safety.Inline(r.Driver)
	return r
}
func containerResource(v container.Summary) core.Resource {
	name := v.ID
	if len(v.Names) > 0 {
		name = strings.TrimPrefix(v.Names[0], "/")
	}
	ports := make([]string, 0, len(v.Ports))
	for _, p := range v.Ports {
		value := fmt.Sprintf("%d/%s", p.PrivatePort, p.Type)
		if p.PublicPort != 0 {
			value = fmt.Sprintf("%s:%d -> %s", p.IP, p.PublicPort, value)
		}
		ports = append(ports, value)
	}
	sort.Strings(ports)
	health := ""
	for _, h := range []string{"unhealthy", "healthy", "starting"} {
		if strings.Contains(v.Status, "("+h+")") || (h == "starting" && strings.Contains(v.Status, "(health: starting)")) {
			health = h
			break
		}
	}
	if v.Health != nil && v.Health.Status != container.NoHealthcheck {
		health = string(v.Health.Status)
	}
	return cleanResource(core.Resource{ID: v.ID, Name: name, Kind: core.Containers, State: string(v.State), Health: health, Image: v.Image, Project: v.Labels[projectLabel], Service: v.Labels[serviceLabel], Ports: strings.Join(ports, ", "), Created: time.Unix(v.Created, 0)})
}

func (c *Client) Snapshot(ctx context.Context) (core.Snapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	out := core.Snapshot{Resources: map[core.Kind][]core.Resource{}, Errors: map[core.Kind]string{}, CollectedAt: time.Now()}
	for _, kind := range []core.Kind{core.Containers, core.Images, core.Volumes, core.Networks, core.Compose} {
		out.Resources[kind] = []core.Resource{}
	}
	info, err := c.engine.Info(ctx, sdk.InfoOptions{})
	if err != nil {
		return out, safeError(err)
	}
	v := info.Info
	out.System = core.System{Version: safety.Inline(v.ServerVersion), OS: safety.Inline(v.OSType), Architecture: safety.Inline(v.Architecture), CPUs: v.NCPU, Memory: v.MemTotal, Containers: v.Containers, Running: v.ContainersRunning, Stopped: v.ContainersStopped}
	version, err := c.engine.ServerVersion(ctx, sdk.ServerVersionOptions{})
	if err == nil {
		out.System.APIVersion = safety.Inline(version.APIVersion)
	}
	containers, err := c.engine.ContainerList(ctx, sdk.ContainerListOptions{All: true})
	if err != nil {
		out.Errors[core.Containers] = safeError(err).Error()
		out.Errors[core.Compose] = out.Errors[core.Containers]
	} else {
		for _, v := range containers.Items {
			r := containerResource(v)
			out.Resources[core.Containers] = append(out.Resources[core.Containers], r)
		}
	}
	out.Resources[core.Compose] = composeProjects(containers.Items)
	images, err := c.engine.ImageList(ctx, sdk.ImageListOptions{})
	if err != nil {
		out.Errors[core.Images] = safeError(err).Error()
	} else {
		for _, v := range images.Items {
			name := strings.Join(v.RepoTags, ", ")
			if name == "" {
				name = "<untagged>"
			}
			out.Resources[core.Images] = append(out.Resources[core.Images], cleanResource(core.Resource{ID: v.ID, Name: name, Kind: core.Images, Size: v.Size, Created: time.Unix(v.Created, 0)}))
		}
	}
	volumes, err := c.engine.VolumeList(ctx, sdk.VolumeListOptions{})
	if err != nil {
		out.Errors[core.Volumes] = safeError(err).Error()
	} else {
		for _, v := range volumes.Items {
			created, _ := time.Parse(time.RFC3339, v.CreatedAt)
			size := int64(-1)
			if v.UsageData != nil {
				size = v.UsageData.Size
			}
			out.Resources[core.Volumes] = append(out.Resources[core.Volumes], cleanResource(core.Resource{ID: v.Name, Name: v.Name, Kind: core.Volumes, Driver: v.Driver, Project: v.Labels[projectLabel], Size: size, Created: created}))
		}
	}
	networks, err := c.engine.NetworkList(ctx, sdk.NetworkListOptions{})
	if err != nil {
		out.Errors[core.Networks] = safeError(err).Error()
	} else {
		for _, v := range networks.Items {
			out.Resources[core.Networks] = append(out.Resources[core.Networks], cleanResource(core.Resource{ID: v.ID, Name: v.Name, Kind: core.Networks, Driver: v.Driver, State: v.Scope, Project: v.Labels[projectLabel], Created: v.Created}))
		}
	}
	for kind := range out.Resources {
		sort.Slice(out.Resources[kind], func(i, j int) bool {
			a, b := out.Resources[kind][i], out.Resources[kind][j]
			if a.Name == b.Name {
				return a.ID < b.ID
			}
			return a.Name < b.Name
		})
	}
	out.CollectedAt = time.Now()
	return out, nil
}

func (c *Client) Inspect(ctx context.Context, kind core.Kind, id string) (core.Detail, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	d := core.Detail{Resource: core.Resource{ID: id, Kind: kind}}
	add := func(name, value string) {
		d.Fields = append(d.Fields, core.Field{Name: name, Value: safety.Inline(value)})
	}
	switch kind {
	case core.Compose:
		return c.inspectProject(ctx, id)
	case core.Containers:
		result, err := c.engine.ContainerInspect(ctx, id, sdk.ContainerInspectOptions{})
		if err != nil {
			return d, safeError(err)
		}
		v := result.Container
		d.Resource.ID = v.ID
		d.Resource.Name = strings.TrimPrefix(v.Name, "/")
		d.Resource.Created, _ = time.Parse(time.RFC3339Nano, v.Created)
		if v.State != nil {
			d.Resource.State = string(v.State.Status)
			if v.State.Health != nil {
				d.Resource.Health = string(v.State.Health.Status)
			}
			add("Started", v.State.StartedAt)
			add("Exit code", strconv.Itoa(v.State.ExitCode))
		}
		if v.Config != nil {
			d.Resource.Image = v.Config.Image
			d.Resource.Project = v.Config.Labels[projectLabel]
			d.Resource.Service = v.Config.Labels[serviceLabel]
		}
		add("Image", d.Resource.Image)
		add("State", d.Resource.State)
		add("Health", d.Resource.Health)
		add("Project", d.Resource.Project)
		add("Service", d.Resource.Service)
		add("Sensitive fields", "Environment, commands, labels, mounts and health output are hidden")
	case core.Images:
		v, err := c.engine.ImageInspect(ctx, id)
		if err != nil {
			return d, safeError(err)
		}
		d.Resource.ID = v.ID
		d.Resource.Name = strings.Join(v.RepoTags, ", ")
		d.Resource.Size = v.Size
		add("Tags", d.Resource.Name)
		add("OS", v.Os)
		add("Architecture", v.Architecture)
		add("Size", strconv.FormatInt(v.Size, 10))
		add("Sensitive fields", "Image configuration and history are hidden")
	case core.Volumes:
		result, err := c.engine.VolumeInspect(ctx, id, sdk.VolumeInspectOptions{})
		if err != nil {
			return d, safeError(err)
		}
		v := result.Volume
		d.Resource.ID = v.Name
		d.Resource.Name = v.Name
		d.Resource.Driver = v.Driver
		d.Resource.Project = v.Labels[projectLabel]
		add("Driver", v.Driver)
		add("Scope", v.Scope)
		add("Created", v.CreatedAt)
		add("Project", d.Resource.Project)
		add("Sensitive fields", "Driver options, mountpoint and labels are hidden")
	case core.Networks:
		result, err := c.engine.NetworkInspect(ctx, id, sdk.NetworkInspectOptions{})
		if err != nil {
			return d, safeError(err)
		}
		v := result.Network
		d.Resource.ID = v.ID
		d.Resource.Name = v.Name
		d.Resource.Driver = v.Driver
		d.Resource.Project = v.Labels[projectLabel]
		add("Driver", v.Driver)
		add("Scope", v.Scope)
		add("Connected containers", strconv.Itoa(len(v.Containers)))
		add("Internal", strconv.FormatBool(v.Internal))
		add("Project", d.Resource.Project)
	default:
		return d, errors.New("Inspection is unavailable for this resource type")
	}
	d.Resource = cleanResource(d.Resource)
	add("ID", d.Resource.ID)
	return d, nil
}

func (c *Client) Stats(ctx context.Context, id string) (core.Stats, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	// Request the previous sample as well; one-shot omits it and cannot yield current CPU usage.
	response, err := c.engine.ContainerStats(ctx, id, sdk.ContainerStatsOptions{Stream: false, IncludePreviousSample: true})
	if err != nil {
		return core.Stats{}, safeError(err)
	}
	defer response.Body.Close()
	var v container.StatsResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&v); err != nil {
		return core.Stats{}, safeError(err)
	}
	return calculateStats(v), nil
}
func calculateStats(v container.StatsResponse) core.Stats {
	out := core.Stats{CollectedAt: v.Read, Available: !v.Read.IsZero()}
	if out.CollectedAt.IsZero() {
		out.CollectedAt = time.Now()
	}
	current, previous := v.CPUStats, v.PreCPUStats
	cpus := current.OnlineCPUs
	if cpus == 0 {
		cpus = uint32(len(current.CPUUsage.PercpuUsage))
	}
	if current.SystemUsage > previous.SystemUsage && current.CPUUsage.TotalUsage >= previous.CPUUsage.TotalUsage && !v.PreRead.IsZero() {
		out.CPUPercent = float64(current.CPUUsage.TotalUsage-previous.CPUUsage.TotalUsage) / float64(current.SystemUsage-previous.SystemUsage) * float64(cpus) * 100
	}
	memory := v.MemoryStats.Usage
	cache, exists := v.MemoryStats.Stats["total_inactive_file"]
	if !exists {
		cache = v.MemoryStats.Stats["inactive_file"]
	}
	if cache < memory {
		memory -= cache
	}
	out.MemoryUsage = int64(memory)
	out.MemoryLimit = int64(v.MemoryStats.Limit)
	for _, network := range v.Networks {
		out.NetworkRx += int64(network.RxBytes)
		out.NetworkTx += int64(network.TxBytes)
	}
	return out
}
func (c *Client) DiskUsage(ctx context.Context) (core.DiskUsage, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	v, err := c.engine.DiskUsage(ctx, sdk.DiskUsageOptions{Images: true, Containers: true, Volumes: true, BuildCache: true})
	if err != nil {
		return core.DiskUsage{}, safeError(err)
	}
	return core.DiskUsage{Images: v.Images.TotalSize, Containers: v.Containers.TotalSize, Volumes: v.Volumes.TotalSize, BuildCache: v.BuildCache.TotalSize, CollectedAt: time.Now()}, nil
}
func (c *Client) ContainerAction(ctx context.Context, id string, action core.Action) error {
	if c.readonly {
		return errors.New("Read-only mode blocks Docker changes")
	}
	if action != core.Start && action != core.Stop && action != core.Restart && action != core.Remove {
		return errors.New("Unsupported container action")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := c.engine.ContainerInspect(ctx, id, sdk.ContainerInspectOptions{})
	if err != nil {
		return safeError(err)
	}
	if result.Container.ID != id {
		return errors.New("Container identity changed; refresh before acting")
	}
	state := result.Container.State
	if state == nil {
		return errors.New("Container state is unavailable; refresh before acting")
	}
	grace := 10
	switch action {
	case core.Start:
		if state.Running || state.Paused || state.Restarting {
			return errors.New("Container is already active; refresh its state")
		}
		_, err = c.engine.ContainerStart(ctx, id, sdk.ContainerStartOptions{})
	case core.Stop:
		if !state.Running || state.Paused {
			return errors.New("Container must be running and unpaused to stop")
		}
		_, err = c.engine.ContainerStop(ctx, id, sdk.ContainerStopOptions{Timeout: &grace})
	case core.Restart:
		if !state.Running || state.Paused {
			return errors.New("Container must be running and unpaused to restart")
		}
		_, err = c.engine.ContainerRestart(ctx, id, sdk.ContainerRestartOptions{Timeout: &grace})
	case core.Remove:
		if state.Running || state.Paused || state.Restarting || (state.Status != container.StateExited && state.Status != container.StateCreated && state.Status != container.StateDead) {
			return errors.New("Stop the container before removing it")
		}
		_, err = c.engine.ContainerRemove(ctx, id, sdk.ContainerRemoveOptions{Force: false, RemoveVolumes: false})
	}
	return safeError(err)
}
