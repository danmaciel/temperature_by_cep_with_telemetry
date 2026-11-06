package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func TestServiceAForwardsW3CTraceContext(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	provider := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	defer provider.Shutdown(context.Background())
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	var forwardedContext trace.SpanContext
	downstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwardedContext = trace.SpanContextFromContext(propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header)))
		if !forwardedContext.IsValid() {
			t.Error("expected W3C traceparent propagated to service B")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"city":"Londrina","temp_c":25}`)
	}))
	defer downstream.Close()

	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	server := httptest.NewServer(otelhttp.NewHandler(newRouter(client, downstream.URL), "service-a.http"))
	defer server.Close()

	resp, err := http.Post(server.URL+"/cep", "application/json", strings.NewReader(`{"cep":"86027410"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected response status: %d", resp.StatusCode)
	}
	if !forwardedContext.IsValid() {
		t.Fatal("the downstream request did not contain a valid trace context")
	}
}
