package lifecycle

import (
	"context"
	"net/http"
	"testing"
)

func TestServeStopsWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	server := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	}
	if err := Serve(ctx, server); err != nil {
		t.Fatalf("expected clean shutdown on context cancellation: %v", err)
	}
}
