package util

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeProviderJSONAcceptsValidResponse(t *testing.T) {
	var value struct {
		City string `json:"city"`
	}
	err := DecodeProviderJSON(strings.NewReader(`{"city":"Marilia"}`), &value)
	if err != nil {
		t.Fatalf("valid response: %v", err)
	}
	if value.City != "Marilia" {
		t.Fatalf("unexpected city: %q", value.City)
	}
}

func TestDecodeProviderJSONEnforcesSizeAndValidJSON(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		wantLarge  bool
		wantError  bool
	}{
		{"valid", `{"value":1}`, false, false},
		{"oversized object", `{"padding":"` + strings.Repeat("a", int(MaxProviderJSONBytes)) + `"}`, true, true},
		{"oversized whitespace", `{"value":1}` + strings.Repeat(" ", int(MaxProviderJSONBytes)), true, true},
		{"multiple objects", `{"value":1} {"value":2}`, false, true},
		{"invalid JSON", `{"value":`, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var dst struct {
				Value int `json:"value"`
			}
			err := DecodeProviderJSON(strings.NewReader(tc.payload), &dst)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, want error=%v", err, tc.wantError)
			}
			if errors.Is(err, ErrProviderResponseTooLarge) != tc.wantLarge {
				t.Fatalf("oversized error=%v, want=%v", err, tc.wantLarge)
			}
		})
	}
}

// The exact limit is permitted; a response with one additional byte is not.
func TestDecodeProviderJSONAtExactLimit(t *testing.T) {
	prefix := `{"padding":"`
	suffix := `"}`
	padding := int(MaxProviderJSONBytes) - len(prefix) - len(suffix)
	body := prefix + strings.Repeat("x", padding) + suffix
	var dst map[string]string
	if err := DecodeProviderJSON(strings.NewReader(body), &dst); err != nil {
		t.Fatalf("response exactly at limit should be accepted: %v", err)
	}
	if len(dst["padding"]) != padding {
		t.Fatalf("unexpected decoded padding length")
	}
	if err := DecodeProviderJSON(strings.NewReader(body+" "), &dst); !errors.Is(err, ErrProviderResponseTooLarge) {
		t.Fatalf("expected oversize error for limit+1 byte, got %v", err)
	}
}
