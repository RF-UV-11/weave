// Package auth is Weave's shared JWT + RBAC building block: issuing and
// verifying access/refresh tokens, and a gRPC interceptor that resolves a
// verified token into {tenant_id, user_id, role} on the request context.
// RBAC roles are always tenant-scoped (docs/architecture/SECURITY.md §5) —
// a role check is "role X within tenant Y," never a bare global role, which
// is why every Claims carries a TenantID alongside the Role.
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type tokenType string

const (
	tokenTypeAccess  tokenType = "access"
	tokenTypeRefresh tokenType = "refresh"
)

type Claims struct {
	TenantID string    `json:"tenant_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	Type     tokenType `json:"typ"`
	jwt.RegisteredClaims
}

func newClaims(tenantID, userID, role string, typ tokenType, ttl time.Duration) *Claims {
	now := time.Now()
	return &Claims{
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		Type:     typ,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
}
