package node

import (
	"context"
	"time"

	"github.com/opctl/opctl/sdks/go/model"
)

func (this core) RemoveAuth(
	ctx context.Context,
	req model.RemoveAuthReq,
) error {
	this.pubSub.Publish(
		model.Event{
			AuthRemoved: &model.AuthRemoved{
				Resources: req.Resources,
			},
			Timestamp: time.Now().UTC(),
		},
	)
	return nil
}
