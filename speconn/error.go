package speconn

import "fmt"

type Code string

const (
	CodeCanceled          Code = "canceled"
	CodeUnknown           Code = "unknown"
	CodeInvalidArgument   Code = "invalid_argument"
	CodeDeadlineExceeded  Code = "deadline_exceeded"
	CodeNotFound          Code = "not_found"
	CodeAlreadyExists     Code = "already_exists"
	CodePermissionDenied  Code = "permission_denied"
	CodeResourceExhausted Code = "resource_exhausted"
	CodeFailedPrecondition Code = "failed_precondition"
	CodeAborted           Code = "aborted"
	CodeOutOfRange        Code = "out_of_range"
	CodeUnimplemented     Code = "unimplemented"
	CodeInternal          Code = "internal"
	CodeUnavailable       Code = "unavailable"
	CodeDataLoss          Code = "data_loss"
	CodeUnauthenticated   Code = "unauthenticated"
)

type SpeconnError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

func (e *SpeconnError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewError(code Code, message string) *SpeconnError {
	return &SpeconnError{Code: code, Message: message}
}

func Errorf(code Code, format string, args ...any) *SpeconnError {
	return &SpeconnError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func (c Code) HTTPStatus() int {
	switch c {
	case CodeInvalidArgument, CodeFailedPrecondition, CodeOutOfRange:
		return 400
	case CodeUnauthenticated:
		return 401
	case CodePermissionDenied:
		return 403
	case CodeNotFound:
		return 404
	case CodeAlreadyExists, CodeAborted:
		return 409
	case CodeResourceExhausted:
		return 429
	case CodeCanceled:
		return 499
	case CodeUnimplemented:
		return 501
	case CodeUnavailable:
		return 503
	case CodeDeadlineExceeded:
		return 504
	default:
		return 500
	}
}

func CodeFromHTTPStatus(status int) Code {
	switch status {
	case 400:
		return CodeInternal
	case 401:
		return CodeUnauthenticated
	case 403:
		return CodePermissionDenied
	case 404:
		return CodeUnimplemented
	case 429:
		return CodeUnavailable
	case 502, 503, 504:
		return CodeUnavailable
	default:
		return CodeUnknown
	}
}

func CodeFromString(s string) Code {
	codes := map[string]Code{
		"canceled":           CodeCanceled,
		"unknown":            CodeUnknown,
		"invalid_argument":   CodeInvalidArgument,
		"deadline_exceeded":  CodeDeadlineExceeded,
		"not_found":          CodeNotFound,
		"already_exists":     CodeAlreadyExists,
		"permission_denied":  CodePermissionDenied,
		"resource_exhausted": CodeResourceExhausted,
		"failed_precondition": CodeFailedPrecondition,
		"aborted":            CodeAborted,
		"out_of_range":       CodeOutOfRange,
		"unimplemented":      CodeUnimplemented,
		"internal":           CodeInternal,
		"unavailable":        CodeUnavailable,
		"data_loss":          CodeDataLoss,
		"unauthenticated":    CodeUnauthenticated,
	}
	if c, ok := codes[s]; ok {
		return c
	}
	return CodeUnknown
}
