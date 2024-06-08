package rules

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/dto"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/util"
)

type Weather struct {
	http *http.Client
}

func NewWeatherUseCase(h *http.Client) *Weather {
	return &Weather{
		http: h,
	}
}

func (w *Weather) Exec(key string, city string, dto *dto.OutDto) *entity.HttpError {

	if key == "" {
		return &entity.HttpError{
			Code:    http.StatusInternalServerError,
			Message: "weather api key not found",
		}
	}

	weatherUrl := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%v&q=%q", key, util.StringPrepare(city))

	resWeather, errWeather := w.http.Get(weatherUrl)

	if errWeather != nil {
		return &entity.HttpError{
			Code:    resWeather.StatusCode,
			Message: errWeather.Error(),
		}
	}

	var weatherData entity.WeatherData
	json.NewDecoder(resWeather.Body).Decode(&weatherData)

	dto.City = city
	dto.Celsius = weatherData.Current.TempC
	dto.Fahrenheit = dto.Celsius*1.8 + 32
	dto.Kelvin = dto.Celsius + 273

	return nil

}
