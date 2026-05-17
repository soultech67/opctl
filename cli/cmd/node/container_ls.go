package node

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/spf13/cobra"
)

type containerDeleteLabel struct {
	dockerName  string
	displayName string
}

var containerDeleteLabels = []containerDeleteLabel{
	{
		dockerName:  "opctl.container-id",
		displayName: "container-id",
	},
	{
		dockerName:  "opctl.container-name",
		displayName: "container-name",
	},
	{
		dockerName:  "opctl.image-ref",
		displayName: "image-ref",
	},
}

func newContainerLsCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	return &cobra.Command{
		Args:  cobra.ExactArgs(0),
		Use:   "ls",
		Short: "List opctl containers",
		Long: "Lists opctl-managed containers and the label filters that can be passed to " +
			"`opctl container delete --label`.",
		Example: "# List opctl containers with copyable delete labels.\n" +
			"opctl container ls\n\n" +
			"# Delete one listed container by exact container ID label.\n" +
			"opctl container delete --label container-id=2a647646e9cc4ef4940f52ad944f5657",
		RunE: func(cmd *cobra.Command, args []string) error {
			containers, err := (*containerRuntime).ListContainersByLabels(cmd.Context(), nil)
			if err != nil {
				return err
			}

			return writeContainerList(cmd.OutOrStdout(), containers)
		},
	}
}

func writeContainerList(
	stdout io.Writer,
	containers []containerruntime.Container,
) error {
	tabWriter := tabwriter.NewWriter(stdout, 0, 8, 1, '\t', 0)

	fmt.Fprintln(tabWriter, "NAME\tID\tIMAGE\tSTARTED")
	for _, container := range containers {
		fmt.Fprintf(
			tabWriter,
			"%s\t%s\t%s\t%s\n",
			formatContainerDisplayName(container),
			formatContainerShortID(container.ID),
			formatContainerValue(container.Image),
			formatContainerStartedAt(container.StartedAt),
		)
		labels := formatContainerDeleteLabels(container.Labels)
		if len(labels) == 0 {
			continue
		}

		fmt.Fprintln(tabWriter, "DELETE LABELS")
		for _, label := range labels {
			fmt.Fprintf(tabWriter, "  %s\n", label)
		}
	}

	return tabWriter.Flush()
}

func formatContainerShortID(containerID string) string {
	if containerID == "" {
		return "-"
	}
	if len(containerID) <= 12 {
		return containerID
	}

	return containerID[:12]
}

func formatContainerValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}

	return value
}

func formatContainerDeleteLabels(labels map[string]string) []string {
	labelFilters := []string{}
	for _, label := range containerDeleteLabels {
		if labelValue := labels[label.dockerName]; labelValue != "" {
			labelFilters = append(labelFilters, fmt.Sprintf("%s=%s", label.displayName, labelValue))
		}
	}

	return labelFilters
}

func formatContainerStartedAt(startedAt time.Time) string {
	if startedAt.IsZero() {
		return "unknown"
	}

	return startedAt.Local().Format("2006/01/02 03:04:05pm MST")
}
