package mongodb

import "testing"

func TestCreateCredentialNeverStoresPlaintext(t *testing.T) {
	v := testVault(t)
	tenant, err := CreateTenant(t.Context(), "Credential Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	connector, err := CreateConnector(t.Context(), tenant.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}

	secret := "sk-super-secret-token-12345"
	cred, err := CreateCredential(t.Context(), v, tenant.GetXId(), connector.GetXId(), secret)
	if err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	if string(cred.GetCiphertext()) == secret {
		t.Fatal("ciphertext must not equal the plaintext secret")
	}
	if len(cred.GetWrappedDek()) == 0 {
		t.Fatal("expected a wrapped DEK to be stored")
	}

	stored, err := GetCredential(t.Context(), tenant.GetXId(), cred.GetXId())
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if string(stored.GetCiphertext()) == secret {
		t.Fatal("re-fetched credential must still not contain the plaintext secret")
	}

	opened, err := OpenCredential(v, stored)
	if err != nil {
		t.Fatalf("OpenCredential: %v", err)
	}
	if opened != secret {
		t.Fatalf("got %q, want %q", opened, secret)
	}
}

func TestGetCredentialWrongTenantNotFound(t *testing.T) {
	v := testVault(t)
	tenantA, err := CreateTenant(t.Context(), "Cred Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Cred Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}
	connector, err := CreateConnector(t.Context(), tenantA.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}
	cred, err := CreateCredential(t.Context(), v, tenantA.GetXId(), connector.GetXId(), "secret")
	if err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	if _, err := GetCredential(t.Context(), tenantB.GetXId(), cred.GetXId()); err == nil {
		t.Fatal("expected GetCredential to fail when resolved under a different tenant (isolation)")
	}
}

func TestDeleteCredential(t *testing.T) {
	v := testVault(t)
	tenant, err := CreateTenant(t.Context(), "Delete Cred Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	connector, err := CreateConnector(t.Context(), tenant.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}
	cred, err := CreateCredential(t.Context(), v, tenant.GetXId(), connector.GetXId(), "secret")
	if err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	if err := DeleteCredential(t.Context(), tenant.GetXId(), cred.GetXId()); err != nil {
		t.Fatalf("DeleteCredential: %v", err)
	}
	if _, err := GetCredential(t.Context(), tenant.GetXId(), cred.GetXId()); err == nil {
		t.Fatal("expected credential to be gone after delete")
	}
}
