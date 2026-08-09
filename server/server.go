// Package server provides the HTTP runtime used by Wahoo applications.
package server

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
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
	Addr              string
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxHeaderBytes    int
	Logger            *slog.Logger
	Observer          RequestObserver
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 15 * time.Second
	}
	if c.ReadHeaderTimeout == 0 {
		c.ReadHeaderTimeout = 5 * time.Second
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
	if c.MaxHeaderBytes == 0 {
		c.MaxHeaderBytes = 1 << 20
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return c
}

// Server owns routing and graceful HTTP lifecycle.
type Server struct {
	config      Config
	mux         *http.ServeMux
	renderer    Renderer
	middlewares []Middleware
}

const (
	maxRequestIDLength = 128
	maxLogValueLength  = 2048
)

type requestIDKey struct{}

// Middleware wraps public HTTP traffic. Register middleware before serving.
type Middleware func(http.Handler) http.Handler

// RateLimiter permits or rejects one application-defined key. Applications
// choose the key and use a shared implementation when they run more than one
// public server instance.
type RateLimiter interface {
	Allow(context.Context, string) (bool, error)
}

// RateLimitKey derives one bounded application-defined limiter key.
type RateLimitKey func(*http.Request) string

// RequestObservation records one completed request. Observers should return
// quickly and must not retain request data beyond their own bounded policy.
type RequestObservation struct {
	RequestID string
	Method    string
	Path      string
	Status    int
	Bytes     int64
	Duration  time.Duration
	Panicked  bool
}

// RequestObserver receives request observations after a response completes.
type RequestObserver interface {
	ObserveRequest(context.Context, RequestObservation)
}

// RequestObserverFunc adapts a function to RequestObserver.
type RequestObserverFunc func(context.Context, RequestObservation)

// ObserveRequest calls f.
func (f RequestObserverFunc) ObserveRequest(ctx context.Context, observation RequestObservation) {
	f(ctx, observation)
}

// HeaderPolicy declares application-owned security response headers. Empty
// fields are not set, so applications can adopt a policy deliberately.
type HeaderPolicy struct {
	ContentSecurityPolicy   string
	PermissionsPolicy       string
	ReferrerPolicy          string
	StrictTransportSecurity string
	FrameOptions            string
	NoSniff                 bool
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

// Use appends middleware to the public request pipeline. Call Use during
// application setup before requests are served.
func (s *Server) Use(middlewares ...Middleware) {
	for _, middleware := range middlewares {
		if middleware != nil {
			s.middlewares = append(s.middlewares, middleware)
		}
	}
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
			s.config.Logger.Error("render page", "path", boundedLogValue(r.URL.Path), "err", err)
			http.Error(w, "rendering failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(document))
	})
	return nil
}

// ServeHTTP applies request ID, recovery, configured middleware, access
// logging, and optional request observation.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	requestID := requestID(r.Header.Get("X-Request-ID"))
	w.Header().Set("X-Request-ID", requestID)
	r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, requestID))
	recorder := &responseRecorder{ResponseWriter: w}
	panicked := false
	defer func() {
		if recovered := recover(); recovered != nil {
			panicked = true
			s.config.Logger.Error("panic recovered", "request_id", requestID, "panic", recovered)
			if !recorder.wroteHeader {
				http.Error(recorder, "internal server error", http.StatusInternalServerError)
			}
		}
		observation := RequestObservation{
			RequestID: requestID,
			Method:    r.Method,
			Path:      boundedLogValue(r.URL.Path),
			Status:    recorder.statusCode(),
			Bytes:     recorder.bytes,
			Duration:  time.Since(started),
			Panicked:  panicked,
		}
		s.config.Logger.Info("request completed", "request_id", observation.RequestID, "method", observation.Method, "path", observation.Path, "status", observation.Status, "bytes", observation.Bytes, "duration_ms", observation.Duration.Milliseconds(), "panicked", observation.Panicked)
		s.observe(r.Context(), observation)
	}()
	var handler http.Handler = s.mux
	for index := len(s.middlewares) - 1; index >= 0; index-- {
		handler = s.middlewares[index](handler)
	}
	handler.ServeHTTP(recorder, r)
}

