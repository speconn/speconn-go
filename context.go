package speconn

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type SpeconnContext struct {
	Headers          http.Header
	ResponseHeaders  http.Header
	ResponseTrailers http.Header
	Signal           context.Context
	MethodName       string
	LocalAddr        string
	RemoteAddr       string
	values           map[string]any
	mu               sync.Mutex
	headersSent      bool
	cancel           context.CancelFunc
}

func NewSpeconnContext(
	headers http.Header,
	methodName string,
	localAddr string,
	remoteAddr string,
	timeoutMs int,
) *SpeconnContext {
	normalizedHeaders := make(http.Header)
	for k, v := range headers {
		normalizedHeaders[strings.ToLower(k)] = v
	}

	var signal context.Context
	var cancel context.CancelFunc

	if timeoutMs > 0 {
		signal, cancel = context.WithTimeout(
			context.Background(),
			time.Duration(timeoutMs)*time.Millisecond,
		)
	} else {
		signal, cancel = context.WithCancel(context.Background())
	}

	return &SpeconnContext{
		Headers:          normalizedHeaders,
		ResponseHeaders:  make(http.Header),
		ResponseTrailers: make(http.Header),
		Signal:           signal,
		MethodName:       methodName,
		LocalAddr:        localAddr,
		RemoteAddr:       remoteAddr,
		values:           make(map[string]any),
		headersSent:      false,
		cancel:           cancel,
	}
}

func NewSpeconnContextFromRequest(
	req *http.Request,
	timeoutMs int,
) *SpeconnContext {
	localAddr := req.Host
	if localAddr == "" {
		localAddr = "localhost"
	}

	return NewSpeconnContext(
		req.Header,
		req.URL.Path,
		localAddr,
		req.RemoteAddr,
		timeoutMs,
	)
}

func (c *SpeconnContext) Deadline() (time.Time, bool) {
	return c.Signal.Deadline()
}

func (c *SpeconnContext) Done() <-chan struct{} {
	return c.Signal.Done()
}

func (c *SpeconnContext) Err() error {
	return c.Signal.Err()
}

func (c *SpeconnContext) SetResponseHeader(key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.headersSent {
		return NewError(CodeInternal, "headers already sent")
	}
	c.ResponseHeaders.Set(strings.ToLower(key), value)
	return nil
}

func (c *SpeconnContext) AddResponseHeader(key, value string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.headersSent {
		return NewError(CodeInternal, "headers already sent")
	}
	c.ResponseHeaders.Add(strings.ToLower(key), value)
	return nil
}

func (c *SpeconnContext) SetResponseTrailer(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ResponseTrailers.Set(strings.ToLower(key), value)
}

func (c *SpeconnContext) MarkHeadersSent() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.headersSent = true
}

func (c *SpeconnContext) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *SpeconnContext) Cleanup() {
	c.Cancel()
}