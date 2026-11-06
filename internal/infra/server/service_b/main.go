package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	httpClient "github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/http"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/server/lifecycle"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/provider"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/rules"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/util"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := provider.InitProvider("service-b")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			log.Printf("trace shutdown: %v", err)
		}
	}()

	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = ":8181"
	}
	server := &http.Server{
		Addr:              port,
		Handler:           otelhttp.NewHandler(newRouter(httpClient.NewHttpClient(), os.Getenv("WEATHER_API_KEY")), "service-b.http"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("service B listening at %s", port)
	if err := lifecycle.Serve(ctx, server); err != nil {
		log.Printf("service B stopped: %v", err)
		return
	}
}

func newRouter(client *http.Client, weatherAPIKey string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	tracer := otel.Tracer("temperature_by_cep_with_telemetry/service-b")

	r.Post("/cep", func(w http.ResponseWriter, request *http.Request) {
		var cep entity.Cep
		if err := json.NewDecoder(http.MaxBytesReader(w, request.Body, 4096)).Decode(&cep); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		cepCtx, cepSpan := tracer.Start(request.Context(), "lookup-cep", trace.WithSpanKind(trace.SpanKindInternal))
		var cepDetails entity.CepIn
		cepErr := rules.NewCepRules(client).Exec(cepCtx, cep.Value, &cepDetails)
		if cepErr != nil {
			cepSpan.SetStatus(codes.Error, cepErr.Message)
			cepSpan.End()
			util.WriteResponse(w, cepErr.Code, cepErr.Message)
			return
		}
		cepSpan.End()

		weatherCtx, weatherSpan := tracer.Start(request.Context(), "lookup-weather", trace.WithSpanKind(trace.SpanKindInternal))
		var result dto.OutDto
		weatherErr := rules.NewWeatherUseCase(client).Exec(weatherCtx, weatherAPIKey, cepDetails.City, &result)
		if weatherErr != nil {
			weatherSpan.SetStatus(codes.Error, weatherErr.Message)
			weatherSpan.End()
			util.WriteResponse(w, weatherErr.Code, weatherErr.Message)
			return
		}
		weatherSpan.End()

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("encode weather response: %v", err)
		}
	})
	return r
}
