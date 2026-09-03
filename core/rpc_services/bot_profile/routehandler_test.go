package botprofile

import (
	"context"
	"fmt"
	"os"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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
	if err := mongodb.InitDatabase(uri, "weave_core_test_bot_profile"); err != nil {
		fmt.Printf("bot_profile: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("bot_profile: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func newTenant(t *testing.T) string {
	t.Helper()
	tn, err := mongodb.CreateTenant(t.Context(), "Bot Profile RPC Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	return tn.GetXId()
}

// callAs runs fn through the real shared-auth interceptor with a freshly
// issued access token for {tenantID, role} — exercising the exact
// authorization path main.go wires up in production, not a shortcut.
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

func TestCreateBotProfileRequiresAuth(t *testing.T) {
	s := NewServer()
	_, err := s.CreateBotProfile(t.Context(), &dataaccessv1.CreateBotProfileRequest{
		TenantId: newTenant(t), Name: "external", Channels: []string{"web-widget"},
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without a token, got %v", err)
	}
}

func TestCreateBotProfileRejectsWrongTenantToken(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	otherTenantID := newTenant(t)

	_, err := callAs(t, otherTenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "external", Channels: []string{"web-widget"},
		})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for a token scoped to a different tenant, got %v", err)
	}
}

func TestCreateBotProfileRejectsNonAdminRole(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)

	_, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "external", Channels: []string{"web-widget"},
		})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for role \"customer\", got %v", err)
	}
}

func TestCreateAndResolveTwoBotProfiles(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)

	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "external", Persona: "personas/external.md",
			Channels:     []string{"web-widget", "whatsapp"},
			RolesAllowed: []databasev1.Role{databasev1.Role_ROLE_CUSTOMER},
		})
	})
	if err != nil {
		t.Fatalf("CreateBotProfile external: %v", err)
	}

	_, err = callAs(t, tenantID, "admin", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "internal", Persona: "personas/internal.md",
			Channels:     []string{"slack"},
			RolesAllowed: []databasev1.Role{databasev1.Role_ROLE_STAFF, databasev1.Role_ROLE_ADMIN},
		})
	})
	if err != nil {
		t.Fatalf("CreateBotProfile internal: %v", err)
	}

	// A customer on the web-widget channel should resolve "external".
	resp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.GetActiveBotProfile(ctx, &dataaccessv1.GetActiveBotProfileRequest{TenantId: tenantID, Channel: "web-widget"})
	})
	if err != nil {
		t.Fatalf("GetActiveBotProfile web-widget: %v", err)
	}
	external := resp.(*dataaccessv1.GetActiveBotProfileResponse).GetBotProfile()
	if external.GetName() != "external" {
		t.Fatalf("expected \"external\", got %q", external.GetName())
	}
	if len(external.GetRolesAllowed()) != 1 || external.GetRolesAllowed()[0] != databasev1.Role_ROLE_CUSTOMER {
		t.Fatalf("expected roles_allowed=[CUSTOMER], got %v", external.GetRolesAllowed())
	}

	// A staff member on the slack channel should resolve "internal".
	resp, err = callAs(t, tenantID, "staff", func(ctx context.Context, req any) (any, error) {
		return s.GetActiveBotProfile(ctx, &dataaccessv1.GetActiveBotProfileRequest{TenantId: tenantID, Channel: "slack"})
	})
	if err != nil {
		t.Fatalf("GetActiveBotProfile slack: %v", err)
	}
	internal := resp.(*dataaccessv1.GetActiveBotProfileResponse).GetBotProfile()
	if internal.GetName() != "internal" {
		t.Fatalf("expected \"internal\", got %q", internal.GetName())
	}
	if len(internal.GetRolesAllowed()) != 2 {
		t.Fatalf("expected 2 roles_allowed, got %v", internal.GetRolesAllowed())
	}
}

func TestGetActiveBotProfileNotFoundForUnregisteredChannel(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "external", Channels: []string{"web-widget"},
		})
	})
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}

	_, err = callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.GetActiveBotProfile(ctx, &dataaccessv1.GetActiveBotProfileRequest{TenantId: tenantID, Channel: "whatsapp"})
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestCreateBotProfileDefaultsVisibilityToInternal(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "unspecified-visibility", Channels: []string{"web-widget"},
		})
	})
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}
	if got := resp.(*dataaccessv1.CreateBotProfileResponse).GetBotProfile().GetVisibility(); got != "internal" {
		t.Fatalf("expected default visibility \"internal\", got %q", got)
	}
}

func TestCreateBotProfileRejectsInvalidVisibility(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	_, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "bad-visibility", Channels: []string{"web-widget"}, Visibility: "public",
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestCreateBotProfilePersistsGuardrails(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	guardrails := []string{"Never disclose internal SKU codes.", "Never disclose supplier names."}
	resp, err := callAs(t, tenantID, "owner", func(ctx context.Context, req any) (any, error) {
		return s.CreateBotProfile(ctx, &dataaccessv1.CreateBotProfileRequest{
			TenantId: tenantID, Name: "guarded", Channels: []string{"web-widget"},
			Visibility: "external", Guardrails: guardrails,
		})
	})
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}
	profile := resp.(*dataaccessv1.CreateBotProfileResponse).GetBotProfile()
	if profile.GetVisibility() != "external" {
		t.Fatalf("got visibility %q", profile.GetVisibility())
	}
	if len(profile.GetGuardrails()) != 2 {
		t.Fatalf("expected 2 guardrails, got %v", profile.GetGuardrails())
	}
}
