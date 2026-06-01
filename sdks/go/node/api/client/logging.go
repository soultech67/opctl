package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/gorilla/websocket"
	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/urltemplates"
	"github.com/sethgrid/pester"
)

// LogControlClient controls the daemon's logging configuration at runtime via
// the node's /logging endpoint. It is intentionally separate from the node.Node
// interface: logging is an operational concern of the daemon process, not part
// of the op-running domain.
type LogControlClient interface {
	// GetLogState returns the daemon's current logging configuration.
	GetLogState(ctx context.Context) (model.LogState, error)
	// SetLogState applies a partial update (level and/or enablement) and returns
	// the resulting configuration.
	SetLogState(ctx context.Context, req model.SetLogStateReq) (model.LogState, error)
}

// NewLogControlClient returns a LogControlClient targeting the node API at
// baseURL (e.g. http://127.0.0.1:42224/api).
func NewLogControlClient(baseURL url.URL) LogControlClient {
	return &apiClient{
		baseURL:    baseURL,
		httpClient: pester.New(),
		wsDialer:   websocket.DefaultDialer,
	}
}

func (c apiClient) GetLogState(
	ctx context.Context,
) (model.LogState, error) {
	reqURL := c.baseURL
	reqURL.Path = path.Join(reqURL.Path, urltemplates.Logging)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return model.LogState{}, err
	}

	return c.doLogStateReq(httpReq)
}

func (c apiClient) SetLogState(
	ctx context.Context,
	req model.SetLogStateReq,
) (model.LogState, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return model.LogState{}, err
	}

	reqURL := c.baseURL
	reqURL.Path = path.Join(reqURL.Path, urltemplates.Logging)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL.String(), bytes.NewBuffer(reqBytes))
	if err != nil {
		return model.LogState{}, err
	}

	return c.doLogStateReq(httpReq)
}

func (c apiClient) doLogStateReq(
	httpReq *http.Request,
) (model.LogState, error) {
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return model.LogState{}, err
	}
	// don't leak resources
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return model.LogState{}, err
	}

	if http.StatusOK != httpResp.StatusCode {
		return model.LogState{}, errors.New(string(bodyBytes))
	}

	logState := model.LogState{}
	if err := json.Unmarshal(bodyBytes, &logState); err != nil {
		return model.LogState{}, err
	}

	return logState, nil
}
