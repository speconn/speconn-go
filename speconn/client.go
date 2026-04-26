package speconn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client[Req any, Res any] struct {
	httpClient *http.Client
	baseURL    string
	path       string
}

func NewClient[Req any, Res any](httpClient *http.Client, baseURL, path string) *Client[Req, Res] {
	return &Client[Req, Res]{
		httpClient: httpClient,
		baseURL:    baseURL,
		path:       path,
	}
}

func (c *Client[Req, Res]) Call(req *Req) (*Res, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("speconn: marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+c.path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &Error{Code: CodeUnavailable, Message: err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{Code: CodeInternal, Message: "read response: " + err.Error()}
	}

	if resp.StatusCode != http.StatusOK {
		var speconnErr Error
		if err := json.Unmarshal(respBody, &speconnErr); err == nil {
			return nil, &speconnErr
		}
		return nil, &Error{Code: CodeUnknown, Message: string(respBody)}
	}

	var res Res
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("speconn: unmarshal response: %w", err)
	}
	return &res, nil
}
