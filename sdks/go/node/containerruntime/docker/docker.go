package docker

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o internal/fakes/commonAPIClient.go github.com/docker/docker/client.CommonAPIClient

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerClientPkg "github.com/docker/docker/client"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
)

func New(
	ctx context.Context,
	host string,
) (containerruntime.ContainerRuntime, error) {
	dockerClient, err := dockerClientPkg.NewClientWithOpts(dockerClientPkg.FromEnv, dockerClientPkg.WithHost(host))
	if err != nil {
		return nil, err
	}

	// degrade client version to version of server
	dockerClient.NegotiateAPIVersion(ctx)

	rc, err := newRunContainer(ctx, dockerClient)
	if err != nil {
		return nil, err
	}

	return _containerRuntime{
		runContainer: rc,
		dockerClient: dockerClient,
	}, nil
}

type _containerRuntime struct {
	runContainer
	dockerClient dockerClientPkg.CommonAPIClient
}

func (cr _containerRuntime) Delete(
	ctx context.Context,
) error {
	if err := cr.deleteOpctlContainers(ctx, getOpctlContainerFilters()); err != nil {
		return err
	}

	return ensureNetworkDetached(ctx, cr.dockerClient)
}

func (cr _containerRuntime) DeleteContainer(
	ctx context.Context,
	containerIDOrName string,
) error {
	return deleteContainer(ctx, cr.dockerClient, containerIDOrName)
}

func (cr _containerRuntime) DeleteContainersByLabels(
	ctx context.Context,
	labels []string,
) error {
	return cr.deleteOpctlContainers(ctx, getOpctlContainerLabelFilters(labels))
}

func (cr _containerRuntime) ListContainersByLabels(
	ctx context.Context,
	labels []string,
) ([]containerruntime.Container, error) {
	dockerContainers, err := cr.listOpctlContainers(ctx, getOpctlContainerLabelFilters(labels))
	if err != nil {
		return nil, err
	}

	containers := []containerruntime.Container{}
	for _, dockerContainer := range dockerContainers {
		startedAt, err := cr.getListedContainerStartedAt(ctx, dockerContainer)
		if err != nil {
			return nil, err
		}

		containers = append(containers, containerruntime.Container{
			ID:        dockerContainer.ID,
			Name:      getListedContainerDisplayName(dockerContainer),
			Image:     dockerContainer.Image,
			StartedAt: startedAt,
			Labels:    cloneStringMap(dockerContainer.Labels),
		})
	}

	return containers, nil
}

func (cr _containerRuntime) getListedContainerStartedAt(
	ctx context.Context,
	dockerContainer types.Container,
) (time.Time, error) {
	fallbackStartedAt := getListedContainerCreatedAt(dockerContainer)
	containerTarget := getListedContainerDeleteTarget(dockerContainer)
	if containerTarget == "" {
		return fallbackStartedAt, nil
	}

	inspectedContainer, err := cr.dockerClient.ContainerInspect(ctx, containerTarget)
	if err != nil {
		if dockerClientPkg.IsErrNotFound(err) {
			return fallbackStartedAt, nil
		}

		return time.Time{}, err
	}

	if inspectedContainer.State == nil || inspectedContainer.State.StartedAt == "" {
		return fallbackStartedAt, nil
	}

	startedAt, err := time.Parse(time.RFC3339Nano, inspectedContainer.State.StartedAt)
	if err != nil || startedAt.IsZero() {
		return fallbackStartedAt, nil
	}

	return startedAt, nil
}

func getListedContainerCreatedAt(dockerContainer types.Container) time.Time {
	if dockerContainer.Created == 0 {
		return time.Time{}
	}

	return time.Unix(dockerContainer.Created, 0)
}

func (cr _containerRuntime) deleteOpctlContainers(
	ctx context.Context,
	containerFilters filters.Args,
) error {
	containers, err := cr.listOpctlContainers(ctx, containerFilters)
	if err != nil {
		return err
	}

	errGroup, egCtx := errgroup.WithContext(ctx)
	for _, dockerContainer := range containers {
		dockerContainer := dockerContainer
		errGroup.Go(func() error {
			containerName := getListedContainerDeleteTarget(dockerContainer)
			if containerName == "" {
				return nil
			}

			return deleteContainer(
				egCtx,
				cr.dockerClient,
				containerName,
			)
		})
	}

	err = errGroup.Wait()
	if err != nil {
		return err
	}

	return nil
}

func (cr _containerRuntime) listOpctlContainers(
	ctx context.Context,
	containerFilters filters.Args,
) ([]types.Container, error) {
	return cr.dockerClient.ContainerList(
		ctx,
		container.ListOptions{
			All:     true,
			Filters: containerFilters,
		},
	)
}

func getOpctlContainerFilters() filters.Args {
	return filters.NewArgs(
		filters.KeyValuePair{
			Key:   "name",
			Value: containerNamePrefix,
		},
		filters.KeyValuePair{
			Key:   "network",
			Value: networkName,
		},
	)
}

func getOpctlContainerLabelFilters(labels []string) filters.Args {
	if len(labels) == 0 {
		return getOpctlContainerFilters()
	}

	filterArgs := filters.NewArgs(
		filters.KeyValuePair{
			Key:   "name",
			Value: containerNamePrefix,
		},
		filters.KeyValuePair{
			Key:   "label",
			Value: getManagedContainerLabelFilter(),
		},
	)
	for _, label := range labels {
		filterArgs.Add("label", label)
	}

	return filterArgs
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}

	clone := map[string]string{}
	for key, value := range source {
		clone[key] = value
	}

	return clone
}

func getListedContainerDeleteTarget(dockerContainer types.Container) string {
	if containerName := getListedOpctlContainerName(dockerContainer.Names); containerName != "" {
		return containerName
	}

	return dockerContainer.ID
}

func getListedContainerDisplayName(dockerContainer types.Container) string {
	if containerName := getListedOpctlContainerName(dockerContainer.Names); containerName != "" {
		return containerName
	}
	if containerName := getFirstListedContainerName(dockerContainer.Names); containerName != "" {
		return containerName
	}

	return dockerContainer.ID
}

func (cr _containerRuntime) Kill(
	ctx context.Context,
) error {
	return cr.Delete(ctx)
}

const containerNamePrefix = "opctl_"
const networkName = "opctl"
