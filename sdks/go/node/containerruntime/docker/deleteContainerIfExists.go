package docker

import (
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
)

func (ctp _containerRuntime) DeleteContainerIfExists(
	ctx context.Context,
	containerID string,
) error {
	dockerInstrInfof("DeleteContainerIfExists starting: containerID=%s", containerID)
	startedAt := time.Now()
	defer func() {
		dockerInstrInfof("DeleteContainerIfExists done in %s: containerID=%s", time.Since(startedAt), containerID)
	}()

	containerNames, err := ctp.getContainerNamesByID(ctx, containerID)
	if err != nil {
		return err
	}

	if len(containerNames) == 0 {
		containerNames = append(containerNames, getLegacyContainerName(containerID))
	}

	for _, containerName := range containerNames {
		err = deleteContainer(ctx, ctp.dockerClient, containerName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ctp _containerRuntime) getContainerNamesByID(
	ctx context.Context,
	containerID string,
) ([]string, error) {
	listCtx, cancel := withDockerTimeout(ctx, dockerInspectTimeout())
	defer cancel()
	var containers []types.Container
	err := instrumentedDockerCall("ContainerList", "lookup by id "+containerID, func() error {
		var listErr error
		containers, listErr = ctp.dockerClient.ContainerList(
			listCtx,
			container.ListOptions{
				All: true,
				Filters: filters.NewArgs(
					filters.KeyValuePair{
						Key:   "label",
						Value: getContainerIDLabelFilter(containerID),
					},
				),
			},
		)
		return listErr
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list containers: %w", err)
	}

	containerNames := []string{}
	for _, dockerContainer := range containers {
		containerName := getListedOpctlContainerName(dockerContainer.Names)
		if containerName == "" {
			containerName = dockerContainer.ID
		}
		if containerName == "" {
			containerName = getFirstListedContainerName(dockerContainer.Names)
		}
		if containerName != "" {
			containerNames = append(containerNames, containerName)
		}
	}

	return containerNames, nil
}

func getListedOpctlContainerName(containerNames []string) string {
	for _, containerName := range containerNames {
		containerName = normalizeDockerContainerName(containerName)
		if isOpctlContainerName(containerName) {
			return containerName
		}
	}

	return ""
}

func getFirstListedContainerName(containerNames []string) string {
	if len(containerNames) == 0 {
		return ""
	}

	return normalizeDockerContainerName(containerNames[0])
}
