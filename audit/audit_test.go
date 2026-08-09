package audit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewEventCopiesMetadataAndNormalizesTime(t *testing.T) {
	t.Parallel()
	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.FixedZone("test", 3600))
	input := EventInput{
		ID:         "evt-123",
		OccurredAt: timestamp,
		Actor:      "user-123",
		Action:     "document.created",
		Target:     "document/456",
		Metadata:   map[string]string{"source": "web"},
	}
	event, err := NewEvent(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Metadata["source"] = "changed"
	if got := event.Metadata()["source"]; got != "web" {
		t.Fatalf("stored metadata = %q, want original", got)
	}
	metadata := event.Metadata()
	metadata["source"] = "changed again"
	if got := event.Metadata()["source"]; got != "web" {
		t.Fatalf("returned metadata mutated event to %q", got)
	}
	if event.ID() != input.ID || event.Actor() != input.Actor || event.Action() != input.Action || event.Target() != input.Target {
		t.Fatal("event accessors returned unexpected values")
	}
	if !event.OccurredAt().Equal(timestamp.UTC()) || event.OccurredAt().Location() != time.UTC {
		t.Fatalf("OccurredAt() = %s, want normalized UTC time", event.OccurredAt())
	}
}

func TestNewEventRejectsInvalidBounds(t *testing.T) {
	t.Parallel()
	if _, err := NewEvent(EventInput{ID: "evt", Action: "bad action"}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("invalid action error = %v, want %v", err, ErrInvalidEvent)
	}
	metadata := make(map[string]string, MaxMetadataEntries+1)
	for i := range MaxMetadataEntries + 1 {
		metadata["key"+string(rune('a'+i))] = "value"
	}
	if _, err := NewEvent(EventInput{ID: "evt", Action: "created", Metadata: metadata}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("too much metadata error = %v, want %v", err, ErrInvalidEvent)
	}
}

func TestWriterFuncValidatesEvent(t *testing.T) {
	t.Parallel()
	event, err := NewEvent(EventInput{ID: "evt", Action: "created"})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	writer := WriterFunc(func(_ context.Context, got Event) error {
		called = true
		if got.ID() != event.ID() {
			t.Fatal("writer received wrong event")
		}
		return nil
	})
	if err := writer.Write(t.Context(), event); err != nil || !called {
		t.Fatalf("Write() = %v, called = %t", err, called)
	}
	called = false
	if err := writer.Write(t.Context(), Event{}); !errors.Is(err, ErrInvalidEvent) || called {
		t.Fatalf("invalid Write() = %v, called = %t", err, called)
	}
	var unavailable WriterFunc
	if err := unavailable.Write(t.Context(), event); !errors.Is(err, ErrWriterUnavailable) {
		t.Fatalf("nil WriterFunc error = %v, want %v", err, ErrWriterUnavailable)
	}
}
