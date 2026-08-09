// Package realtime provides bounded browser realtime transports.
package realtime

import (
	"context"
	"errors"
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
	maxTopicBytes       = 128
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
	clients      map[string]map[chan Event]struct{}
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
		clients:      make(map[string]map[chan Event]struct{}),
		maxClients:   defaultMaxClients,
		maxStreamAge: defaultMaxStreamAge,
	}
	for _, option := range options {
		option(hub)
	}
	return hub
}

// Publish sends a global event to every connection registered through
// ServeHTTP. Prefer PublishTo for tenant- or subject-scoped application data.
func (h *Hub) Publish(event Event) {
	h.publish("", event)
}

// PublishTo sends an event only to clients subscribed to topic. Applications
// must authorize a request before they expose a topic handler.
func (h *Hub) PublishTo(topic string, event Event) error {
	if !validTopic(topic) {
		return errors.New("invalid event topic")
	}
	h.publish(topic, event)
	return nil
}

func (h *Hub) publish(topic string, event Event) {
	event = copyEvent(event)
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients[topic] {
		select {
		case client <- copyEvent(event):
		default:
			close(client)
			delete(h.clients[topic], client)
		}
	}
}

// ServeHTTP streams events until the request is canceled.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.serveTopic("", w, r)
}

// Topic returns a handler for one application-selected topic. It does not
// authenticate callers or resolve a tenant; applications must do that before
// registering or invoking the returned handler.
func (h *Hub) Topic(topic string) (http.Handler, error) {
	if !validTopic(topic) {
		return nil, errors.New("invalid event topic")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.serveTopic(topic, w, r)
	}), nil
}

func (h *Hub) serveTopic(topic string, w http.ResponseWriter, r *http.Request) {
	client := make(chan Event, 1)
	h.mu.Lock()
	if h.clientCount() >= h.maxClients {
		h.mu.Unlock()
		http.Error(w, "stream capacity reached", http.StatusServiceUnavailable)
		return
	}
	if h.clients[topic] == nil {
		h.clients[topic] = make(map[chan Event]struct{})
	}
	h.clients[topic][client] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if _, ok := h.clients[topic][client]; ok {
			delete(h.clients[topic], client)
			close(client)
		}
		if len(h.clients[topic]) == 0 {
			delete(h.clients, topic)
		}
		h.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Time{})
	if err := controller.Flush(); err != nil {
		return
	}
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
			if err := controller.Flush(); err != nil {
				return
			}
		case event, ok := <-client:
			if !ok {
				return
			}
			if err := writeEvent(w, event); err != nil {
				return
			}
			if err := controller.Flush(); err != nil {
				return
			}
		}
	}
}

func (h *Hub) clientCount() int {
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}

func copyEvent(event Event) Event {
	event.Data = append([]byte(nil), event.Data...)
	return event
}

func validTopic(topic string) bool {
	if len(topic) == 0 || len(topic) > maxTopicBytes {
		return false
	}
	for index := range topic {
		char := topic[index]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' && char != ':' {
			return false
		}
	}
	return true
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
