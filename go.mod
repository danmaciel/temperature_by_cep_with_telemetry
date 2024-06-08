module github.com/danmaciel/temperature_by_cep_with_telemetry

go 1.22.3

require (
	github.com/go-chi/chi/v5 v5.0.12
	github.com/openzipkin/zipkin-go v0.4.3
	go.opentelemetry.io/otel v1.27.0
	go.opentelemetry.io/otel/exporters/zipkin v1.27.0
	go.opentelemetry.io/otel/sdk v1.27.0
	golang.org/x/text v0.16.0
)

require (
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	go.opentelemetry.io/otel/metric v1.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.27.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
)
