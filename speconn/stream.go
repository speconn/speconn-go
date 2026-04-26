package speconn

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
)

const (
	flagCompressed byte = 1 << 0
	flagEndStream  byte = 1 << 1

	headerContentType     = "Content-Type"
	headerConnectVersion  = "Connect-Protocol-Version"
	contentTypeConnectJSON = "application/connect+json"
)

func NewServerStreamHandler[Req any, Res any](path string, fn func(req *Req, send func(*Res) error) error) (string, http.Handler) {
	return path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeJSONError(w, CodeInternal, "streaming not supported", 500)
			return
		}

		var req Req
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, CodeInternal, err.Error(), 500)
				return
			}
		}

		w.Header().Set(headerContentType, contentTypeConnectJSON)
		w.Header().Set(headerConnectVersion, "1")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		s := &serverStream{w: w, flusher: flusher}
		send := func(msg *Res) error {
			return s.sendJSON(msg)
		}
		if err := fn(&req, send); err != nil {
			s.sendEndStream(err)
			return
		}
		s.sendEndStream(nil)
	})
}

type serverStream struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (s *serverStream) sendJSON(msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.writeEnvelope(0, payload)
}

func (s *serverStream) writeEnvelope(flags byte, payload []byte) error {
	header := make([]byte, 5)
	header[0] = flags
	binary.BigEndian.PutUint32(header[1:5], uint32(len(payload)))
	if _, err := s.w.Write(header); err != nil {
		return err
	}
	if _, err := s.w.Write(payload); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

func (s *serverStream) sendEndStream(err error) {
	trailer := map[string]any{}
	if err != nil {
		code := CodeInternal
		msg := err.Error()
		if se, ok := err.(*Error); ok {
			code = se.Code
			msg = se.Message
		}
		trailer["error"] = map[string]string{"code": string(code), "message": msg}
	}
	payload, _ := json.Marshal(trailer)
	s.writeEnvelope(flagEndStream, payload)
}

type ClientStream[Res any] struct {
	body   io.ReadCloser
	reader *frameReader
}

type frameReader struct {
	body io.Reader
	buf  [5]byte
}

func (r *frameReader) next() (flags byte, payload []byte, err error) {
	if _, err = io.ReadFull(r.body, r.buf[:]); err != nil {
		return 0, nil, err
	}
	flags = r.buf[0]
	length := binary.BigEndian.Uint32(r.buf[1:5])
	payload = make([]byte, length)
	if length > 0 {
		if _, err = io.ReadFull(r.body, payload); err != nil {
			return 0, nil, err
		}
	}
	return flags, payload, nil
}

func NewStreamClient[Req any, Res any](httpClient *http.Client, baseURL, path string, req *Req) (*ClientStream[Res], error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set(headerContentType, contentTypeConnectJSON)
	httpReq.Header.Set(headerConnectVersion, "1")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, &Error{Code: CodeUnavailable, Message: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		var e Error
		json.NewDecoder(resp.Body).Decode(&e)
		resp.Body.Close()
		return nil, &e
	}
	return &ClientStream[Res]{
		body:   resp.Body,
		reader: &frameReader{body: resp.Body},
	}, nil
}

func (cs *ClientStream[Res]) Receive() (*Res, error) {
	for {
		flags, payload, err := cs.reader.next()
		if err != nil {
			return nil, err
		}
		if flags&flagEndStream != 0 {
			var trailer struct {
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			json.Unmarshal(payload, &trailer)
			if trailer.Error != nil {
				return nil, &Error{Code: Code(trailer.Error.Code), Message: trailer.Error.Message}
			}
			return nil, io.EOF
		}
		var msg Res
		if err := json.Unmarshal(payload, &msg); err != nil {
			return nil, err
		}
		return &msg, nil
	}
}

func (cs *ClientStream[Res]) Close() error {
	return cs.body.Close()
}
