package doctor

import (
	"fmt"
	"strings"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

func newLogLevelCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	levels := strings.Join(model.LogLevels, "|")

	return &cobra.Command{
		Args: cobra.MaximumNArgs(1),
		Use:  "log-level [" + levels + "]",
		Example: strings.Join([]string{
			"opctl doctor log-level         # show current level",
			"opctl doctor log-level debug   # raise verbosity to debug",
			"opctl doctor log-level info    # back to default",
		}, "\n"),
		Short: "Show or set the log level on a running node (no restart)",
		Long: strings.Join([]string{
			"Show the running node's log level, or change it in real time without",
			"restarting the node. Accepted levels (most to least verbose): " + levels + ".",
		}, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			logControlClient, err := newLogControlClient(nodeConfig)
			if err != nil {
				return err
			}

			ctx := cmd.Context()

			if len(args) == 0 {
				state, err := logControlClient.GetLogState(ctx)
				if err != nil {
					return unreachableNodeErr(nodeConfig, err)
				}
				printLogState(cmd, state)
				return nil
			}

			level, ok := model.NormalizeLogLevel(args[0])
			if !ok {
				return fmt.Errorf(
					"invalid log level %q; valid levels: %s",
					args[0],
					strings.Join(model.LogLevels, ", "),
				)
			}

			state, err := logControlClient.SetLogState(ctx, model.SetLogStateReq{Level: &level})
			if err != nil {
				return unreachableNodeErr(nodeConfig, err)
			}
			printLogState(cmd, state)

			return nil
		},
	}
}
