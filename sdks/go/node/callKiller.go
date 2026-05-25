package node

import (
	"context"
	"log"
	"time"

	"github.com/opctl/opctl/sdks/go/model"

	"github.com/opctl/opctl/sdks/go/node/containerruntime"
	"github.com/opctl/opctl/sdks/go/node/pubsub"
)

//counterfeiter:generate -o internal/fakes/callKiller.go . callKiller
type callKiller interface {
	Kill(
		ctx context.Context,
		callID string,
		rootCallID string,
	)
}

func newCallKiller(
	stateStore stateStore,
	containerRuntime containerruntime.ContainerRuntime,
	eventPublisher pubsub.EventPublisher,
) callKiller {
	return _callKiller{
		stateStore:       stateStore,
		containerRuntime: containerRuntime,
		eventPublisher:   eventPublisher,
	}
}

type _callKiller struct {
	stateStore       stateStore
	containerRuntime containerruntime.ContainerRuntime
	eventPublisher   pubsub.EventPublisher
}

func (ckr _callKiller) Kill(
	ctx context.Context,
	callID string,
	rootCallID string,
) {
	log.Printf("[opctl kill] callKiller.Kill enter: callID=%s rootCallID=%s", callID, rootCallID)
	startedAt := time.Now()

	if err := ckr.containerRuntime.DeleteContainerIfExists(ctx, callID); err != nil {
		log.Printf("[opctl kill] DeleteContainerIfExists failed in %s: callID=%s err=%v",
			time.Since(startedAt), callID, err)
	}

	children := ckr.stateStore.ListWithParentID(callID)
	log.Printf("[opctl kill] callKiller.Kill propagating to %d child(ren): callID=%s elapsed=%s",
		len(children), callID, time.Since(startedAt))

	for _, childCallGraph := range children {
		ckr.eventPublisher.Publish(
			model.Event{
				CallKillRequested: &model.CallKillRequested{
					Request: model.KillOpReq{
						OpID:       childCallGraph.ID,
						RootCallID: rootCallID,
					},
				},
				Timestamp: time.Now().UTC(),
			},
		)
	}

	log.Printf("[opctl kill] callKiller.Kill exit in %s: callID=%s", time.Since(startedAt), callID)
}
