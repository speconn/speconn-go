package speconn

import (
	"net/http"
	"strings"
)

type CORSInterceptor struct {
	AllowOrigin  string
	AllowMethods string
	AllowHeaders string
	MaxAge       string
}

func NewCORSInterceptor() *CORSInterceptor {
	return &CORSInterceptor{
		AllowOrigin:  "*",
		AllowMethods: "POST, OPTIONS",
		AllowHeaders: "Content-Type, Connect-Protocol-Version, Authorization",
		MaxAge:       "86400",
	}
}

func (c *CORSInterceptor) Before(req *SpeconnRequest) error {
	return nil
}

func (c *CORSInterceptor) After(req *SpeconnRequest, resp *SpeconnResponse) {
	if resp.Headers == nil {
		resp.Headers = make(http.Header)
	}
	resp.Headers.Set("Access-Control-Allow-Origin", c.AllowOrigin)
	resp.Headers.Set("Access-Control-Allow-Methods", c.AllowMethods)
	resp.Headers.Set("Access-Control-Allow-Headers", c.AllowHeaders)
	resp.Headers.Set("Access-Control-Max-Age", c.MaxAge)
}

type AuthInterceptor struct{}

func NewAuthInterceptor() *AuthInterceptor {
	return &AuthInterceptor{}
}

func (a *AuthInterceptor) Before(req *SpeconnRequest) error {
	if req.Values == nil {
		req.Values = make(map[string]any)
	}
	if auth := req.Headers.Get("Authorization"); len(auth) > 7 && strings.HasPrefix(auth, "Bearer ") {
		user := strings.TrimPrefix(auth, "Bearer ")
		req.Values["user"] = user
		if req.Body == nil {
			req.Body = map[string]any{}
		}
		req.Body["_user"] = user
	}
	return nil
}

func (a *AuthInterceptor) After(req *SpeconnRequest, resp *SpeconnResponse) {}
