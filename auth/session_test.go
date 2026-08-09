package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type sessionStore struct {
	session *Session
	err     error
}

func (s sessionStore) FindSessionByTokenHash(_ context.Context, _ string) (*Session, error) {
	return s.session, s.err
}

func TestAuthenticateSession(t *testing.T) {
	t.Parallel()
	token := "session-token-with-at-least-thirty-two-bytes"
	hash, err := TokenHash(token)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	session, err := AuthenticateSession(context.Background(), sessionStore{session: &Session{
		ID:        "ses_123",
		UserID:    "usr_123",
		TokenHash: hash,
		ExpiresAt: now.Add(time.Hour),
	}}, token, now)
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "ses_123" {
		t.Fatalf("session ID = %q", session.ID)
	}
}

func TestAuthenticateSessionReturnsStoreFailure(t *testing.T) {
	t.Parallel()
	databaseError := errors.New("database unavailable")
	_, err := AuthenticateSession(context.Background(), sessionStore{err: databaseError}, "session-token-with-at-least-thirty-two-bytes", time.Now())
	if !errors.Is(err, databaseError) {
		t.Fatalf("AuthenticateSession() error = %v", err)
	}
}

func TestVerifyCSRFToken(t *testing.T) {
	t.Parallel()
	if !VerifyCSRFToken("expected", "expected") {
		t.Fatal("matching CSRF tokens were rejected")
	}
	if VerifyCSRFToken("expected", "different") {
		t.Fatal("different CSRF tokens were accepted")
	}
}
