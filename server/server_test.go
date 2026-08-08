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
