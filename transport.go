package speconn

import (
	"bytes"
	"io"
	"net/http"
)

// HttpClient is the interface Speconn expects HTTP clients to implement.
// The standard library's *http.Client implements HttpClient.
type HttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

// defaultHttpClient wraps http.DefaultClient.
var defaultHttpClient HttpClient = http.DefaultClient
