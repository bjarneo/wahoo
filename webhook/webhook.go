// Package webhook signs and verifies bounded HMAC-SHA256 webhook payloads.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	// MinSecretBytes requires at least 256 bits of secret material.
	MinSecretBytes = 32
	// MaxSecretBytes bounds HMAC setup work from misconfigured secrets.
	MaxSecretBytes = 1024
	// MaxSecrets bounds configured current and rotated secrets.
	MaxSecrets = 8
	// DefaultMaxAge is used when Config.MaxAge is zero.
	DefaultMaxAge = 5 * time.Minute
	// MaxAge caps configured signature freshness windows.
	MaxAge = 24 * time.Hour
	// DefaultMaxBodyBytes is used when Config.MaxBodyBytes is zero.
	DefaultMaxBodyBytes = 1 << 20
	// MaxBodyBytes caps webhook payloads accepted by this package.
	MaxBodyBytes = 8 << 20
	// MaxSignatureHeaderBytes caps signature header parsing.
	MaxSignatureHeaderBytes = 1024
	// MaxSignatures caps v1 signatures accepted in one header.
	MaxSignatures = 8
)

var (
	// ErrInvalidSecret reports an empty, weak, or oversized secret.
	ErrInvalidSecret = errors.New("invalid webhook secret")
	// ErrInvalidTimestamp reports an invalid signature timestamp.
	ErrInvalidTimestamp = errors.New("invalid webhook timestamp")
	// ErrInvalidSignature reports malformed or unmatched signature input.
	ErrInvalidSignature = errors.New("invalid webhook signature")
	// ErrStaleTimestamp reports a signature outside the configured freshness window.
	ErrStaleTimestamp = errors.New("stale webhook timestamp")
	// ErrBodyTooLarge reports a body above the configured or absolute body limit.
	ErrBodyTooLarge = errors.New("webhook body too large")
)

// Config controls verification bounds. Secrets may include a current secret and
// a bounded set of previous secrets during rotation.
type Config struct {
	Secrets      [][]byte
	MaxAge       time.Duration
	MaxBodyBytes int
}

// Verifier validates webhook signature headers against one or more secrets.
type Verifier struct {
	secrets      [][]byte
	maxAge       time.Duration
	maxBodyBytes int
}

// NewVerifier validates configuration and copies its secrets.
func NewVerifier(config Config) (*Verifier, error) {
	if len(config.Secrets) == 0 || len(config.Secrets) > MaxSecrets {
		return nil, ErrInvalidSecret
	}
	secrets := make([][]byte, len(config.Secrets))
	for i, secret := range config.Secrets {
		if !validSecret(secret) {
			return nil, ErrInvalidSecret
		}
		secrets[i] = append([]byte(nil), secret...)
	}
	if config.MaxAge == 0 {
		config.MaxAge = DefaultMaxAge
	}
	if config.MaxAge < 0 || config.MaxAge > MaxAge {
		return nil, ErrInvalidTimestamp
	}
	if config.MaxBodyBytes == 0 {
		config.MaxBodyBytes = DefaultMaxBodyBytes
	}
	if config.MaxBodyBytes < 1 || config.MaxBodyBytes > MaxBodyBytes {
		return nil, ErrBodyTooLarge
	}
	return &Verifier{
		secrets:      secrets,
		maxAge:       config.MaxAge,
		maxBodyBytes: config.MaxBodyBytes,
	}, nil
}

// Sign returns a Signature header in the form t=<unix-seconds>,v1=<hex-hmac>.
// The HMAC input is the exact ASCII timestamp, a period, and the raw body.
func Sign(secret []byte, timestamp time.Time, body []byte) (string, error) {
	if !validSecret(secret) {
		return "", ErrInvalidSecret
	}
	if len(body) > MaxBodyBytes {
		return "", ErrBodyTooLarge
	}
	unix, err := unixTimestamp(timestamp)
	if err != nil {
		return "", err
	}
	return "t=" + strconv.FormatInt(unix, 10) + ",v1=" + hex.EncodeToString(sum(secret, unix, body)), nil
}

