package auth

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

func newListCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	return &cobra.Command{
		Args:    cobra.ExactArgs(0),
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List stored auth entries",
		Long: "Lists default auth entries stored via `opctl auth add`. " +
			"Passwords are not shown; use `opctl auth remove <RESOURCES>` to clear an entry.",
		Example: "# List stored auth entries.\nopctl auth list\n\n" +
			"# Remove a stored entry.\nopctl auth remove docker.io",
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

			auths, err := node.ListAuths(ctx)
			if err != nil {
				return err
			}

			return writeAuthList(cmd.OutOrStdout(), auths)
		},
	}
}

func writeAuthList(
	stdout io.Writer,
	auths []model.Auth,
) error {
	if len(auths) == 0 {
		fmt.Fprintln(stdout, "No stored auth entries. Use `opctl auth add <RESOURCES> -u <user> -p <password>` to add one.")
		return nil
	}

	sort.SliceStable(auths, func(i, j int) bool {
		return auths[i].Resources < auths[j].Resources
	})

	tabWriter := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintln(tabWriter, "RESOURCES\tUSERNAME")
	for _, auth := range auths {
		username := auth.Username
		if username == "" {
			username = "-"
		}
		fmt.Fprintf(tabWriter, "%s\t%s\n", auth.Resources, username)
	}
	return tabWriter.Flush()
}
