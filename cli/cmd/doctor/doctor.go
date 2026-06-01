// Package doctor implements `opctl doctor`, a group of diagnostic and runtime
// control commands for a running opctl node. Unlike `opctl run`, these commands
// never spawn a node; they talk to an already-running one over its localhost
// API and require no elevation.
package doctor

import (
	"fmt"
	"net/url"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/client"
	"github.com/spf13/cobra"
)

// NewDoctorCmd returns the `opctl doctor` command group.
func NewDoctorCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Inspect and control a running opctl node",
	}

	doctorCmd.AddCommand(newLogsCmd(nodeConfig))
	doctorCmd.AddCommand(newLogLevelCmd(nodeConfig))
	doctorCmd.AddCommand(newTailLogsCmd(nodeConfig))

	return doctorCmd
}

func newLogControlClient(
	nodeConfig *local.NodeConfig,
) (client.LogControlClient, error) {
	baseURL, err := url.Parse(fmt.Sprintf("http://%s/api", nodeConfig.APIListenAddress))
	if err != nil {
		return nil, err
	}

	return client.NewLogControlClient(*baseURL), nil
}

// unreachableNodeErr wraps a transport error with a hint that the node is
// likely not running, since these commands don't auto-spawn one.
func unreachableNodeErr(
	nodeConfig *local.NodeConfig,
	err error,
) error {
	return fmt.Errorf(
		"unable to reach the opctl node API at %s; is a node running? (start one with \"opctl node create\")\n  cause: %w",
		nodeConfig.APIListenAddress,
		err,
	)
}

func printLogState(
	cmd *cobra.Command,
	state model.LogState,
) {
	status := "off"
	if state.Enabled {
		status = "on"
	}

	fmt.Fprintf(
		cmd.OutOrStdout(),
		"logging: %s\nlevel:   %s\nformat:  %s\nfile:    %s\n",
		status,
		state.Level,
		state.Format,
		state.Filepath,
	)
}
