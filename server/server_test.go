package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestServeHTTPAddsRequestContextAndObservation(t *testing.T) {
	t.Parallel()
	observations := make(chan RequestObservation, 1)
	s := New(Config{Observer: RequestObserverFunc(func(_ context.Context, observation RequestObservation) {
		observations <- observation
	})})
	s.Use(SecurityHeaders(HeaderPolicy{NoSniff: true}))
	s.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if got := RequestID(r.Context()); got != "request_123" {
			t.Errorf("request ID = %q", got)
		}
		_, _ = io.WriteString(w, "ok")
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Request-ID", "request_123")
	s.ServeHTTP(recorder, request)
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	select {
	case observation := <-observations:
		if observation.Status != http.StatusOK || observation.Bytes != 2 || observation.Panicked {
			t.Fatalf("observation = %#v", observation)
		}
	case <-time.After(time.Second):
		t.Fatal("request observation was not sent")
	}
}

func TestMaxBodyBytes(t *testing.T) {
	t.Parallel()
	s := New(Config{})
	s.Use(MaxBodyBytes(2))
	s.HandleFunc("POST /", func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Fatal("ReadAll returned nil error for oversized body")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("abc")))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

type denyLimiter struct{}

func (denyLimiter) Allow(context.Context, string) (bool, error) {
	return false, nil
}

func TestRateLimit(t *testing.T) {
	t.Parallel()
	s := New(Config{})
	s.Use(RateLimit(denyLimiter{}, func(*http.Request) string { return "usr_123" }))
	s.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	s.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", recorder.Code)
	}
}
