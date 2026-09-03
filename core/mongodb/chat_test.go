package mongodb

import "testing"

func TestCreateSessionAndAppendMessages(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Chat Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	sess, err := CreateSession(t.Context(), tenant.GetXId(), "usr_1", "profile_1", "web-widget")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.GetTenantId() != tenant.GetXId() || sess.GetChannel() != "web-widget" {
		t.Fatalf("unexpected session: %+v", sess)
	}

	if _, err := AppendMessage(t.Context(), tenant.GetXId(), sess.GetXId(), "user", "what's my order status?", "", ""); err != nil {
		t.Fatalf("AppendMessage user: %v", err)
	}
	if _, err := AppendMessage(t.Context(), tenant.GetXId(), sess.GetXId(), "assistant", "your order shipped", "track_order", "weave_managed"); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}

	messages, err := GetSessionMessages(t.Context(), tenant.GetXId(), sess.GetXId(), 0)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].GetRole() != "user" || messages[1].GetRole() != "assistant" {
		t.Fatalf("expected oldest-first order [user, assistant], got [%s, %s]", messages[0].GetRole(), messages[1].GetRole())
	}
	if messages[1].GetToolUsed() != "track_order" || messages[1].GetConnectorUsed() != "weave_managed" {
		t.Fatalf("tool_used/connector_used didn't round-trip: %+v", messages[1])
	}
}

func TestGetSessionMessagesRespectsLimitMostRecent(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Chat Limit Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	sess, err := CreateSession(t.Context(), tenant.GetXId(), "usr_1", "profile_1", "web-widget")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	contents := []string{"first", "second", "third", "fourth"}
	for _, c := range contents {
		if _, err := AppendMessage(t.Context(), tenant.GetXId(), sess.GetXId(), "user", c, "", ""); err != nil {
			t.Fatalf("AppendMessage %q: %v", c, err)
		}
	}

	messages, err := GetSessionMessages(t.Context(), tenant.GetXId(), sess.GetXId(), 2)
	if err != nil {
		t.Fatalf("GetSessionMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages (limit), got %d", len(messages))
	}
	// Most recent 2, oldest-first: "third" then "fourth".
	if messages[0].GetContent() != "third" || messages[1].GetContent() != "fourth" {
		t.Fatalf("expected [third, fourth], got [%s, %s]", messages[0].GetContent(), messages[1].GetContent())
	}
}

func TestSessionsAndMessagesIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Chat Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Chat Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	sessA, err := CreateSession(t.Context(), tenantA.GetXId(), "usr_1", "profile_1", "web-widget")
	if err != nil {
		t.Fatalf("CreateSession A: %v", err)
	}
	if _, err := AppendMessage(t.Context(), tenantA.GetXId(), sessA.GetXId(), "user", "tenant A secret", "", ""); err != nil {
		t.Fatalf("AppendMessage A: %v", err)
	}

	if _, err := GetSession(t.Context(), tenantB.GetXId(), sessA.GetXId()); err == nil {
		t.Fatal("expected tenant B to be unable to look up tenant A's session (isolation)")
	}

	messagesForB, err := GetSessionMessages(t.Context(), tenantB.GetXId(), sessA.GetXId(), 0)
	if err != nil {
		t.Fatalf("GetSessionMessages B: %v", err)
	}
	if len(messagesForB) != 0 {
		t.Fatalf("expected tenant B to see no messages for tenant A's session, got %+v", messagesForB)
	}
}
