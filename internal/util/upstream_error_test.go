package util

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
)

func TestUpstreamHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err error
		want int
	}{
		{"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"wrapped deadline", &url.Error{Op: "Get", URL: "https://example.invalid", Err: context.DeadlineExceeded}, http.StatusGatewayTimeout},
		{"network timeout", &net.DNSError{IsTimeout: true}, http.StatusGatewayTimeout},
		{"connection refused", errors.New("connection refused"), http.StatusBadGateway},
		{"caller cancellation", context.Canceled, http.StatusBadGateway},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := UpstreamHTTPStatus(tc.err); got != tc.want {
				t.Fatalf("got HTTP %d, want HTTP %d", got, tc.want)
			}
		})
	}
}
