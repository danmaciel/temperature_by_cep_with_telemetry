package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	httpClient "github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/http"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/rules"
)

func main() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	shutdown, err := initProvider()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	tracer := otel.Tracer(os.Getenv("TRACE_NAME"))
	r.Post("/cep", func(w http.ResponseWriter, r *http.Request) {
		var cep entity.Cep

		err := json.NewDecoder(r.Body).Decode(&cep)
		if err != nil {
			http.Error(w, "Erro ao decodificar o JSON", http.StatusBadRequest)
			return
		}
		ctx, span := tracer.Start(context.Background(), "Busca_CEP")
		span.SetAttributes(attribute.String("span.type", "Busca_CEP"))

		clientHttp := httpClient.NewHttpClient()

		cepRules := rules.NewCepRules(clientHttp)

		var cepModel entity.CepIn
		errCepIn := cepRules.Exec(cep.Value, &cepModel)
		span.End()
		if errCepIn != nil {
			WriteResponse(w, errCepIn.Code, errCepIn.Message)
			return
		}

		_, span2 := tracer.Start(ctx, "Busca_Temperatura")
		span2.SetAttributes(attribute.String("span.type", "Busca_Temperatura"))

		defer span2.End()
		weatherUseCase := rules.NewWeatherUseCase(clientHttp)
		key := os.Getenv("WEATHER_API_KEY")

		if key == "" {
			WriteResponse(w, http.StatusInternalServerError, "weather api key not found")
			return
		}

		var dto dto.OutDto
		errWeather := weatherUseCase.Exec(key, cepModel.City, &dto)

		if errWeather != nil {
			WriteResponse(w, errWeather.Code, errWeather.Message)
			return
		}

		result, errOnJson := json.Marshal(dto)

		if errOnJson != nil {
			WriteResponse(w, http.StatusInternalServerError, "error on generate json from data")
			return
		}

		GetResponseHeader(w)
		w.Write(result)

	})

	go func() {
		fmt.Println("Serviço B escutando na porta 8181...")
		if err := http.ListenAndServe(":8181", r); err != nil {
			log.Fatal(err)
		}
	}()

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully, CTRL+C pressed...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

}

func WriteResponse(w http.ResponseWriter, statusCode int, message string) {
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}

func GetResponseHeader(w http.ResponseWriter) {
	w.Header().Add("status-code", "200")
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("charset", "utf-8")
}

func initProvider() (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(os.Getenv("TELEMETRY_SERVICE_NAME")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := grpc.NewClient(os.Getenv("OTEL_EXPORTER_ZIPKIN_ENDPOINT"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}
