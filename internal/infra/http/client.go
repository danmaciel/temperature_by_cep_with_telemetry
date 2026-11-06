package http

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHttpClient validates TLS certificates, bounds requests and emits client spans.
func NewHttpClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   15 * time.Second,
	}
}
