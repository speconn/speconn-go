package speconn

import (
	"bytes"
	"io"
	"net/http"
)

type TransportResponse struct {
	Status  int
	Body    []byte
	Headers http.Header
}

type Transport interface {
	Post(url string, contentType string, body []byte, headers map[string]string) (*TransportResponse, error)
}

type DefaultTransport struct {
	Client *http.Client
}

func NewDefaultTransport() *DefaultTransport {
	return &DefaultTransport{Client: http.DefaultClient}
}

func (t *DefaultTransport) Post(url string, contentType string, body []byte, headers map[string]string) (*TransportResponse, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, NewError(CodeInternal, err.Error())
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := t.Client.Do(req)
	if err != nil {
		return nil, NewError(CodeUnavailable, err.Error())
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewError(CodeInternal, "read response: "+err.Error())
	}

	return &TransportResponse{
		Status:  resp.StatusCode,
		Body:    respBody,
		Headers: resp.Header,
	}, nil
}
