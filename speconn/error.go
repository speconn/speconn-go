package speconn

import "fmt"

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("speconn: %s: %s", e.Code, e.Message)
}

func NewError(code Code, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
