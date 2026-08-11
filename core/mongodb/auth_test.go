package mongodb

import (
	"testing"

	databasev1 "weave/core/gen/database/v1"
)

func TestCreateUserHashesPassword(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Auth Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	u, err := CreateUser(t.Context(), tenant.GetXId(), "owner@acme.test", "correct horse battery staple", databasev1.Role_ROLE_OWNER)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if u.GetPasswordHash() == "correct horse battery staple" {
		t.Fatal("password_hash must not equal the plaintext password")
	}
	if !VerifyPassword(u, "correct horse battery staple") {
		t.Fatal("expected VerifyPassword to accept the correct password")
	}
	if VerifyPassword(u, "wrong password") {
		t.Fatal("expected VerifyPassword to reject an incorrect password")
	}
}

func TestGetUserByEmail(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Lookup Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := CreateUser(t.Context(), tenant.GetXId(), "staff@acme.test", "secret-password", databasev1.Role_ROLE_STAFF); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := GetUserByEmail(t.Context(), tenant.GetXId(), "staff@acme.test")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.GetRole() != databasev1.Role_ROLE_STAFF {
		t.Fatalf("got role %v", got.GetRole())
	}
}

func TestGetUserByEmailWrongTenantNotFound(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Auth Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Auth Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}
	if _, err := CreateUser(t.Context(), tenantA.GetXId(), "shared@acme.test", "secret-password", databasev1.Role_ROLE_OWNER); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, err := GetUserByEmail(t.Context(), tenantB.GetXId(), "shared@acme.test"); err == nil {
		t.Fatal("expected GetUserByEmail to fail when resolved under a different tenant (isolation)")
	}
}

func TestSameEmailAllowedAcrossTenants(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Cross Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Cross Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	if _, err := CreateUser(t.Context(), tenantA.GetXId(), "person@example.com", "secret-password", databasev1.Role_ROLE_OWNER); err != nil {
		t.Fatalf("CreateUser A: %v", err)
	}
	if _, err := CreateUser(t.Context(), tenantB.GetXId(), "person@example.com", "different-secret", databasev1.Role_ROLE_CUSTOMER); err != nil {
		t.Fatalf("expected the same email to be registrable under a different tenant, got: %v", err)
	}
}
