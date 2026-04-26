package speconn

type Request[T any] struct {
	Msg     *T
	Headers map[string][]string
}

type Response[T any] struct {
	Msg     *T
	Headers map[string][]string
}

func NewRequest[T any](msg *T) *Request[T] {
	return &Request[T]{Msg: msg, Headers: make(map[string][]string)}
}

func NewResponse[T any](msg *T) *Response[T] {
	return &Response[T]{Msg: msg, Headers: make(map[string][]string)}
}
