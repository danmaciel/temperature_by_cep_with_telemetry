package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
)

func main() {

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Post("/cep", func(w http.ResponseWriter, r *http.Request) {

		var cep entity.Cep
		err := json.NewDecoder(r.Body).Decode(&cep)
		if err != nil {
			http.Error(w, "Erro ao decodificar o JSON", http.StatusBadRequest)
			return
		}

		if len(cep.Value) != 8 {
			http.Error(w, "Invalid zipcode", http.StatusUnprocessableEntity)

			return
		}
		runServiceB(w, cep)
	})

	go func() {
		fmt.Println("Serviço A escutando na porta 8080...")

		if err := http.ListenAndServe(":8080", r); err != nil {
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

func runServiceB(w http.ResponseWriter, cep entity.Cep) {
	url := "http://service_b:8181/cep" // Usando o nome do serviço no docker-compose
	result, _ := json.Marshal(cep)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(result))
	if err != nil {
		http.Error(w, "Erro ao enviar CEP para o serviço B"+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)

	if err != nil {
		http.Error(w, "Erro o copiar resposta do serviço B:"+err.Error(), http.StatusInternalServerError)
	}
}
