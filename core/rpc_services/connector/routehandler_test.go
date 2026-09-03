package connector

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	"weave/core/vault"
)

var testVault *vault.Vault

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if err := mongodb.InitDatabase(uri, "weave_core_test_connector"); err != nil {
		fmt.Printf("connector: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	v, err := vault.New("5eJ9k0h2xW1qzT7yD3mN8pR4sL6vC9bA0fG2hJ5kM8o=")
	if err != nil {
		fmt.Printf("connector: vault.New: %v\n", err)
		os.Exit(1)
	}
	testVault = v

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("connector: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func newTenant(t *testing.T) string {
	t.Helper()
	tn, err := mongodb.CreateTenant(t.Context(), "Connector RPC Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	return tn.GetXId()
}

func TestRegisterConnectorRejectsPrivateEndpointWhenNotAllowed(t *testing.T) {
	// Proves the SSRF guard (core/netguard) is actually wired in and
	// active by default — every other test in this file passes
	// allowPrivate=true purely because local dev/test fixtures run on
	// loopback; this test is the one that exercises the real,
	// secure-by-default posture.
	s := NewServer(testVault, net.DefaultResolver, false)
	_, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: newTenant(t), Name: "x", Transport: "http", Endpoint: "http://127.0.0.1:9999",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for a loopback endpoint, got %v", err)
	}
}

func TestRegisterConnectorRequiresTenantID(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	_, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		Name: "x", Transport: "http", Endpoint: "http://example.invalid",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterConnectorRejectsUnknownTransport(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	_, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: newTenant(t), Name: "x", Transport: "carrier-pigeon", Endpoint: "http://example.invalid",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterConnectorEncryptsCredential(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	tenantID := newTenant(t)

	resp, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId:         tenantID,
		Name:             "booking-mcp",
		Transport:        "http",
		Endpoint:         "http://example.invalid",
		CredentialSecret: "sk-test-secret",
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}
	if resp.GetConnector().GetCredentialRefId() == "" {
		t.Fatal("expected a credential_ref_id to be set")
	}

	cred, err := mongodb.GetCredential(t.Context(), tenantID, resp.GetConnector().GetCredentialRefId())
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if string(cred.GetCiphertext()) == "sk-test-secret" {
		t.Fatal("credential must be encrypted at rest, not stored in plaintext")
	}
	opened, err := mongodb.OpenCredential(testVault, cred)
	if err != nil {
		t.Fatalf("OpenCredential: %v", err)
	}
	if opened != "sk-test-secret" {
		t.Fatalf("got %q, want the original secret back", opened)
	}
}

func TestListConnectorsIsolatedPerTenant(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	tenantA := newTenant(t)
	tenantB := newTenant(t)

	if _, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantA, Name: "acme-booking-mcp", Transport: "http", Endpoint: "http://example.invalid",
	}); err != nil {
		t.Fatalf("RegisterConnector A: %v", err)
	}
	if _, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantB, Name: "acme-booking-mcp", Transport: "http", Endpoint: "http://example.invalid",
	}); err != nil {
		t.Fatalf("RegisterConnector B: %v", err)
	}

	listA, err := s.ListConnectors(t.Context(), &dataaccessv1.ListConnectorsRequest{TenantId: tenantA})
	if err != nil {
		t.Fatalf("ListConnectors A: %v", err)
	}
	listB, err := s.ListConnectors(t.Context(), &dataaccessv1.ListConnectorsRequest{TenantId: tenantB})
	if err != nil {
		t.Fatalf("ListConnectors B: %v", err)
	}

	if len(listA.GetConnectors()) != 1 || len(listB.GetConnectors()) != 1 {
		t.Fatalf("expected one connector per tenant, got %d for A and %d for B", len(listA.GetConnectors()), len(listB.GetConnectors()))
	}
}

