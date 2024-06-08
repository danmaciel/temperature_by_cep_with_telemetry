package entity

type Current struct {
	TempC float64 `json:"temp_c"`
}

type WeatherData struct {
	Current Current `json:"current"`
}
