package node

import (
	"context"

	"github.com/opctl/opctl/sdks/go/model"
)

func (this core) ListAuths(
	ctx context.Context,
) ([]model.Auth, error) {
	return this.stateStore.ListAuths(), nil
}
