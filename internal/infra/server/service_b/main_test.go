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
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type mockTransport func(*http.Request) (*http.Response, error)

func (f mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestServiceBContinuesIncomingTraceForBothLookups(t *testing.T) {
	originalProvider := otel.GetTracerProvider()
	originalPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(originalProvider)
		otel.SetTextMapPropagator(originalPropagator)
	})

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	defer provider.Shutdown(context.Background())
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	client := &http.Client{Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
		body := ""
		switch req.URL.Host {
		case "viacep.com.br":
			body = `{"localidade":"Londrina","cep":"86027410"}`
		case "api.weatherapi.com":
			body = `{"current":{"temp_c":25}}`
		default:
			t.Errorf("unexpected provider: %s", req.URL.Host)
			return nil, context.Canceled
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	traceID, err := trace.TraceIDFromHex("00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	parentID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatal(err)
	}
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID: parentID,
		TraceFlags: trace.FlagsSampled,
		Remote: true,
	})

	request := httptest.NewRequest(http.MethodPost, "/cep", strings.NewReader(`{"cep":"86027410"}`))
	parentContext := trace.ContextWithSpanContext(request.Context(), parent)
	propagation.TraceContext{}.Inject(parentContext, propagation.HeaderCarrier(request.Header))
	response := httptest.NewRecorder()
	handler := otelhttp.NewHandler(newRouter(otel.Tracer("test-service-b"), client, "test-key"), "service-b-http")
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d; body: %s", response.Code, response.Body.String())
	}
	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("expected HTTP server span and two lookup spans; got %d", len(spans))
	}
	for _, span := range spans {
		if span.SpanContext().TraceID() != traceID {
			t.Errorf("span %s does not continue the incoming trace", span.Name())
		}
	}
}
