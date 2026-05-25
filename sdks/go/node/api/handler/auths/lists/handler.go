package lists

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"net/http"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node"
)

//counterfeiter:generate -o fakes/handler.go . Handler
type Handler interface {
	Handle(
		res http.ResponseWriter,
		req *http.Request,
	)
}

// NewHandler returns an initialized Handler instance
func NewHandler(
	node node.Node,
) Handler {
	return _handler{
		node: node,
	}
}

type _handler struct {
	node node.Node
}

func (hdlr _handler) Handle(
	httpResp http.ResponseWriter,
	httpReq *http.Request,
) {
	auths, err := hdlr.node.ListAuths(httpReq.Context())
	if err != nil {
		http.Error(httpResp, err.Error(), http.StatusInternalServerError)
		return
	}
	if auths == nil {
		auths = []model.Auth{}
	}

	httpResp.Header().Set("Content-Type", "application/json")
	httpResp.WriteHeader(http.StatusOK)
	json.NewEncoder(httpResp).Encode(auths)
}
