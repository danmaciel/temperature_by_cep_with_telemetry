package rules

import (
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
	return &Cep{
		http: h,
	}
}

func (c *Cep) Exec(cep string, vc *entity.CepIn) *entity.HttpError {
	newCep := strings.ReplaceAll(cep, "-", "")

	if !c.isCepValid(newCep) {
		return &entity.HttpError{
			Code:    http.StatusUnprocessableEntity,
			Message: "invalid zipcode",
		}
	}

	res, err := c.http.Get(fmt.Sprintf("https://viacep.com.br/ws/%v/json/", newCep))

	json.NewDecoder(res.Body).Decode(&vc)

	if err != nil || vc.City == "" {
		return &entity.HttpError{
			Code:    http.StatusNotFound,
			Message: "can not find zipcode",
		}
	}

	return nil
}

func (c *Cep) isCepValid(cep string) bool {
	return cep != "" && len(cep) == 8
}
