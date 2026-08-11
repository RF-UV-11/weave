package mongodb

import "testing"

func TestCreateAndGetTenant(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Acme Clinic", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tenant.GetXId() == "" {
		t.Fatal("expected a generated _id")
	}

	got, err := GetTenant(t.Context(), tenant.GetXId())
	if err != nil {
		t.Fatalf("GetTenant: %v", err)
	}
	if got.GetDisplayName() != "Acme Clinic" || got.GetTenantType() != "business" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetTenantNotFound(t *testing.T) {
	if _, err := GetTenant(t.Context(), "tnt_does_not_exist"); err == nil {
		t.Fatal("expected error for a nonexistent tenant")
	}
}

func TestListTenantsIncludesCreated(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "List Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	tenants, err := ListTenants(t.Context(), 0)
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}

	found := false
	for _, tn := range tenants {
		if tn.GetXId() == tenant.GetXId() {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ListTenants to include the newly created tenant")
	}
}
