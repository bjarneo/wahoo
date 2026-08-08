// Package realtime provides bounded browser realtime transports.
package realtime

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
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
	mu      sync.Mutex
	clients map[chan Event]struct{}
}

// NewHub creates an empty event hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
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
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-client:
			if !ok {
				return
			}
			writeEvent(w, event)
			flusher.Flush()
		}
	}
}

func writeEvent(w io.Writer, event Event) {
	if event.ID != "" {
		_, _ = fmt.Fprintf(w, "id: %s\n", singleLine(event.ID))
	}
	if event.Name != "" {
		_, _ = fmt.Fprintf(w, "event: %s\n", singleLine(event.Name))
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(event.Data), "\r", ""), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", "")
	return strings.ReplaceAll(value, "\n", "")
}
