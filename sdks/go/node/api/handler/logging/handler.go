// Package logging implements the node API handler for inspecting and changing
// the daemon's logging configuration at runtime.
package logging

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"encoding/json"
	"net/http"

	"github.com/opctl/opctl/sdks/go/internal/urlpath"
	"github.com/opctl/opctl/sdks/go/model"
	nodelogging "github.com/opctl/opctl/sdks/go/node/logging"
)

//counterfeiter:generate -o fakes/handler.go . Handler
type Handler interface {
	Handle(
		httpResp http.ResponseWriter,
		httpReq *http.Request,
	)
}

// NewHandler returns an initialized Handler instance. Logging state lives in
// the daemon process (the node/logging package), so this handler manipulates it
// directly rather than going through the node interface.
func NewHandler() Handler {
	return _handler{}
}

type _handler struct {
}

func (hdlr _handler) Handle(
	httpResp http.ResponseWriter,
	httpReq *http.Request,
) {
	pathSegment, err := urlpath.NextSegment(httpReq.URL)
	if err != nil {
		http.Error(httpResp, err.Error(), http.StatusBadRequest)
		return
	}

	if pathSegment != "" {
		http.NotFoundHandler().ServeHTTP(httpResp, httpReq)
		return
	}

	switch httpReq.Method {
	case http.MethodGet:
		writeState(httpResp)
	case http.MethodPost, http.MethodPut:
		hdlr.apply(httpResp, httpReq)
	default:
		http.Error(httpResp, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (hdlr _handler) apply(
	httpResp http.ResponseWriter,
	httpReq *http.Request,
) {
	req := model.SetLogStateReq{}
	if err := json.NewDecoder(httpReq.Body).Decode(&req); err != nil {
		http.Error(httpResp, err.Error(), http.StatusBadRequest)
		return
	}

	// validate the level before mutating anything so a bad request leaves state
	// unchanged.
	if req.Level != nil {
		if err := nodelogging.SetLevel(*req.Level); err != nil {
			http.Error(httpResp, err.Error(), http.StatusBadRequest)
			return
		}
	}

	if req.Enabled != nil {
		nodelogging.SetEnabled(*req.Enabled)
	}

	writeState(httpResp)
}

func writeState(httpResp http.ResponseWriter) {
	// marshal before committing the status so an encode failure surfaces as a
	// 500 rather than a 200 with a truncated body.
	body, err := json.Marshal(nodelogging.State())
	if err != nil {
		http.Error(httpResp, err.Error(), http.StatusInternalServerError)
		return
	}

	httpResp.Header().Set("Content-Type", "application/json; charset=UTF-8")
	httpResp.WriteHeader(http.StatusOK)
	httpResp.Write(body)
}
