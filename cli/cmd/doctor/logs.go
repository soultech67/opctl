package doctor

import (
	"fmt"
	"strings"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

func newLogsCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	return &cobra.Command{
		Args: cobra.MaximumNArgs(1),
		Use:  "logs [on|off]",
		Example: strings.Join([]string{
			"opctl doctor logs        # show current logging state",
			"opctl doctor logs on     # enable daemon logging",
			"opctl doctor logs off    # disable daemon logging",
		}, "\n"),
		Short: "Show or toggle logging on a running node (no restart)",
		Long: strings.Join([]string{
			"Show the running node's logging state, or turn daemon logging on/off",
			"in real time without restarting the node. The change takes effect",
			"immediately and applies to the rotating log file under the data dir.",
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

			enabled, err := parseOnOff(args[0])
			if err != nil {
				return err
			}

			state, err := logControlClient.SetLogState(ctx, model.SetLogStateReq{Enabled: &enabled})
			if err != nil {
				return unreachableNodeErr(nodeConfig, err)
			}
			printLogState(cmd, state)

			if !enabled {
				fmt.Fprintln(cmd.OutOrStdout(), strings.Join([]string{
					"",
					"note: this suppresses ALL daemon logging, including the always-on Docker",
					"kill-path/cleanup paper trail. To cut noise without losing it, prefer",
					"\"opctl doctor log-level warn\" (or error) over turning logging off.",
				}, "\n"))
			}

			return nil
		},
	}
}

func parseOnOff(arg string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "on", "true", "enable", "enabled", "1":
		return true, nil
	case "off", "false", "disable", "disabled", "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected 'on' or 'off', got %q", arg)
	}
}
