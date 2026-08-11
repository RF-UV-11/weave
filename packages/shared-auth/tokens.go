package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// AccessTokenTTL and RefreshTokenTTL match the "short-lived access +
	// rotated refresh" shape in docs/architecture/SECURITY.md §5. Refresh
	// token rotation/revocation storage is a known gap (Phase 2 ships
	// stateless JWT refresh tokens only) — see PLAN.md's Phase 2 notes.
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

var (
	ErrInvalidToken = errors.New("auth: invalid token")
	ErrWrongType    = errors.New("auth: wrong token type")
)

func sign(secret []byte, c *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// IssueAccessToken issues a short-lived token carrying {tenant_id, user_id, role}.
func IssueAccessToken(secret []byte, tenantID, userID, role string) (string, time.Time, error) {
	c := newClaims(tenantID, userID, role, tokenTypeAccess, AccessTokenTTL)
	signed, err := sign(secret, c)
	return signed, c.ExpiresAt.Time, err
}

// IssueRefreshToken issues a longer-lived token used only to mint a new
// access token via Refresh.
func IssueRefreshToken(secret []byte, tenantID, userID, role string) (string, time.Time, error) {
	c := newClaims(tenantID, userID, role, tokenTypeRefresh, RefreshTokenTTL)
	signed, err := sign(secret, c)
	return signed, c.ExpiresAt.Time, err
}

func parse(secret []byte, tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// VerifyAccessToken parses and verifies an access token, rejecting a
// refresh token presented in its place.
func VerifyAccessToken(secret []byte, tokenStr string) (*Claims, error) {
	claims, err := parse(secret, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Type != tokenTypeAccess {
		return nil, ErrWrongType
	}
	return claims, nil
}

// VerifyRefreshToken parses and verifies a refresh token, rejecting an
// access token presented in its place.
func VerifyRefreshToken(secret []byte, tokenStr string) (*Claims, error) {
	claims, err := parse(secret, tokenStr)
	if err != nil {
		return nil, err
	}
	if claims.Type != tokenTypeRefresh {
		return nil, ErrWrongType
	}
	return claims, nil
}
