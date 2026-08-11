package auth

import (
	"context"
	"fmt"
	"os"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	databasev1 "weave/core/gen/database/v1"
	"weave/core/mongodb"
	sharedauth "weave/packages/shared-auth"
)

var testSecret = []byte("test-jwt-secret-not-for-prod")

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if err := mongodb.InitDatabase(uri, "weave_core_test_auth"); err != nil {
		fmt.Printf("auth: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("auth: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func newTenant(t *testing.T) string {
	t.Helper()
	tn, err := mongodb.CreateTenant(t.Context(), "Auth RPC Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	return tn.GetXId()
}

func TestRegisterRequiresTenantID(t *testing.T) {
	s := NewServer(testSecret)
	_, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		Email: "a@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	s := NewServer(testSecret)
	_, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: newTenant(t), Email: "a@b.com", Password: "short", Role: databasev1.Role_ROLE_OWNER,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterRejectsUnspecifiedRole(t *testing.T) {
	s := NewServer(testSecret)
	_, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: newTenant(t), Email: "a@b.com", Password: "password123",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterRejectsDuplicateEmailWithinTenant(t *testing.T) {
	s := NewServer(testSecret)
	tenantID := newTenant(t)
	req := &dataaccessv1.RegisterRequest{TenantId: tenantID, Email: "dup@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER}
	if _, err := s.Register(t.Context(), req); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err := s.Register(t.Context(), req)
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", err)
	}
}

func TestLoginRoundTrip(t *testing.T) {
	s := NewServer(testSecret)
	tenantID := newTenant(t)
	if _, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: tenantID, Email: "login@b.com", Password: "password123", Role: databasev1.Role_ROLE_STAFF,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	resp, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: tenantID, Email: "login@b.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.GetAccessToken() == "" || resp.GetRefreshToken() == "" {
		t.Fatal("expected both tokens to be issued")
	}

	claims, err := sharedauth.VerifyAccessToken(testSecret, resp.GetAccessToken())
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.TenantID != tenantID || claims.Role != "staff" {
		t.Fatalf("got %+v", claims)
	}
	if resp.GetUser().GetPasswordHash() != "" {
		t.Fatal("expected password_hash to be redacted from the Login response")
	}
}

func TestRegisterRedactsPasswordHash(t *testing.T) {
	s := NewServer(testSecret)
	resp, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: newTenant(t), Email: "redact@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER,
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.GetUser().GetPasswordHash() != "" {
		t.Fatal("expected password_hash to be redacted from the Register response")
	}
}

func TestLoginRejectsWrongPassword(t *testing.T) {
	s := NewServer(testSecret)
	tenantID := newTenant(t)
	if _, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: tenantID, Email: "wrongpw@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: tenantID, Email: "wrongpw@b.com", Password: "not-the-password"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestLoginRejectsUnknownUser(t *testing.T) {
	s := NewServer(testSecret)
	_, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: newTenant(t), Email: "nobody@b.com", Password: "password123"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestLoginRejectsRightUserWrongTenant(t *testing.T) {
	s := NewServer(testSecret)
	tenantA := newTenant(t)
	tenantB := newTenant(t)
	if _, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: tenantA, Email: "isolated@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: tenantB, Email: "isolated@b.com", Password: "password123"})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated when logging into the wrong tenant, got %v", err)
	}
}

func TestRefreshIssuesNewTokens(t *testing.T) {
	s := NewServer(testSecret)
	tenantID := newTenant(t)
	if _, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: tenantID, Email: "refresh@b.com", Password: "password123", Role: databasev1.Role_ROLE_ADMIN,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	login, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: tenantID, Email: "refresh@b.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	refreshed, err := s.Refresh(t.Context(), &dataaccessv1.RefreshRequest{RefreshToken: login.GetRefreshToken()})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.GetAccessToken() == "" || refreshed.GetRefreshToken() == "" {
		t.Fatal("expected both tokens to be reissued")
	}
	claims, err := sharedauth.VerifyAccessToken(testSecret, refreshed.GetAccessToken())
	if err != nil {
		t.Fatalf("VerifyAccessToken: %v", err)
	}
	if claims.TenantID != tenantID || claims.Role != "admin" {
		t.Fatalf("got %+v", claims)
	}
}

func TestRefreshRejectsAccessTokenInItsPlace(t *testing.T) {
	s := NewServer(testSecret)
	tenantID := newTenant(t)
	if _, err := s.Register(t.Context(), &dataaccessv1.RegisterRequest{
		TenantId: tenantID, Email: "wrongtype@b.com", Password: "password123", Role: databasev1.Role_ROLE_OWNER,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	login, err := s.Login(t.Context(), &dataaccessv1.LoginRequest{TenantId: tenantID, Email: "wrongtype@b.com", Password: "password123"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	_, err = s.Refresh(t.Context(), &dataaccessv1.RefreshRequest{RefreshToken: login.GetAccessToken()})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated when an access token is presented as a refresh token, got %v", err)
	}
}
