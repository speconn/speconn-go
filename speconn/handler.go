package speconn

import (
	"encoding/json"
	"net/http"
)

type UnaryHandler[Req any, Res any] func(req *Req) (*Res, error)

func NewUnaryHandler[Req any, Res any](path string, fn UnaryHandler[Req, Res]) (string, http.Handler) {
	return path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, NewError(CodeInvalidArgument, err.Error()))
			return
		}

		res, err := fn(&req)
		if err != nil {
			if speconnErr, ok := err.(*Error); ok {
				writeError(w, speconnErr)
			} else {
				writeError(w, NewError(CodeInternal, err.Error()))
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	})
}

func writeError(w http.ResponseWriter, err *Error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(CodeToHTTP(err.Code))
	json.NewEncoder(w).Encode(err)
}
