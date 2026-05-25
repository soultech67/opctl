package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	dockerClientPkg "github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

func deleteContainer(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
	dockerContainerName string,
) error {
	// ContainerStop's stopTimeout is Docker's *in-container* grace period;
	// the API call itself is bounded by our mutation timeout to prevent it
	// blocking forever when Docker is wedged.
	stopTimeout := 3
	stopCtx, cancelStop := withDockerTimeout(ctx, dockerMutationTimeout())
	_ = instrumentedDockerCall("ContainerStop", dockerContainerName, func() error {
		return dockerClient.ContainerStop(
			stopCtx,
			dockerContainerName,
			container.StopOptions{
				Timeout: &stopTimeout,
			},
		)
	})
	cancelStop()

	removeCtx, cancelRemove := withDockerTimeout(ctx, dockerMutationTimeout())
	defer cancelRemove()
	err := instrumentedDockerCall("ContainerRemove", dockerContainerName, func() error {
		return dockerClient.ContainerRemove(
			removeCtx,
			dockerContainerName,
			container.RemoveOptions{
				RemoveVolumes: true,
				Force:         true,
			},
		)
	})
	if err != nil {
		if isContainerDeleteAlreadyDone(err) {
			return nil
		}
		return fmt.Errorf("unable to delete container: %w", err)
	}

	return nil
}

func isContainerDeleteAlreadyDone(err error) bool {
	return dockerClientPkg.IsErrNotFound(err) ||
		(errdefs.IsConflict(err) &&
			strings.Contains(err.Error(), "removal of container") &&
			strings.Contains(err.Error(), "already in progress"))
}
