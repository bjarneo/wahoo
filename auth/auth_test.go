package auth

import "testing"

func TestPassword(t *testing.T) {
	t.Parallel()
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "correct horse battery staple") {
		t.Fatal("expected password to match")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("expected password not to match")
	}
}

func TestNewToken(t *testing.T) {
	t.Parallel()
	one, err := NewToken(32)
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if one == two || len(one) < 40 {
		t.Fatal("expected distinct opaque tokens")
	}
}

func TestDummyPasswordHash(t *testing.T) {
	t.Parallel()
	if !CheckPassword(dummyPasswordHash, "password") {
		t.Fatal("dummy password hash does not verify")
	}
}
