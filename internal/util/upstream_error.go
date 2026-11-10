package util

import (
	"context"
	"errors"
	"net"
	"net/http"
)

// UpstreamHTTPStatus distinguishes a provider timeout from other upstream
// transport failures. A canceled caller is not an upstream timeout.
func UpstreamHTTPStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}
