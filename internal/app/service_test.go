package app

import (
	"context"
	"errors"
	"testing"

	"github.com/exodialabs/exodock/internal/core"
)

type backendStub struct {
	core.Backend
	state, identity string
	calls           int
	inspectErr      error
}

func (b *backendStub) Inspect(_ context.Context, _ core.Kind, id string) (core.Detail, error) {
	if b.inspectErr != nil {
		return core.Detail{}, b.inspectErr
	}
	identity := id
	if b.identity != "" {
		identity = b.identity
	}
	return core.Detail{Resource: core.Resource{ID: identity, State: b.state}}, nil
}
func (b *backendStub) ContainerAction(_ context.Context, _ string, _ core.Action) error {
	b.calls++
	return nil
}

func TestMutationGuards(t *testing.T) {
	for _, tt := range []struct {
		name            string
		readonly        bool
		request         core.ActionRequest
		state, identity string
		want            error
		allowed         bool
	}{
		{"readonly", true, core.ActionRequest{ID: "id", Action: core.Start, Confirmed: true}, "exited", "", ErrReadOnly, false},
		{"unconfirmed", false, core.ActionRequest{ID: "id", Action: core.Start}, "exited", "", ErrConfirmation, false},
		{"bad action", false, core.ActionRequest{ID: "id", Action: "prune", Confirmed: true}, "exited", "", ErrInvalidAction, false},
		{"empty id", false, core.ActionRequest{Action: core.Start, Confirmed: true}, "exited", "", ErrInvalidAction, false},
		{"running remove", false, core.ActionRequest{ID: "id", Action: core.Remove, Confirmed: true}, "running", "", nil, false},
		{"identity changed", false, core.ActionRequest{ID: "id", Action: core.Start, Confirmed: true}, "exited", "other", nil, false},
		{"stopped remove", false, core.ActionRequest{ID: "id", Action: core.Remove, Confirmed: true}, "exited", "", nil, true},
		{"start", false, core.ActionRequest{ID: "id", Action: core.Start, Confirmed: true}, "exited", "", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := &backendStub{state: tt.state, identity: tt.identity}
			err := New(b, tt.readonly).Execute(context.Background(), tt.request)
			if tt.allowed {
				if err != nil || b.calls != 1 {
					t.Fatalf("%v calls=%d", err, b.calls)
				}
			} else {
				if err == nil || b.calls != 0 {
					t.Fatalf("guard bypass: %v calls=%d", err, b.calls)
				}
				if tt.want != nil && !errors.Is(err, tt.want) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestInspectFailurePreventsMutation(t *testing.T) {
	want := errors.New("disconnected")
	b := &backendStub{inspectErr: want}
	err := New(b, false).Execute(context.Background(), core.ActionRequest{ID: "id", Action: core.Stop, Confirmed: true})
	if !errors.Is(err, want) || b.calls != 0 {
		t.Fatal(err, b.calls)
	}
}
