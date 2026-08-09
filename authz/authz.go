// Package authz defines explicit tenant authorization boundaries.
package authz

import (
	"context"
	"errors"
)

const maxIdentifierBytes = 128

var (
	// ErrUnauthenticated reports a missing authenticated subject.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden reports a denied authorization request.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalidScope reports malformed subject, tenant, or action input.
	ErrInvalidScope = errors.New("invalid authorization scope")
)

// Scope is the authenticated principal and tenant resolved by an application.
// Wahoo never infers either value from a request header or URL.
type Scope struct {
	SubjectID string
	TenantID  string
}

// Action identifies one domain operation, such as "project.read".
type Action string

// Request is an explicit authorization query.
type Request struct {
	Scope  Scope
	Action Action
}

// Authorizer decides whether a scope can perform a domain action.
type Authorizer interface {
	Authorize(context.Context, Request) error
}

type scopeKey struct{}

// WithScope attaches an application-resolved scope to ctx.
func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

// ScopeFrom returns an application-resolved scope from ctx.
func ScopeFrom(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}

// Require validates and authorizes one explicit action.
func Require(ctx context.Context, authorizer Authorizer, action Action) error {
	scope, ok := ScopeFrom(ctx)
	if !ok || scope.SubjectID == "" {
		return ErrUnauthenticated
	}
	request := Request{Scope: scope, Action: action}
	if err := request.Validate(); err != nil {
		return err
	}
	if authorizer == nil {
		return ErrForbidden
	}
	if err := authorizer.Authorize(ctx, request); err != nil {
		return err
	}
	return nil
}

// Validate reports malformed authorization input before application policy is
// evaluated.
func (r Request) Validate() error {
	if !validIdentifier(r.Scope.SubjectID) || !validIdentifier(r.Scope.TenantID) || !validIdentifier(string(r.Action)) {
		return ErrInvalidScope
	}
	return nil
}

func validIdentifier(value string) bool {
	if len(value) == 0 || len(value) > maxIdentifierBytes {
		return false
	}
	for index := range value {
		char := value[index]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' && char != '.' && char != ':' {
			return false
		}
	}
	return true
}
