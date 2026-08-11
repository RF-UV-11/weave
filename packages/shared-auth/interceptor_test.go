package auth

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func withBearer(ctx context.Context, token string) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}

func noopHandler(ctx context.Context, req any) (any, error) {
	claims, ok := FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "no claims on context")
	}
	return claims, nil
}

func TestInterceptorAcceptsValidToken(t *testing.T) {
	tok, _, err := IssueAccessToken(testSecret, "tnt_1", "usr_1", "owner")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	interceptor := UnaryServerInterceptor(testSecret)
	resp, err := interceptor(withBearer(context.Background(), tok), nil,
		&grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}, noopHandler)
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	claims := resp.(*Claims)
	if claims.TenantID != "tnt_1" {
		t.Fatalf("got %+v", claims)
	}
}

func TestInterceptorRejectsMissingToken(t *testing.T) {
	interceptor := UnaryServerInterceptor(testSecret)
	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}, noopHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestInterceptorRejectsInvalidToken(t *testing.T) {
	interceptor := UnaryServerInterceptor(testSecret)
	_, err := interceptor(withBearer(context.Background(), "garbage"), nil,
		&grpc.UnaryServerInfo{FullMethod: "/some.Service/Method"}, noopHandler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestInterceptorSkipsListedMethods(t *testing.T) {
	interceptor := UnaryServerInterceptor(testSecret, "/some.Service/Login")
	handlerCalled := false
	_, err := interceptor(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/some.Service/Login"},
		func(ctx context.Context, req any) (any, error) {
			handlerCalled = true
			return nil, nil
		})
	if err != nil {
		t.Fatalf("expected skipped method to bypass auth, got %v", err)
	}
	if !handlerCalled {
		t.Fatal("expected handler to be invoked for a skipped method")
	}
}

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), claimsCtxKey{}, &Claims{TenantID: "tnt_1", Role: "owner"})
	if err := RequireRole(ctx, "admin", "owner"); err != nil {
		t.Fatalf("RequireRole: %v", err)
	}
}

func TestRequireRoleRejectsNonMatchingRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), claimsCtxKey{}, &Claims{TenantID: "tnt_1", Role: "customer"})
	err := RequireRole(ctx, "admin", "owner")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestRequireRoleRejectsMissingClaims(t *testing.T) {
	err := RequireRole(context.Background(), "owner")
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestRequireTenantAllowsMatchingTenant(t *testing.T) {
	ctx := context.WithValue(context.Background(), claimsCtxKey{}, &Claims{TenantID: "tnt_1"})
	if err := RequireTenant(ctx, "tnt_1"); err != nil {
		t.Fatalf("RequireTenant: %v", err)
	}
}

func TestRequireTenantRejectsMismatchedTenant(t *testing.T) {
	ctx := context.WithValue(context.Background(), claimsCtxKey{}, &Claims{TenantID: "tnt_1"})
	err := RequireTenant(ctx, "tnt_2")
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}
