package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewHttpClientRejectsUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHttpClient()
	_, err := client.Get(server.URL)
	if err == nil {
		t.Fatal("expected TLS certificate verification to reject the test server's untrusted certificate")
	}
}

func TestNewHttpClientHasTimeout(t *testing.T) {
	if NewHttpClient().Timeout <= 0 {
		t.Fatal("external requests must have a finite timeout")
	}
}
