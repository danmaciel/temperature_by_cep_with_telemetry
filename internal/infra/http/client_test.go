package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewHttpClientRejectsUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHttpClient()
	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("expected TLS certificate verification to reject the test server's untrusted certificate")
	}
}

func TestNewHttpClientHasTimeout(t *testing.T) {
	if NewHttpClient().Timeout <= 0 {
		t.Fatal("external requests must have a finite timeout")
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (fn testTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

// Check the actual spans captured in memory instead of assuming that
// WithFilter is effective just because the option is configured.
func TestWeatherAPIKeyDoesNotAppearInExportableHTTPSpans(t *testing.T) {
	const secret = "sentinel-weather-api-key-never-export"
	originalProvider := otel.GetTracerProvider()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(originalProvider)
		_ = provider.Shutdown(context.Background())
	})

	var weatherReached, cepReached bool
	client := newHTTPClientWithTransport(testTransport(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Hostname() {
		case "api.weatherapi.com":
			weatherReached = true
			if request.URL.Query().Get("key") != secret {
				t.Error("WeatherAPI request did not receive the expected test credential")
			}
		case "viacep.com.br":
			cepReached = true
		default:
			t.Errorf("unexpected destination %q", request.URL.Hostname())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("{}")),
			Header:     make(http.Header),
		}, nil
	}))

	_, span := otel.Tracer("weather-test").Start(context.Background(), "manual-weather-operation")
	for _, target := range []string{
		"https://api.weatherapi.com/v1/current.json?key=" + secret + "&q=City",
		"https://viacep.com.br/ws/01001000/json/",
	} {
		response, err := client.Get(target)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
	}
	span.End()

	if !weatherReached || !cepReached {
		t.Fatalf("both provider requests must reach the transport (weather=%v, cep=%v)", weatherReached, cepReached)
	}

	spans := recorder.Ended()
	if len(spans) != 2 {
		t.Fatalf("expected manual span and one ViaCEP HTTP client span, got %d", len(spans))
	}
	for _, recorded := range spans {
		// Includes span name, attribute values, status and events.
		if strings.Contains(fmt.Sprint(recorded), secret) {
			t.Fatal("a trace span contains the WeatherAPI test credential")
		}
	}
}
