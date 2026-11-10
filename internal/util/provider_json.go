package util

import (
	"encoding/json"
	"errors"
	"io"
)

// MaxProviderJSONBytes is a conservative safety bound for the tiny JSON
// responses expected from ViaCEP and WeatherAPI in this demonstration.
const MaxProviderJSONBytes int64 = 64 * 1024

var ErrProviderResponseTooLarge = errors.New("provider response exceeds size limit")

// DecodeProviderJSON bounds memory use before decoding any provider data.
// The extra byte distinguishes an exact-limit response from an oversized one.
// json.Unmarshal also rejects trailing JSON values and non-whitespace content.
func DecodeProviderJSON(body io.Reader, destination any) error {
	data, err := io.ReadAll(io.LimitReader(body, MaxProviderJSONBytes+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > MaxProviderJSONBytes {
		return ErrProviderResponseTooLarge
	}
	return json.Unmarshal(data, destination)
}
