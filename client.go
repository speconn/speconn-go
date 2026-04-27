package speconn

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type SpeconnClient[Req any, Res any] struct {
	baseURL   string
	path      string
	transport Transport
}

func NewClient[Req any, Res any](baseURL, path string) *SpeconnClient[Req, Res] {
	return NewClientWithTransport[Req, Res](baseURL, path, NewDefaultTransport())
}

func NewClientWithTransport[Req any, Res any](baseURL, path string, transport Transport) *SpeconnClient[Req, Res] {
	return &SpeconnClient[Req, Res]{
		baseURL:   baseURL,
		path:      path,
		transport: transport,
	}
}

func (c *SpeconnClient[Req, Res]) Call(req *Req) (*Res, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("speconn: marshal request: %w", err)
	}

	resp, err := c.transport.Post(c.baseURL+c.path, "application/json", body, nil)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		var speconnErr SpeconnError
		if err := json.Unmarshal(resp.Body, &speconnErr); err == nil && speconnErr.Code != "" {
			return nil, &speconnErr
		}
		return nil, NewError(CodeFromHTTPStatus(resp.Status), string(resp.Body))
	}

	var res Res
	if err := json.Unmarshal(resp.Body, &res); err != nil {
		return nil, fmt.Errorf("speconn: unmarshal response: %w", err)
	}
	return &res, nil
}

func (c *SpeconnClient[Req, Res]) Stream(req *Req) ([]Res, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("speconn: marshal request: %w", err)
	}

	headers := map[string]string{
		"Connect-Protocol-Version": "1",
	}

	resp, err := c.transport.Post(c.baseURL+c.path, "application/connect+json", body, headers)
	if err != nil {
		return nil, err
	}

	if resp.Status != 200 {
		var speconnErr SpeconnError
		if err := json.Unmarshal(resp.Body, &speconnErr); err == nil && speconnErr.Code != "" {
			return nil, &speconnErr
		}
		return nil, NewError(CodeFromHTTPStatus(resp.Status), string(resp.Body))
	}

	var results []Res
	reader := &frameReader{data: resp.Body}

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
		results = append(results, msg)
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
