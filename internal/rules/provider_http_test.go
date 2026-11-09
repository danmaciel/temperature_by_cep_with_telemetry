package rules

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestCepRequestUsesHTTPSAndRequestContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "viacep.com.br" {
			t.Errorf("unexpected CEP provider URL: %s", req.URL)
		}
		if req.Context() != ctx {
			t.Error("request context not propagated to CEP provider")
		}
		return response(http.StatusOK, `{"localidade":"Sao Paulo","cep":"01001000"}`), nil
	})}

	var cep entity.CepIn
	if err := NewCepRules(client).Exec(ctx, "01001000", &cep); err != nil {
		t.Fatalf("unexpected CEP error: %+v", err)
	}
	if cep.City != "Sao Paulo" {
		t.Fatalf("unexpected city: %q", cep.City)
	}
}

func TestCepHandlesTransportErrorWithoutPanic(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("down")
	})}
	var cep entity.CepIn
	err := NewCepRules(client).Exec(context.Background(), "01001000", &cep)
	if err == nil || err.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP 502 for CEP transport failure; got %+v", err)
	}
}

func TestCepRejectsInvalidCharacters(t *testing.T) {
	var cep entity.CepIn
	err := NewCepRules(&http.Client{}).Exec(context.Background(), "abcdefgh", &cep)
	if err == nil || err.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected HTTP 422 for invalid CEP; got %+v", err)
	}
}

func TestWeatherUsesHTTPSAndCorrectConversions(t *testing.T) {
	ctx := context.Background()
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" || req.URL.Host != "api.weatherapi.com" {
			t.Errorf("unexpected WeatherAPI URL: %s", req.URL)
		}
		if req.URL.Query().Get("q") != "Sao Paulo" {
			t.Errorf("unexpected city parameter: %q", req.URL.Query().Get("q"))
		}
		if req.Context() != ctx {
			t.Error("request context not propagated to WeatherAPI")
		}
		return response(http.StatusOK, `{"current":{"temp_c":25}}`), nil
	})}
	var out dto.OutDto
	if err := NewWeatherUseCase(client).Exec(ctx, "test-key", "São Paulo", &out); err != nil {
		t.Fatalf("unexpected weather error: %+v", err)
	}
	if out.Celsius != 25 || out.Fahrenheit != 77 || out.Kelvin != 298.15 {
		t.Fatalf("unexpected temperatures: %+v", out)
	}
}

func TestWeatherHandlesTransportErrorWithoutPanic(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("down")
	})}
	var out dto.OutDto
	err := NewWeatherUseCase(client).Exec(context.Background(), "key", "City", &out)
	if err == nil || err.Code != http.StatusBadGateway {
		t.Fatalf("expected HTTP 502 for WeatherAPI transport failure; got %+v", err)
	}
}


func TestWeatherRejectsIncompleteProviderData(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty object", `{}`},
		{"missing temperature", `{"current":{}}`},
		{"null current", `{"current":null}`},
		{"null temperature", `{"current":{"temp_c":null}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, tc.body), nil
			})}
			var out dto.OutDto
			err := NewWeatherUseCase(client).Exec(context.Background(), "test-key", "City", &out)
			if err == nil || err.Code != http.StatusBadGateway {
				t.Fatalf("expected HTTP 502 for incomplete WeatherAPI response; got %+v", err)
			}
		})
	}
}

func TestWeatherAcceptsZeroCelsiusAsValidTemperature(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"current":{"temp_c":0}}`), nil
	})}
	var out dto.OutDto
	err := NewWeatherUseCase(client).Exec(context.Background(), "test-key", "City", &out)
	if err != nil {
		t.Fatalf("valid zero Celsius must be accepted: %+v", err)
	}
	if out.Celsius != 0 || out.Fahrenheit != 32 || out.Kelvin != 273.15 {
		t.Fatalf("incorrect conversion for zero Celsius: %+v", out)
	}
}
