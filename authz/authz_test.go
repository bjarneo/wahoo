package authz

import (
	"context"
	"testing"
)

type allowAuthorizer struct{}

func (allowAuthorizer) Authorize(_ context.Context, _ Request) error {
	return nil
}

func TestRequire(t *testing.T) {
	t.Parallel()
	ctx := WithScope(context.Background(), Scope{SubjectID: "usr_123", TenantID: "ten_123"})
	if err := Require(ctx, allowAuthorizer{}, "project.read"); err != nil {
		t.Fatal(err)
	}
}

func TestRequireRejectsMissingScope(t *testing.T) {
	t.Parallel()
	if err := Require(context.Background(), allowAuthorizer{}, "project.read"); err != ErrUnauthenticated {
		t.Fatalf("Require() error = %v", err)
	}
}
