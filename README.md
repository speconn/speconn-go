# speconn-go

Go runtime for [Speconn](https://github.com/speconn) RPC.

## Install

```bash
go get github.com/speconn/speconn-go
```

## Usage

### Server

```go
package main

import (
    "net/http"
    "github.com/speconn/speconn-runtime-golang/speconn"
)

type CheckRequest struct {
    Status  string `json:"status"`
    Service string `json:"service"`
}

type CheckResponse struct {
    Status  string `json:"status"`
    Service string `json:"service"`
}

func main() {
    path, handler := speconn.NewUnaryHandler(
        "/health.v1.HealthService/check",
        func(req *CheckRequest) (*CheckResponse, error) {
            return &CheckResponse{Status: "ok", Service: "my-service"}, nil
        },
    )
    mux := http.NewServeMux()
    mux.Handle(path, handler)
    http.ListenAndServe(":8080", mux)
}
```

### Client

```go
client := speconn.NewClient[CheckRequest, CheckResponse](
    http.DefaultClient,
    "http://localhost:8080",
    "/health.v1.HealthService/check",
)
resp, err := client.Call(&CheckRequest{})
```
