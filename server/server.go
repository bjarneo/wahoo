// Package server provides the HTTP runtime used by Wahoo applications.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Renderer renders a request into a complete HTML document. A React SSR
// adapter can implement this interface by calling a Node renderer over stdin,
// HTTP, or a Unix socket.
type Renderer interface {
	Render(context.Context, *http.Request) (string, error)
}

// RendererFunc adapts a function to Renderer.
type RendererFunc func(context.Context, *http.Request) (string, error)

// Render calls f.
func (f RendererFunc) Render(ctx context.Context, r *http.Request) (string, error) {
	return f(ctx, r)
}

// Config controls the HTTP runtime.
type Config struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	Logger          *slog.Logger
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 15 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = 15 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

// Server owns routing and graceful HTTP lifecycle.
type Server struct {
	config   Config
	mux      *http.ServeMux
	renderer Renderer
}

// New creates a server with production-safe timeout defaults.
func New(config Config) *Server {
	config = config.withDefaults()
	return &Server{config: config, mux: http.NewServeMux()}
}

// SetRenderer configures the renderer used by Page.
func (s *Server) SetRenderer(renderer Renderer) {
	s.renderer = renderer
}

// Handle registers a standard library handler.
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// HandleFunc registers a standard library handler function.
func (s *Server) HandleFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Page registers an SSR page. The renderer must be configured first.
func (s *Server) Page(pattern string) error {
	if s.renderer == nil {
		return errors.New("server: configure a renderer before registering pages")
	}
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		document, err := s.renderer.Render(r.Context(), r)
		if err != nil {
			s.config.Logger.Error("render page", "path", r.URL.Path, "err", err)
			http.Error(w, "rendering failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(document))
	})
	return nil
}

// ServeHTTP applies request ID, recovery, and access logging middleware.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	w.Header().Set("X-Request-ID", requestID)
	defer func() {
		if recovered := recover(); recovered != nil {
			s.config.Logger.Error("panic recovered", "request_id", requestID, "panic", recovered)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		s.config.Logger.Info("request completed", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	}()
	s.mux.ServeHTTP(w, r)
}

// HTTPServer returns the configured net/http server. It is useful when an
// application needs to attach additional listeners or test ServeHTTP.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:         s.config.Addr,
		Handler:      s,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}
}

// Run serves until SIGINT or SIGTERM, then drains connections.
func (s *Server) Run(ctx context.Context) error {
	httpServer := s.HTTPServer()
	errCh := make(chan error, 1)
	go func() {
		s.config.Logger.Info("server listening", "addr", s.config.Addr)
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop:
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// JSON writes a JSON response with an optional status code.
func JSON(w http.ResponseWriter, status int, value any) error {
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(value)
}

// HTML writes an HTML response with an optional status code.
func HTML(w http.ResponseWriter, status int, body string) error {
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(body))
	return err
}

// TemplateRenderer is a small renderer for non-React pages and test fixtures.
type TemplateRenderer struct {
	Template *template.Template
}

// Render executes the template using the request path as data.
func (r TemplateRenderer) Render(_ context.Context, req *http.Request) (string, error) {
	if r.Template == nil {
		return "", errors.New("server: template is nil")
	}
	var out strings.Builder
	if err := r.Template.Execute(&out, map[string]string{"Path": req.URL.Path}); err != nil {
		return "", err
	}
	return out.String(), nil
}
