package mongodb

import "testing"

func TestCreateAndGetConnector(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Connector Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	c, err := CreateConnector(t.Context(), tenant.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}
	if c.GetStatus() != "pending" {
		t.Fatalf("expected initial status \"pending\", got %q", c.GetStatus())
	}

	got, err := GetConnector(t.Context(), tenant.GetXId(), c.GetXId())
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.GetName() != "booking-mcp" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetConnectorWrongTenantNotFound(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	c, err := CreateConnector(t.Context(), tenantA.GetXId(), "shared-name", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}

	if _, err := GetConnector(t.Context(), tenantB.GetXId(), c.GetXId()); err == nil {
		t.Fatal("expected GetConnector to fail when resolved under a different tenant (isolation)")
	}
}

func TestListConnectorsIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Isolation Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Isolation Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	// Both tenants register a connector with the colliding name — must not
	// collide, and each tenant's list must show only their own.
	if _, err := CreateConnector(t.Context(), tenantA.GetXId(), "acme-booking-mcp", "http", "http://example.invalid", ""); err != nil {
		t.Fatalf("CreateConnector A: %v", err)
	}
	if _, err := CreateConnector(t.Context(), tenantB.GetXId(), "acme-booking-mcp", "http", "http://example.invalid", ""); err != nil {
		t.Fatalf("CreateConnector B: %v", err)
	}

	listA, err := ListConnectors(t.Context(), tenantA.GetXId(), 0)
	if err != nil {
		t.Fatalf("ListConnectors A: %v", err)
	}
	listB, err := ListConnectors(t.Context(), tenantB.GetXId(), 0)
	if err != nil {
		t.Fatalf("ListConnectors B: %v", err)
	}

	if len(listA) != 1 || len(listB) != 1 {
		t.Fatalf("expected exactly one connector per tenant, got %d for A and %d for B", len(listA), len(listB))
	}
	if listA[0].GetTenantId() != tenantA.GetXId() {
		t.Fatalf("tenant A's list leaked a connector belonging to %q", listA[0].GetTenantId())
	}
	if listB[0].GetTenantId() != tenantB.GetXId() {
		t.Fatalf("tenant B's list leaked a connector belonging to %q", listB[0].GetTenantId())
	}
}

func TestUpdateConnectorManifest(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Manifest Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	c, err := CreateConnector(t.Context(), tenant.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}

	updated, err := UpdateConnectorManifest(t.Context(), tenant.GetXId(), c.GetXId(), `{"tools":[]}`, "active")
	if err != nil {
		t.Fatalf("UpdateConnectorManifest: %v", err)
	}
	if updated.GetStatus() != "active" {
		t.Fatalf("expected status \"active\", got %q", updated.GetStatus())
	}
	if updated.GetCapabilityManifest() != `{"tools":[]}` {
		t.Fatalf("got manifest %q", updated.GetCapabilityManifest())
	}
	if updated.GetManifestRefreshedAt() == "" {
		t.Fatal("expected manifest_refreshed_at to be set")
	}
}

func TestDeleteConnector(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Delete Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	c, err := CreateConnector(t.Context(), tenant.GetXId(), "booking-mcp", "http", "http://example.invalid", "")
	if err != nil {
		t.Fatalf("CreateConnector: %v", err)
	}

	if err := DeleteConnector(t.Context(), tenant.GetXId(), c.GetXId()); err != nil {
		t.Fatalf("DeleteConnector: %v", err)
	}
	if _, err := GetConnector(t.Context(), tenant.GetXId(), c.GetXId()); err == nil {
		t.Fatal("expected connector to be gone after delete")
	}
}
