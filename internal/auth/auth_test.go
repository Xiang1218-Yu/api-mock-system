package auth

import (
	"testing"
	"time"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPassword(hash, "hunter2"); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Error("wrong password accepted")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	a, err := New("secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := a.Issue("user-123", "alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := a.Parse(tok)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("userID = %q", claims.UserID)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("email = %q", claims.Email)
	}
}

func TestJWTRejectsBadToken(t *testing.T) {
	a, _ := New("secret", time.Hour)
	if _, err := a.Parse("not-a-token"); err == nil {
		t.Error("expected error for malformed token")
	}
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	a1, _ := New("secret1", time.Hour)
	a2, _ := New("secret2", time.Hour)
	tok, _ := a1.Issue("u", "e@x.com")
	if _, err := a2.Parse(tok); err == nil {
		t.Error("expected signature mismatch error")
	}
}

func TestNewRejectsEmptySecret(t *testing.T) {
	if _, err := New("", time.Hour); err == nil {
		t.Error("expected error for empty secret")
	}
}
