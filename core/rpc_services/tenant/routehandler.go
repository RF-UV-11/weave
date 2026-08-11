package tenant

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
)

func (s *Server) CreateTenant(ctx context.Context, req *dataaccessv1.CreateTenantRequest) (*dataaccessv1.CreateTenantResponse, error) {
	if req.GetDisplayName() == "" {
		return nil, status.Error(codes.InvalidArgument, "display_name is required")
	}
	tenantType := req.GetTenantType()
	if tenantType != "business" && tenantType != "individual" {
		return nil, status.Error(codes.InvalidArgument, "tenant_type must be \"business\" or \"individual\"")
	}

	t, err := mongodb.CreateTenant(ctx, req.GetDisplayName(), tenantType)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.CreateTenantResponse{Tenant: t}, nil
}

func (s *Server) GetTenant(ctx context.Context, req *dataaccessv1.GetTenantRequest) (*dataaccessv1.GetTenantResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	t, err := mongodb.GetTenant(ctx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}
	return &dataaccessv1.GetTenantResponse{Tenant: t}, nil
}

func (s *Server) ListTenants(ctx context.Context, req *dataaccessv1.ListTenantsRequest) (*dataaccessv1.ListTenantsResponse, error) {
	limit := int32(0)
	if req.GetPage() != nil {
		limit = req.GetPage().GetPageSize()
	}
	tenants, err := mongodb.ListTenants(ctx, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.ListTenantsResponse{Tenants: tenants}, nil
}
