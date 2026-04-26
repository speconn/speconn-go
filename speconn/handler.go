package speconn

import (
	"encoding/json"
	"net/http"
)

type UnaryHandlerFunc[Req any, Res any] func(req *Req) (*Res, error)

func NewUnaryHandler[Req any, Res any](path string, fn UnaryHandlerFunc[Req, Res]) (string, http.Handler) {
	return path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, CodeUnimplemented, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req Req
		if r.ContentLength > 0 {
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, CodeInvalidArgument, err.Error(), 0)
				return
			}
		}

		res, err := fn(&req)
		if err != nil {
			code := CodeInternal
			msg := err.Error()
			status := 0
			if se, ok := err.(*Error); ok {
				code = se.Code
				msg = se.Message
				status = CodeToHTTP(se.Code)
			}
			writeJSONError(w, code, msg, status)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	})
}

func writeJSONError(w http.ResponseWriter, code Code, message string, status int) {
	if status == 0 {
		status = CodeToHTTP(code)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(&Error{Code: code, Message: message})
}
