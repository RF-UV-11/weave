package mongodb

import "testing"

func TestGetOrCreateManagedConnectorIsIdempotent(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "HTTP Tool Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	first, err := GetOrCreateManagedConnector(t.Context(), tenant.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector (1st): %v", err)
	}
	if first.GetTransport() != "weave_managed" {
		t.Fatalf("expected transport weave_managed, got %q", first.GetTransport())
	}
	if first.GetEndpoint() != "http://gateway.internal/"+tenant.GetXId()+"/mcp" {
		t.Fatalf("unexpected endpoint %q", first.GetEndpoint())
	}

	second, err := GetOrCreateManagedConnector(t.Context(), tenant.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector (2nd): %v", err)
	}
	if second.GetXId() != first.GetXId() {
		t.Fatalf("expected the same connector to be reused, got %q vs %q", first.GetXId(), second.GetXId())
	}
}

func TestGetOrCreateManagedConnectorIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "HTTP Tool Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "HTTP Tool Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	connA, err := GetOrCreateManagedConnector(t.Context(), tenantA.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector A: %v", err)
	}
	connB, err := GetOrCreateManagedConnector(t.Context(), tenantB.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector B: %v", err)
	}
	if connA.GetXId() == connB.GetXId() {
		t.Fatal("expected distinct managed connectors per tenant")
	}
}

func TestCreateAndListHttpTools(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "List HTTP Tools Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	connector, err := GetOrCreateManagedConnector(t.Context(), tenant.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector: %v", err)
	}

	tool, err := CreateHttpTool(t.Context(), tenant.GetXId(), connector.GetXId(), "get_order_status",
		"Look up an order's shipping status", "https://api.acme.test/orders/{id}", "GET", `{"type":"object"}`, "", "external", "general", "none")
	if err != nil {
		t.Fatalf("CreateHttpTool: %v", err)
	}
	if tool.GetName() != "get_order_status" {
		t.Fatalf("got %+v", tool)
	}
	if tool.GetVisibility() != "external" || tool.GetCategory() != "general" {
		t.Fatalf("visibility/category didn't round-trip: %+v", tool)
	}

	tools, err := ListHttpTools(t.Context(), tenant.GetXId())
	if err != nil {
		t.Fatalf("ListHttpTools: %v", err)
	}
	if len(tools) != 1 || tools[0].GetXId() != tool.GetXId() {
		t.Fatalf("expected the created tool to be listed, got %+v", tools)
	}
}

func TestListHttpToolsIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "HTTP List Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "HTTP List Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}
	connA, err := GetOrCreateManagedConnector(t.Context(), tenantA.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector: %v", err)
	}
	if _, err := CreateHttpTool(t.Context(), tenantA.GetXId(), connA.GetXId(), "shared_name", "desc", "https://x", "GET", "{}", "", "internal", "general", "none"); err != nil {
		t.Fatalf("CreateHttpTool: %v", err)
	}

	toolsB, err := ListHttpTools(t.Context(), tenantB.GetXId())
	if err != nil {
		t.Fatalf("ListHttpTools B: %v", err)
	}
	if len(toolsB) != 0 {
		t.Fatalf("expected tenant B to see no tools, got %+v", toolsB)
	}
}

func TestCreateHttpToolPersistsAuthMode(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Auth Mode Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	connector, err := GetOrCreateManagedConnector(t.Context(), tenant.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector: %v", err)
	}

	tool, err := CreateHttpTool(t.Context(), tenant.GetXId(), connector.GetXId(), "get_my_transactions",
		"Look up the calling user's own transactions", "https://api.acme.test/me/transactions", "GET", `{"type":"object"}`,
		"cred_1", "external", "general", "user_token")
	if err != nil {
		t.Fatalf("CreateHttpTool: %v", err)
	}
	if tool.GetAuthMode() != "user_token" {
		t.Fatalf("got auth_mode %q", tool.GetAuthMode())
	}

	fetched, err := GetHttpTool(t.Context(), tenant.GetXId(), tool.GetXId())
	if err != nil {
		t.Fatalf("GetHttpTool: %v", err)
	}
	if fetched.GetAuthMode() != "user_token" {
		t.Fatalf("auth_mode didn't round-trip: %+v", fetched)
	}
}

func TestDeleteHttpTool(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Delete HTTP Tool Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	connector, err := GetOrCreateManagedConnector(t.Context(), tenant.GetXId(), "http://gateway.internal")
	if err != nil {
		t.Fatalf("GetOrCreateManagedConnector: %v", err)
	}
	tool, err := CreateHttpTool(t.Context(), tenant.GetXId(), connector.GetXId(), "tool_to_delete", "desc", "https://x", "GET", "{}", "", "internal", "general", "none")
	if err != nil {
		t.Fatalf("CreateHttpTool: %v", err)
	}

	if err := DeleteHttpTool(t.Context(), tenant.GetXId(), tool.GetXId()); err != nil {
		t.Fatalf("DeleteHttpTool: %v", err)
	}
	if _, err := GetHttpTool(t.Context(), tenant.GetXId(), tool.GetXId()); err == nil {
		t.Fatal("expected the tool to be gone after delete")
	}
}
