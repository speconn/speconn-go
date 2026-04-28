package speconn

import "net/http"

type SpeconnContext struct {
	Headers http.Header
	Values  map[string]any
}

func (c *SpeconnContext) Value(key string) any {
	return c.Values[key]
}

func (c *SpeconnContext) SetValue(key string, value any) {
	c.Values[key] = value
}

func (c *SpeconnContext) User() string {
	if v, ok := c.Values["user"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
