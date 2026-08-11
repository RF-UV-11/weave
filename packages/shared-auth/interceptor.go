package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type claimsCtxKey struct{}

// UnaryServerInterceptor verifies the "authorization: Bearer <token>" gRPC
// metadata on every call except those in skipMethods (matched against
// info.FullMethod, e.g. "/core.data_access.v1.AuthService/Login" — auth
// itself can't require a token to log in). On success the verified Claims
// are attached to the context for handlers to read via FromContext.
func UnaryServerInterceptor(secret []byte, skipMethods ...string) grpc.UnaryServerInterceptor {
	skip := make(map[string]bool, len(skipMethods))
	for _, m := range skipMethods {
		skip[m] = true
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if skip[info.FullMethod] {
			return handler(ctx, req)
		}

		token, err := bearerToken(ctx)
		if err != nil {
			return nil, err
		}
		claims, err := VerifyAccessToken(secret, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired access token")
		}

		return handler(context.WithValue(ctx, claimsCtxKey{}, claims), req)
	}
}

func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "missing authorization metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "missing authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(values[0], prefix) {
		return "", status.Error(codes.Unauthenticated, "authorization header must be \"Bearer <token>\"")
	}
	return strings.TrimPrefix(values[0], prefix), nil
}

// FromContext returns the Claims attached by UnaryServerInterceptor, if any.
func FromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsCtxKey{}).(*Claims)
	return claims, ok
}

// RequireRole checks the context's verified Claims has one of the allowed
// roles. Never trust a client-supplied role — this only ever reads the role
// resolved server-side from a verified token.
func RequireRole(ctx context.Context, allowed ...string) error {
	claims, ok := FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no verified claims on context")
	}
	for _, role := range allowed {
		if claims.Role == role {
			return nil
		}
	}
	return status.Errorf(codes.PermissionDenied, "role %q is not permitted to perform this action", claims.Role)
}

// RequireTenant checks the context's verified Claims belongs to tenantID —
// the authoritative tenant scope for the request is always the one bound
// to the token, never a value the client passes on the request body
// (docs/architecture/SECURITY.md §2).
func RequireTenant(ctx context.Context, tenantID string) error {
	claims, ok := FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no verified claims on context")
	}
	if claims.TenantID != tenantID {
		return status.Error(codes.PermissionDenied, "token is not scoped to this tenant")
	}
	return nil
}
