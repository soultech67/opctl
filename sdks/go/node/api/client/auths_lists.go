package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/urltemplates"
)

func (c apiClient) ListAuths(
	ctx context.Context,
) ([]model.Auth, error) {

	reqURL := c.baseURL
	reqURL.Path = path.Join(reqURL.Path, urltemplates.Auths_Lists)

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		reqURL.String(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if http.StatusOK != httpResp.StatusCode {
		return nil, errors.New(string(bodyBytes))
	}

	auths := []model.Auth{}
	if err := json.Unmarshal(bodyBytes, &auths); err != nil {
		return nil, err
	}

	return auths, nil
}
