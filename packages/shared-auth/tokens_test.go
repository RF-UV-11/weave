package auth

import (
	"testing"
	"time"
)

var testSecret = []byte("test-secret-key-not-for-prod")

func TestIssueAndVerifyAccessToken(t *testing.T) {
	tok, expiresAt, err := IssueAccessToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("expected expiry in the future")
	}

	claims, err := VerifyAccessToken(testSecret, tok)
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.TenantID != "tnt_1" || claims.UserID != "usr_1" || claims.Role != "owner" {
		t.Fatalf("got %+v", claims)
	}
}

func TestIssueAndVerifyRefreshToken(t *testing.T) {
	tok, _, err := IssueRefreshToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueRefreshToken: %v", err)
	}
	claims, err := VerifyRefreshToken(testSecret, tok)
	if err != nil {
		t.Fatalf("VerifyRefreshToken: %v", err)
	}
	if claims.TenantID != "tnt_1" {
		t.Fatalf("got %+v", claims)
	}
}

func TestVerifyAccessTokenRejectsRefreshToken(t *testing.T) {
	tok, _, err := IssueRefreshToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueRefreshToken: %v", err)
	}
	if _, err := VerifyAccessToken(testSecret, tok); err != ErrWrongType {
		t.Fatalf("expected ErrWrongType, got %v", err)
	}
}

func TestVerifyRefreshTokenRejectsAccessToken(t *testing.T) {
	tok, _, err := IssueAccessToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := VerifyRefreshToken(testSecret, tok); err != ErrWrongType {
		t.Fatalf("expected ErrWrongType, got %v", err)
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	tok, _, err := IssueAccessToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if _, err := VerifyAccessToken([]byte("a-completely-different-secret"), tok); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyRejectsGarbageToken(t *testing.T) {
	if _, err := VerifyAccessToken(testSecret, "not-a-jwt"); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	c := newClaims("tnt_1", "usr_1", "owner", tokenTypeAccess, -1*time.Minute)
	tok, err := sign(testSecret, c)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := VerifyAccessToken(testSecret, tok); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for an expired token, got %v", err)
	}
}
