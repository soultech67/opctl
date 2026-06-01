package node

import (
	"context"
	"log"
	"time"

	"github.com/opctl/opctl/sdks/go/model"
)

func (this core) KillOp(
	ctx context.Context,
	req model.KillOpReq,
) error {
	// Kill-path instrumentation: low-volume (fires only on user-initiated kill
	// or parallel-needs cascade) and high-value when diagnosing "Ctrl+C left
	// Docker in a bad state" — see ../containerruntime/docker/instrumentation.go.
	log.Printf("[opctl kill] KillOp received: opID=%s rootCallID=%s", req.OpID, req.RootCallID)

	// killing an op is async
	this.pubSub.Publish(
		model.Event{
			CallKillRequested: &model.CallKillRequested{
				Request: req,
			},
			Timestamp: time.Now().UTC(),
		},
	)
	return nil
}
