// Package entitlement makes provider-neutral feature access decisions.
package entitlement

import (
	"errors"
	"time"
)

const (
	// MaxGrants bounds entitlement aggregation work for one decision.
	MaxGrants = 128
	// MaxFeatureBytes bounds provider-neutral feature identifiers.
	MaxFeatureBytes = 128
)

var (
	// ErrInvalidRequirement reports an invalid requested feature or quantity.
	ErrInvalidRequirement = errors.New("invalid entitlement requirement")
	// ErrInvalidGrant reports malformed provider-normalized entitlement data.
	ErrInvalidGrant = errors.New("invalid entitlement grant")
	// ErrInvalidTime reports a missing decision timestamp.
	ErrInvalidTime = errors.New("invalid entitlement decision time")
)

// Requirement describes the feature and quantity an operation needs.
type Requirement struct {
	Feature  string
	Quantity uint64
}

// Validate reports whether r can be evaluated.
func (r Requirement) Validate() error {
	if !validFeature(r.Feature) || r.Quantity == 0 {
		return ErrInvalidRequirement
	}
	return nil
}

// Grant is provider-normalized evidence that a feature is available. A zero
// StartsAt means immediately active. A zero EndsAt means no scheduled end. A
// RevokedAt at or before the decision time disables the grant.
type Grant struct {
	Feature   string
	Quantity  uint64
	StartsAt  time.Time
	EndsAt    time.Time
	RevokedAt time.Time
}

// Validate reports whether g is internally consistent provider-neutral data.
func (g Grant) Validate() error {
	if !validFeature(g.Feature) || g.Quantity == 0 {
		return ErrInvalidGrant
	}
	if !g.StartsAt.IsZero() && !g.EndsAt.IsZero() && !g.EndsAt.After(g.StartsAt) {
		return ErrInvalidGrant
	}
	return nil
}

// Reason explains an entitlement decision without exposing provider-specific
// subscription or billing status.
type Reason uint8

const (
	// ReasonUnknown is the zero value for an uninitialized Decision.
	ReasonUnknown Reason = iota
	// ReasonGranted reports enough active quantity.
	ReasonGranted
	// ReasonNoGrant reports no grant for the required feature.
	ReasonNoGrant
	// ReasonNotStarted reports matching grants that have not started.
	ReasonNotStarted
	// ReasonExpired reports matching grants that have ended.
	ReasonExpired
	// ReasonRevoked reports matching grants that are revoked.
	ReasonRevoked
	// ReasonInsufficientQuantity reports active grants below the requested quantity.
	ReasonInsufficientQuantity
)

// Decision is a fail-closed access result. Granted is capped at the requested
// quantity to avoid exposing misleading totals or overflowing aggregation.
type Decision struct {
	Allowed bool
	Reason  Reason
	Granted uint64
}

// Decide evaluates bounded grants for one requirement at now. It does not know
// billing states or providers; callers normalize provider data into Grant.
func Decide(now time.Time, requirement Requirement, grants []Grant) (Decision, error) {
	if now.IsZero() {
		return Decision{}, ErrInvalidTime
	}
	if err := requirement.Validate(); err != nil {
		return Decision{}, err
	}
	if len(grants) > MaxGrants {
		return Decision{}, ErrInvalidGrant
	}

	var (
		granted    uint64
		matched    bool
		notStarted bool
		expired    bool
		revoked    bool
		active     bool
	)
	for _, grant := range grants {
		if err := grant.Validate(); err != nil {
			return Decision{}, err
		}
		if grant.Feature != requirement.Feature {
			continue
		}
		matched = true
		if !grant.RevokedAt.IsZero() && !grant.RevokedAt.After(now) {
			revoked = true
			continue
		}
		if !grant.StartsAt.IsZero() && grant.StartsAt.After(now) {
			notStarted = true
			continue
		}
		if !grant.EndsAt.IsZero() && !grant.EndsAt.After(now) {
			expired = true
			continue
		}
		active = true
		remaining := requirement.Quantity - granted
		if grant.Quantity >= remaining {
			granted = requirement.Quantity
		} else {
			granted += grant.Quantity
		}
	}
	if granted == requirement.Quantity {
		return Decision{Allowed: true, Reason: ReasonGranted, Granted: granted}, nil
	}
	if active {
		return Decision{Reason: ReasonInsufficientQuantity, Granted: granted}, nil
	}
	if !matched {
		return Decision{Reason: ReasonNoGrant}, nil
	}
	if revoked {
		return Decision{Reason: ReasonRevoked}, nil
	}
	if notStarted {
		return Decision{Reason: ReasonNotStarted}, nil
	}
	if expired {
		return Decision{Reason: ReasonExpired}, nil
	}
	return Decision{Reason: ReasonNoGrant}, nil
}

func validFeature(value string) bool {
	if len(value) == 0 || len(value) > MaxFeatureBytes {
		return false
	}
	for i := range value {
		char := value[i]
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return true
}
