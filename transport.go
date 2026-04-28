package speconn

import (
	"io"
	"net/http"
)

type SpeconnTransport interface {
	Do(*http.Request) (*http.Response, error)
}

var defaultTransport SpeconnTransport = http.DefaultClient
