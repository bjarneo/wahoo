package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// MinRawTokenBytes requires a high-entropy opaque token representation.
	MinRawTokenBytes = 32
	// MaxRawTokenBytes bounds opaque token processing at public boundaries.
	MaxRawTokenBytes = 4096
)

var (
	// ErrSessionExpired reports an expired session.
	ErrSessionExpired = errors.New("session expired")
	// ErrSessionRevoked reports a revoked session.
	ErrSessionRevoked = errors.New("session revoked")
)

// Session is provider-neutral session state stored by an application.
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt time.Time
}

// SessionStore looks up opaque session token hashes. Applications must store
// only hashes, revoke sessions on logout, and enforce their cookie policy.
type SessionStore interface {
	FindSessionByTokenHash(context.Context, string) (*Session, error)
}

// TokenHash returns a deterministic SHA-256 hash for an opaque token. It does
// not log or retain the raw token.
func TokenHash(token string) (string, error) {
	if len(token) < MinRawTokenBytes || len(token) > MaxRawTokenBytes {
		return "", ErrTokenInvalid
	}
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:]), nil
}

// AuthenticateSession validates a raw session token and returns its active
// session. Storage failures are returned so applications do not mask outages.
func AuthenticateSession(ctx context.Context, store SessionStore, token string, now time.Time) (Session, error) {
	if store == nil || now.IsZero() {
		return Session{}, ErrTokenInvalid
	}
	hash, err := TokenHash(token)
	if err != nil {
		return Session{}, err
	}
	session, err := store.FindSessionByTokenHash(ctx, hash)
	if err != nil {
		return Session{}, fmt.Errorf("find session: %w", err)
	}
	if session == nil || subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(hash)) != 1 {
		return Session{}, ErrTokenInvalid
	}
	if err := session.ActiveAt(now); err != nil {
		return Session{}, err
	}
	return *session, nil
}

// ActiveAt reports whether a session is active at now.
func (s Session) ActiveAt(now time.Time) error {
	if now.IsZero() || s.ID == "" || s.UserID == "" || !validTokenHash(s.TokenHash) || s.ExpiresAt.IsZero() {
		return ErrTokenInvalid
	}
	if !s.RevokedAt.IsZero() && !s.RevokedAt.After(now) {
		return ErrSessionRevoked
	}
	if !s.ExpiresAt.After(now) {
		return ErrSessionExpired
	}
	return nil
}

// NewCSRFToken creates an opaque token suitable for an application-owned
// synchronizer or double-submit CSRF design.
func NewCSRFToken() (string, error) {
	return NewToken(32)
}

// VerifyCSRFToken compares opaque CSRF tokens in constant time.
func VerifyCSRFToken(expected, supplied string) bool {
	if expected == "" || supplied == "" || len(expected) > MaxRawTokenBytes || len(supplied) > MaxRawTokenBytes {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(supplied)) == 1
}

func validTokenHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}
