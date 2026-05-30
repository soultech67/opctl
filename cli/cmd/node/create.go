package node

import (
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
