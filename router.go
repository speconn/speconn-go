package speconn

import (
	"encoding/json"
	"net/http"
)

type SpeconnRequest struct {
	Path        string
	Headers     http.Header
	Body        map[string]any
	ContentType string
	Values      map[string]any
}

type SpeconnResponse struct {
	Status  int
	Headers http.Header
	Body    any
}

type Interceptor interface {
	Before(req *SpeconnRequest) error
	After(req *SpeconnRequest, resp *SpeconnResponse)
}

type SpeconnRouter struct {
	unaryRoutes  map[string]func(any) (any, error)
	streamRoutes map[string]func(any, func(any))
	interceptors []Interceptor
}

type RouterOption func(*SpeconnRouter)

func WithInterceptors(interceptors ...Interceptor) RouterOption {
	return func(r *SpeconnRouter) {
		r.interceptors = append(r.interceptors, interceptors...)
	}
}

func NewRouter(opts ...RouterOption) *SpeconnRouter {
	r := &SpeconnRouter{
		unaryRoutes:  make(map[string]func(any) (any, error)),
		streamRoutes: make(map[string]func(any, func(any))),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *SpeconnRouter) Unary(path string, handler func(req any) (any, error)) *SpeconnRouter {
	r.unaryRoutes[path] = handler
	return r
}

func (r *SpeconnRouter) ServerStream(path string, handler func(req any, send func(any))) *SpeconnRouter {
	r.streamRoutes[path] = handler
	return r
}

func (r *SpeconnRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := req.URL.Path
	contentType := req.Header.Get("Content-Type")

	var body map[string]any
	if req.ContentLength > 0 {
		body = map[string]any{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeRouterError(w, CodeInvalidArgument, err.Error(), 0)
			return
		}
	} else {
		body = map[string]any{}
	}

	speconnReq := &SpeconnRequest{
		Path:        path,
		Headers:     req.Header,
		Body:        body,
		ContentType: contentType,
		Values:      make(map[string]any),
	}

	for _, i := range r.interceptors {
		if err := i.Before(speconnReq); err != nil {
			code := CodeInternal
			msg := err.Error()
			status := 0
			if se, ok := err.(*SpeconnError); ok {
				code = se.Code
				msg = se.Message
				status = code.HTTPStatus()
			}
			writeRouterError(w, code, msg, status)
			return
		}
	}

	isStream := speconnReq.ContentType != "" && containsStream(speconnReq.ContentType)

	var handlerResult any
	var handlerErr error

	if isStream {
		if h, ok := r.streamRoutes[speconnReq.Path]; ok {
			r.handleStream(w, h, speconnReq)
			return
		}
	}

	if h, ok := r.unaryRoutes[speconnReq.Path]; ok {
		handlerResult, handlerErr = h(speconnReq.Body)
	} else if !isStream {
		writeRouterError(w, CodeUnimplemented, "not found", http.StatusNotFound)
		return
	} else {
		writeRouterError(w, CodeUnimplemented, "not found", http.StatusNotFound)
		return
	}

	if handlerErr != nil {
		code := CodeInternal
		msg := handlerErr.Error()
		status := 0
		if se, ok := handlerErr.(*SpeconnError); ok {
			code = se.Code
			msg = se.Message
			status = code.HTTPStatus()
		}
		writeRouterError(w, code, msg, status)
		return
	}

	speconnResp := &SpeconnResponse{
		Status:  200,
		Headers: make(http.Header),
		Body:    handlerResult,
	}

	for _, i := range r.interceptors {
		i.After(speconnReq, speconnResp)
	}

	for k, v := range speconnResp.Headers {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(speconnResp.Status)
	json.NewEncoder(w).Encode(speconnResp.Body)
}

func (r *SpeconnRouter) handleStream(w http.ResponseWriter, handler func(any, func(any)), speconnReq *SpeconnRequest) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeRouterError(w, CodeInternal, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "application/connect+json")
	w.Header().Set("Connect-Protocol-Version", "1")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(msg any) {
		payload, err := json.Marshal(msg)
		if err != nil {
			return
		}
		frame := EncodeEnvelope(0, payload)
		w.Write(frame)
		flusher.Flush()
	}

	handler(speconnReq.Body, send)

	endPayload, _ := json.Marshal(map[string]any{})
	w.Write(EncodeEnvelope(FlagEndStream, endPayload))
	flusher.Flush()
}

func containsStream(ct string) bool {
	return ct != "" && (ct == "application/connect+json" || len(ct) > 22 && ct[:22] == "application/connect+json")
}

func writeRouterError(w http.ResponseWriter, code Code, message string, status int) {
	if status == 0 {
		status = code.HTTPStatus()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&SpeconnError{Code: code, Message: message})
}
