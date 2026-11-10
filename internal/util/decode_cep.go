package util

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
)

// MaxCEPBodyBytes bounds input for a request that only contains an 8-digit CEP.
// Both public and internal HTTP boundaries enforce the same limit.
const MaxCEPBodyBytes int64 = 1024

// DecodeCEP accepts exactly one JSON value (a CEP object) and rejects
// oversized bodies before attempting any provider request.
func DecodeCEP(w http.ResponseWriter, r *http.Request) (entity.Cep, int) {
	var cep entity.Cep
	r.Body = http.MaxBytesReader(w, r.Body, MaxCEPBodyBytes)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cep); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return cep, http.StatusRequestEntityTooLarge
		}
		return cep, http.StatusBadRequest
	}
	// Reject trailing JSON values or excess whitespace beyond the byte limit.
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return cep, http.StatusRequestEntityTooLarge
		}
		return cep, http.StatusBadRequest
	}
	return cep, http.StatusOK
}
