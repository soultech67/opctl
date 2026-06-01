package node

import (
	"github.com/opctl/opctl/cli/internal/clicolorer"
	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/opctl/opctl/sdks/go/node/logging"
	"github.com/spf13/cobra"
)

var (
	containerRuntime = new(containerruntime.ContainerRuntime)
)

func NewNodeCmd(
	cliColorer clicolorer.CliColorer,
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	nodeCmd := cobra.Command{
		Use:   "node",
		Short: "Manage nodes",
	}

	nodeCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		// For `node create` (the daemon process) initialize the durable logger
		// before anything else can log — including the container runtime ping
		// below — so early startup diagnostics land in the rotating log file.
		// Other node subcommands are short-lived CLI processes and must NOT open
		// the daemon's rotating log file (lumberjack isn't safe for concurrent
		// writers across processes).
		if cmd.Name() == "create" {
			if err := logging.Init(nodeConfig.DataDir); err != nil {
				return err
			}
		}

		var err error
		*containerRuntime, err = getContainerRuntime(
			cmd.Context(),
			*nodeConfig,
		)

		return err
	}

	nodeCmd.AddCommand(
		newContainerCmd(
			containerRuntime,
		),
	)
	nodeCmd.AddCommand(
		newCreateCmd(
			cliColorer,
			containerRuntime,
			nodeConfig,
		),
	)
	nodeCmd.AddCommand(
		newDeleteCmd(
			containerRuntime,
			nodeConfig,
		),
	)
	nodeCmd.AddCommand(
		newKillCmd(
			containerRuntime,
			nodeConfig,
		),
	)

	return &nodeCmd
}
