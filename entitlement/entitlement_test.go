package entitlement

import (
	"errors"
	"testing"
	"time"
)

func TestDecide(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	requirement := Requirement{Feature: "reports.export", Quantity: 2}
	tests := []struct {
		name    string
		grants  []Grant
		allowed bool
		reason  Reason
		granted uint64
	}{
		{
			name:   "no matching grant",
			grants: []Grant{{Feature: "other", Quantity: 2}},
			reason: ReasonNoGrant,
		},
		{
			name:   "not started",
			grants: []Grant{{Feature: requirement.Feature, Quantity: 2, StartsAt: now.Add(time.Minute)}},
			reason: ReasonNotStarted,
		},
		{
			name:   "expired",
			grants: []Grant{{Feature: requirement.Feature, Quantity: 2, EndsAt: now}},
			reason: ReasonExpired,
		},
		{
			name:   "revoked",
			grants: []Grant{{Feature: requirement.Feature, Quantity: 2, RevokedAt: now}},
			reason: ReasonRevoked,
		},
		{
			name:    "insufficient active quantity",
			grants:  []Grant{{Feature: requirement.Feature, Quantity: 1}},
			reason:  ReasonInsufficientQuantity,
			granted: 1,
		},
		{
			name: "aggregates active grants",
			grants: []Grant{
				{Feature: requirement.Feature, Quantity: 1},
				{Feature: requirement.Feature, Quantity: 1, RevokedAt: now.Add(time.Minute)},
			},
			allowed: true,
			reason:  ReasonGranted,
			granted: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := Decide(now, requirement, test.grants)
			if err != nil {
				t.Fatal(err)
			}
			if got.Allowed != test.allowed || got.Reason != test.reason || got.Granted != test.granted {
				t.Fatalf("Decide() = %+v, want allowed=%t reason=%d granted=%d", got, test.allowed, test.reason, test.granted)
			}
		})
	}
}

func TestDecideRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	now := time.Now()
	validRequirement := Requirement{Feature: "feature", Quantity: 1}
	if _, err := Decide(time.Time{}, validRequirement, nil); !errors.Is(err, ErrInvalidTime) {
		t.Fatalf("zero time error = %v, want %v", err, ErrInvalidTime)
	}
	if _, err := Decide(now, Requirement{Feature: "feature"}, nil); !errors.Is(err, ErrInvalidRequirement) {
		t.Fatalf("zero quantity error = %v, want %v", err, ErrInvalidRequirement)
	}
	invalidGrant := Grant{Feature: "feature", Quantity: 1, StartsAt: now, EndsAt: now}
	if _, err := Decide(now, validRequirement, []Grant{invalidGrant}); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("invalid grant error = %v, want %v", err, ErrInvalidGrant)
	}
	grants := make([]Grant, MaxGrants+1)
	for i := range grants {
		grants[i] = Grant{Feature: "feature", Quantity: 1}
	}
	if _, err := Decide(now, validRequirement, grants); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("too many grants error = %v, want %v", err, ErrInvalidGrant)
	}
}
