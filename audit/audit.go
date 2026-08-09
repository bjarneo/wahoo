// Package audit defines immutable, bounded security and business audit events.
package audit

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxIDBytes limits an event's idempotency identifier.
	MaxIDBytes = 128
	// MaxActorBytes limits the principal identifier recorded on an event.
	MaxActorBytes = 256
	// MaxActionBytes limits an event action identifier.
	MaxActionBytes = 128
	// MaxTargetBytes limits a resource identifier.
	MaxTargetBytes = 512
	// MaxMetadataEntries limits structured event metadata fields.
	MaxMetadataEntries = 32
	// MaxMetadataKeyBytes limits one metadata key.
	MaxMetadataKeyBytes = 64
	// MaxMetadataValueBytes limits one metadata value.
	MaxMetadataValueBytes = 1024
	// MaxMetadataBytes limits all event metadata strings.
	MaxMetadataBytes = 8 << 10
)

var (
	// ErrInvalidEvent reports input outside the audit event contract.
	ErrInvalidEvent = errors.New("invalid audit event")
	// ErrWriterUnavailable reports an attempt to call a nil WriterFunc.
	ErrWriterUnavailable = errors.New("audit writer unavailable")
)

// EventInput supplies the immutable fields for a new Event. ID and Action are
// required. A zero OccurredAt is set to the current UTC time.
type EventInput struct {
	ID         string
	OccurredAt time.Time
	Actor      string
	Action     string
	Target     string
	Metadata   map[string]string
}

// Event is a validated immutable audit record. Metadata returns a copy so event
// data cannot be changed after construction.
type Event struct {
	id         string
	occurredAt time.Time
	actor      string
	action     string
	target     string
	metadata   map[string]string
}

// NewEvent validates input and copies its metadata.
func NewEvent(input EventInput) (Event, error) {
	if !validIdentifier(input.ID, MaxIDBytes) || !validIdentifier(input.Action, MaxActionBytes) || !validOptionalText(input.Actor, MaxActorBytes) || !validOptionalText(input.Target, MaxTargetBytes) {
		return Event{}, ErrInvalidEvent
	}
	if len(input.Metadata) > MaxMetadataEntries {
		return Event{}, ErrInvalidEvent
	}
	metadata := make(map[string]string, len(input.Metadata))
	total := 0
	for key, value := range input.Metadata {
		if !validMetadataKey(key) || !validOptionalText(value, MaxMetadataValueBytes) {
			return Event{}, ErrInvalidEvent
		}
		total += len(key) + len(value)
		if total > MaxMetadataBytes {
			return Event{}, ErrInvalidEvent
		}
		metadata[key] = value
	}
	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}
	return Event{
		id:         input.ID,
		occurredAt: occurredAt.Round(0).UTC(),
		actor:      input.Actor,
		action:     input.Action,
		target:     input.Target,
		metadata:   metadata,
	}, nil
}

// Validate reports whether e is a valid Event. It is useful for Writer
// implementations that receive an Event from an untrusted process boundary.
func (e Event) Validate() error {
	if !validIdentifier(e.id, MaxIDBytes) || e.occurredAt.IsZero() || !validIdentifier(e.action, MaxActionBytes) || !validOptionalText(e.actor, MaxActorBytes) || !validOptionalText(e.target, MaxTargetBytes) {
		return ErrInvalidEvent
	}
	if len(e.metadata) > MaxMetadataEntries {
		return ErrInvalidEvent
	}
	total := 0
	for key, value := range e.metadata {
		if !validMetadataKey(key) || !validOptionalText(value, MaxMetadataValueBytes) {
			return ErrInvalidEvent
		}
		total += len(key) + len(value)
		if total > MaxMetadataBytes {
			return ErrInvalidEvent
		}
	}
	return nil
}

// ID returns the event's idempotency identifier.
func (e Event) ID() string {
	return e.id
}

// OccurredAt returns the UTC event time without a monotonic clock value.
func (e Event) OccurredAt() time.Time {
	return e.occurredAt
}

// Actor returns the optional principal identifier.
func (e Event) Actor() string {
	return e.actor
}

// Action returns the required action identifier.
func (e Event) Action() string {
	return e.action
}

// Target returns the optional resource identifier.
func (e Event) Target() string {
	return e.target
}

// Metadata returns a copy of the event metadata.
func (e Event) Metadata() map[string]string {
	if len(e.metadata) == 0 {
		return nil
	}
	metadata := make(map[string]string, len(e.metadata))
	for key, value := range e.metadata {
		metadata[key] = value
	}
	return metadata
}

// Writer persists one immutable audit event. Implementations must preserve the
// event as an append-only record and must not mutate it.
type Writer interface {
	Write(context.Context, Event) error
}

// WriterFunc adapts a function to Writer.
type WriterFunc func(context.Context, Event) error

// Write validates event and calls f.
func (f WriterFunc) Write(ctx context.Context, event Event) error {
	if f == nil {
		return ErrWriterUnavailable
	}
	if err := event.Validate(); err != nil {
		return err
	}
	return f(ctx, event)
}

func validIdentifier(value string, limit int) bool {
	if len(value) == 0 || len(value) > limit {
		return false
	}
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '_' && char != '-' && char != ':' && char != '/' {
			return false
		}
	}
	return true
}

func validOptionalText(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func validMetadataKey(value string) bool {
	if len(value) == 0 || len(value) > MaxMetadataKeyBytes {
		return false
	}
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}
