package chat

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	"weave/core/mongodb"
	sharedauth "weave/packages/shared-auth"
)

var allowedRoles = map[string]bool{"user": true, "assistant": true, "tool": true}

// CreateSession requires an authenticated caller scoped to the target
// tenant — orchestrator calls this once per new conversation, forwarding
// the resolved end user's own JWT (never a service-wide token), same
// pattern as tools/assembly.py's other core calls.
func (s *Server) CreateSession(ctx context.Context, req *dataaccessv1.CreateSessionRequest) (*dataaccessv1.CreateSessionResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetChannel() == "" {
		return nil, status.Error(codes.InvalidArgument, "channel is required")
	}

	sess, err := mongodb.CreateSession(ctx, req.GetTenantId(), req.GetUserId(), req.GetBotProfileId(), req.GetChannel())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.CreateSessionResponse{Session: sess}, nil
}

// AppendMessage requires the session to already exist for this tenant —
// looked up explicitly rather than trusting the caller-supplied
// session_id, so a session_id from tenant A can never be used to write
// into tenant B's conversation (docs/architecture/SECURITY.md §2).
func (s *Server) AppendMessage(ctx context.Context, req *dataaccessv1.AppendMessageRequest) (*dataaccessv1.AppendMessageResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	if !allowedRoles[req.GetRole()] {
		return nil, status.Error(codes.InvalidArgument, "role must be one of user, assistant, tool")
	}

	if _, err := mongodb.GetSession(ctx, req.GetTenantId(), req.GetSessionId()); err != nil {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	msg, err := mongodb.AppendMessage(ctx, req.GetTenantId(), req.GetSessionId(), req.GetRole(), req.GetContent(),
		req.GetToolUsed(), req.GetConnectorUsed())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.AppendMessageResponse{Message: msg}, nil
}

// GetSessionMessages, same session-ownership check as AppendMessage.
func (s *Server) GetSessionMessages(ctx context.Context, req *dataaccessv1.GetSessionMessagesRequest) (*dataaccessv1.GetSessionMessagesResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetSessionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	if _, err := mongodb.GetSession(ctx, req.GetTenantId(), req.GetSessionId()); err != nil {
		return nil, status.Error(codes.NotFound, "session not found")
	}

	messages, err := mongodb.GetSessionMessages(ctx, req.GetTenantId(), req.GetSessionId(), req.GetLimit())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.GetSessionMessagesResponse{Messages: messages}, nil
}
