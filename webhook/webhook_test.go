package webhook

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerifyAcceptsCurrentAndRotatedSecrets(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	current := bytes.Repeat([]byte("c"), MinSecretBytes)
	previous := bytes.Repeat([]byte("p"), MinSecretBytes)
	previousForSigning := append([]byte(nil), previous...)
	verifier, err := NewVerifier(Config{Secrets: [][]byte{current, previous}, MaxAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	previous[0] = 'x'
	header, err := Sign(previousForSigning, now, []byte(`{"id":"evt_1"}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := verifier.VerifyAt(now.Add(30*time.Second), header, []byte(`{"id":"evt_1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(now) {
		t.Fatalf("VerifyAt() timestamp = %s, want %s", got, now)
	}
}

func TestVerifyRejectsTamperingAndStaleTimestamps(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	secret := bytes.Repeat([]byte("s"), MinSecretBytes)
	verifier, err := NewVerifier(Config{Secrets: [][]byte{secret}, MaxAge: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	header, err := Sign(secret, now, []byte("body"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.VerifyAt(now, header, []byte("changed")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered body error = %v, want %v", err, ErrInvalidSignature)
	}
	if _, err := verifier.VerifyAt(now.Add(2*time.Minute), header, []byte("body")); !errors.Is(err, ErrStaleTimestamp) {
		t.Fatalf("stale signature error = %v, want %v", err, ErrStaleTimestamp)
	}
	if _, err := verifier.VerifyAt(now, "t=not-a-time,v1=abcd", []byte("body")); !errors.Is(err, ErrInvalidTimestamp) {
		t.Fatalf("invalid timestamp error = %v, want %v", err, ErrInvalidTimestamp)
	}
}

func TestBodyAndConfigurationBounds(t *testing.T) {
	t.Parallel()
	secret := bytes.Repeat([]byte("s"), MinSecretBytes)
	if _, err := Sign(secret[:MinSecretBytes-1], time.Now(), nil); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("short secret error = %v, want %v", err, ErrInvalidSecret)
	}
	if _, err := NewVerifier(Config{Secrets: [][]byte{secret}, MaxBodyBytes: MaxBodyBytes + 1}); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("oversized configured body error = %v, want %v", err, ErrBodyTooLarge)
	}
	verifier, err := NewVerifier(Config{Secrets: [][]byte{secret}, MaxBodyBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.ReadBody(strings.NewReader("12345")); !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("ReadBody() error = %v, want %v", err, ErrBodyTooLarge)
	}
	body, err := verifier.ReadBody(strings.NewReader("1234"))
	if err != nil || string(body) != "1234" {
		t.Fatalf("ReadBody() = %q, %v", body, err)
	}
}
