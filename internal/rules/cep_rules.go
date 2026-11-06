package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danmaciel/temperature_by_cep_with_telemetry/internal/entity"
)

type Cep struct {
	http *http.Client
}

func NewCepRules(h *http.Client) *Cep {
	return &Cep{http: h}
}

func (c *Cep) Exec(ctx context.Context, cep string, vc *entity.CepIn) *entity.HttpError {
	newCep := strings.ReplaceAll(cep, "-", "")
	if !c.isCepValid(newCep) {
		return &entity.HttpError{Code: http.StatusUnprocessableEntity, Message: "invalid zipcode"}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://viacep.com.br/ws/%s/json/", newCep), nil)
	if err != nil {
		return &entity.HttpError{Code: http.StatusInternalServerError, Message: "failed to create zipcode request"}
	}
	res, err := c.http.Do(req)
	if err != nil {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "zipcode provider unavailable"}
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "zipcode provider returned an error"}
	}
	if err := json.NewDecoder(res.Body).Decode(vc); err != nil {
		return &entity.HttpError{Code: http.StatusBadGateway, Message: "invalid zipcode provider response"}
	}
	if vc.City == "" {
		return &entity.HttpError{Code: http.StatusNotFound, Message: "can not find zipcode"}
	}
	return nil
}

func (c *Cep) isCepValid(cep string) bool {
	if len(cep) != 8 {
		return false
	}
	for _, ch := range cep {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
