package speconn

import (
	"encoding/json"
	"net/http"
)

type SpeconnRouter struct {
	unaryRoutes  map[string]http.Handler
	streamRoutes map[string]http.Handler
}

func NewRouter() *SpeconnRouter {
	return &SpeconnRouter{
		unaryRoutes:  make(map[string]http.Handler),
		streamRoutes: make(map[string]http.Handler),
	}
}

func (r *SpeconnRouter) Unary(path string, handler func(req any) (any, error)) *SpeconnRouter {
	r.unaryRoutes[path] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			writeRouterError(w, CodeUnimplemented, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body any
		if req.ContentLength > 0 {
			body = map[string]any{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				writeRouterError(w, CodeInvalidArgument, err.Error(), 0)
				return
			}
		} else {
			body = map[string]any{}
		}

		res, err := handler(body)
		if err != nil {
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

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	})
	return r
}

func (r *SpeconnRouter) ServerStream(path string, handler func(req any, send func(any)) ) *SpeconnRouter {
	r.streamRoutes[path] = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			writeRouterError(w, CodeUnimplemented, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeRouterError(w, CodeInternal, "streaming not supported", 500)
			return
		}

		var body any
		if req.ContentLength > 0 {
			body = map[string]any{}
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				writeRouterError(w, CodeInvalidArgument, err.Error(), 0)
				return
			}
		} else {
			body = map[string]any{}
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

		handler(body, send)

		endPayload, _ := json.Marshal(map[string]any{})
		w.Write(EncodeEnvelope(FlagEndStream, endPayload))
		flusher.Flush()
	})
	return r
}

func (r *SpeconnRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	if h, ok := r.unaryRoutes[path]; ok {
		h.ServeHTTP(w, req)
		return
	}
	if h, ok := r.streamRoutes[path]; ok {
		h.ServeHTTP(w, req)
		return
	}
	writeRouterError(w, CodeUnimplemented, "not found", http.StatusNotFound)
}

func writeRouterError(w http.ResponseWriter, code Code, message string, status int) {
	if status == 0 {
		status = code.HTTPStatus()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&SpeconnError{Code: code, Message: message})
}

