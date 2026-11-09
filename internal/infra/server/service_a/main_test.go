package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
)

func TestServiceAForwardsTraceparentToServiceB(t *testing.T) {
	originalProvider := otel.GetTracerProvider()
	originalPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(originalProvider)
		otel.SetTextMapPropagator(originalPropagator)
	})

	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer provider.Shutdown(context.Background())
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	var downstreamContext trace.SpanContext
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extracted := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		downstreamContext = trace.SpanContextFromContext(extracted)
		_, _ = io.WriteString(w, "ok")
	}))
	defer downstream.Close()

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 5 * time.Second}
	handler := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardToServiceB(r.Context(), w, entity.Cep{Value: "01001000"}, downstream.URL, client)
	}), "service-a-http")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/cep", nil))

	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("unexpected response: code=%d, body=%q", response.Code, response.Body.String())
	}
	if !downstreamContext.IsValid() {
		t.Fatal("service B did not receive a valid W3C trace context")
	}
}

func TestValidCEPRequiresEightASCIIDigits(t *testing.T) {
	cases := []struct {
		value	string
		valid	bool
	}{
		{"01001000", true},
		{"00000000", true},
		{"abcdefgh", false},
		{"01001-00", false},
		{"0100100", false},
		{"010010000", false},
		{"０１００１０００", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := validCEP(tc.value); got != tc.valid {
			t.Errorf("validCEP(%q) = %v; want %v", tc.value, got, tc.valid)
		}
	}
}

func TestServiceAPropagatesControlledDownstreamError(t *testing.T) {
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "weather provider unavailable")
	}))
	defer downstream.Close()

	response := httptest.NewRecorder()
	forwardToServiceB(context.Background(), response, entity.Cep{Value: "01001000"},
		downstream.URL, &http.Client{Timeout: 5 * time.Second})

	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected propagated HTTP 502; got %d", response.Code)
	}
	if response.Body.String() != "weather provider unavailable" {
		t.Fatalf("unexpected downstream error body %q", response.Body.String())
	}
}

func TestServiceAHidesTransportErrorDetails(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	response := httptest.NewRecorder()
	// The invalid URL produces a deterministic request-creation error without
	// contacting any outside service.
	forwardToServiceB(context.Background(), response, entity.Cep{Value: "01001000"},
		"://invalid-endpoint", client)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected controlled HTTP 500; got %d", response.Code)
	}
	if response.Body.String() != "failed to create service request\n" {
		t.Fatalf("unexpected response details: %q", response.Body.String())
	}
}
