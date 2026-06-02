package node

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/opctl/opctl/cli/internal/clicolorer"
	"github.com/opctl/opctl/cli/internal/euid0"
	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/cli/internal/pidfile"
	core "github.com/opctl/opctl/sdks/go/node"
	"github.com/opctl/opctl/sdks/go/node/api"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/opctl/opctl/sdks/go/node/datadir"
	"github.com/opctl/opctl/sdks/go/node/dns"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

func newCreateCmd(
	cliColorer clicolorer.CliColorer,
	containerRuntime *containerruntime.ContainerRuntime,
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	createCmd := cobra.Command{
		Args:  cobra.ExactArgs(0),
		Use:   "create",
		Short: "Create an opctl node",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// The daemon's stdout/stderr may be a pipe to the short-lived
			// `opctl run` that spawned it (see the local node provider). When
			// that process exits the pipe breaks, and the next write to fd 1/2
			// would otherwise raise SIGPIPE — fatal by default — silently
			// killing the daemon. Ignore it so such writes just fail with EPIPE
			// while the daemon keeps running and logging to its file.
			signal.Ignore(syscall.SIGPIPE)

			if err := euid0.Ensure(); err != nil {
				return err
			}

			dataDir, err := datadir.New(nodeConfig.DataDir)
			if err != nil {
				return err
			}

			slog.Info(
				"opctl node starting",
				"dataDir", dataDir.Path(),
				"apiListenAddress", nodeConfig.APIListenAddress,
				"dnsListenAddress", nodeConfig.DNSListenAddress,
				"containerRuntime", nodeConfig.ContainerRuntime,
			)

			gotLock, err := pidfile.TryGetLock(
				ctx,
				nodeConfig.DataDir,
			)
			if err != nil {
				return err
			}

			if !gotLock {
				return fmt.Errorf("node already running; to kill use \"sudo opctl node kill\"")
			}

			// Reconcile DNS resolver configs leaked by a prior node that didn't
			// shut down cleanly. A SIGKILL/crash/terminal-close leaves
			// /etc/resolver/opctl_* files behind (the kernel reclaims the route +
			// tun on process exit, but these on-disk files persist), and they
			// accumulate across restarts until resolution for the opctl domain
			// set degrades. We hold the pidfile lock here, so we're the sole node
			// and own these files; we re-register them for our own containers as
			// they start, so clearing stale ones now is safe.
			if cleanupErr := dns.DeleteResolverCfgs(ctx); cleanupErr != nil {
				slog.Warn("opctl node starting: failed to clear stale DNS resolver configs", "error", cleanupErr)
			}

			eg, ctx := errgroup.WithContext(ctx)

			// catch signals to ensure shutdown properly happens
			ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			eg.Go(
				func() error {
					fmt.Println(
						cliColorer.Info(fmt.Sprintf("opctl API listening at %s", nodeConfig.APIListenAddress)),
					)
					slog.Info("opctl API listening", "address", nodeConfig.APIListenAddress)

					c, err := core.New(
						ctx,
						*containerRuntime,
						dataDir.Path(),
					)
					if err != nil {
						return err
					}

					return api.Listen(
						ctx,
						nodeConfig.APIListenAddress,
						c,
					)
				},
			)

			eg.Go(
				func() error {
					fmt.Println(
						cliColorer.Info(fmt.Sprintf("opctl DNS listening at %s", nodeConfig.DNSListenAddress)),
					)
					slog.Info("opctl DNS listening", "address", nodeConfig.DNSListenAddress)

					return dns.Listen(
						ctx,
						nodeConfig.DNSListenAddress,
					)
				},
			)

			err = eg.Wait()
			stop()

			// Best-effort: remove the /etc/resolver/opctl_* files this node
			// created so a graceful stop doesn't leave them pointing at a
			// now-dead DNS server. ctx is already cancelled, so use a fresh one.
			// (A SIGKILL skips this; the startup sweep above is the backstop.)
			if cleanupErr := dns.DeleteResolverCfgs(context.Background()); cleanupErr != nil {
				slog.Error("opctl node stopping: failed to clean up DNS resolver configs", "error", cleanupErr)
			}

			// Record why the daemon's server loop ended so an unexpected exit
			// (the non-panic "daemon vanished" mode) leaves a post-mortem trace.
			if err != nil {
				slog.Error("opctl node stopping: server loop returned an error", "error", err)
			} else {
				slog.Info("opctl node stopping")
			}

			return err
		},
	}

	return &createCmd
}
