package main

import (
	"context"
	"encoding/json"
	"errors"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	httpClient "github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/http"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/provider"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/rules"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/util"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := provider.InitProvider("service-b")
	if err != nil {
		log.Fatal(err)
	}
	tracer := otel.Tracer("service-b")
	client := httpClient.NewHttpClient()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Post("/cep", func(w http.ResponseWriter, request *http.Request) {
		var cep entity.Cep
		if err := json.NewDecoder(request.Body).Decode(&cep); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		cepCtx, cepSpan := tracer.Start(request.Context(), "Busca_CEP")
		var location entity.CepIn
		cepErr := rules.NewCepRules(client).Exec(cepCtx, cep.Value, &location)
		if cepErr != nil {
			cepSpan.SetStatus(codes.Error, cepErr.Message)
			cepSpan.End()
			util.WriteResponse(w, cepErr.Code, cepErr.Message)
			return
		}
		cepSpan.End()

		key := os.Getenv("WEATHER_API_KEY")
		if key == "" {
			util.WriteResponse(w, http.StatusInternalServerError, "weather api key not found")
			return
		}

		weatherCtx, weatherSpan := tracer.Start(request.Context(), "Busca_Temperatura")
		weatherSpan.SetAttributes(attribute.String("weather.city", location.City))
		defer weatherSpan.End()

		var output dto.OutDto
		if weatherErr := rules.NewWeatherUseCase(client).Exec(weatherCtx, key, location.City, &output); weatherErr != nil {
			weatherSpan.SetStatus(codes.Error, weatherErr.Message)
			util.WriteResponse(w, weatherErr.Code, weatherErr.Message)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(output); err != nil {
			log.Printf("Service B response encoding failed: %v", err)
		}
	})

	server := &http.Server{
		Addr:              ":8181",
		Handler:           otelhttp.NewHandler(r, "service-b-http"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Println("Service B listening on :8181")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("Service B stopping")
	case err := <-serverErrors:
		log.Printf("Service B server failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Service B graceful shutdown failed: %v", err)
	}
	if err := shutdownTracer(shutdownCtx); err != nil {
		log.Printf("Service B tracer shutdown failed: %v", err)
	}
}
