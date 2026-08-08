package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSON(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	if err := JSON(recorder, http.StatusCreated, map[string]string{"ok": "yes"}); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}

func TestPage(t *testing.T) {
	t.Parallel()
	s := New(Config{})
	s.SetRenderer(RendererFunc(func(context.Context, *http.Request) (string, error) {
		return "<html>ok</html>", nil
	}))
	if err := s.Page("/"); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "<html>ok</html>" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestRequestID(t *testing.T) {
	t.Parallel()
	if got := requestID("request_123.abc"); got != "request_123.abc" {
		t.Fatalf("requestID() = %q, want supplied value", got)
	}
	if got := requestID("invalid value"); !validRequestID(got) || got == "invalid value" {
		t.Fatalf("requestID() = %q, want generated valid value", got)
	}
}

func TestBoundedLogValue(t *testing.T) {
	t.Parallel()
	value := make([]byte, maxLogValueLength+1)
	for i := range value {
		value[i] = 'a'
	}
	if got := boundedLogValue(string(value)); len(got) != maxLogValueLength+3 {
		t.Fatalf("boundedLogValue length = %d, want %d", len(got), maxLogValueLength+3)
	}
}
