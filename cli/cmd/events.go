package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
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

Use --since (-t) and/or --roots to replay just a subset of the durable history — e.g.
one op's output after it (or the daemon) has stopped:
  opctl events -t 3d
  opctl events --roots <rootCallId>
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

	eventsCmd.Flags().StringVarP(
		&since,
		"since",
		"t",
		"",
		"only show events at or after this point — a duration back from now (s/m/h/d units, e.g. 30s, 90m, 24h, 3d) or an RFC3339 timestamp; default: replay the entire history",
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
	if d, err := parseSinceDuration(since); err == nil {
		return time.Now().Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, since); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf(
		"invalid --since %q: want a duration back from now with s/m/h/d units (e.g. 30s, 90m, 24h, 3d) or an RFC3339 timestamp (e.g. 2026-05-31T12:00:00Z)",
		since,
	)
}

// sinceDaysRegexp matches a leading whole-day component, e.g. "3d" or "1d12h".
var sinceDaysRegexp = regexp.MustCompile(`^(\d+)d(.*)$`)

// parseSinceDuration parses a Go duration, additionally accepting a leading
// day component ("3d", "1d12h"), which time.ParseDuration doesn't support.
func parseSinceDuration(s string) (time.Duration, error) {
	m := sinceDaysRegexp.FindStringSubmatch(s)
	if m == nil {
		return time.ParseDuration(s)
	}

	days, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, err
	}
	d := time.Duration(days) * 24 * time.Hour

	if m[2] != "" {
		rest, err := time.ParseDuration(m[2])
		if err != nil {
			return 0, err
		}
		d += rest
	}

	return d, nil
}
