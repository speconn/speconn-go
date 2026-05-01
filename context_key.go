package speconn

type ContextKey[T any] struct {
	ID           string
	DefaultValue T
}

func SetValue[T any](ctx *SpeconnContext, key ContextKey[T], value T) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	ctx.values[key.ID] = value
}

func GetValue[T any](ctx *SpeconnContext, key ContextKey[T]) T {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	if v, ok := ctx.values[key.ID]; ok {
		if typed, ok := v.(T); ok {
			return typed
		}
	}
	return key.DefaultValue
}

func DeleteValue[T any](ctx *SpeconnContext, key ContextKey[T]) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	delete(ctx.values, key.ID)
}

var (
	UserKey = ContextKey[string]{
		ID:           "user",
		DefaultValue: "",
	}

	RequestIDKey = ContextKey[string]{
		ID:           "request-id",
		DefaultValue: "",
	}

	UserIDKey = ContextKey[int64]{
		ID:           "user-id",
		DefaultValue: 0,
	}
)

func (c *SpeconnContext) User() string {
	return GetValue(c, UserKey)
}

func (c *SpeconnContext) SetUser(user string) {
	SetValue(c, UserKey, user)
}

func (c *SpeconnContext) RequestID() string {
	return GetValue(c, RequestIDKey)
}

func (c *SpeconnContext) SetRequestID(id string) {
	SetValue(c, RequestIDKey, id)
}