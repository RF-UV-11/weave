package chat

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
	"weave/core/mongodb"
	sharedauth "weave/packages/shared-auth"
)

var testSecret = []byte("test-jwt-secret-not-for-prod")

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}
	if err := mongodb.InitDatabase(uri, "weave_core_test_chat"); err != nil {
		fmt.Printf("chat: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}

	code := m.Run()

	if err := mongodb.Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("chat: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func newTenant(t *testing.T) string {
	t.Helper()
	tn, err := mongodb.CreateTenant(t.Context(), "Chat RPC Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	return tn.GetXId()
}

// callAs runs fn through the real shared-auth interceptor, same pattern
// as rpc_services/bot_profile's tests — exercises the exact
// authorization path main.go wires up in production.
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

func TestCreateSessionRequiresAuth(t *testing.T) {
	s := NewServer()
	_, err := s.CreateSession(t.Context(), &dataaccessv1.CreateSessionRequest{
		TenantId: newTenant(t), Channel: "web-widget",
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated without a token, got %v", err)
	}
}

func TestCreateSessionRejectsWrongTenantToken(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	otherTenantID := newTenant(t)

	_, err := callAs(t, otherTenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.CreateSession(ctx, &dataaccessv1.CreateSessionRequest{TenantId: tenantID, Channel: "web-widget"})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied for a token scoped to a different tenant, got %v", err)
	}
}

func TestCreateSessionAppendAndGetMessagesRoundTrip(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)

	sessResp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.CreateSession(ctx, &dataaccessv1.CreateSessionRequest{
			TenantId: tenantID, UserId: "usr_1", BotProfileId: "profile_1", Channel: "web-widget",
		})
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := sessResp.(*dataaccessv1.CreateSessionResponse).GetSession().GetXId()

	_, err = callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.AppendMessage(ctx, &dataaccessv1.AppendMessageRequest{
			TenantId: tenantID, SessionId: sessionID, Role: "user", Content: "hi",
		})
	})
	if err != nil {
		t.Fatalf("AppendMessage user: %v", err)
	}
	_, err = callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.AppendMessage(ctx, &dataaccessv1.AppendMessageRequest{
			TenantId: tenantID, SessionId: sessionID, Role: "assistant", Content: "hello!",
		})
	})
	if err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}

	msgsResp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.GetSessionMessages(ctx, &dataaccessv1.GetSessionMessagesRequest{TenantId: tenantID, SessionId: sessionID})
	})
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	messages := msgsResp.(*dataaccessv1.GetSessionMessagesResponse).GetMessages()
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].GetRole() != "user" || messages[1].GetRole() != "assistant" {
		t.Fatalf("expected [user, assistant], got [%s, %s]", messages[0].GetRole(), messages[1].GetRole())
	}
}

func TestAppendMessageRejectsInvalidRole(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)

	sessResp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.CreateSession(ctx, &dataaccessv1.CreateSessionRequest{TenantId: tenantID, Channel: "web-widget"})
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := sessResp.(*dataaccessv1.CreateSessionResponse).GetSession().GetXId()

	_, err = callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.AppendMessage(ctx, &dataaccessv1.AppendMessageRequest{
			TenantId: tenantID, SessionId: sessionID, Role: "system", Content: "nope",
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument for role \"system\", got %v", err)
	}
}

func TestAppendMessageRejectsSessionFromAnotherTenant(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)
	otherTenantID := newTenant(t)

	sessResp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.CreateSession(ctx, &dataaccessv1.CreateSessionRequest{TenantId: tenantID, Channel: "web-widget"})
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sessionID := sessResp.(*dataaccessv1.CreateSessionResponse).GetSession().GetXId()

	// otherTenantID's own token, targeting tenant A's real session_id —
	// RequireTenant passes (matches otherTenantID's own claim), but the
	// session lookup itself must still reject it (docs/architecture/
	// SECURITY.md §2's cross-tenant isolation).
	_, err = callAs(t, otherTenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.AppendMessage(ctx, &dataaccessv1.AppendMessageRequest{
			TenantId: otherTenantID, SessionId: sessionID, Role: "user", Content: "leak attempt",
		})
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound for a session_id belonging to a different tenant, got %v", err)
	}
}

func TestGetSessionMessagesNotFoundForUnknownSession(t *testing.T) {
	s := NewServer()
	tenantID := newTenant(t)

	_, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.GetSessionMessages(ctx, &dataaccessv1.GetSessionMessagesRequest{TenantId: tenantID, SessionId: "ses_does_not_exist"})
	})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}
