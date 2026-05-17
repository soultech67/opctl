package docker

import (
	"runtime"

	"github.com/docker/docker/api/types/network"
	. "github.com/opctl/opctl/sdks/go/node/containerruntime/docker/internal/fakes"
)

func configureNetworkInspect(fakeDockerClient *FakeCommonAPIClient) {
	fakeDockerClient.NetworkInspectReturns(
		network.Inspect{
			Options: expectedNetworkCreateOptions(),
		},
		nil,
	)
}

func expectedNetworkCreateOptions() map[string]string {
	options := map[string]string{}
	if runtime.GOOS == "darwin" {
		options[gatewayModeIpV4] = natUnprotected
		options["com.docker.network.bridge.gateway_mode_ipv6"] = natUnprotected
	}
	return options
}
