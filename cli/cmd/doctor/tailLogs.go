package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/node/logging"
	"github.com/spf13/cobra"
)

func newTailLogsCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Args:  cobra.NoArgs,
		Use:   "tail-logs",
		Short: "Tail the node's rotating log file to the console (follows; Ctrl-C to stop)",
		Long: "Follow the daemon's durable log file (the same one written under the\n" +
			"data dir). Honors OPCTL_LOG_FILE via the running node when reachable,\n" +
			"otherwise falls back to <data-dir>/logs/node.log.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			logPath := resolveLogFilePath(cmd.Context(), nodeConfig)

			// Pre-check so we can give a clearer message than tail's.
			if f, err := os.Open(logPath); err != nil {
				if os.IsPermission(err) {
					return fmt.Errorf("cannot read %s; try \"sudo opctl doctor tail-logs\"", logPath)
				}
				return fmt.Errorf("no log file at %s; is the node running with logging enabled? (\"opctl doctor logs\")", logPath)
			} else {
				f.Close()
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "tailing %s (Ctrl-C to stop)\n", logPath)

			// tail -F follows by name and re-opens across log rotation.
			tail := exec.Command("tail", "-n", strconv.Itoa(lines), "-F", logPath)
			tail.Stdout = cmd.OutOrStdout()
			tail.Stderr = cmd.ErrOrStderr()
			return tail.Run()
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 20, "number of existing lines to show before following")

	return cmd
}

// resolveLogFilePath asks the running node for its current log file path (which
// reflects OPCTL_LOG_FILE); if the node is unreachable it falls back to the
// default path under the data dir so an offline log can still be inspected.
func resolveLogFilePath(
	ctx context.Context,
	nodeConfig *local.NodeConfig,
) string {
	if logControlClient, err := newLogControlClient(nodeConfig); err == nil {
		if state, err := logControlClient.GetLogState(ctx); err == nil && state.Filepath != "" {
			return state.Filepath
		}
	}
	return logging.LogFilePath(nodeConfig.DataDir)
}
