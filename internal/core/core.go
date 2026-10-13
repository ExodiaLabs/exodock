// Package core defines the small contracts shared by the UI and Docker adapter.
package core

import (
	"context"
	"time"
)

type Kind string

const (
	Containers Kind = "containers"
	Images     Kind = "images"
	Volumes    Kind = "volumes"
	Networks   Kind = "networks"
	Compose    Kind = "compose"
)

// Resource.ID is an opaque raw identity: never normalize it before lookup or mutation.
// Names and metadata are display-safe; renderers must sanitize IDs when showing them.
type Resource struct {
	ID      string
	Name    string
	Kind    Kind
	State   string
	Health  string
	Image   string
	Project string
	Service string
	Ports   string
	Driver  string
	Size    int64
	Created time.Time
}

type System struct {
	Version      string
	APIVersion   string
	OS           string
	Architecture string
	CPUs         int
	Memory       int64
	Containers   int
	Running      int
	Stopped      int
}

type Snapshot struct {
	Resources   map[Kind][]Resource
	System      System
	CollectedAt time.Time
	Errors      map[Kind]string
}

type Detail struct {
	Resource Resource
	Fields   []Field
}

type Field struct{ Name, Value string }

type Stats struct {
	CPUPercent  float64
	MemoryUsage int64
	MemoryLimit int64
	NetworkRx   int64
	NetworkTx   int64
	CollectedAt time.Time
	Available   bool
}

type LogOptions struct {
	Tail   int
	Since  time.Time
	Stream string // all, stdout or stderr
}

type LogEntry struct {
	Text   string
	Stream string
}

type LogStream interface {
	Next(context.Context) (LogEntry, error)
	Close() error
}

type DiskUsage struct {
	Images      int64
	Containers  int64
	Volumes     int64
	BuildCache  int64
	CollectedAt time.Time
}

type Action string

const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
	Remove  Action = "remove"
)

// Backend returns allowlisted details, never raw inspect payloads or secrets.
// Resource IDs remain opaque; Detail.Fields and other display metadata are safe.
type Backend interface {
	Snapshot(context.Context) (Snapshot, error)
	Inspect(context.Context, Kind, string) (Detail, error)
	Stats(context.Context, string) (Stats, error)
	Logs(context.Context, string, LogOptions) (LogStream, error)
	DiskUsage(context.Context) (DiskUsage, error)
	ContainerAction(context.Context, string, Action) error
	Close() error
}

type ActionRequest struct {
	ID        string
	Action    Action
	Confirmed bool
}

type Executor interface {
	Execute(context.Context, ActionRequest) error
}
