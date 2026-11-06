package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/infra/server/lifecycle"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/provider"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := provider.InitProvider("service-a")
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

	serviceBURL := os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		serviceBURL = "http://service_b:8181/cep"
	}
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = ":8080"
	}
	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   15 * time.Second,
	}
	server := &http.Server{
		Addr:              port,
		Handler:           otelhttp.NewHandler(newRouter(client, serviceBURL), "service-a.http"),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("service A listening at %s", port)
	if err := lifecycle.Serve(ctx, server); err != nil {
		log.Printf("service A stopped: %v", err)
		return
	}
}

func newRouter(client *http.Client, serviceBURL string) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/cep", func(w http.ResponseWriter, request *http.Request) {
		var cep entity.Cep
		if err := json.NewDecoder(http.MaxBytesReader(w, request.Body, 4096)).Decode(&cep); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(cep.Value) != 8 {
			http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
			return
		}
		for _, digit := range cep.Value {
			if digit < '0' || digit > '9' {
				http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
				return
			}
		}
		forwardCEP(request.Context(), w, client, serviceBURL, cep)
	})
	return r
}

func forwardCEP(ctx context.Context, w http.ResponseWriter, client *http.Client, serviceBURL string, cep entity.Cep) {
	payload, err := json.Marshal(cep)
	if err != nil {
		http.Error(w, "invalid zipcode payload", http.StatusInternalServerError)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceBURL, bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "cannot create service B request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// The otelhttp transport creates a client span and injects traceparent.
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "service B unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("forward service B response: %v", err)
	}
}
