package speconn

import (
	"io"
	"net/http"
	"strconv"

	"github.com/specodec/specodec-go"
)

type unaryHandler[Req any, Res any] struct {
	path     string
	handler  func(*SpeconnContext, *Req) (*Res, error)
	reqCodec specodec.SpecCodec[Req]
	resCodec specodec.SpecCodec[Res]
}

func NewUnaryHandler[Req any, Res any](
	path string,
	handler func(*SpeconnContext, *Req) (*Res, error),
	reqCodec specodec.SpecCodec[Req],
	resCodec specodec.SpecCodec[Res],
) (http.Handler, error) {
	return &unaryHandler[Req, Res]{
		path:     path,
		handler:  handler,
		reqCodec: reqCodec,
		resCodec: resCodec,
	}, nil
}

func (h *unaryHandler[Req, Res]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	contentType := r.Header.Get("Content-Type")
	accept := r.Header.Get("Accept")
	if accept == "" {
		accept = contentType
	}

	timeoutMs := 0
	if timeoutHeader := r.Header.Get("Speconn-Timeout-Ms"); timeoutHeader != "" {
		if parsed, err := strconv.Atoi(timeoutHeader); err == nil && parsed > 0 {
			timeoutMs = parsed
		}
	}

	ctx := NewSpeconnContextFromRequest(r, timeoutMs)
	defer ctx.Cleanup()

	var body []byte
	if r.ContentLength > 0 {
		body, _ = io.ReadAll(r.Body)
	} else {
		body = []byte{}
	}

	reqFmt := ExtractFormat(contentType)
	resFmt := ExtractFormat(accept)

	req := specodec.Dispatch(h.reqCodec, body, reqFmt)

	res, err := h.handler(ctx, req)
	if err != nil {
		code := CodeInternal
		msg := err.Error()
		status := 0
		if se, ok := err.(*SpeconnError); ok {
			code = se.Code
			msg = se.Message
			status = code.HTTPStatus()
		}
		if status == 0 {
			status = code.HTTPStatus()
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write((&SpeconnError{Code: code, Message: msg}).Encode("json"))
		return
	}

	for k, v := range ctx.ResponseHeaders {
		w.Header()[k] = v
	}

	result := specodec.Respond(h.resCodec, res, resFmt)
	w.Header().Set("Content-Type", FormatToMime(result.Name, false))
	w.WriteHeader(http.StatusOK)
	w.Write(result.Body)
}

type serverStreamHandler[Req any, Res any] struct {
	path     string
	handler  func(*SpeconnContext, *Req, func(*Res) error) error
	reqCodec specodec.SpecCodec[Req]
	resCodec specodec.SpecCodec[Res]
}

func NewServerStreamHandler[Req any, Res any](
	path string,
	handler func(*SpeconnContext, *Req, func(*Res) error) error,
	reqCodec specodec.SpecCodec[Req],
	resCodec specodec.SpecCodec[Res],
) http.Handler {
	return &serverStreamHandler[Req, Res]{
		path:     path,
		handler:  handler,
		reqCodec: reqCodec,
		resCodec: resCodec,
	}
}

func (h *serverStreamHandler[Req, Res]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(NewError(CodeInternal, "streaming not supported").Encode("json"))
		return
	}

	contentType := r.Header.Get("Content-Type")
	accept := r.Header.Get("Accept")
	if accept == "" {
		accept = contentType
	}

	timeoutMs := 0
	if timeoutHeader := r.Header.Get("Speconn-Timeout-Ms"); timeoutHeader != "" {
		if parsed, err := strconv.Atoi(timeoutHeader); err == nil && parsed > 0 {
			timeoutMs = parsed
		}
	}

	ctx := NewSpeconnContextFromRequest(r, timeoutMs)
	defer ctx.Cleanup()

	var body []byte
	if r.ContentLength > 0 {
		body, _ = io.ReadAll(r.Body)
	} else {
		body = []byte{}
	}

	reqFmt := ExtractFormat(contentType)
	resFmt := ExtractFormat(accept)

	req := specodec.Dispatch(h.reqCodec, body, reqFmt)

	for k, v := range ctx.ResponseHeaders {
		w.Header()[k] = v
	}

	w.Header().Set("Content-Type", FormatToMime(resFmt, true))
	w.Header().Set("Connect-Protocol-Version", "1")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx.MarkHeadersSent()

	send := func(msg *Res) error {
		result := specodec.Respond(h.resCodec, msg, resFmt)
		frame := EncodeEnvelope(0, result.Body)
		_, err := w.Write(frame)
		flusher.Flush()
		return err
	}

	err := h.handler(ctx, req, send)

	if err != nil {
		se, ok := err.(*SpeconnError)
		if !ok { se = NewError(CodeInternal, err.Error()) }
		w.Write(EncodeEnvelope(FlagEndStream, se.Encode(resFmt)))
	} else {
		w.Write(EncodeEnvelope(FlagEndStream, []byte{}))
	}
	flusher.Flush()
}


