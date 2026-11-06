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

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestServiceBContinuesRemoteTraceIntoLookupSpans(t *testing.T) {
	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)
	defer provider.Shutdown(context.Background())
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	client := &http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Host {
		case "viacep.com.br":
			body = `{"localidade":"Londrina","cep":"86027410"}`
		case "api.weatherapi.com":
			body = `{"current":{"temp_c":25}}`
		default:
			t.Errorf("unexpected external host: %s", req.URL.Host)
			return nil, context.Canceled
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	handler := otelhttp.NewHandler(newRouter(client, "test-key"), "service-b.http")

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
	propagation.TraceContext{}.Inject(trace.ContextWithSpanContext(request.Context(), parent), propagation.HeaderCarrier(request.Header))
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)

	if result.Code != http.StatusOK {
		t.Fatalf("expected 200, received %d: %s", result.Code, result.Body.String())
	}
	spans := recorder.Ended()
	if len(spans) != 3 {
		t.Fatalf("expected server, CEP and weather spans, received %d", len(spans))
	}
	for _, span := range spans {
		if span.SpanContext().TraceID() != traceID {
			t.Errorf("span %s uses a different trace ID", span.Name())
		}
	}
}
