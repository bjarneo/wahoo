// Package database defines application-owned migration and readiness contracts.
package database

import (
	"context"
	"errors"
	"fmt"
)

// Pinger checks whether one required dependency is ready to serve requests.
type Pinger interface {
	PingContext(context.Context) error
}

// Dependency identifies one readiness dependency. Dependencies are checked in
// the caller-provided order so failures are deterministic.
type Dependency struct {
	Name   string
	Pinger Pinger
}

// Migrator applies application-owned schema migrations. Wahoo does not choose
// a database driver, migration file format, locking strategy, or rollout time.
type Migrator interface {
	Migrate(context.Context) error
}

// Ready checks dependencies in order and returns the first failure with its
// application-supplied name. Callers must use a context with a short deadline.
func Ready(ctx context.Context, dependencies ...Dependency) error {
	if ctx == nil {
		return errors.New("nil readiness context")
	}
	for _, dependency := range dependencies {
		if dependency.Name == "" || dependency.Pinger == nil {
			return errors.New("invalid readiness dependency")
		}
		if err := dependency.Pinger.PingContext(ctx); err != nil {
			return fmt.Errorf("dependency %s: %w", dependency.Name, err)
		}
	}
	return nil
}
