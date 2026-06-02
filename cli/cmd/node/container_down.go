package node

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/opctl/opctl/cli/internal/euid0"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newContainerDownCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(1),
		Use:   "down NAME",
		Short: "Cleanly shut down running opctl container(s) by name",
		Long: "Gracefully stops and removes the RUNNING opctl-managed container(s) whose " +
			"container-name is NAME (the short name shown by `opctl container ls`, e.g. " +
			"artifacts-api).\n\n" +
			"  - exactly one running match: it is shut down.\n" +
			"  - several running under the same NAME: you are prompted to pick which to shut " +
			"down, with enough info to tell them apart -- or pass --force to take them all down.\n" +
			"  - no running match: nothing is done (stopped containers are ignored).\n\n" +
			"This is the everyday \"take a service down\" command. To remove STOPPED leftovers use " +
			"`opctl container prune`; to remove containers by an arbitrary label in any state use " +
			"`opctl container delete`.",
		Example: "# Shut down the running artifacts-api container.\n" +
			"opctl container down artifacts-api\n\n" +
			"# Shut down every running container under that name, no prompt.\n" +
			"opctl container down artifacts-api --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			name := strings.TrimSpace(args[0])
			if name == "" {
				return fmt.Errorf("NAME is required (e.g. `opctl container down artifacts-api`)")
			}

			if err := euid0.Ensure(); err != nil {
				return err
			}

			// NAME identifies the container by its opctl.container-name label.
			labelFilter := normalizeContainerDeleteLabelFilter("container-name=" + name)
			containers, err := (*containerRuntime).ListContainersByLabels(ctx, []string{labelFilter})
			if err != nil {
				return err
			}

			return downContainers(
				ctx,
				cmd.OutOrStdout(),
				name,
				containers,
				force,
				term.IsTerminal(int(os.Stdout.Fd())),
				promptContainerDeleteSelection,
				func(ctx context.Context, target string) error {
					return (*containerRuntime).DeleteContainer(ctx, target)
				},
			)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Shut down ALL running containers matching NAME without prompting")

	return cmd
}

// downContainers filters the matched containers to the running ones, selects
// which to shut down (all when --force or a single match; an interactive choice
// otherwise), and shuts each down cleanly via downContainer. Side-effecting
// dependencies are injected so the logic is unit-testable.
func downContainers(
	ctx context.Context,
	stdout io.Writer,
	name string,
	containers []containerruntime.Container,
	force bool,
	isInteractive bool,
	prompt containerDeletePrompter,
	downContainer func(ctx context.Context, target string) error,
) error {
	running := []containerruntime.Container{}
	for _, container := range containers {
		if container.State == runningContainerState {
			running = append(running, container)
		}
	}

	if len(running) == 0 {
		if len(containers) > 0 {
			_, _ = fmt.Fprintf(stdout,
				"no RUNNING opctl-managed container named %q (%d matched but are not running; use `opctl container prune` to remove stopped ones)\n",
				name, len(containers))
		} else {
			_, _ = fmt.Fprintf(stdout, "no running opctl-managed container named %q\n", name)
		}
		return nil
	}

	toShutDown := running
	if !force && len(running) > 1 {
		if !isInteractive {
			return fmt.Errorf(
				"%d running containers are named %q; rerun in an interactive terminal to choose, or pass --force to shut them all down",
				len(running), name)
		}

		_, _ = fmt.Fprintf(stdout, "%d running containers are named %q:\n", len(running), name)
		for i, container := range running {
			_, _ = fmt.Fprintf(stdout, "[%d] %s (%s) started %s\n",
				i+1,
				formatContainerDisplayName(container),
				formatContainerStatus(container),
				formatContainerStartedAt(container.StartedAt),
			)
		}
		_, _ = fmt.Fprintln(stdout)

		rawSelection, err := prompt(fmt.Sprintf("Select container(s) to shut down [1-%d] (or rerun with --force for all): ", len(running)))
		if err != nil {
			return err
		}

		selectedIndexes, err := parseContainerSelection(rawSelection, len(running))
		if err != nil {
			return err
		}

		toShutDown = []containerruntime.Container{}
		for _, selectedIndex := range selectedIndexes {
			toShutDown = append(toShutDown, running[selectedIndex])
		}
	}

	for _, container := range toShutDown {
		target, err := getContainerDeleteTarget(container)
		if err != nil {
			return err
		}
		if err := downContainer(ctx, target); err != nil {
			return fmt.Errorf("failed to shut down %s: %w", formatContainerDisplayName(container), err)
		}
		_, _ = fmt.Fprintf(stdout, "shut down %s\n", formatContainerDisplayName(container))
	}

	return nil
}
