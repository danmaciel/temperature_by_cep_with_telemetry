package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/util"
)

type Weather struct {
	http *http.Client
}

func NewWeatherUseCase(h *http.Client) *Weather {
	return &Weather{http: h}
}

func (w *Weather) Exec(ctx context.Context, key string, city string, out *dto.OutDto) *entity.HttpError {
	if key == "" {
		return &entity.HttpError{Code: http.StatusInternalServerError, Message: "weather api key not found"}
	}

	weatherURL := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s", url.QueryEscape(key), util.StringPrepare(city))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, weatherURL, nil)
	if err != nil {
		return &entity.HttpError{Code: http.StatusInternalServerError, Message: "failed to create weather request"}
	}
	res, err := w.http.Do(req)
	if err != nil {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "weather provider unavailable"}
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "weather provider returned an error"}
	}
	var weatherData entity.WeatherData
	if err := json.NewDecoder(res.Body).Decode(&weatherData); err != nil {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "invalid weather provider response"}
	}
	out.City = city
	out.Celsius = weatherData.Current.TempC
	out.Fahrenheit = out.Celsius*1.8 + 32
	out.Kelvin = out.Celsius + 273.15
	return nil
}
