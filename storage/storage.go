// Package storage defines validated, provider-neutral object storage values.
package storage

import (
	"context"
	"errors"
	"io"
	"mime"
	"reflect"
	"strings"
	"unicode/utf8"
)

const (
	// MaxKeyBytes limits a provider-neutral object key.
	MaxKeyBytes = 1024
	// MaxMetadataEntries limits user metadata fields on one object.
	MaxMetadataEntries = 32
	// MaxMetadataKeyBytes limits one user metadata key.
	MaxMetadataKeyBytes = 64
	// MaxMetadataValueBytes limits one user metadata value.
	MaxMetadataValueBytes = 1024
	// MaxMetadataBytes limits all metadata strings on one object.
	MaxMetadataBytes = 8 << 10
	// MaxContentTypeBytes limits the Content-Type metadata value.
	MaxContentTypeBytes = 256
	// MaxCacheControlBytes limits the Cache-Control metadata value.
	MaxCacheControlBytes = 512
	// MaxValueBytes limits the declared size of one object value.
	MaxValueBytes int64 = 64 << 20
)

var (
	// ErrInvalidKey reports a key that cannot be used consistently by storage providers.
	ErrInvalidKey = errors.New("invalid storage key")
	// ErrInvalidMetadata reports metadata outside the portable storage contract.
	ErrInvalidMetadata = errors.New("invalid storage metadata")
	// ErrInvalidValue reports an invalid value reader or declared size.
	ErrInvalidValue = errors.New("invalid storage value")
)

// Key is a portable object key. Obtain one with ParseKey, or validate values
// constructed by an external boundary with Validate.
type Key string

// ParseKey validates value and returns it as a Key.
func ParseKey(value string) (Key, error) {
	key := Key(value)
	if err := key.Validate(); err != nil {
		return "", err
	}
	return key, nil
}

// Validate reports whether k is a portable object key. Keys use ASCII letters,
// digits, '.', '_', '-', and '/', with no empty, '.' or '..' path segments.
func (k Key) Validate() error {
	value := string(k)
	if len(value) == 0 || len(value) > MaxKeyBytes || value[0] == '/' || value[len(value)-1] == '/' {
		return ErrInvalidKey
	}
	segmentStart := 0
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '_' && char != '-' && char != '/' {
			return ErrInvalidKey
		}
		if char != '/' {
			continue
		}
		if i == segmentStart || value[segmentStart:i] == "." || value[segmentStart:i] == ".." {
			return ErrInvalidKey
		}
		segmentStart = i + 1
	}
	if value[segmentStart:] == "." || value[segmentStart:] == ".." {
		return ErrInvalidKey
	}
	return nil
}

// Metadata is portable object metadata. Its user values are stored privately so
// callers cannot mutate a value after it crosses a storage boundary.
type Metadata struct {
	ContentType  string
	CacheControl string
	values       map[string]string
}

// NewMetadata validates and copies user values.
func NewMetadata(contentType, cacheControl string, values map[string]string) (Metadata, error) {
	metadata := Metadata{
		ContentType:  contentType,
		CacheControl: cacheControl,
	}
	if values != nil {
		metadata.values = make(map[string]string, len(values))
		for key, value := range values {
			metadata.values[key] = value
		}
	}
	if err := metadata.Validate(); err != nil {
		return Metadata{}, err
	}
	return metadata, nil
}

// Validate reports whether m fits the portable metadata contract.
func (m Metadata) Validate() error {
	if !validText(m.ContentType, MaxContentTypeBytes) || !validText(m.CacheControl, MaxCacheControlBytes) {
		return ErrInvalidMetadata
	}
	if m.ContentType != "" {
		if _, _, err := mime.ParseMediaType(m.ContentType); err != nil {
			return ErrInvalidMetadata
		}
	}
	if len(m.values) > MaxMetadataEntries {
		return ErrInvalidMetadata
	}
	total := len(m.ContentType) + len(m.CacheControl)
	for key, value := range m.values {
		if !validMetadataKey(key) || !validText(value, MaxMetadataValueBytes) {
			return ErrInvalidMetadata
		}
		total += len(key) + len(value)
		if total > MaxMetadataBytes {
			return ErrInvalidMetadata
		}
	}
	return nil
}

// Values returns a copy of the user-defined metadata values.
func (m Metadata) Values() map[string]string {
	if len(m.values) == 0 {
		return nil
	}
	values := make(map[string]string, len(m.values))
	for key, value := range m.values {
		values[key] = value
	}
	return values
}

// Value is a single-use object body with a required exact size. Providers must
// reject a stream shorter or longer than Size. Reader exposes at most Size+1
// bytes so a provider can detect an overlong stream without unbounded reads.
type Value struct {
	body     io.Reader
	size     int64
	metadata Metadata
}

// NewValue validates a body and its exact declared size.
func NewValue(body io.Reader, size int64, metadata Metadata) (Value, error) {
	if nilReader(body) || size < 0 || size > MaxValueBytes {
		return Value{}, ErrInvalidValue
	}
	if err := metadata.Validate(); err != nil {
		return Value{}, err
	}
	return Value{
		body:     body,
		size:     size,
		metadata: metadata,
	}, nil
}

// Reader returns the single-use value body capped at Size+1 bytes.
func (v Value) Reader() io.Reader {
	return io.LimitReader(v.body, v.size+1)
}

// Size returns the exact number of bytes the value body must contain.
func (v Value) Size() int64 {
	return v.size
}

// Metadata returns the value metadata.
func (v Value) Metadata() Metadata {
	return v.metadata
}

// Object is metadata returned by a Store. A Store must return a separately
// closable reader from Get so remote resources can be released promptly.
type Object struct {
	Key      Key
	Size     int64
	Metadata Metadata
}

// Validate reports whether o fits this package's object contract.
func (o Object) Validate() error {
	if err := o.Key.Validate(); err != nil {
		return err
	}
	if o.Size < 0 || o.Size > MaxValueBytes {
		return ErrInvalidValue
	}
	return o.Metadata.Validate()
}

// Store is the provider-neutral object storage boundary. It has no list method
// because unbounded listings need application-specific pagination semantics.
type Store interface {
	Put(context.Context, Key, Value) (Object, error)
	Get(context.Context, Key) (Object, io.ReadCloser, error)
	Delete(context.Context, Key) error
}

func validMetadataKey(value string) bool {
	if len(value) == 0 || len(value) > MaxMetadataKeyBytes {
		return false
	}
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

func validText(value string, limit int) bool {
	if len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func nilReader(reader io.Reader) bool {
	if reader == nil {
		return true
	}
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
