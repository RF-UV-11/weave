package mongodb

import (
	"testing"

	databasev1 "weave/core/gen/database/v1"
)

func TestCreateAndGetActiveBotProfileByChannel(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Bot Profile Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	external, err := CreateBotProfile(t.Context(), tenant.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget", "whatsapp"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false, "", "")
	if err != nil {
		t.Fatalf("CreateBotProfile external: %v", err)
	}
	internal, err := CreateBotProfile(t.Context(), tenant.GetXId(), "internal", "personas/internal.md",
		nil, []string{"slack"}, []databasev1.Role{databasev1.Role_ROLE_STAFF, databasev1.Role_ROLE_ADMIN}, "internal", nil, false, "", "")
	if err != nil {
		t.Fatalf("CreateBotProfile internal: %v", err)
	}

	gotExternal, err := GetActiveBotProfileByChannel(t.Context(), tenant.GetXId(), "web-widget")
	if err != nil {
		t.Fatalf("GetActiveBotProfileByChannel web-widget: %v", err)
	}
	if gotExternal.GetXId() != external.GetXId() {
		t.Fatalf("expected external profile for web-widget, got %q", gotExternal.GetName())
	}

	gotInternal, err := GetActiveBotProfileByChannel(t.Context(), tenant.GetXId(), "slack")
	if err != nil {
		t.Fatalf("GetActiveBotProfileByChannel slack: %v", err)
	}
	if gotInternal.GetXId() != internal.GetXId() {
		t.Fatalf("expected internal profile for slack, got %q", gotInternal.GetName())
	}
}

func TestGetActiveBotProfileByChannelIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "Profile Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "Profile Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}

	if _, err := CreateBotProfile(t.Context(), tenantA.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false, "", ""); err != nil {
		t.Fatalf("CreateBotProfile A: %v", err)
	}

	if _, err := GetActiveBotProfileByChannel(t.Context(), tenantB.GetXId(), "web-widget"); err == nil {
		t.Fatal("expected no profile to resolve for tenant B on a channel only tenant A registered (isolation)")
	}
}

func TestGetActiveBotProfileByChannelNoMatch(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "No Match Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := CreateBotProfile(t.Context(), tenant.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false, "", ""); err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}

	if _, err := GetActiveBotProfileByChannel(t.Context(), tenant.GetXId(), "whatsapp"); err == nil {
		t.Fatal("expected no match for an unregistered channel")
	}
}

func TestCreateBotProfilePersistsVisibilityAndGuardrails(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "Guardrail Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	guardrails := []string{"Never disclose internal SKU codes.", "Never disclose supplier names."}
	profile, err := CreateBotProfile(t.Context(), tenant.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", guardrails, true, "", "")
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}
	if profile.GetVisibility() != "external" {
		t.Fatalf("got visibility %q", profile.GetVisibility())
	}
	if len(profile.GetGuardrails()) != 2 {
		t.Fatalf("expected 2 guardrails, got %v", profile.GetGuardrails())
	}
	if !profile.GetWebSearchEnabled() {
		t.Fatal("expected web_search_enabled to be true")
	}

	fetched, err := GetActiveBotProfileByChannel(t.Context(), tenant.GetXId(), "web-widget")
	if err != nil {
		t.Fatalf("GetActiveBotProfileByChannel: %v", err)
	}
	if fetched.GetVisibility() != "external" || len(fetched.GetGuardrails()) != 2 || !fetched.GetWebSearchEnabled() {
		t.Fatalf("guardrails/visibility/web_search_enabled didn't round-trip: %+v", fetched)
	}
}

func TestCreateBotProfilePersistsLlmProviderAndModel(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "LLM Provider Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	profile, err := CreateBotProfile(t.Context(), tenant.GetXId(), "external", "persona text",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false,
		"openai", "gpt-4o-mini")
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}
	if profile.GetLlmProvider() != "openai" || profile.GetLlmModel() != "gpt-4o-mini" {
		t.Fatalf("got llm_provider=%q llm_model=%q", profile.GetLlmProvider(), profile.GetLlmModel())
	}

	fetched, err := GetActiveBotProfileByChannel(t.Context(), tenant.GetXId(), "web-widget")
	if err != nil {
		t.Fatalf("GetActiveBotProfileByChannel: %v", err)
	}
	if fetched.GetLlmProvider() != "openai" || fetched.GetLlmModel() != "gpt-4o-mini" {
		t.Fatalf("llm_provider/llm_model didn't round-trip: %+v", fetched)
	}
}

func TestCreateBotProfileDefaultsLlmProviderAndModelToEmpty(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "LLM Default Test Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	profile, err := CreateBotProfile(t.Context(), tenant.GetXId(), "internal", "persona text",
		nil, []string{"slack"}, []databasev1.Role{databasev1.Role_ROLE_STAFF}, "internal", nil, false, "", "")
	if err != nil {
		t.Fatalf("CreateBotProfile: %v", err)
	}
	if profile.GetLlmProvider() != "" || profile.GetLlmModel() != "" {
		t.Fatalf("expected empty llm_provider/llm_model by default, got %q/%q", profile.GetLlmProvider(), profile.GetLlmModel())
	}
}

func TestListBotProfiles(t *testing.T) {
	tenant, err := CreateTenant(t.Context(), "List Bot Profiles Co", "business")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := CreateBotProfile(t.Context(), tenant.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false, "", ""); err != nil {
		t.Fatalf("CreateBotProfile external: %v", err)
	}
	if _, err := CreateBotProfile(t.Context(), tenant.GetXId(), "internal", "personas/internal.md",
		nil, []string{"slack"}, []databasev1.Role{databasev1.Role_ROLE_STAFF}, "internal", nil, false, "", ""); err != nil {
		t.Fatalf("CreateBotProfile internal: %v", err)
	}

	profiles, err := ListBotProfiles(t.Context(), tenant.GetXId())
	if err != nil {
		t.Fatalf("ListBotProfiles: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
}

func TestListBotProfilesIsolatedPerTenant(t *testing.T) {
	tenantA, err := CreateTenant(t.Context(), "List Profiles Tenant A", "business")
	if err != nil {
		t.Fatalf("CreateTenant A: %v", err)
	}
	tenantB, err := CreateTenant(t.Context(), "List Profiles Tenant B", "business")
	if err != nil {
		t.Fatalf("CreateTenant B: %v", err)
	}
	if _, err := CreateBotProfile(t.Context(), tenantA.GetXId(), "external", "personas/external.md",
		nil, []string{"web-widget"}, []databasev1.Role{databasev1.Role_ROLE_CUSTOMER}, "external", nil, false, "", ""); err != nil {
		t.Fatalf("CreateBotProfile A: %v", err)
	}

	profilesB, err := ListBotProfiles(t.Context(), tenantB.GetXId())
	if err != nil {
		t.Fatalf("ListBotProfiles B: %v", err)
	}
	if len(profilesB) != 0 {
		t.Fatalf("expected tenant B to see no profiles, got %+v", profilesB)
	}
}
