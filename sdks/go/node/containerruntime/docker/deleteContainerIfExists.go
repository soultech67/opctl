package docker

import (
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
)

func (ctp _containerRuntime) DeleteContainerIfExists(
	ctx context.Context,
	containerID string,
) error {
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
	containers, err := ctp.dockerClient.ContainerList(
		ctx,
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
