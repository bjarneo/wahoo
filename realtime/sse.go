// Package realtime provides bounded browser realtime transports.
package realtime

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxClients   = 1000
	defaultMaxStreamAge = 15 * time.Minute
	heartbeatInterval   = 25 * time.Second
)

// Event is an SSE message.
type Event struct {
	ID   string
	Name string
	Data []byte
}

// Hub fans events out to connected clients. Each client has one buffered
// message slot; slow clients are disconnected rather than growing memory.
type Hub struct {
	mu           sync.Mutex
	clients      map[chan Event]struct{}
	maxClients   int
	maxStreamAge time.Duration
}

// HubOption configures a Hub.
type HubOption func(*Hub)

// WithMaxClients limits simultaneous SSE clients. Values below one use the
// default limit.
func WithMaxClients(limit int) HubOption {
	return func(h *Hub) {
		if limit > 0 {
			h.maxClients = limit
		}
	}
}

// WithMaxStreamAge limits the lifetime of each SSE connection. Values below
// one use the default lifetime.
func WithMaxStreamAge(age time.Duration) HubOption {
	return func(h *Hub) {
		if age > 0 {
			h.maxStreamAge = age
		}
	}
}

// NewHub creates an empty event hub.
func NewHub(options ...HubOption) *Hub {
	hub := &Hub{
		clients:      make(map[chan Event]struct{}),
		maxClients:   defaultMaxClients,
		maxStreamAge: defaultMaxStreamAge,
	}
	for _, option := range options {
		option(hub)
	}
	return hub
}

// Publish sends an event to every connected client. Slow consumers are
// removed so a single browser cannot block the application.
func (h *Hub) Publish(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client <- event:
		default:
			close(client)
			delete(h.clients, client)
		}
	}
}

// ServeHTTP streams events until the request is canceled.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	client := make(chan Event, 1)
	h.mu.Lock()
	if len(h.clients) >= h.maxClients {
		h.mu.Unlock()
		http.Error(w, "stream capacity reached", http.StatusServiceUnavailable)
		return
	}
	h.clients[client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if _, ok := h.clients[client]; ok {
			delete(h.clients, client)
			close(client)
		}
		h.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()
	ctx, cancel := context.WithTimeout(r.Context(), h.maxStreamAge)
	defer cancel()
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-client:
			if !ok {
				return
			}
			if err := writeEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeEvent(w io.Writer, event Event) error {
	if event.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", singleLine(event.ID)); err != nil {
			return err
		}
	}
	if event.Name != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", singleLine(event.Name)); err != nil {
			return err
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(event.Data), "\r", ""), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}
