// Package api provides bounded JSON decoding and stable public error responses.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/bjarneo/wahoo/server"
)

const (
	// DefaultMaxBodyBytes is a conservative default for JSON API requests.
	DefaultMaxBodyBytes int64 = 1 << 20
	// MaxBodyBytes prevents accidental unbounded JSON request configuration.
	MaxBodyBytes         int64 = 8 << 20
	maxErrorCodeBytes          = 64
	maxErrorMessageBytes       = 1024
	maxFieldErrors             = 32
	maxFieldBytes              = 128
	maxFieldMessageBytes       = 256
)

var (
	// ErrUnsupportedMediaType reports a non-JSON request body.
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	// ErrBodyTooLarge reports a request above its configured body limit.
	ErrBodyTooLarge = errors.New("request body too large")
	// ErrMalformedJSON reports invalid, unknown-field, or multi-document JSON.
	ErrMalformedJSON = errors.New("malformed JSON")
	// ErrInvalidBodyLimit reports an unsafe JSON body limit configuration.
	ErrInvalidBodyLimit = errors.New("invalid JSON body limit")
)

// FieldError identifies one invalid request field without exposing internals.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ErrorBody is the stable JSON representation of a public API error.
type ErrorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	RequestID string       `json:"request_id,omitempty"`
	Fields    []FieldError `json:"fields,omitempty"`
}

// ErrorResponse is the envelope written by WriteError.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

// Validatable is implemented by request bodies that validate domain fields.
type Validatable interface {
	Validate() []FieldError
}

// DecodeJSON strictly decodes one JSON object from r into target. It rejects
// unsupported content types, bodies above limit, unknown fields, and trailing
// JSON values.
func DecodeJSON(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	if limit == 0 {
		limit = DefaultMaxBodyBytes
	}
	if limit < 1 || limit > MaxBodyBytes {
		return ErrInvalidBodyLimit
	}
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return ErrUnsupportedMediaType
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "application/json" {
		return ErrUnsupportedMediaType
	}

	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return decodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrMalformedJSON
	}
	return nil
}

// Validate returns field errors from a request value when it implements
// Validatable. It returns nil for valid values and values without validation.
func Validate(value any) []FieldError {
	validatable, ok := value.(Validatable)
	if !ok {
		return nil
	}
	return validatable.Validate()
}

// WriteError writes the stable API error envelope. Messages must be safe for a
// public client and must not contain internal error details.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, fields ...FieldError) error {
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusInternalServerError
	}
	return server.JSON(w, status, ErrorResponse{Error: ErrorBody{
		Code:      boundedCode(code),
		Message:   boundedMessage(message),
		RequestID: server.RequestID(r.Context()),
		Fields:    boundedFields(fields),
	}})
}

// WriteDecodeError maps a bounded decode failure to a public API response.
func WriteDecodeError(w http.ResponseWriter, r *http.Request, err error) error {
	switch {
	case errors.Is(err, ErrUnsupportedMediaType):
		return WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
	case errors.Is(err, ErrBodyTooLarge):
		return WriteError(w, r, http.StatusRequestEntityTooLarge, "request_body_too_large", "request body exceeds the configured limit")
	default:
		return WriteError(w, r, http.StatusBadRequest, "invalid_request", "request body must contain one valid JSON object")
	}
}

func decodeError(err error) error {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return ErrBodyTooLarge
	}
	return ErrMalformedJSON
}

func boundedCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" {
		return "internal_error"
	}
	if len(code) > maxErrorCodeBytes {
		return code[:maxErrorCodeBytes]
	}
	return code
}

func boundedMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "request failed"
	}
	if len(message) > maxErrorMessageBytes {
		return message[:maxErrorMessageBytes]
	}
	return message
}

func boundedFields(fields []FieldError) []FieldError {
	if len(fields) == 0 {
		return nil
	}
	if len(fields) > maxFieldErrors {
		fields = fields[:maxFieldErrors]
	}
	result := make([]FieldError, 0, len(fields))
	for _, field := range fields {
		field.Field = strings.TrimSpace(field.Field)
		field.Message = strings.TrimSpace(field.Message)
		if field.Field == "" || field.Message == "" {
			continue
		}
		if len(field.Field) > maxFieldBytes {
			field.Field = field.Field[:maxFieldBytes]
		}
		if len(field.Message) > maxFieldMessageBytes {
			field.Message = field.Message[:maxFieldMessageBytes]
		}
		result = append(result, field)
	}
	return result
}
