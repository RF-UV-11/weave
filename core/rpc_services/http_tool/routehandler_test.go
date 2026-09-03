package httptool

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	"weave/core/vault"
	sharedauth "weave/packages/shared-auth"
)

var testSecret = []byte("test-jwt-secret-not-for-prod")
var testVault *vault.Vault

const gatewayBaseURL = "http://mcp-gateway.internal"

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if err := mongodb.InitDatabase(uri, "weave_core_test_http_tool"); err != nil {
		fmt.Printf("http_tool: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	v, err := vault.New("5eJ9k0h2xW1qzT7yD3mN8pR4sL6vC9bA0fG2hJ5kM8o=")
	if err != nil {
		fmt.Printf("http_tool: vault.New: %v\n", err)
		os.Exit(1)
	}
	testVault = v

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("http_tool: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func newTenant(t *testing.T) string {
	t.Helper()
	tn, err := mongodb.CreateTenant(t.Context(), "HTTP Tool RPC Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	return tn.GetXId()
}

func callAs(t *testing.T, tenantID, role string, fn grpc.UnaryHandler) (any, error) {
	t.Helper()
	tok, _, err := sharedauth.IssueAccessToken(testSecret, tenantID, "usr_test", role)
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	ctx := metadata.NewIncomingContext(t.Context(), metadata.Pairs("authorization", "Bearer "+tok))
	interceptor := sharedauth.UnaryServerInterceptor(testSecret)
	return interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test/Method"}, fn)
}

func TestRegisterHttpToolRejectsPrivateEndpointWhenNotAllowed(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, false)
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "x", Description: "does x", HttpEndpoint: "http://169.254.169.254/", HttpMethod: "GET",
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for a cloud-metadata endpoint, got %v", err)
	}
}

func TestRegisterHttpToolRequiresAuth(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	_, err := s.RegisterHttpTool(t.Context(), &dataaccessv1.RegisterHttpToolRequest{
		TenantId: newTenant(t), Name: "x", Description: "does x", HttpEndpoint: "https://x", HttpMethod: "GET",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestRegisterHttpToolRejectsNonAdminRole(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "x", Description: "does x", HttpEndpoint: "https://x", HttpMethod: "GET",
		})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestRegisterHttpToolRejectsMissingDescription(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "x", HttpEndpoint: "https://x", HttpMethod: "GET",
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterHttpToolRejectsBadMethod(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "x", Description: "does x", HttpEndpoint: "https://x", HttpMethod: "TRACE",
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestRegisterHttpToolCreatesManagedConnectorOnce(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)

	register := func(name string) *dataaccessv1.RegisterHttpToolResponse {
		resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
			return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
				TenantId: tenantID, Name: name, Description: "does " + name, HttpEndpoint: "https://x", HttpMethod: "GET",
			})
		})
		if err != nil {
			t.Fatalf("RegisterHttpTool(%s): %v", name, err)
		}
		return resp.(*dataaccessv1.RegisterHttpToolResponse)
	}

	first := register("tool_one")
	second := register("tool_two")

	if first.GetHttpTool().GetConnectorId() != second.GetHttpTool().GetConnectorId() {
		t.Fatalf("expected both tools to share one managed connector, got %q vs %q",
			first.GetHttpTool().GetConnectorId(), second.GetHttpTool().GetConnectorId())
	}

	connector, err := mongodb.GetConnector(t.Context(), tenantID, first.GetHttpTool().GetConnectorId())
	if err != nil {
		t.Fatalf("GetConnector: %v", err)
	}
	if connector.GetEndpoint() != gatewayBaseURL+"/"+tenantID+"/mcp" {
		t.Fatalf("unexpected managed connector endpoint %q", connector.GetEndpoint())
	}
}

func TestRegisterHttpToolEncryptsCredential(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)

	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "secure_tool", Description: "needs a credential",
			HttpEndpoint: "https://x", HttpMethod: "POST", CredentialSecret: "sk-test-secret",
		})
	})
	if err != nil {
		t.Fatalf("RegisterHttpTool: %v", err)
	}
	tool := resp.(*dataaccessv1.RegisterHttpToolResponse).GetHttpTool()
	if tool.GetCredentialRefId() == "" {
		t.Fatal("expected a credential_ref_id to be set")
	}

	cred, err := mongodb.GetCredential(t.Context(), tenantID, tool.GetCredentialRefId())
	if err != nil {
		t.Fatalf("GetCredential: %v", err)
	}
	if string(cred.GetCiphertext()) == "sk-test-secret" {
		t.Fatal("credential must be encrypted at rest, not stored in plaintext")
	}
}

