package realtime

import (
	"bytes"
	"testing"
)

func TestWriteEventEscapesLineBreaks(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	if err := writeEvent(&out, Event{
		ID:   "1\nevil",
		Name: "update\r\nevil",
		Data: []byte("first\n\nevent: injected"),
	}); err != nil {
		t.Fatal(err)
	}
	const want = "id: 1evil\nevent: updateevil\ndata: first\ndata: \ndata: event: injected\n\n"
	if got := out.String(); got != want {
		t.Fatalf("writeEvent() = %q, want %q", got, want)
	}
}

func TestHubDefaults(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	if hub.maxClients != defaultMaxClients {
		t.Fatalf("max clients = %d, want %d", hub.maxClients, defaultMaxClients)
	}
	if hub.maxStreamAge != defaultMaxStreamAge {
		t.Fatalf("max stream age = %s, want %s", hub.maxStreamAge, defaultMaxStreamAge)
	}
}

func TestPublishToRejectsInvalidTopic(t *testing.T) {
	t.Parallel()
	if err := NewHub().PublishTo("tenant space", Event{}); err == nil {
		t.Fatal("PublishTo() returned nil for invalid topic")
	}
}

func TestPublishCopiesEventData(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	client := make(chan Event, 1)
	hub.clients[""] = map[chan Event]struct{}{client: {}}
	data := []byte("before")
	hub.Publish(Event{Data: data})
	data[0] = 'a'
	if got := string((<-client).Data); got != "before" {
		t.Fatalf("event data = %q", got)
	}
}
