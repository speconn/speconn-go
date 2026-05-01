package speconn

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"

	"github.com/specodec/specodec-go"
)

type Request[T any] struct {
	Msg    *T
	Header http.Header
}

func NewRequest[T any](msg *T) *Request[T] {
	return &Request[T]{Msg: msg, Header: make(http.Header)}
}

type Response[T any] struct {
	Msg    *T
	Header http.Header
}

type SpeconnClient[Req any, Res any] struct {
	baseURL   string
	path      string
	transport SpeconnTransport
}

func NewClient[Req any, Res any](baseURL, path string) *SpeconnClient[Req, Res] {
	return NewClientWithTransport[Req, Res](baseURL, path, defaultTransport)
}

func NewClientWithTransport[Req any, Res any](baseURL, path string, transport SpeconnTransport) *SpeconnClient[Req, Res] {
	return &SpeconnClient[Req, Res]{
		baseURL:   baseURL,
		path:      path,
		transport: transport,
	}
}

func getContentType(h http.Header) string {
	return h.Get("Content-Type")
}

func getAccept(h http.Header) string {
	accept := h.Get("Accept")
	if accept == "" {
		accept = getContentType(h)
	}
	return accept
}

func (c *SpeconnClient[Req, Res]) doPost(url string, body []byte, reqHeader http.Header) (int, []byte, http.Header, error) {
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, NewError(CodeInternal, err.Error())
	}
	for k, vs := range reqHeader {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	resp, err := c.transport.Do(httpReq)
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

func (c *SpeconnClient[Req, Res]) Call(req *Request[Req], reqCodec specodec.SpecCodec[Req], resCodec specodec.SpecCodec[Res]) (*Response[Res], error) {
	contentType := getContentType(req.Header)
	accept := getAccept(req.Header)

	reqFmt := ExtractFormat(contentType)
	resFmt := ExtractFormat(accept)

	result := specodec.Respond(reqCodec, req.Msg, reqFmt)
	req.Header.Set("Content-Type", FormatToMime(reqFmt, false))
	req.Header.Set("Accept", FormatToMime(resFmt, false))

	status, respBody, respHeader, err := c.doPost(c.baseURL+c.path, result.Body, req.Header)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		return nil, DecodeError(respBody, resFmt)
	}

	res := specodec.Dispatch(resCodec, respBody, resFmt)
	return &Response[Res]{Msg: res, Header: respHeader}, nil
}

func (c *SpeconnClient[Req, Res]) Stream(req *Request[Req], reqCodec specodec.SpecCodec[Req], resCodec specodec.SpecCodec[Res]) ([]*Response[Res], error) {
	reqFmt := ExtractFormat(getContentType(req.Header))
	resFmt := ExtractFormat(getAccept(req.Header))
	accept := FormatToMime(resFmt, true)

	result := specodec.Respond(reqCodec, req.Msg, reqFmt)

	streamHeader := req.Header.Clone()
	streamHeader.Set("Content-Type", FormatToMime(reqFmt, true))
	streamHeader.Set("Accept", accept)
	if streamHeader.Get("Connect-Protocol-Version") == "" {
		streamHeader.Set("Connect-Protocol-Version", "1")
	}

	status, respBody, _, err := c.doPost(c.baseURL+c.path, result.Body, streamHeader)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		return nil, DecodeError(respBody, resFmt)
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
			if len(payload) > 0 {
				return results, DecodeError(payload, resFmt)
			}
			break
		}
		res := specodec.Dispatch(resCodec, payload, resFmt)
		results = append(results, &Response[Res]{Msg: res, Header: make(http.Header)})
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