func TestRefreshManifestCachesToolsList(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"book_appointment","description":"Book an appointment slot"}]}}`))
	}))
	defer stub.Close()

	s := NewServer(testVault, net.DefaultResolver, true)
	tenantID := newTenant(t)
	reg, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantID, Name: "booking-mcp", Transport: "http", Endpoint: stub.URL,
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}

	refreshed, err := s.RefreshManifest(t.Context(), &dataaccessv1.RefreshManifestRequest{
		TenantId: tenantID, ConnectorId: reg.GetConnector().GetXId(),
	})
	if err != nil {
		t.Fatalf("RefreshManifest: %v", err)
	}
	if refreshed.GetConnector().GetStatus() != "active" {
		t.Fatalf("expected status \"active\", got %q", refreshed.GetConnector().GetStatus())
	}
	if !strings.Contains(refreshed.GetConnector().GetCapabilityManifest(), "book_appointment") {
		t.Fatalf("expected cached manifest to contain the tool name, got %q", refreshed.GetConnector().GetCapabilityManifest())
	}
}

func TestRefreshManifestRejectsToolMissingDescription(t *testing.T) {
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// "book_appointment" has no description — the connector integrator
		// must supply full descriptions for every tool it exposes
		// (docs/architecture/ARCHITECTURE.md §3).
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"book_appointment"}]}}`))
	}))
	defer stub.Close()

	s := NewServer(testVault, net.DefaultResolver, true)
	tenantID := newTenant(t)
	reg, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantID, Name: "undescribed-mcp", Transport: "http", Endpoint: stub.URL,
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}

	_, err = s.RefreshManifest(t.Context(), &dataaccessv1.RefreshManifestRequest{
		TenantId: tenantID, ConnectorId: reg.GetConnector().GetXId(),
	})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("expected FailedPrecondition, got %v", err)
	}

	got, err := mongodb.GetConnector(t.Context(), tenantID, reg.GetConnector().GetXId())
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.GetStatus() != "error" {
		t.Fatalf("expected status \"error\" after a rejected manifest, got %q", got.GetStatus())
	}
}

func TestRefreshManifestMarksErrorOnUnreachableConnector(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	tenantID := newTenant(t)
	reg, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantID, Name: "unreachable-mcp", Transport: "http", Endpoint: "http://127.0.0.1:1",
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}

	if _, err := s.RefreshManifest(t.Context(), &dataaccessv1.RefreshManifestRequest{
		TenantId: tenantID, ConnectorId: reg.GetConnector().GetXId(),
	}); err == nil {
		t.Fatal("expected RefreshManifest to fail for an unreachable connector")
	}

	got, err := mongodb.GetConnector(t.Context(), tenantID, reg.GetConnector().GetXId())
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if got.GetStatus() != "error" {
		t.Fatalf("expected status \"error\" after a failed refresh, got %q", got.GetStatus())
	}
}

func TestDeregisterConnectorRemovesConnectorAndCredential(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	tenantID := newTenant(t)
	reg, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantID, Name: "booking-mcp", Transport: "http", Endpoint: "http://example.invalid",
		CredentialSecret: "sk-test-secret",
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}
	credID := reg.GetConnector().GetCredentialRefId()

	if _, err := s.DeregisterConnector(t.Context(), &dataaccessv1.DeregisterConnectorRequest{
		TenantId: tenantID, ConnectorId: reg.GetConnector().GetXId(),
	}); err != nil {
		t.Fatalf("DeregisterConnector: %v", err)
	}

	if _, err := mongodb.GetConnector(t.Context(), tenantID, reg.GetConnector().GetXId()); err == nil {
		t.Fatal("expected connector to be gone after deregister")
	}
	if _, err := mongodb.GetCredential(t.Context(), tenantID, credID); err == nil {
		t.Fatal("expected credential to be gone after deregister")
	}
}

func TestDeregisterConnectorWrongTenantNotFound(t *testing.T) {
	s := NewServer(testVault, net.DefaultResolver, true)
	tenantA := newTenant(t)
	tenantB := newTenant(t)
	reg, err := s.RegisterConnector(t.Context(), &dataaccessv1.RegisterConnectorRequest{
		TenantId: tenantA, Name: "booking-mcp", Transport: "http", Endpoint: "http://example.invalid",
	})
	if err != nil {
		t.Fatalf("RegisterConnector: %v", err)
	}

	_, err = s.DeregisterConnector(t.Context(), &dataaccessv1.DeregisterConnectorRequest{
		TenantId: tenantB, ConnectorId: reg.GetConnector().GetXId(),
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound when deregistering under the wrong tenant, got %v", err)
	}
}