// Verify validates header and body using the current time.
func (v *Verifier) Verify(header string, body []byte) (time.Time, error) {
	return v.VerifyAt(time.Now(), header, body)
}

// VerifyAt validates header and body at now. It is useful where the caller owns
// the clock, including deterministic tests and message replay processing.
func (v *Verifier) VerifyAt(now time.Time, header string, body []byte) (time.Time, error) {
	if v == nil {
		return time.Time{}, ErrInvalidSignature
	}
	if len(body) > v.maxBodyBytes {
		return time.Time{}, ErrBodyTooLarge
	}
	timestamp, signatures, err := parseHeader(header)
	if err != nil {
		return time.Time{}, err
	}
	if now.IsZero() {
		return time.Time{}, ErrInvalidTimestamp
	}
	if age := now.Sub(timestamp); age > v.maxAge || age < -v.maxAge {
		return time.Time{}, ErrStaleTimestamp
	}

	valid := false
	for _, secret := range v.secrets {
		expected := sum(secret, timestamp.Unix(), body)
		for _, signature := range signatures {
			if hmac.Equal(expected, signature) {
				valid = true
			}
		}
	}
	if !valid {
		return time.Time{}, ErrInvalidSignature
	}
	return timestamp, nil
}

// ReadBody reads at most the verifier's body limit plus one byte. It lets HTTP
// handlers retain the exact raw body without an unbounded io.ReadAll call.
func (v *Verifier) ReadBody(body io.Reader) ([]byte, error) {
	if v == nil || body == nil {
		return nil, ErrBodyTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(body, int64(v.maxBodyBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > v.maxBodyBytes {
		return nil, ErrBodyTooLarge
	}
	return data, nil
}

func parseHeader(header string) (time.Time, [][]byte, error) {
	if len(header) == 0 || len(header) > MaxSignatureHeaderBytes {
		return time.Time{}, nil, ErrInvalidSignature
	}
	var (
		unix       int64
		timeFound  bool
		signatures [][]byte
	)
	for _, part := range strings.Split(header, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || value == "" {
			return time.Time{}, nil, ErrInvalidSignature
		}
		switch key {
		case "t":
			if timeFound {
				return time.Time{}, nil, ErrInvalidSignature
			}
			parsed, err := parseUnix(value)
			if err != nil {
				return time.Time{}, nil, err
			}
			unix = parsed
			timeFound = true
		case "v1":
			if len(signatures) == MaxSignatures || len(value) != sha256.Size*2 {
				return time.Time{}, nil, ErrInvalidSignature
			}
			signature, err := hex.DecodeString(value)
			if err != nil || len(signature) != sha256.Size {
				return time.Time{}, nil, ErrInvalidSignature
			}
			signatures = append(signatures, signature)
		default:
			return time.Time{}, nil, ErrInvalidSignature
		}
	}
	if !timeFound || len(signatures) == 0 {
		return time.Time{}, nil, ErrInvalidSignature
	}
	return time.Unix(unix, 0).UTC(), signatures, nil
}

func parseUnix(value string) (int64, error) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, ErrInvalidTimestamp
	}
	for i := range value {
		if value[i] < '0' || value[i] > '9' {
			return 0, ErrInvalidTimestamp
		}
	}
	unix, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, ErrInvalidTimestamp
	}
	return unix, nil
}

func unixTimestamp(timestamp time.Time) (int64, error) {
	if timestamp.IsZero() || timestamp.Unix() < 0 {
		return 0, ErrInvalidTimestamp
	}
	return timestamp.Unix(), nil
}

func sum(secret []byte, unix int64, body []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write(strconv.AppendInt(nil, unix, 10))
	_, _ = mac.Write([]byte{'.'})
	_, _ = mac.Write(body)
	return mac.Sum(nil)
}

func validSecret(secret []byte) bool {
	return len(secret) >= MinSecretBytes && len(secret) <= MaxSecretBytes
}
