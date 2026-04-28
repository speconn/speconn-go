package speconn

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Request wraps a message with headers, following ConnectRPC's pattern.
type Request[T any] struct {
	Msg    *T
	Header http.Header
}

// NewRequest creates a new Request with an empty header map.
func NewRequest[T any](msg *T) *Request[T] {
	return &Request[T]{Msg: msg, Header: make(http.Header)}
}

// Response wraps a response message with headers.
type Response[T any] struct {
	Msg    *T
	Header http.Header
}

type SpeconnClient[Req any, Res any] struct {
	baseURL    string
	path       string
	httpClient HttpClient
}

func NewClient[Req any, Res any](baseURL, path string) *SpeconnClient[Req, Res] {
	return NewClientWithHttpClient[Req, Res](baseURL, path, defaultHttpClient)
}

func NewClientWithHttpClient[Req any, Res any](baseURL, path string, httpClient HttpClient) *SpeconnClient[Req, Res] {
	return &SpeconnClient[Req, Res]{
		baseURL:    baseURL,
		path:       path,
		httpClient: httpClient,
	}
}

func (c *SpeconnClient[Req, Res]) doPost(url, contentType string, body []byte, reqHeader http.Header) (int, []byte, http.Header, error) {
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, NewError(CodeInternal, err.Error())
	}
	httpReq.Header.Set("Content-Type", contentType)
	for k, vs := range reqHeader {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, nil, nil, NewError(CodeUnavailable, err.Error())
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, nil, NewError(CodeInternal, "read response: "+err.Error())
	}
	return resp.StatusCode, respBody, resp.Header, nil
}

func (c *SpeconnClient[Req, Res]) Call(req *Request[Req]) (*Response[Res], error) {
	body, err := json.Marshal(req.Msg)
	if err != nil {
		return nil, fmt.Errorf("speconn: marshal request: %w", err)
	}

	status, respBody, respHeader, err := c.doPost(c.baseURL+c.path, "application/json", body, req.Header)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		var speconnErr SpeconnError
		if err := json.Unmarshal(respBody, &speconnErr); err == nil && speconnErr.Code != "" {
			return nil, &speconnErr
		}
		return nil, NewError(CodeFromHTTPStatus(status), string(respBody))
	}

	var res Res
	if err := json.Unmarshal(respBody, &res); err != nil {
		return nil, fmt.Errorf("speconn: unmarshal response: %w", err)
	}
	return &Response[Res]{Msg: &res, Header: respHeader}, nil
}

func (c *SpeconnClient[Req, Res]) Stream(req *Request[Req]) ([]*Response[Res], error) {
	body, err := json.Marshal(req.Msg)
	if err != nil {
		return nil, fmt.Errorf("speconn: marshal request: %w", err)
	}

	streamHeader := req.Header.Clone()
	streamHeader.Set("Connect-Protocol-Version", "1")

	status, respBody, _, err := c.doPost(c.baseURL+c.path, "application/connect+json", body, streamHeader)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		var speconnErr SpeconnError
		if err := json.Unmarshal(respBody, &speconnErr); err == nil && speconnErr.Code != "" {
			return nil, &speconnErr
		}
		return nil, NewError(CodeFromHTTPStatus(status), string(respBody))
	}

	var results []*Response[Res]
	reader := &frameReader{data: respBody}

	for {
		flags, payload, err := reader.next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, NewError(CodeDataLoss, "reading stream frame: "+err.Error())
		}
		if flags&FlagEndStream != 0 {
			var trailer struct {
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			json.Unmarshal(payload, &trailer)
			if trailer.Error != nil {
				return results, NewError(CodeFromString(trailer.Error.Code), trailer.Error.Message)
			}
			break
		}
		var msg Res
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, NewError(CodeInternal, "unmarshal stream message: "+err.Error())
		}
		results = append(results, &Response[Res]{Msg: &msg, Header: make(http.Header)})
	}

	return results, nil
}

type frameReader struct {
	data []byte
	off  int
}

func (r *frameReader) next() (flags byte, payload []byte, err error) {
	if r.off+5 > len(r.data) {
		return 0, nil, io.EOF
	}
	header := r.data[r.off : r.off+5]
	flags = header[0]
	length := binary.BigEndian.Uint32(header[1:5])
	r.off += 5

	if uint32(len(r.data)-r.off) < length {
		return 0, nil, fmt.Errorf("speconn: truncated frame")
	}
	payload = r.data[r.off : r.off+int(length)]
	r.off += int(length)
	return flags, payload, nil
}
