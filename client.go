package speconn

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"iter"
	"net/http"
	"time"

	"github.com/specodec/specodec-go"
)

type CallOptions struct {
	Headers   http.Header
	TimeoutMs int64
}

func NewCallOptions() *CallOptions {
	return &CallOptions{Headers: make(http.Header)}
}

type Response[T any] struct {
	Msg      *T
	Headers  http.Header
	Trailers http.Header
}

type StreamResponse[T any] struct {
	Headers  http.Header
	Trailers http.Header
	msgs     []*T
}

func (s *StreamResponse[T]) All() iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		for _, msg := range s.msgs {
			if !yield(msg, nil) {
				return
			}
		}
	}
}

func (s *StreamResponse[T]) addMsg(msg *T) {
	s.msgs = append(s.msgs, msg)
}

func (s *StreamResponse[T]) setTrailers(t http.Header) {
	s.Trailers = t
}

func splitHeadersTrailers(rawHeaders http.Header) (headers, trailers http.Header) {
	headers = make(http.Header)
	trailers = make(http.Header)
	for k, vs := range rawHeaders {
		if len(k) > 8 && k[:8] == "Trailer-" {
			trailers[k[8:]] = vs
		} else {
			headers[k] = vs
		}
	}
	return headers, trailers
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

func (c *SpeconnClient[Req, Res]) doPost(url string, body []byte, reqHeader http.Header, timeoutMs int64) (int, []byte, http.Header, error) {
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, NewError(CodeInternal, err.Error())
	}
	for k, vs := range reqHeader {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}
	if timeoutMs > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
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

func (c *SpeconnClient[Req, Res]) Call(req *Req, reqCodec specodec.SpecCodec[Req], resCodec specodec.SpecCodec[Res], opts *CallOptions) (*Response[Res], error) {
	if opts == nil {
		opts = NewCallOptions()
	}
	contentType := getContentType(opts.Headers)
	accept := getAccept(opts.Headers)

	reqFmt := ExtractFormat(contentType)
	resFmt := ExtractFormat(accept)

	result := specodec.Respond(reqCodec, req, reqFmt)
	opts.Headers.Set("Content-Type", FormatToMime(reqFmt, false))
	opts.Headers.Set("Accept", FormatToMime(resFmt, false))

	status, respBody, respHeader, err := c.doPost(c.baseURL+c.path, result.Body, opts.Headers, opts.TimeoutMs)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		return nil, DecodeError(respBody, resFmt)
	}

	res := specodec.Dispatch(resCodec, respBody, resFmt)
	headers, trailers := splitHeadersTrailers(respHeader)
	return &Response[Res]{Msg: res, Headers: headers, Trailers: trailers}, nil
}

func (c *SpeconnClient[Req, Res]) Stream(req *Req, reqCodec specodec.SpecCodec[Req], resCodec specodec.SpecCodec[Res], opts *CallOptions) (*StreamResponse[Res], error) {
	if opts == nil {
		opts = NewCallOptions()
	}
	reqFmt := ExtractFormat(getContentType(opts.Headers))
	resFmt := ExtractFormat(getAccept(opts.Headers))
	accept := FormatToMime(resFmt, true)

	result := specodec.Respond(reqCodec, req, reqFmt)

	streamHeader := opts.Headers.Clone()
	streamHeader.Set("Content-Type", FormatToMime(reqFmt, true))
	streamHeader.Set("Accept", accept)
	if streamHeader.Get("Connect-Protocol-Version") == "" {
		streamHeader.Set("Connect-Protocol-Version", "1")
	}

	status, respBody, respHeader, err := c.doPost(c.baseURL+c.path, result.Body, streamHeader, opts.TimeoutMs)
	if err != nil {
		return nil, err
	}

	if status != 200 {
		return nil, DecodeError(respBody, resFmt)
	}

	headers, trailers := splitHeadersTrailers(respHeader)
	streamResp := &StreamResponse[Res]{Headers: headers, Trailers: trailers}

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
				return streamResp, DecodeError(payload, resFmt)
			}
			break
		}
		res := specodec.Dispatch(resCodec, payload, resFmt)
		streamResp.addMsg(res)
	}

	streamResp.setTrailers(trailers)
	return streamResp, nil
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