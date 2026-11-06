package http

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHttpClient validates TLS certificates, bounds requests and emits client spans.
func NewHttpClient() *http.Client {
	return &http.Client{
		// WeatherAPI authenticates through a URL query parameter. Do not export
		// its full URL (and API key) as an HTTP client span attribute.
		// The surrounding "Busca_Temperatura" span still measures this call.
		Transport: otelhttp.NewTransport(http.DefaultTransport, otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Hostname() != "api.weatherapi.com"
		})),
		Timeout:   15 * time.Second,
	}
}
