package botprofile

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	sharedauth "weave/packages/shared-auth"
)

// CreateBotProfile requires an authenticated owner/admin of the target
// tenant — a bot profile controls which connectors/roles/channels are
// reachable, so creating one is a privileged operation, unlike
// Tenant/Connector's current (pre-auth-lockdown) RPCs.
func (s *Server) CreateBotProfile(ctx context.Context, req *dataaccessv1.CreateBotProfileRequest) (*dataaccessv1.CreateBotProfileResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if err := sharedauth.RequireRole(ctx, "owner", "admin"); err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if len(req.GetChannels()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one channel is required")
	}
	visibility := req.GetVisibility()
	if visibility == "" {
		visibility = "internal"
	}
	if visibility != "internal" && visibility != "external" {
		return nil, status.Error(codes.InvalidArgument, "visibility must be \"internal\" or \"external\"")
	}

	p, err := mongodb.CreateBotProfile(ctx, req.GetTenantId(), req.GetName(), req.GetPersona(),
		req.GetConnectorIds(), req.GetChannels(), req.GetRolesAllowed(), visibility, req.GetGuardrails())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.CreateBotProfileResponse{BotProfile: p}, nil
}

// GetActiveBotProfile resolves which bot profile serves a given channel for
// a tenant. Requires an authenticated caller scoped to that tenant — this
// is the RPC orchestrator's ChatSvc calls per turn (docs/architecture/
// ARCHITECTURE.md §2), so it will eventually be called with a
// service-identity token rather than an end-user one; that's Phase 3 scope.
func (s *Server) GetActiveBotProfile(ctx context.Context, req *dataaccessv1.GetActiveBotProfileRequest) (*dataaccessv1.GetActiveBotProfileResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetChannel() == "" {
		return nil, status.Error(codes.InvalidArgument, "channel is required")
	}

	p, err := mongodb.GetActiveBotProfileByChannel(ctx, req.GetTenantId(), req.GetChannel())
	if err != nil {
		return nil, status.Error(codes.NotFound, "no bot profile is active for this tenant on this channel")
	}
	return &dataaccessv1.GetActiveBotProfileResponse{BotProfile: p}, nil
}
