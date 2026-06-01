package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path"

	"github.com/opctl/opctl/sdks/go/model"
	"github.com/opctl/opctl/sdks/go/node/api/urltemplates"
)

func (c apiClient) RemoveAuth(
	ctx context.Context,
	req model.RemoveAuthReq,
) error {

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return err
	}

	reqURL := c.baseURL
	reqURL.Path = path.Join(reqURL.Path, urltemplates.Auths_Removes)

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		reqURL.String(),
		bytes.NewBuffer(reqBytes),
	)
	if err != nil {
		return err
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if http.StatusOK != httpResp.StatusCode && http.StatusNoContent != httpResp.StatusCode {
		return errors.New(string(bodyBytes))
	}

	return nil
}
