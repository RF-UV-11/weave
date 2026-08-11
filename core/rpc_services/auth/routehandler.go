package auth

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sharedauth "weave/packages/shared-auth"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	databasev1 "weave/core/gen/database/v1"
	"weave/core/mongodb"
)

func (s *Server) Register(ctx context.Context, req *dataaccessv1.RegisterRequest) (*dataaccessv1.RegisterResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}
	if len(req.GetPassword()) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}
	if roleToString(req.GetRole()) == "" {
		return nil, status.Error(codes.InvalidArgument, "role must be one of owner, admin, staff, customer")
	}

	if _, err := mongodb.GetUserByEmail(ctx, req.GetTenantId(), req.GetEmail()); err == nil {
		return nil, status.Error(codes.AlreadyExists, "a user with this email already exists for this tenant")
	}

	u, err := mongodb.CreateUser(ctx, req.GetTenantId(), req.GetEmail(), req.GetPassword(), req.GetRole())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.RegisterResponse{User: redactPasswordHash(u)}, nil
}

func (s *Server) Login(ctx context.Context, req *dataaccessv1.LoginRequest) (*dataaccessv1.LoginResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if req.GetEmail() == "" || req.GetPassword() == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	u, err := mongodb.GetUserByEmail(ctx, req.GetTenantId(), req.GetEmail())
	if err != nil || !mongodb.VerifyPassword(u, req.GetPassword()) {
		// Same error for "no such user" and "wrong password" — don't leak
		// which one it was.
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	role := roleToString(u.GetRole())
	access, _, err := sharedauth.IssueAccessToken(s.jwtSecret, req.GetTenantId(), u.GetXId(), role)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	refresh, _, err := sharedauth.IssueRefreshToken(s.jwtSecret, req.GetTenantId(), u.GetXId(), role)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &dataaccessv1.LoginResponse{AccessToken: access, RefreshToken: refresh, User: redactPasswordHash(u)}, nil
}

// redactPasswordHash clears the bcrypt hash before a User crosses the RPC
// boundary — not a plaintext secret, but internal hashing detail a caller
// never needs and shouldn't see reflected back at them.
func redactPasswordHash(u *databasev1.User) *databasev1.User {
	u.PasswordHash = ""
	return u
}

func (s *Server) Refresh(ctx context.Context, req *dataaccessv1.RefreshRequest) (*dataaccessv1.RefreshResponse, error) {
	if req.GetRefreshToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}

	claims, err := sharedauth.VerifyRefreshToken(s.jwtSecret, req.GetRefreshToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	access, _, err := sharedauth.IssueAccessToken(s.jwtSecret, claims.TenantID, claims.UserID, claims.Role)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	newRefresh, _, err := sharedauth.IssueRefreshToken(s.jwtSecret, claims.TenantID, claims.UserID, claims.Role)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &dataaccessv1.RefreshResponse{AccessToken: access, RefreshToken: newRefresh}, nil
}
