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
	stopTimeout := 3
	dockerClient.ContainerStop(
		ctx,
		dockerContainerName,
		container.StopOptions{
			Timeout: &stopTimeout,
		},
	)

	err := dockerClient.ContainerRemove(
		ctx,
		dockerContainerName,
		container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		},
	)
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
