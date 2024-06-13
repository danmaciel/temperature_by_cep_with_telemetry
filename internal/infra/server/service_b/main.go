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
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	httpClient "github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/http"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/provider"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/rules"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/util"
)

func main() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	shutdown, err := provider.InitProvider("Service_B")
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	tracer := otel.GetTracerProvider().Tracer("SERVICE_B")
	r.Post("/cep", func(w http.ResponseWriter, r *http.Request) {
		var cep entity.Cep

		propagator := otel.GetTextMapPropagator()
		ctxTracer := propagator.Extract(ctx, propagation.HeaderCarrier(r.Header))

		err := json.NewDecoder(r.Body).Decode(&cep)
		if err != nil {
			http.Error(w, "Erro ao decodificar o JSON", http.StatusBadRequest)
			return
		}
		ctxTracer, span := tracer.Start(ctxTracer, "Busca_CEP", trace.WithSpanKind(trace.SpanKindClient))

		clientHttp := httpClient.NewHttpClient()

		cepRules := rules.NewCepRules(clientHttp)

		var cepModel entity.CepIn
		errCepIn := cepRules.Exec(cep.Value, &cepModel)
		span.End()
		if errCepIn != nil {
			util.WriteResponse(w, errCepIn.Code, errCepIn.Message)
			return
		}

		_, span2 := tracer.Start(ctxTracer, "Busca_Temperatura", trace.WithSpanKind(trace.SpanKindClient))
		span2.SetAttributes(attribute.String("span.type", "Busca_Temperatura"))

		defer span2.End()
		weatherUseCase := rules.NewWeatherUseCase(clientHttp)
		key := os.Getenv("WEATHER_API_KEY")

		if key == "" {
			util.WriteResponse(w, http.StatusInternalServerError, "weather api key not found")
			return
		}

		var dto dto.OutDto
		errWeather := weatherUseCase.Exec(key, cepModel.City, &dto)

		if errWeather != nil {
			util.WriteResponse(w, errWeather.Code, errWeather.Message)
			return
		}

		result, errOnJson := json.Marshal(dto)

		if errOnJson != nil {
			util.WriteResponse(w, http.StatusInternalServerError, "error on generate json from data")
			return
		}

		util.GetResponseHeader(w)
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
