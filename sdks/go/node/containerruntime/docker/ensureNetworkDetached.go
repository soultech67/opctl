package docker

import (
	"context"

	dockerClientPkg "github.com/docker/docker/client"
)

func ensureNetworkDetached(
	ctx context.Context,
	dockerClient dockerClientPkg.CommonAPIClient,
) error {
	err := dockerClient.NetworkRemove(ctx, networkName)
	if err != nil && !dockerClientPkg.IsErrNotFound(err) {
		return err
	}

	// The WireGuard utun is owned by the daemon process and reclaimed by the
	// kernel when it exits (which, via `opctl node kill`, has already happened
	// by the time this runs). The previous darwin branch here ran
	// `ip link delete tun%d` -- a Linux command with a macOS-wrong interface
	// name -- and then DISCARDED the resulting error (a constructed-but-unused
	// fmt.Errorf), so it never actually deleted anything. Removed.
	return nil
}
