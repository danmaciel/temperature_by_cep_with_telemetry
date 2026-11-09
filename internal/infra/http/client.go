package http

import (
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewHttpClient validates TLS certificates, bounds requests and emits client
// spans for provider calls except WeatherAPI (whose URL carries a secret).
func NewHttpClient() *http.Client {
	return newHTTPClientWithTransport(http.DefaultTransport)
}

// Allow tests to inspect emitted spans using an in-memory transport. The
// production transport remains the standard TLS-verifying implementation.
func newHTTPClientWithTransport(base http.RoundTripper) *http.Client {
	return &http.Client{
		// WeatherAPI uses a key in the URL query. Instrumenting its HTTP
		// request would risk exporting that key as a URL span attribute.
		// The surrounding manual Busca_Temperatura span remains active.
		Transport: otelhttp.NewTransport(base, otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Hostname() != "api.weatherapi.com"
		})),
		Timeout: 15 * time.Second,
	}
}
