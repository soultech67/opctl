package auth

import (
	"fmt"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

func newRemoveCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	resourcesArgName := "RESOURCES"

	return &cobra.Command{
		Args:    cobra.ExactArgs(1),
		Use:     fmt.Sprintf("remove %s", resourcesArgName),
		Aliases: []string{"rm"},
		Short:   "Remove a stored auth entry by resources prefix",
		Long: "Removes a default auth entry stored via `opctl auth add`. " +
			"The RESOURCES argument must match the value passed to `opctl auth add` (case-insensitive).",
		Example: "# Remove the stored docker.io auth entry.\nopctl auth remove docker.io",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			np, err := local.New(*nodeConfig)
			if err != nil {
				return err
			}

			node, err := np.CreateNodeIfNotExists(ctx)
			if err != nil {
				return err
			}

			if err := node.RemoveAuth(
				ctx,
				model.RemoveAuthReq{
					Resources: args[0],
				},
			); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Removed auth for %s\n", args[0])
			return nil
		},
	}
}
