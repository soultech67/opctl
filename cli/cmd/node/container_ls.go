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

// runningContainerState is the canonical runtime state for a container actively
// running. opctl container ls defaults to filtering by this value to mirror
// `docker ps` semantics.
const runningContainerState = "running"

// containerListOptions captures the display-shaping flags for `container ls`.
type containerListOptions struct {
	// IncludeImage adds the IMAGE column. Hidden by default because image refs
	// are long and tend to be the column that pushes rows past the terminal
	// width.
	IncludeImage bool
	// Verbose prints a DELETE LABELS section below the table, with one block
	// per container, suitable for copy-paste into `opctl container delete --label`.
	Verbose bool
}

func newContainerLsCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	var (
		all          bool
		includeImage bool
		verbose      bool
		filter       string
	)

	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(0),
		Use:   "ls",
		Short: "List opctl containers",
		Long: "Lists opctl-managed containers. By default only running containers are shown " +
			"(mirrors `docker ps` semantics) and the table omits the IMAGE column because " +
			"image refs tend to wrap rows. Pass `--all/-a` to include non-running containers, " +
			"`--images/-i` to add the IMAGE column, and `--verbose/-v` to also print the " +
			"copy-paste-friendly delete-label filters below the table.\n\n" +
			"`--filter NAME` shows only containers whose name contains NAME (case-insensitive). " +
			"The `opctl_` prefix is implied, so `--filter artifacts-api` matches " +
			"opctl_artifacts-api_<id> -- no need to type the prefix; `_` and `-` are " +
			"interchangeable. (Only opctl-managed containers are listed here; for non-opctl " +
			"containers a service starts itself, e.g. localstack's redis, use `docker ps`.)",
		Example: "# List running opctl containers.\n" +
			"opctl container ls\n\n" +
			"# Show just the artifacts-api container(s).\n" +
			"opctl container ls --filter artifacts-api\n\n" +
			"# Include stopped/created containers too.\n" +
			"opctl container ls -a\n\n" +
			"# Show the IMAGE column.\n" +
			"opctl container ls -i\n\n" +
			"# Show delete-label filters for each container.\n" +
			"opctl container ls -v\n\n" +
			"# Delete one listed container by exact container ID label.\n" +
			"opctl container delete --label container-id=2a647646e9cc4ef4940f52ad944f5657",
		RunE: func(cmd *cobra.Command, args []string) error {
			containers, err := (*containerRuntime).ListContainersByLabels(cmd.Context(), nil)
			if err != nil {
				return err
			}

			listed := filterContainersByName(filterContainersForList(containers, all), filter)
			if strings.TrimSpace(filter) != "" && len(listed) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no opctl-managed containers match %q\n", filter)
				return nil
			}

			return writeContainerList(
				cmd.OutOrStdout(),
				listed,
				containerListOptions{
					IncludeImage: includeImage,
					Verbose:      verbose,
				},
			)
		},
	}
	cmd.Flags().BoolVarP(&all, "all", "a", false, "Show all opctl containers (default shows just running)")
	cmd.Flags().BoolVarP(&includeImage, "images", "i", false, "Include the IMAGE column in the table")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Also print delete-label filters below the table")
	cmd.Flags().StringVar(&filter, "filter", "", "Show only containers whose name contains this substring (opctl_ prefix implied; case-insensitive)")

	return cmd
}

// filterContainersByName narrows the list to containers whose name contains the
// (case-insensitive) filter substring. The opctl_ prefix is implied -- callers
// pass just the op's container name (e.g. "artifacts-api"). Matching is done
// against both the display name and the raw opctl_ container name, with `_` and
// `-` treated as equivalent so either separator works.
func filterContainersByName(containers []containerruntime.Container, filter string) []containerruntime.Container {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return containers
	}

	needle := normalizeContainerFilterText(filter)
	matched := []containerruntime.Container{}
	for _, container := range containers {
		if strings.Contains(normalizeContainerFilterText(formatContainerDisplayName(container)), needle) ||
			strings.Contains(normalizeContainerFilterText(container.Name), needle) {
			matched = append(matched, container)
		}
	}

	return matched
}