// RequestID returns the request ID assigned by Server, or an empty string when
// called outside an HTTP request handled by Server.
func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

// SecurityHeaders returns middleware that applies an explicit application
// header policy. It does not infer TLS, proxy, or CORS settings.
func SecurityHeaders(policy HeaderPolicy) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if policy.ContentSecurityPolicy != "" {
				w.Header().Set("Content-Security-Policy", policy.ContentSecurityPolicy)
			}
			if policy.PermissionsPolicy != "" {
				w.Header().Set("Permissions-Policy", policy.PermissionsPolicy)
			}
			if policy.ReferrerPolicy != "" {
				w.Header().Set("Referrer-Policy", policy.ReferrerPolicy)
			}
			if policy.StrictTransportSecurity != "" {
				w.Header().Set("Strict-Transport-Security", policy.StrictTransportSecurity)
			}
			if policy.FrameOptions != "" {
				w.Header().Set("X-Frame-Options", policy.FrameOptions)
			}
			if policy.NoSniff {
				w.Header().Set("X-Content-Type-Options", "nosniff")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBodyBytes limits request bodies before application handlers read them.
// The application must select a limit appropriate for each public surface.
func MaxBodyBytes(limit int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limit < 1 {
				http.Error(w, "request body limit is not configured", http.StatusInternalServerError)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit applies an application-selected limiter before the next handler.
// Limiter failures return 503 rather than allowing traffic without a policy.
func RateLimit(limiter RateLimiter, key RateLimitKey) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || key == nil {
				http.Error(w, "rate limit is not configured", http.StatusInternalServerError)
				return
			}
			allowed, err := limiter.Allow(r.Context(), key(r))
			if err != nil {
				http.Error(w, "rate limit unavailable", http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "1")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestID(value string) string {
	if validRequestID(value) {
		return value
	}
	var data [16]byte
	if _, err := rand.Read(data[:]); err == nil {
		return hex.EncodeToString(data[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func validRequestID(value string) bool {
	if value == "" || len(value) > maxRequestIDLength {
		return false
	}
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

func boundedLogValue(value string) string {
	if len(value) <= maxLogValueLength {
		return value
	}
	return value[:maxLogValueLength] + "..."
}

// HTTPServer returns the configured net/http server. It is useful when an
// application needs to attach additional listeners or test ServeHTTP.
func (s *Server) HTTPServer() *http.Server {
	return &http.Server{
		Addr:              s.config.Addr,
		Handler:           s,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
		ReadTimeout:       s.config.ReadTimeout,
		WriteTimeout:      s.config.WriteTimeout,
		IdleTimeout:       s.config.IdleTimeout,
		MaxHeaderBytes:    s.config.MaxHeaderBytes,
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

func (s *Server) observe(ctx context.Context, observation RequestObservation) {
	if s.config.Observer == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			s.config.Logger.Error("request observer panicked", "request_id", observation.RequestID, "panic", recovered)
		}
	}()
	s.config.Observer.ObserveRequest(ctx, observation)
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	written, err := w.ResponseWriter.Write(data)
	w.bytes += int64(written)
	return written, err
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseRecorder) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	if !w.wroteHeader {
		w.status = http.StatusSwitchingProtocols
		w.wroteHeader = true
	}
	return hijacker.Hijack()
}

func (w *responseRecorder) Push(target string, options *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, options)
}

func (w *responseRecorder) ReadFrom(reader io.Reader) (int64, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if readerFrom, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		written, err := readerFrom.ReadFrom(reader)
		w.bytes += written
		return written, err
	}
	return io.Copy(struct{ io.Writer }{w}, reader)
}

func (w *responseRecorder) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
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
