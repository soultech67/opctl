// Package containerruntime defines an interface abstracting container runtime interactions.
// A fake implementation is included to allow faking said interactions.
package containerruntime

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"context"
	"io"
	"time"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
)

// Container identifies an opctl-managed container known to a container runtime.
type Container struct {
	ID    string
	Name  string
	Image string
	// State is the canonical short state from the runtime, e.g. "running", "created",
	// "exited", "paused", "dead", "restarting". Used for filtering (see opctl
	// container ls -a / opctl container prune).
	State string
	// Status is the human-readable status from the runtime, e.g. "Up 5 minutes",
	// "Exited (0) 1 hour ago". Displayed by `opctl container ls`.
	Status    string
	StartedAt time.Time
	Labels    map[string]string
}

// ContainerRuntime defines the interface container runtimes must implement to be supported by opctl
//
//counterfeiter:generate -o fakes/containerRuntime.go . ContainerRuntime
type ContainerRuntime interface {
	// Delete deletes opctl managed resources from the container runtime
	Delete(
		ctx context.Context,
	) error

	DeleteContainerIfExists(
		ctx context.Context,
		containerID string,
	) error

	DeleteContainer(
		ctx context.Context,
		containerIDOrName string,
	) error

	DeleteContainersByLabels(
		ctx context.Context,
		labels []string,
	) error

	ListContainersByLabels(
		ctx context.Context,
		labels []string,
	) ([]Container, error)

	// Kill stops/kills opctl managed resources within the container runtime
	Kill(
		ctx context.Context,
	) error

	// RunContainer creates, starts, and waits on a container. ExitCode &/Or an error will be returned
	RunContainer(
		ctx context.Context,
		req *model.ContainerCall,
		// @TODO: get rid of in combination with eventPublisher
		rootCallID string,
		// @TODO: get rid of this; just use stdout/stderr
		eventPublisher pubsub.EventPublisher,
		stdout io.WriteCloser,
		stderr io.WriteCloser,
	) (*int64, error)
}
