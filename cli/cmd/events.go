package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/opctl/opctl/cli/internal/clioutput"
	"github.com/opctl/opctl/cli/internal/nodeprovider/local"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/spf13/cobra"
)

func newEventsCmd(
	cliOutput clioutput.CliOutput,
	nodeConfig *local.NodeConfig,
) *cobra.Command {
	var (
		since string
		roots []string
	)

	eventsCmd := &cobra.Command{
		Args:  cobra.MaximumNArgs(0),
		Use:   "events",
		Short: "Stream events from an opctl node",
		Long: `If an opctl node isn't reachable, one will be started automatically. Events are delivered
over a websocket connection. Past events are replayed when streaming starts. As new
events occur, they are streamed in realtime.

Use --since and/or --roots to replay just a subset of the durable history — e.g. one
op's output after it (or the daemon) has stopped:
  opctl events --roots <rootCallId>
  opctl events --since 24h
`,
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			filter := model.EventFilter{}
			if since != "" {
				sinceTime, err := parseSince(since)
				if err != nil {
					return err
				}
				filter.Since = &sinceTime
			}
			if len(roots) > 0 {
				filter.Roots = roots
			}

			np, err := local.New(*nodeConfig)
			if err != nil {
				return err
			}

			node, err := np.CreateNodeIfNotExists(ctx)
			if err != nil {
				return err
			}

			eventChannel, err := node.GetEventStream(
				ctx,
				&model.GetEventStreamReq{Filter: filter},
			)
			if err != nil {
				return err
			}

			for {
				event, isEventChannelOpen := <-eventChannel
				if !isEventChannelOpen {
					return errors.New("connection to event stream lost")
				}

				cliOutput.Event(&event)
			}
		},
	}

	eventsCmd.Flags().StringVar(
		&since,
		"since",
		"",
		"only show events at or after this point — a duration relative to now (e.g. 90m, 24h) or an RFC3339 timestamp",
	)
	eventsCmd.Flags().StringSliceVar(
		&roots,
		"roots",
		nil,
		"only show events under these root call IDs (comma-separated or repeated); e.g. the root op id of an opctl run",
	)

	return eventsCmd
}

// parseSince accepts either a Go duration relative to now (e.g. "90m", "24h") or
// an RFC3339 timestamp, returning the absolute lower bound for an event filter.
func parseSince(since string) (time.Time, error) {
	if d, err := time.ParseDuration(since); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf(
		"invalid --since %q: want a duration (e.g. 90m, 24h) or an RFC3339 timestamp (e.g. 2026-05-31T12:00:00Z)",
		since,
	)
}
