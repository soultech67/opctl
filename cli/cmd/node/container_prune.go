package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/opctl/opctl/cli/internal/euid0"
	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/spf13/cobra"
)

// containerPruneConfirmer reads a confirmation answer from the user. Extracted
// so tests can inject a deterministic response.
type containerPruneConfirmer func(prompt string) (string, error)

func newContainerPruneCmd(
	containerRuntime *containerruntime.ContainerRuntime,
) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Args:  cobra.ExactArgs(0),
		Use:   "prune",
		Short: "Remove all stopped opctl containers",
		Long: "Removes opctl-managed containers that are not currently running " +
			"(created, exited, dead, restarting). Mirrors `docker container prune` semantics; " +
			"prompts before deleting unless --force/-f is passed.",
		Example: "# Prompt before removing stopped opctl-managed containers.\n" +
			"opctl container prune\n\n" +
			"# Skip the prompt.\n" +
			"opctl container prune --force",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := euid0.Ensure(); err != nil {
				return err
			}

			containers, err := (*containerRuntime).ListContainersByLabels(ctx, nil)
			if err != nil {
				return err
			}

			return pruneContainers(
				ctx,
				cmd.OutOrStdout(),
				containers,
				force,
				promptContainerPruneConfirmation,
				func(ctx context.Context, target string) error {
					return (*containerRuntime).DeleteContainer(ctx, target)
				},
			)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")

	return cmd
}

// pruneContainers selects the non-running containers, optionally prompts the
// user, deletes each via deleteContainer, and writes progress to stdout.
// All side-effecting dependencies are injected so the logic is unit-testable.
func pruneContainers(
	ctx context.Context,
	stdout io.Writer,
	containers []containerruntime.Container,
	force bool,
	confirm containerPruneConfirmer,
	deleteContainer func(ctx context.Context, target string) error,
) error {
	prunable := selectContainersToPrune(containers)
	if len(prunable) == 0 {
		_, _ = fmt.Fprintln(stdout, "no stopped opctl-managed containers to prune")
		return nil
	}

	if !force {
		_, _ = fmt.Fprintf(stdout, "WARNING! This will remove %d stopped opctl-managed container(s):\n", len(prunable))
		for _, container := range prunable {
			_, _ = fmt.Fprintf(stdout, "  - %s (%s)\n",
				formatContainerDisplayName(container),
				formatContainerStatus(container),
			)
		}

		answer, err := confirm("Are you sure you want to continue? [y/N]: ")
		if err != nil {
			return err
		}
		if !isAffirmative(answer) {
			_, _ = fmt.Fprintln(stdout, "prune cancelled")
			return nil
		}
	}

	removed := 0
	for _, container := range prunable {
		target, err := getContainerDeleteTarget(container)
		if err != nil {
			return err
		}
		if err := deleteContainer(ctx, target); err != nil {
			return fmt.Errorf("failed to delete %s: %w", formatContainerDisplayName(container), err)
		}
		removed++
	}

	_, _ = fmt.Fprintf(stdout, "removed %d stopped opctl-managed container(s)\n", removed)
	return nil
}

// selectContainersToPrune returns containers that are NOT running. An empty
// State is treated as running to be conservative — we don't have enough info
// to know it's stopped.
func selectContainersToPrune(containers []containerruntime.Container) []containerruntime.Container {
	prunable := []containerruntime.Container{}
	for _, container := range containers {
		if container.State == "" || container.State == runningContainerState {
			continue
		}
		prunable = append(prunable, container)
	}

	return prunable
}

func isAffirmative(answer string) bool {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	}

	return false
}

func promptContainerPruneConfirmation(prompt string) (string, error) {
	_, _ = fmt.Fprint(os.Stdout, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	return strings.TrimRight(line, "\r\n"), nil
}
