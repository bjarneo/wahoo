package storage

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestParseKey(t *testing.T) {
	t.Parallel()
	valid := []string{
		"assets/logo-1.png",
		"tenants/acme_1/documents/report.v2",
		"a",
	}
	for _, value := range valid {
		key, err := ParseKey(value)
		if err != nil || string(key) != value {
			t.Errorf("ParseKey(%q) = %q, %v", value, key, err)
		}
	}

	invalid := []string{
		"",
		"/root",
		"root/",
		"root//child",
		".",
		"../secret",
		"root/../secret",
		"contains space",
		strings.Repeat("a", MaxKeyBytes+1),
	}
	for _, value := range invalid {
		if _, err := ParseKey(value); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseKey(%q) error = %v, want %v", value, err, ErrInvalidKey)
		}
	}
}

func TestMetadataCopiesValuesAndValidatesBounds(t *testing.T) {
	t.Parallel()
	input := map[string]string{"source": "upload", "retention-days": "30"}
	metadata, err := NewMetadata("text/plain; charset=utf-8", "private, max-age=60", input)
	if err != nil {
		t.Fatal(err)
	}
	input["source"] = "changed"
	if got := metadata.Values()["source"]; got != "upload" {
		t.Fatalf("stored metadata = %q, want original value", got)
	}
	values := metadata.Values()
	values["source"] = "changed again"
	if got := metadata.Values()["source"]; got != "upload" {
		t.Fatalf("returned metadata mutated stored value to %q", got)
	}

	for _, values := range []map[string]string{
		{"Uppercase": "value"},
		{"valid": "line\nbreak"},
	} {
		if _, err := NewMetadata("", "", values); !errors.Is(err, ErrInvalidMetadata) {
			t.Fatalf("NewMetadata(%v) error = %v, want %v", values, err, ErrInvalidMetadata)
		}
	}
	if _, err := NewMetadata("not a media type;", "", nil); !errors.Is(err, ErrInvalidMetadata) {
		t.Fatalf("invalid content type error = %v, want %v", err, ErrInvalidMetadata)
	}
}

func TestValueBoundsReaderAndObject(t *testing.T) {
	t.Parallel()
	metadata, err := NewMetadata("application/octet-stream", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewValue(bytes.NewBufferString("four"), 3, metadata)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(value.Reader())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "four" {
		t.Fatalf("Reader() = %q, want declared size plus one byte", got)
	}
	if value.Size() != 3 || value.Metadata().ContentType != "application/octet-stream" {
		t.Fatal("value accessors returned unexpected data")
	}

	var typedNil *bytes.Buffer
	if _, err := NewValue(typedNil, 0, metadata); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("NewValue(typed nil) error = %v, want %v", err, ErrInvalidValue)
	}
	if _, err := NewValue(bytes.NewReader(nil), MaxValueBytes+1, metadata); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("NewValue(oversize) error = %v, want %v", err, ErrInvalidValue)
	}

	key, err := ParseKey("objects/report")
	if err != nil {
		t.Fatal(err)
	}
	if err := (Object{Key: key, Size: 3, Metadata: metadata}).Validate(); err != nil {
		t.Fatalf("Object.Validate() error = %v", err)
	}
	if err := (Object{Key: key, Size: -1, Metadata: metadata}).Validate(); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("invalid object size error = %v, want %v", err, ErrInvalidValue)
	}
}
