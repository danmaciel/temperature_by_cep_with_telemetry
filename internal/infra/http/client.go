package http

import (
	"net/http"
	"time"
)

// NewHttpClient relies on Go's default TLS certificate verification.
// Requests are bounded so a remote API cannot hang the service indefinitely.
func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
	}
}