func TestListHttpToolsIsolatedPerTenant(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantA := newTenant(t)
	tenantB := newTenant(t)

	if _, err := callAs(t, tenantA, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantA, Name: "a_tool", Description: "belongs to A", HttpEndpoint: "https://x", HttpMethod: "GET",
		})
	}); err != nil {
		t.Fatalf("RegisterHttpTool: %v", err)
	}

	listA, err := s.ListHttpTools(t.Context(), &dataaccessv1.ListHttpToolsRequest{TenantId: tenantA})
	if err != nil {
		t.Fatalf("ListHttpTools A: %v", err)
	}
	listB, err := s.ListHttpTools(t.Context(), &dataaccessv1.ListHttpToolsRequest{TenantId: tenantB})
	if err != nil {
		t.Fatalf("ListHttpTools B: %v", err)
	}
	if len(listA.GetHttpTools()) != 1 {
		t.Fatalf("expected tenant A to see its own tool, got %v", listA.GetHttpTools())
	}
	if len(listB.GetHttpTools()) != 0 {
		t.Fatalf("expected tenant B to see no tools, got %v", listB.GetHttpTools())
	}
}

func TestDeregisterHttpToolRemovesToolAndCredential(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)

	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "to_delete", Description: "will be deregistered",
			HttpEndpoint: "https://x", HttpMethod: "GET", CredentialSecret: "sk-test-secret",
		})
	})
	if err != nil {
		t.Fatalf("RegisterHttpTool: %v", err)
	}
	tool := resp.(*dataaccessv1.RegisterHttpToolResponse).GetHttpTool()

	_, err = callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.DeregisterHttpTool(ctx, &dataaccessv1.DeregisterHttpToolRequest{
			TenantId: tenantID, HttpToolId: tool.GetXId(),
		})
	})
	if err != nil {
		t.Fatalf("DeregisterHttpTool: %v", err)
	}

	if _, err := mongodb.GetHttpTool(t.Context(), tenantID, tool.GetXId()); err == nil {
		t.Fatal("expected the tool to be gone after deregister")
	}
	if _, err := mongodb.GetCredential(t.Context(), tenantID, tool.GetCredentialRefId()); err == nil {
		t.Fatal("expected the credential to be gone after deregister")
	}
}

func TestRevealHttpToolCredentialReturnsPlaintext(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)

	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "reveal_tool", Description: "needs a credential",
			HttpEndpoint: "https://x", HttpMethod: "POST", CredentialSecret: "sk-reveal-me",
		})
	})
	if err != nil {
		t.Fatalf("RegisterHttpTool: %v", err)
	}
	tool := resp.(*dataaccessv1.RegisterHttpToolResponse).GetHttpTool()

	revealed, err := s.RevealHttpToolCredential(t.Context(), &dataaccessv1.RevealHttpToolCredentialRequest{
		TenantId: tenantID, HttpToolId: tool.GetXId(),
	})
	if err != nil {
		t.Fatalf("RevealHttpToolCredential: %v", err)
	}
	if revealed.GetSecret() != "sk-reveal-me" {
		t.Fatalf("got %q, want %q", revealed.GetSecret(), "sk-reveal-me")
	}
}

func TestRevealHttpToolCredentialEmptyWhenNoCredential(t *testing.T) {
	s := NewServer(testVault, gatewayBaseURL, net.DefaultResolver, true)
	tenantID := newTenant(t)

	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.RegisterHttpTool(ctx, &dataaccessv1.RegisterHttpToolRequest{
			TenantId: tenantID, Name: "no_credential_tool", Description: "needs nothing",
			HttpEndpoint: "https://x", HttpMethod: "GET",
		})
	})
	if err != nil {
		t.Fatalf("RegisterHttpTool: %v", err)
	}
	tool := resp.(*dataaccessv1.RegisterHttpToolResponse).GetHttpTool()

	revealed, err := s.RevealHttpToolCredential(t.Context(), &dataaccessv1.RevealHttpToolCredentialRequest{
		TenantId: tenantID, HttpToolId: tool.GetXId(),
	})
	if err != nil {
		t.Fatalf("RevealHttpToolCredential: %v", err)
	}
	if revealed.GetSecret() != "" {
		t.Fatalf("expected an empty secret for a tool with no credential, got %q", revealed.GetSecret())
	}
}
