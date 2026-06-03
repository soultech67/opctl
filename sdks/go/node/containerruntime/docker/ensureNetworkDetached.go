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
	// kernel when that process exits, so nothing here needs to force its
	// deletion. The previous darwin branch ran `ip link delete tun%d` -- a Linux
	// command with a macOS-wrong interface name -- and then DISCARDED the
	// resulting error (a constructed-but-unused fmt.Errorf), so it never
	// actually deleted anything. Removed.
	return nil
}
