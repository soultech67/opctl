package removes

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
	removeAuthReq := model.RemoveAuthReq{}
	if err := json.NewDecoder(httpReq.Body).Decode(&removeAuthReq); err != nil {
		http.Error(httpResp, err.Error(), http.StatusBadRequest)
		return
	}

	if err := hdlr.node.RemoveAuth(httpReq.Context(), removeAuthReq); err != nil {
		http.Error(httpResp, err.Error(), http.StatusInternalServerError)
		return
	}

	httpResp.WriteHeader(http.StatusNoContent)
}
