package memory

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	dataaccessv1 "weave/core/gen/core/data_access/v1"
	sharedauth "weave/packages/shared-auth"
)

const defaultTopK = 5

func (s *Server) UpsertMemory(ctx context.Context, req *dataaccessv1.UpsertMemoryRequest) (*dataaccessv1.UpsertMemoryResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if req.GetText() == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}
	if len(req.GetEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "embedding is required")
	}

	id, err := s.qdrant.Upsert(ctx, req.GetTenantId(), req.GetUserId(), req.GetText(), req.GetEmbedding())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dataaccessv1.UpsertMemoryResponse{MemoryId: id}, nil
}

func (s *Server) SearchMemory(ctx context.Context, req *dataaccessv1.SearchMemoryRequest) (*dataaccessv1.SearchMemoryResponse, error) {
	if req.GetTenantId() == "" {
		return nil, status.Error(codes.InvalidArgument, "tenant_id is required")
	}
	if err := sharedauth.RequireTenant(ctx, req.GetTenantId()); err != nil {
		return nil, err
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}
	if len(req.GetQueryEmbedding()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "query_embedding is required")
	}
	topK := int(req.GetTopK())
	if topK <= 0 {
		topK = defaultTopK
	}

	results, err := s.qdrant.Search(ctx, req.GetTenantId(), req.GetUserId(), req.GetQueryEmbedding(), topK)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbResults := make([]*dataaccessv1.MemoryResult, 0, len(results))
	for _, r := range results {
		pbResults = append(pbResults, &dataaccessv1.MemoryResult{MemoryId: r.MemoryID, Text: r.Text, Score: r.Score})
	}
	return &dataaccessv1.SearchMemoryResponse{Results: pbResults}, nil
}
