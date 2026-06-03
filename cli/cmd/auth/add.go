package auth

import (
	"fmt"
	"strings"

	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

var (
	addUsername string
	addPassword string
)

func newAddCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	resourcesArgName := "RESOURCES"

	addCmd := cobra.Command{
		Args: cobra.ExactArgs(1),
		Example: `# add default auth for docker.io
opctl auth add docker.io -u='my-username' -p='my-password'

# add default auth for github.com
opctl auth add github.com -u='my-username' -p='my-password'
`,
		Use: fmt.Sprintf(
			"add %s",
			resourcesArgName,
		),
		Short: "Add default auth used to pull ops and images",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("%s must be a non-empty resources prefix (e.g. docker.io, github.com)", resourcesArgName)
			}

			np, err := local.New(*nodeConfig)
			if err != nil {
				return err
			}

			node, err := np.CreateNodeIfNotExists(ctx)
			if err != nil {
				return err
			}

			if err := node.AddAuth(
				ctx,
				model.AddAuthReq{
					Creds: model.Creds{
						Username: addUsername,
						Password: addPassword,
					},
					Resources: args[0],
				},
			); err != nil {
				return err
			}

			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Stored auth for %s (user: %s)\n",
				args[0],
				addUsername,
			)
			return nil
		},
	}
	addCmd.Flags().StringVarP(&addUsername, "username", "u", "", "Username")
	addCmd.Flags().StringVarP(&addPassword, "password", "p", "", "Password")

	return &addCmd
}
