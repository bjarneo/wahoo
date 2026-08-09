// Package billing defines provider-neutral checkout, subscription, and usage contracts.
package billing

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	maxIdentifierBytes = 128
	maxURLBytes        = 2048
	maxQuantity        = 1_000_000_000
)

var (
	// ErrInvalidRequest reports malformed provider-neutral billing input.
	ErrInvalidRequest = errors.New("invalid billing request")
	// ErrInvalidSubscription reports malformed provider-normalized state.
	ErrInvalidSubscription = errors.New("invalid subscription")
	// ErrInvalidUsage reports malformed, non-idempotent, or unbounded usage.
	ErrInvalidUsage = errors.New("invalid usage record")
)

// CheckoutRequest requests a provider checkout for one application customer.
// The application authorizes the customer and selects a server-owned price ID.
type CheckoutRequest struct {
	CustomerID string
	PriceID    string
	SuccessURL string
	CancelURL  string
}

// Validate reports whether r is safe to hand to a billing provider.
func (r CheckoutRequest) Validate() error {
	if !validIdentifier(r.CustomerID) || !validIdentifier(r.PriceID) || !validReturnURL(r.SuccessURL) || !validReturnURL(r.CancelURL) {
		return ErrInvalidRequest
	}
	return nil
}

// Checkout is the provider response needed to redirect the browser.
type Checkout struct {
	ID  string
	URL string
}

// Validate reports whether c has a bounded provider checkout URL.
func (c Checkout) Validate() error {
	if !validIdentifier(c.ID) || !validReturnURL(c.URL) {
		return ErrInvalidRequest
	}
	return nil
}

// SubscriptionStatus is a normalized provider state. Applications map detailed
// provider states to it before they make entitlement decisions.
type SubscriptionStatus string

const (
	SubscriptionTrialing SubscriptionStatus = "trialing"
	SubscriptionActive   SubscriptionStatus = "active"
	SubscriptionPastDue  SubscriptionStatus = "past_due"
	SubscriptionCanceled SubscriptionStatus = "canceled"
)

// Subscription is normalized provider state. It does not include payment
// methods, invoices, tax, or provider-specific metadata.
type Subscription struct {
	ID         string
	CustomerID string
	PriceID    string
	Status     SubscriptionStatus
	CurrentEnd time.Time
}

// Validate reports whether s is usable for application entitlement mapping.
func (s Subscription) Validate() error {
	if !validIdentifier(s.ID) || !validIdentifier(s.CustomerID) || !validIdentifier(s.PriceID) || !validStatus(s.Status) || s.CurrentEnd.IsZero() {
		return ErrInvalidSubscription
	}
	return nil
}

// Usage is an idempotent quantity recorded for metered billing. The application
// must allocate ID once per source event and retain it before a provider call.
type Usage struct {
	ID         string
	CustomerID string
	Metric     string
	Quantity   int64
	OccurredAt time.Time
}

// Validate reports whether u is safe to submit to a usage adapter.
func (u Usage) Validate() error {
	if !validIdentifier(u.ID) || !validIdentifier(u.CustomerID) || !validIdentifier(u.Metric) || u.Quantity < 1 || u.Quantity > maxQuantity || u.OccurredAt.IsZero() {
		return ErrInvalidUsage
	}
	return nil
}

// Provider creates checkout sessions and returns normalized subscription state.
// Applications own provider credentials, webhook reconciliation, and retries.
type Provider interface {
	CreateCheckout(context.Context, CheckoutRequest) (Checkout, error)
	Subscription(context.Context, string) (Subscription, error)
	CancelSubscription(context.Context, string) (Subscription, error)
}

// UsageRecorder records one idempotent metered usage event.
type UsageRecorder interface {
	RecordUsage(context.Context, Usage) error
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

func validReturnURL(value string) bool {
	if len(value) == 0 || len(value) > maxURLBytes {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00")
}

func validStatus(status SubscriptionStatus) bool {
	switch status {
	case SubscriptionTrialing, SubscriptionActive, SubscriptionPastDue, SubscriptionCanceled:
		return true
	default:
		return false
	}
}
