package speconn

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Handler func(ctx *SpeconnContext, body []byte, ct string, accept string) ([]byte, error)

type StreamHandler func(ctx *SpeconnContext, body []byte, ct string, accept string, send func([]byte) error) error

type Interceptor interface {
	Before(ctx *SpeconnContext, req *http.Request) error
	After(ctx *SpeconnContext, w http.ResponseWriter) error
}

type Router struct {
	mu           sync.RWMutex
	unaryRoutes  map[string]Handler
	streamRoutes map[string]StreamHandler
	interceptors []Interceptor
}

func NewRouter() *Router {
	return &Router{
		unaryRoutes:  make(map[string]Handler),
		streamRoutes: make(map[string]StreamHandler),
		interceptors: make([]Interceptor, 0),
	}
}

func (r *Router) AddUnaryHandler(path string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unaryRoutes[path] = handler
}

func (r *Router) AddStreamHandler(path string, handler StreamHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streamRoutes[path] = handler
}

func (r *Router) AddInterceptor(interceptor Interceptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.interceptors = append(r.interceptors, interceptor)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	timeoutMs := 0
	if timeoutHeader := req.Header.Get("Speconn-Timeout-Ms"); timeoutHeader != "" {
		if parsed, err := strconv.Atoi(timeoutHeader); err == nil && parsed > 0 {
			timeoutMs = parsed
		}
	}

	ctx := NewSpeconnContextFromRequest(req, timeoutMs)

	for _, interceptor := range r.interceptors {
		if err := interceptor.Before(ctx, req); err != nil {
			handleError(w, err)
			ctx.Cleanup()
			return
		}
	}

	path := req.URL.Path
	ct := req.Header.Get("Content-Type")
	accept := req.Header.Get("Accept")
	if accept == "" {
		accept = ct
	}

	var body []byte
	if req.ContentLength > 0 {
		body, _ = io.ReadAll(req.Body)
	} else {
		body = []byte{}
	}

	r.mu.RLock()
	isStream := strings.Contains(ct, "connect+")
	
	if isStream {
		if handler, ok := r.streamRoutes[path]; ok {
			r.mu.RUnlock()
			handleStreamRPC(w, ctx, handler, body, ct, accept)
		} else {
			r.mu.RUnlock()
			http.NotFound(w, req)
		}
	} else {
		if handler, ok := r.unaryRoutes[path]; ok {
			r.mu.RUnlock()
			handleUnaryRPC(w, ctx, handler, body, ct, accept)
		} else {
			r.mu.RUnlock()
			http.NotFound(w, req)
		}
	}

	for _, interceptor := range r.interceptors {
		interceptor.After(ctx, w)
	}

	ctx.Cleanup()
}

func handleUnaryRPC(w http.ResponseWriter, ctx *SpeconnContext, handler Handler, body []byte, ct string, accept string) {
	respBody, err := handler(ctx, body, ct, accept)
	if err != nil {
		handleError(w, err)
		return
	}

	for k, v := range ctx.ResponseHeaders {
		w.Header()[k] = v
	}

	w.Header().Set("Content-Type", FormatToMime(ExtractFormat(accept), false))
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

func handleStreamRPC(w http.ResponseWriter, ctx *SpeconnContext, handler StreamHandler, body []byte, ct string, accept string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(NewError(CodeInternal, "streaming not supported").Encode("json"))
		return
	}

	for k, v := range ctx.ResponseHeaders {
		w.Header()[k] = v
	}

	w.Header().Set("Content-Type", FormatToMime(ExtractFormat(accept), true))
	w.Header().Set("Connect-Protocol-Version", "1")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ctx.MarkHeadersSent()

	send := func(data []byte) error {
		envelope := EncodeEnvelope(0, data)
		w.Write(envelope)
		flusher.Flush()
		return nil
	}

	err := handler(ctx, body, ct, accept, send)

	resFmt := ExtractFormat(accept)
	if err != nil {
		se, ok := err.(*SpeconnError)
		if !ok { se = NewError(CodeInternal, err.Error()) }
		w.Write(EncodeEnvelope(FlagEndStream, se.Encode(resFmt)))
	} else {
		w.Write(EncodeEnvelope(FlagEndStream, []byte{}))
	}
	flusher.Flush()
}

func handleError(w http.ResponseWriter, err error) {
	se, ok := err.(*SpeconnError)
	if !ok { se = NewError(CodeInternal, err.Error()) }
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(se.Code.HTTPStatus())
	w.Write(se.Encode("json"))
}