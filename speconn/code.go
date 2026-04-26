package speconn

type Code string

const (
	CodeCanceled           Code = "canceled"
	CodeUnknown            Code = "unknown"
	CodeInvalidArgument    Code = "invalid_argument"
	CodeDeadlineExceeded   Code = "deadline_exceeded"
	CodeNotFound           Code = "not_found"
	CodeAlreadyExists      Code = "already_exists"
	CodePermissionDenied   Code = "permission_denied"
	CodeResourceExhausted  Code = "resource_exhausted"
	CodeFailedPrecondition Code = "failed_precondition"
	CodeAborted            Code = "aborted"
	CodeOutOfRange         Code = "out_of_range"
	CodeUnimplemented      Code = "unimplemented"
	CodeInternal           Code = "internal"
	CodeUnavailable        Code = "unavailable"
	CodeDataLoss           Code = "data_loss"
	CodeUnauthenticated    Code = "unauthenticated"
)

func CodeToHTTP(c Code) int {
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
	case CodeDeadlineExceeded:
		return 504
	case CodeUnavailable:
		return 503
	default:
		return 500
	}
}