func normalizeContainerFilterText(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "-")
}

// filterContainersForList narrows the list to running-only when all is false.
// An empty container.State (e.g. from a runtime that doesn't populate it) is
// passed through so we don't accidentally hide containers in that case.
func filterContainersForList(containers []containerruntime.Container, all bool) []containerruntime.Container {
	if all {
		return containers
	}

	filtered := []containerruntime.Container{}
	for _, container := range containers {
		if container.State == "" || container.State == runningContainerState {
			filtered = append(filtered, container)
		}
	}

	return filtered
}

func writeContainerList(
	stdout io.Writer,
	containers []containerruntime.Container,
	opts containerListOptions,
) error {
	if err := writeContainerTable(stdout, containers, opts.IncludeImage); err != nil {
		return err
	}

	if opts.Verbose {
		writeContainerDeleteLabels(stdout, containers)
	}

	return nil
}

// writeContainerTable prints just the column-aligned summary. Every row has the
// same number of cells (no interleaved sub-rows), so text/tabwriter can size
// the columns correctly across all rows. padchar is ' ' — using '\t' here makes
// the terminal expand padding at its own tab stops, which has nothing to do
// with the column widths tabwriter computed and produces ragged output.
func writeContainerTable(
	stdout io.Writer,
	containers []containerruntime.Container,
	includeImage bool,
) error {
	tabWriter := tabwriter.NewWriter(stdout, 0, 8, 2, ' ', 0)

	header := []string{"NAME", "ID", "STATUS", "STARTED"}
	if includeImage {
		// Insert IMAGE between ID and STATUS — mirrors docker ps column order.
		header = []string{"NAME", "ID", "IMAGE", "STATUS", "STARTED"}
	}
	fmt.Fprintln(tabWriter, strings.Join(header, "\t"))

	for _, container := range containers {
		cells := []string{
			formatContainerDisplayName(container),
			formatContainerShortID(container.ID),
			formatContainerStatus(container),
			formatContainerStartedAt(container.StartedAt),
		}
		if includeImage {
			cells = []string{
				formatContainerDisplayName(container),
				formatContainerShortID(container.ID),
				formatContainerValue(container.Image),
				formatContainerStatus(container),
				formatContainerStartedAt(container.StartedAt),
			}
		}
		fmt.Fprintln(tabWriter, strings.Join(cells, "\t"))
	}

	return tabWriter.Flush()
}

// writeContainerDeleteLabels emits a free-form labels section below the table.
// Free-form (not tabwriter) on purpose — the labels are long, vary per
// container, and aren't a real grid, so keeping them out of the tabwriter
// stream is what lets the table above stay aligned.
func writeContainerDeleteLabels(
	stdout io.Writer,
	containers []containerruntime.Container,
) {
	containersWithLabels := []containerruntime.Container{}
	for _, container := range containers {
		if len(formatContainerDeleteLabels(container.Labels)) > 0 {
			containersWithLabels = append(containersWithLabels, container)
		}
	}
	if len(containersWithLabels) == 0 {
		return
	}

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "DELETE LABELS")
	for _, container := range containersWithLabels {
		fmt.Fprintf(stdout, "  %s\n", formatContainerDisplayName(container))
		for _, label := range formatContainerDeleteLabels(container.Labels) {
			fmt.Fprintf(stdout, "    %s\n", label)
		}
	}
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

// formatContainerStatus prefers the human-readable Status from the runtime
// ("Up 5 minutes", "Exited (0) 2 hours ago"). Falls back to the short State
// when Status isn't populated, then to "-" when neither is.
func formatContainerStatus(container containerruntime.Container) string {
	if status := strings.TrimSpace(container.Status); status != "" {
		return status
	}
	if state := strings.TrimSpace(container.State); state != "" {
		return state
	}

	return "-"
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
