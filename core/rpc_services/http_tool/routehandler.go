package httptool

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	"weave/core/netguard"
	sharedauth "weave/packages/shared-auth"
)

var allowedHTTPMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

// RegisterHttpTool requires an authenticated owner/admin of the target
// tenant — same privilege level as CreateBotProfile, since registering a
// tool shapes what the bot can do, same as a bot profile does.
func (s *Server) RegisterHttpTool(ctx context.Context, req *dataaccessv1.RegisterHttpToolRequest) (*dataaccessv1.RegisterHttpToolResponse, error) {
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
	if strings.TrimSpace(req.GetDescription()) == "" {
		return nil, status.Error(codes.InvalidArgument, "description is required")
	}
	if req.GetHttpEndpoint() == "" {
		return nil, status.Error(codes.InvalidArgument, "http_endpoint is required")
	}
	method := strings.ToUpper(req.GetHttpMethod())
	if !allowedHTTPMethods[method] {
		return nil, status.Error(codes.InvalidArgument, "http_method must be one of GET, POST, PUT, PATCH, DELETE")
	}
	visibility := req.GetVisibility()
	if visibility == "" {
		visibility = "internal"
	}
	if visibility != "internal" && visibility != "external" {
		return nil, status.Error(codes.InvalidArgument, "visibility must be \"internal\" or \"external\"")
	}
	category := req.GetCategory()
	if category == "" {
		category = "general"
	}
	if category != "general" && category != "analytics" {
		return nil, status.Error(codes.InvalidArgument, "category must be \"general\" or \"analytics\"")
	}
	// SSRF guard — mcp-gateway will make a real outbound request to this
	// endpoint on the tenant's behalf every time the tool is called; see
	// core/netguard for what this blocks and why.
	if err := netguard.ValidatePublicURL(ctx, s.resolver, req.GetHttpEndpoint(), s.allowPrivate); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	connector, err := mongodb.GetOrCreateManagedConnector(ctx, req.GetTenantId(), s.gatewayBaseURL)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	credentialRefID := ""
	if secret := req.GetCredentialSecret(); secret != "" {
		cred, err := mongodb.CreateCredential(ctx, s.vault, req.GetTenantId(), connector.GetXId(), secret)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		credentialRefID = cred.GetXId()
	}

	tool, err := mongodb.CreateHttpTool(ctx, req.GetTenantId(), connector.GetXId(), req.GetName(), req.GetDescription(),
		req.GetHttpEndpoint(), method, req.GetParamsSchema(), credentialRefID, visibility, category)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.RegisterHttpToolResponse{HttpTool: tool}, nil
}

// ListHttpTools is unauthenticated for now, same known gap as
// ConnectorService's ListConnectors (see main.go's unauthenticatedMethods
// comment) — mcp-gateway calls this per tools/list request and has no
// per-end-user JWT to present.
func (s *Server) ListHttpTools(ctx context.Context, req *dataaccessv1.ListHttpToolsRequest) (*dataaccessv1.ListHttpToolsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	tools, err := mongodb.ListHttpTools(ctx, req.GetTenantId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.ListHttpToolsResponse{HttpTools: tools}, nil
}

func (s *Server) DeregisterHttpTool(ctx context.Context, req *dataaccessv1.DeregisterHttpToolRequest) (*dataaccessv1.DeregisterHttpToolResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if err := sharedauth.RequireRole(ctx, "owner", "admin"); err != nil {
		return nil, err
	}
	if req.GetHttpToolId() == "" {
		return nil, status.Error(codes.InvalidArgument, "http_tool_id is required")
	}

	tool, err := mongodb.GetHttpTool(ctx, req.GetTenantId(), req.GetHttpToolId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "http tool not found")
	}
	if tool.GetCredentialRefId() != "" {
		if err := mongodb.DeleteCredential(ctx, req.GetTenantId(), tool.GetCredentialRefId()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if err := mongodb.DeleteHttpTool(ctx, req.GetTenantId(), req.GetHttpToolId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.DeregisterHttpToolResponse{}, nil
}

func (s *Server) RevealHttpToolCredential(ctx context.Context, req *dataaccessv1.RevealHttpToolCredentialRequest) (*dataaccessv1.RevealHttpToolCredentialResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetHttpToolId() == "" {
		return nil, status.Error(codes.InvalidArgument, "http_tool_id is required")
	}

	tool, err := mongodb.GetHttpTool(ctx, req.GetTenantId(), req.GetHttpToolId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "http tool not found")
	}
	if tool.GetCredentialRefId() == "" {
		return &dataaccessv1.RevealHttpToolCredentialResponse{}, nil
	}

	cred, err := mongodb.GetCredential(ctx, req.GetTenantId(), tool.GetCredentialRefId())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	secret, err := mongodb.OpenCredential(s.vault, cred)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.RevealHttpToolCredentialResponse{Secret: secret}, nil
}
