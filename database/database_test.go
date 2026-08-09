package database

import (
	"context"
	"errors"
	"testing"
)

type pingerFunc func(context.Context) error

func (f pingerFunc) PingContext(ctx context.Context) error {
	return f(ctx)
}

func TestReady(t *testing.T) {
	t.Parallel()
	if err := Ready(context.Background(), Dependency{Name: "database", Pinger: pingerFunc(func(context.Context) error { return nil })}); err != nil {
		t.Fatal(err)
	}
}

func TestReadyWrapsDependencyFailure(t *testing.T) {
	t.Parallel()
	want := errors.New("unavailable")
	err := Ready(context.Background(), Dependency{Name: "database", Pinger: pingerFunc(func(context.Context) error { return want })})
	if !errors.Is(err, want) {
		t.Fatalf("Ready() error = %v", err)
	}
}
