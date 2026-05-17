package node

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/opctl/opctl/cli/internal/euid0"
	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/peterh/liner"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewContainerCmd(
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	containerRuntime := new(containerruntime.ContainerRuntime)
	containerCmd := newContainerCmd(containerRuntime)

	containerCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true

		var err error
		*containerRuntime, err = getContainerRuntime(
			cmd.Context(),
			*nodeConfig,
		)

		return err
	}

	return containerCmd
}

func newContainerCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	containerCmd := cobra.Command{
		Use:   "container",
		Short: "Manage opctl containers",
	}

	containerCmd.AddCommand(
		newContainerDeleteCmd(containerRuntime),
	)
	containerCmd.AddCommand(
		newContainerLsCmd(containerRuntime),
	)

	return &containerCmd
}

func newContainerDeleteCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	labelFilters := []string{}

	deleteCmd := cobra.Command{
		Args:  cobra.ExactArgs(0),
		Use:   "delete",
		Short: "Delete opctl containers matching labels",
		Long: "Deletes opctl-managed containers matching all provided Docker label filters. " +
			"If one container matches, it is deleted. If multiple containers match, an interactive terminal must select the container or containers to delete.",
		Example: "# Delete the opctl-managed container with this container name label.\n" +
			"opctl container delete --label container-name=astro-local-localstack\n\n" +
			"# Select from opctl-managed containers for this image.\n" +
			"opctl container delete --label image-ref=localstack/localstack-pro:latest",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if len(labelFilters) == 0 {
				return fmt.Errorf("at least one --label must be provided")
			}
			for _, labelFilter := range labelFilters {
				if labelFilter == "" {
					return fmt.Errorf("--label cannot be empty")
				}
			}
			labelFilters = normalizeContainerDeleteLabelFilters(labelFilters)

			if err := euid0.Ensure(); err != nil {
				return err
			}

			containers, err := (*containerRuntime).ListContainersByLabels(ctx, labelFilters)
			if err != nil {
				return err
			}

			containersToDelete, err := selectContainersToDelete(
				cmd.OutOrStdout(),
				containers,
				len(labelFilters),
				term.IsTerminal(int(os.Stdout.Fd())),
				promptContainerDeleteSelection,
			)
			if err != nil {
				return err
			}

			for _, containerToDelete := range containersToDelete {
				deleteTarget, err := getContainerDeleteTarget(containerToDelete)
				if err != nil {
					return err
				}

				if err := (*containerRuntime).DeleteContainer(ctx, deleteTarget); err != nil {
					return err
				}
			}

			return nil
		},
	}

	deleteCmd.Flags().StringArrayVarP(&labelFilters, "label", "l", []string{}, "Docker label filter or shorthand from 'opctl container ls'; may be repeated and accepts key or key=value")

	return &deleteCmd
}

func normalizeContainerDeleteLabelFilters(labelFilters []string) []string {
	normalizedLabelFilters := []string{}
	for _, labelFilter := range labelFilters {
		normalizedLabelFilters = append(
			normalizedLabelFilters,
			normalizeContainerDeleteLabelFilter(labelFilter),
		)
	}

	return normalizedLabelFilters
}

func normalizeContainerDeleteLabelFilter(labelFilter string) string {
	labelKey, labelValue, hasLabelValue := strings.Cut(strings.TrimSpace(labelFilter), "=")
	for _, label := range containerDeleteLabels {
		if labelKey == label.displayName {
			labelKey = label.dockerName
			break
		}
	}
	if !hasLabelValue {
		return labelKey
	}

	return fmt.Sprintf("%s=%s", labelKey, labelValue)
}

type containerDeletePrompter func(prompt string) (string, error)

func selectContainersToDelete(
	stdout io.Writer,
	containers []containerruntime.Container,
	labelCount int,
	isInteractive bool,
	prompt containerDeletePrompter,
) ([]containerruntime.Container, error) {
	switch len(containers) {
	case 0:
		_, _ = fmt.Fprintln(stdout, "no opctl-managed containers match labels")
		return nil, nil
	case 1:
		return containers, nil
	}

	if !isInteractive {
		return nil, fmt.Errorf("multiple containers match labels; rerun in an interactive terminal or narrow the labels")
	}

	labelNoun := "labels"
	if labelCount == 1 {
		labelNoun = "label"
	}
	_, _ = fmt.Fprintf(stdout, "multiple containers match %s\n", labelNoun)
	for i, container := range containers {
		_, _ = fmt.Fprintf(stdout, "[%d] [ ] %s started %s\n",
			i+1,
			formatContainerDisplayName(container),
			formatContainerStartedAt(container.StartedAt),
		)
	}
	_, _ = fmt.Fprintln(stdout)

	rawSelection, err := prompt(fmt.Sprintf("Select container(s) to remove [1-%d]: ", len(containers)))
	if err != nil {
		return nil, err
	}

	selectedIndexes, err := parseContainerSelection(rawSelection, len(containers))
	if err != nil {
		return nil, err
	}

	containersToDelete := []containerruntime.Container{}
	for _, selectedIndex := range selectedIndexes {
		containersToDelete = append(containersToDelete, containers[selectedIndex])
	}

	return containersToDelete, nil
}

func promptContainerDeleteSelection(prompt string) (string, error) {
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	return line.Prompt(prompt)
}

func parseContainerSelection(rawSelection string, containerCount int) ([]int, error) {
	rawSelection = strings.TrimSpace(rawSelection)
	if rawSelection == "" {
		return nil, fmt.Errorf("selection cannot be empty")
	}

	selectedIndexes := []int{}
	seenIndexes := map[int]struct{}{}
	for _, rawSelectionPart := range strings.Split(rawSelection, ",") {
		rawSelectionPart = strings.TrimSpace(rawSelectionPart)
		if rawSelectionPart == "" {
			return nil, fmt.Errorf("selection cannot include empty values")
		}

		selection, err := strconv.Atoi(rawSelectionPart)
		if err != nil {
			return nil, fmt.Errorf("selection %q is not a number", rawSelectionPart)
		}
		if selection < 1 || selection > containerCount {
			return nil, fmt.Errorf("selection %d is outside the range 1-%d", selection, containerCount)
		}

		selectedIndex := selection - 1
		if _, ok := seenIndexes[selectedIndex]; ok {
			continue
		}
		seenIndexes[selectedIndex] = struct{}{}
		selectedIndexes = append(selectedIndexes, selectedIndex)
	}

	return selectedIndexes, nil
}

func getContainerDeleteTarget(container containerruntime.Container) (string, error) {
	if container.ID != "" {
		return container.ID, nil
	}
	if container.Name != "" {
		return container.Name, nil
	}

	return "", fmt.Errorf("selected container has no id or name")
}

func formatContainerDisplayName(container containerruntime.Container) string {
	if container.Name != "" {
		return strings.ReplaceAll(strings.TrimPrefix(container.Name, "opctl_"), "_", "-")
	}
	if container.ID != "" {
		return container.ID
	}
	if container.Image != "" {
		return container.Image
	}

	return "unknown"
}
