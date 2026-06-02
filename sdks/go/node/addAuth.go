package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opctl/opctl/sdks/go/model"
)

func (this core) AddAuth(
	ctx context.Context,
	req model.AddAuthReq,
) error {
	this.pubSub.Publish(
		model.Event{
			AuthAdded: &model.AuthAdded{
				Auth: model.Auth{
					Creds:     req.Creds,
					Resources: req.Resources,
				},
			},
			Timestamp: time.Now().UTC(),
		},
	)

	// The event above is applied to the durable store asynchronously by the
	// stateStore goroutine, so returning here would let `opctl auth ls` (or an
	// auth-dependent pull) run before the write lands and see nothing. Wait
	// until the specific entry is actually readable. EqualFold guards against a
	// broader prefix entry matching first (TryGetAuth returns the longest match,
	// which is this exact entry once stored).
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if auth := this.stateStore.TryGetAuth(req.Resources); auth != nil &&
			strings.EqualFold(auth.Resources, req.Resources) {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for stored auth for %q to become durable", req.Resources)
		case <-ticker.C:
		}
	}
}
