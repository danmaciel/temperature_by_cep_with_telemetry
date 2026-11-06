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
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/provider"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracer, err := provider.InitProvider("service-a")
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Post("/cep", func(w http.ResponseWriter, request *http.Request) {
		var cep entity.Cep
		if err := json.NewDecoder(request.Body).Decode(&cep); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(cep.Value) != 8 {
			http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
			return
		}
		runServiceB(request.Context(), w, cep)
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           otelhttp.NewHandler(r, "service-a-http"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Println("Service A listening on :8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("Service A stopping")
	case err := <-serverErrors:
		log.Printf("Service A server failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Service A graceful shutdown failed: %v", err)
	}
	if err := shutdownTracer(shutdownCtx); err != nil {
		log.Printf("Service A tracer shutdown failed: %v", err)
	}
}

func runServiceB(ctx context.Context, w http.ResponseWriter, cep entity.Cep) {
	client := &http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport), Timeout: 15 * time.Second}
	forwardToServiceB(ctx, w, cep, "http://service_b:8181/cep", client)
}

// forwardToServiceB makes the downstream endpoint injectable for regression tests.
func forwardToServiceB(ctx context.Context, w http.ResponseWriter, cep entity.Cep, endpoint string, client *http.Client) {
	payload, err := json.Marshal(cep)
	if err != nil {
		http.Error(w, "failed to encode zipcode", http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		http.Error(w, "failed to create service request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// otelhttp injects the active span context into the outgoing request.
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "downstream service unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Service A response stream failed: %v", err)
	}
}
