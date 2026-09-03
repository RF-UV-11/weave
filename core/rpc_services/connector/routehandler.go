package connector

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mcpclient"
	"weave/core/mongodb"
	"weave/core/netguard"
)

func (s *Server) RegisterConnector(ctx context.Context, req *dataaccessv1.RegisterConnectorRequest) (*dataaccessv1.RegisterConnectorResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	transport := req.GetTransport()
	if transport != "http" && transport != "stdio" {
		return nil, status.Error(codes.InvalidArgument, "transport must be \"http\" or \"stdio\"")
	}
	if req.GetEndpoint() == "" {
		return nil, status.Error(codes.InvalidArgument, "endpoint is required")
	}
	if transport == "http" {
		// SSRF guard: a tenant-supplied endpoint must not resolve to
		// internal/private infrastructure — core (and, transitively,
		// orchestrator/mcp-gateway) will make real outbound requests to
		// it. See core/netguard for what this blocks and why.
		if err := netguard.ValidatePublicURL(ctx, s.resolver, req.GetEndpoint(), s.allowPrivate); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	c, err := mongodb.CreateConnector(ctx, req.GetTenantId(), req.GetName(), transport, req.GetEndpoint(), "")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if secret := req.GetCredentialSecret(); secret != "" {
		cred, err := mongodb.CreateCredential(ctx, s.vault, req.GetTenantId(), c.GetXId(), secret)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		c, err = mongodb.SetConnectorCredentialRef(ctx, req.GetTenantId(), c.GetXId(), cred.GetXId())
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &dataaccessv1.RegisterConnectorResponse{Connector: c}, nil
}

func (s *Server) ListConnectors(ctx context.Context, req *dataaccessv1.ListConnectorsRequest) (*dataaccessv1.ListConnectorsResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	limit := int32(0)
	if req.GetPage() != nil {
		limit = req.GetPage().GetPageSize()
	}
	connectors, err := mongodb.ListConnectors(ctx, req.GetTenantId(), limit)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.ListConnectorsResponse{Connectors: connectors}, nil
}

func (s *Server) RefreshManifest(ctx context.Context, req *dataaccessv1.RefreshManifestRequest) (*dataaccessv1.RefreshManifestResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetConnectorId() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_id is required")
	}

	c, err := mongodb.GetConnector(ctx, req.GetTenantId(), req.GetConnectorId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "connector not found")
	}
	if c.GetTransport() != "http" {
		return nil, status.Error(codes.Unimplemented, "manifest refresh is only implemented for http-transport connectors")
	}

	manifest, err := mcpclient.ListTools(ctx, c.GetEndpoint())
	if err != nil {
		if _, updateErr := mongodb.UpdateConnectorManifest(ctx, req.GetTenantId(), req.GetConnectorId(), "", "error"); updateErr != nil {
			return nil, status.Error(codes.Internal, updateErr.Error())
		}
		var missingDesc *mcpclient.ErrMissingDescription
		if errors.As(err, &missingDesc) {
			// Every tool a connector exposes must carry a description —
			// docs/architecture/ARCHITECTURE.md §3 — so this is a
			// registration-time contract violation on the connector's
			// side, not a transient availability problem.
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	updated, err := mongodb.UpdateConnectorManifest(ctx, req.GetTenantId(), req.GetConnectorId(), string(manifest), "active")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.RefreshManifestResponse{Connector: updated}, nil
}

func (s *Server) DeregisterConnector(ctx context.Context, req *dataaccessv1.DeregisterConnectorRequest) (*dataaccessv1.DeregisterConnectorResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetConnectorId() == "" {
		return nil, status.Error(codes.InvalidArgument, "connector_id is required")
	}

	c, err := mongodb.GetConnector(ctx, req.GetTenantId(), req.GetConnectorId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "connector not found")
	}
	if c.GetCredentialRefId() != "" {
		if err := mongodb.DeleteCredential(ctx, req.GetTenantId(), c.GetCredentialRefId()); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if err := mongodb.DeleteConnector(ctx, req.GetTenantId(), req.GetConnectorId()); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.DeregisterConnectorResponse{}, nil
}
