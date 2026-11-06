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
