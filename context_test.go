package speconn

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContextFields(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer token")
	headers.Set("X-Custom", "value")

	ctx := NewSpeconnContext(
		headers,
		"/test.Service/Method",
		"localhost:8001",
		"192.168.1.100:54321",
		5000,
	)

	assert.Equal(t, "/test.Service/Method", ctx.MethodName)
	assert.Equal(t, "localhost:8001", ctx.LocalAddr)
	assert.Equal(t, "192.168.1.100:54321", ctx.RemoteAddr)

	assert.Equal(t, "Bearer token", ctx.Headers.Get("authorization"))
	assert.Equal(t, "value", ctx.Headers.Get("x-custom"))

	assert.Equal(t, 0, len(ctx.ResponseHeaders))

	deadline, hasDeadline := ctx.Deadline()
	assert.True(t, hasDeadline)
	assert.True(t, deadline.After(time.Now()))
}

func TestResponseHeaders(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	err := ctx.SetResponseHeader("X-Custom", "value1")
	assert.NoError(t, err)

	err = ctx.SetResponseHeader("X-Custom", "value2")
	assert.NoError(t, err)
	assert.Equal(t, "value2", ctx.ResponseHeaders.Get("x-custom"))

	err = ctx.AddResponseHeader("X-Multi", "v1")
	assert.NoError(t, err)
	err = ctx.AddResponseHeader("X-Multi", "v2")
	assert.NoError(t, err)
	assert.Equal(t, []string{"v1", "v2"}, ctx.ResponseHeaders["x-multi"])

	ctx.MarkHeadersSent()
	err = ctx.SetResponseHeader("X-Another", "value3")
	assert.Error(t, err)
	assert.Equal(t, CodeInternal, err.(*SpeconnError).Code)
	assert.Contains(t, err.Error(), "headers already sent")
}

func TestResponseTrailers(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	ctx.SetResponseTrailer("X-Total-Count", "100")
	ctx.SetResponseTrailer("X-Request-Id", "abc-123")

	assert.Equal(t, "100", ctx.ResponseTrailers.Get("x-total-count"))
	assert.Equal(t, "abc-123", ctx.ResponseTrailers.Get("x-request-id"))
}

func TestTimeoutSignal(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 100)

	select {
	case <-ctx.Done():
		assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Timeout not triggered after 100ms")
	}
}

func TestNoTimeout(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	deadline, hasDeadline := ctx.Deadline()
	assert.False(t, hasDeadline)
	assert.Equal(t, time.Time{}, deadline)

	select {
	case <-ctx.Done():
		t.Fatal("Context should not be done without timeout")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestManualCancel(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	go func() {
		time.Sleep(50 * time.Millisecond)
		ctx.Cancel()
	}()

	select {
	case <-ctx.Done():
		assert.Equal(t, context.Canceled, ctx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be cancelled")
	}
}

func TestContextKeyTyped(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	TestKey := ContextKey[string]{ID: "test", DefaultValue: "default"}

	SetValue(ctx, TestKey, "value1")
	value := GetValue(ctx, TestKey)
	assert.Equal(t, "value1", value)

	DeleteValue(ctx, TestKey)
	value = GetValue(ctx, TestKey)
	assert.Equal(t, "default", value)

	IntKey := ContextKey[int]{ID: "int-test", DefaultValue: 0}
	SetValue(ctx, IntKey, 42)
	intValue := GetValue(ctx, IntKey)
	assert.Equal(t, 42, intValue)
}

func TestContextKeyPredefined(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 0)

	ctx.SetUser("alice")
	assert.Equal(t, "alice", ctx.User())

	ctx.SetRequestID("req-123")
	assert.Equal(t, "req-123", ctx.RequestID())

	userID := GetValue(ctx, UserIDKey)
	assert.Equal(t, int64(0), userID)

	SetValue(ctx, UserIDKey, int64(100))
	userID = GetValue(ctx, UserIDKey)
	assert.Equal(t, int64(100), userID)
}

func TestHeadersNormalization(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer token")
	headers.Set("CONTENT-TYPE", "application/json")
	headers.Set("X-Custom-Header", "value")

	ctx := NewSpeconnContext(headers, "/test", "localhost:8001", "client:123", 0)

	assert.Equal(t, "Bearer token", ctx.Headers.Get("authorization"))
	assert.Equal(t, "application/json", ctx.Headers.Get("content-type"))
	assert.Equal(t, "value", ctx.Headers.Get("x-custom-header"))
}

func TestNewSpeconnContextFromRequest(t *testing.T) {
	req, _ := http.NewRequest("POST", "/test.Service/Method", nil)
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Speconn-Timeout-Ms", "5000")
	req.Host = "localhost:8001"
	req.RemoteAddr = "192.168.1.100:54321"

	ctx := NewSpeconnContextFromRequest(req, 5000)

	assert.Equal(t, "/test.Service/Method", ctx.MethodName)
	assert.Equal(t, "localhost:8001", ctx.LocalAddr)
	assert.Equal(t, "192.168.1.100:54321", ctx.RemoteAddr)
	assert.Equal(t, "Bearer token", ctx.Headers.Get("authorization"))

	deadline, hasDeadline := ctx.Deadline()
	assert.True(t, hasDeadline)
	assert.True(t, deadline.After(time.Now()))
}

func TestCleanup(t *testing.T) {
	ctx := NewSpeconnContext(nil, "/test", "localhost:8001", "client:123", 1000)

	ctx.Cleanup()

	select {
	case <-ctx.Done():
		assert.Equal(t, context.Canceled, ctx.Err())
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Context should be cancelled after Cleanup")
	}
}