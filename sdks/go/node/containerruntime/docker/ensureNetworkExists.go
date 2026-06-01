package docker

import (
	"fmt"
	"runtime"
	"strings"

	"context"

	"github.com/docker/docker/api/types/network"
	dockerClientPkg "github.com/docker/docker/client"
)

const (
	natUnprotected  = "nat-unprotected"
	gatewayModeIpV4 = "com.docker.network.bridge.gateway_mode_ipv4"
)

func ensureNetworkExists(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
	networkName string,
) error {
	options := map[string]string{}

	if runtime.GOOS == "darwin" {
		options[gatewayModeIpV4] = natUnprotected
		options["com.docker.network.bridge.gateway_mode_ipv6"] = natUnprotected
	}

	// always attempt to create to avoid races
	createCtx, cancelCreate := withDockerTimeout(ctx, dockerMutationTimeout())
	networkCreateErr := instrumentedDockerCall("NetworkCreate", networkName, func() error {
		_, err := dockerClient.NetworkCreate(
			createCtx,
			networkName,
			network.CreateOptions{
				Attachable: true,
				Options:    options,
			},
		)
		return err
	})
	cancelCreate()
	// return errors not related to already existing...
	if networkCreateErr != nil && !strings.Contains(networkCreateErr.Error(), "exists") {
		return fmt.Errorf("unable to create network: %w", networkCreateErr)
	}

	return ensureNetworkAttached(
		ctx,
		dockerClient,
	)
}
