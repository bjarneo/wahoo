// Package auth contains provider-neutral authentication primitives.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("token invalid")
)

// User is the minimum identity needed by the built-in auth flows.
type User struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	PasswordHash  string    `json:"-"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// Store is implemented by an application package backed by SQL, a document
// store, or a test double. Keeping it here makes auth independent of storage.
type Store interface {
	FindUserByEmail(context.Context, string) (*User, error)
	FindUserByID(context.Context, string) (*User, error)
	CreateUser(context.Context, User) error
	UpdatePassword(context.Context, string, string) error
}

// HashPassword hashes a password with bcrypt's current recommended cost.
func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password to a stored hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// VerifyLogin returns a user only when the email and password are valid.
func VerifyLogin(ctx context.Context, store Store, email, password string) (*User, error) {
	user, err := store.FindUserByEmail(ctx, email)
	if err != nil || user == nil || !CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// NewToken generates a URL-safe opaque token. Store a hash of the returned
// value and only show the raw value to the user once.
func NewToken(bytes int) (string, error) {
	if bytes < 32 {
		bytes = 32
	}
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
