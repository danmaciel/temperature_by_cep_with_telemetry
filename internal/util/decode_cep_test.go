package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeCEPBodyLimitsAndSingleJSONValue(t *testing.T) {
	cases := []struct {
		name string
		body string
		wantStatus int
		wantCEP string
	}{
		{"valid", `{"cep":"01001000"}`, http.StatusOK, "01001000"},
		{"trailing JSON", `{"cep":"01001000"} {"cep":"99999999"}`, http.StatusBadRequest, ""},
		{"invalid JSON", `{"cep":`, http.StatusBadRequest, ""},
		{"oversized JSON", `{"cep":"01001000","padding":"` + strings.Repeat("a", 1024) + `"}`, http.StatusRequestEntityTooLarge, ""},
		{"oversized whitespace", `{"cep":"01001000"}` + strings.Repeat(" ", 1024), http.StatusRequestEntityTooLarge, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/cep", strings.NewReader(tc.body))
			got, status := DecodeCEP(httptest.NewRecorder(), request)
			if status != tc.wantStatus {
				t.Fatalf("status: got %d, want %d", status, tc.wantStatus)
			}
			if status == http.StatusOK && got.Value != tc.wantCEP {
				t.Fatalf("cep: got %q, want %q", got.Value, tc.wantCEP)
			}
		})
	}
}
