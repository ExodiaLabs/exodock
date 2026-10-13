// Package app enforces mutation policy independently of the terminal controls.
package app

import (
	"context"
	"errors"
	"time"

	"github.com/exodialabs/exodock/internal/core"
)

var (
	ErrReadOnly      = errors.New("read-only mode: Docker changes are disabled")
	ErrConfirmation  = errors.New("confirm this action before changing Docker")
	ErrInvalidAction = errors.New("invalid container action")
)

type Service struct {
	backend  core.Backend
	readonly bool
}

func New(backend core.Backend, readonly bool) *Service {
	return &Service{backend: backend, readonly: readonly}
}

func (s *Service) Execute(ctx context.Context, request core.ActionRequest) error {
	if s.readonly {
		return ErrReadOnly
	}
	if !request.Confirmed {
		return ErrConfirmation
	}
	if request.ID == "" {
		return ErrInvalidAction
	}
	switch request.Action {
	case core.Start, core.Stop, core.Restart, core.Remove:
	default:
		return ErrInvalidAction
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	// Re-read the exact ID after confirmation: a refresh or external client may have changed it.
	detail, err := s.backend.Inspect(ctx, core.Containers, request.ID)
	if err != nil {
		return err
	}
	if detail.Resource.ID != request.ID {
		return errors.New("container identity changed; refresh and confirm again")
	}
	if request.Action == core.Remove && detail.Resource.State != "exited" && detail.Resource.State != "dead" && detail.Resource.State != "created" {
		return errors.New("stop this container before removing it; volumes will be preserved")
	}
	if err = s.backend.ContainerAction(ctx, request.ID, request.Action); err != nil {
		return err
	}
	return nil
}
