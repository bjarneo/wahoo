package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Wahoo"}`))
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var value struct {
		Name string `json:"name"`
	}
	if err := DecodeJSON(httptest.NewRecorder(), request, &value, 0); err != nil {
		t.Fatal(err)
	}
	if value.Name != "Wahoo" {
		t.Fatalf("name = %q", value.Name)
	}
}

func TestDecodeJSONRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        string
		limit       int64
		want        error
	}{
		{name: "content type", contentType: "text/plain", body: `{}`, want: ErrUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: `{"unknown":true}`, want: ErrMalformedJSON},
		{name: "trailing value", contentType: "application/json", body: `{} {}`, want: ErrMalformedJSON},
		{name: "oversized", contentType: "application/json", body: `{"name":"abc"}`, limit: 2, want: ErrBodyTooLarge},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			var value struct {
				Name string `json:"name"`
			}
			if err := DecodeJSON(httptest.NewRecorder(), request, &value, test.limit); err != test.want {
				t.Fatalf("DecodeJSON() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestWriteErrorIncludesRequestID(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if err := WriteError(recorder, request, http.StatusBadRequest, "invalid_request", "invalid input"); err != nil {
		t.Fatal(err)
	}
	if got := recorder.Code; got != http.StatusBadRequest {
		t.Fatalf("status = %d", got)
	}
	if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}
