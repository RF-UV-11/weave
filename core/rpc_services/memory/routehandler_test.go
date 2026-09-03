package memory

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
	"weave/core/qdrant"
	sharedauth "weave/packages/shared-auth"
)

var testSecret = []byte("test-jwt-secret-not-for-prod")

const testEmbeddingDim = 8

func TestMain(m *testing.M) {
	addr := os.Getenv("QDRANT_ADDR")
	if addr == "" {
		addr = "localhost:6334"
	}
	q, err := qdrant.New(addr, testEmbeddingDim)
	if err != nil {
		fmt.Printf("memory: skipping integration tests, can't construct client for %s: %v\n", addr, err)
		os.Exit(0)
	}
	if _, err := q.Search(context.Background(), "tnt_probe", "usr_probe", make([]float32, testEmbeddingDim), 1); err != nil {
		fmt.Printf("memory: skipping integration tests, no Qdrant reachable at %s: %v\n", addr, err)
		os.Exit(0)
	}
	testQdrant = q
	os.Exit(m.Run())
}

var testQdrant *qdrant.Client

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

func oneHot(i int) []float32 {
	v := make([]float32, testEmbeddingDim)
	v[i] = 1.0
	return v
}

func TestUpsertMemoryRequiresAuth(t *testing.T) {
	s := NewServer(testQdrant)
	_, err := s.UpsertMemory(t.Context(), &dataaccessv1.UpsertMemoryRequest{
		TenantId: "tnt_1", UserId: "usr_1", Text: "hi", Embedding: oneHot(0),
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated, got %v", err)
	}
}

func TestUpsertMemoryRejectsWrongTenantToken(t *testing.T) {
	s := NewServer(testQdrant)
	_, err := callAs(t, "tnt_other", "customer", func(ctx context.Context, req any) (any, error) {
		return s.UpsertMemory(ctx, &dataaccessv1.UpsertMemoryRequest{
			TenantId: "tnt_1", UserId: "usr_1", Text: "hi", Embedding: oneHot(0),
		})
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", err)
	}
}

func TestUpsertThenSearchMemoryRoundTrip(t *testing.T) {
	s := NewServer(testQdrant)
	tenantID := "tnt_memory_rpc_test_1"

	_, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.UpsertMemory(ctx, &dataaccessv1.UpsertMemoryRequest{
			TenantId: tenantID, UserId: "usr_1", Text: "user's name is Jordan", Embedding: oneHot(0),
		})
	})
	if err != nil {
		t.Fatalf("UpsertMemory: %v", err)
	}

	resp, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.SearchMemory(ctx, &dataaccessv1.SearchMemoryRequest{
			TenantId: tenantID, UserId: "usr_1", QueryEmbedding: oneHot(0),
		})
	})
	if err != nil {
		t.Fatalf("SearchMemory: %v", err)
	}
	results := resp.(*dataaccessv1.SearchMemoryResponse).GetResults()
	if len(results) == 0 || results[0].GetText() != "user's name is Jordan" {
		t.Fatalf("expected to find the upserted memory, got %+v", results)
	}
}

func TestUpsertMemoryRejectsMissingText(t *testing.T) {
	s := NewServer(testQdrant)
	tenantID := "tnt_memory_rpc_test_2"

	_, err := callAs(t, tenantID, "customer", func(ctx context.Context, req any) (any, error) {
		return s.UpsertMemory(ctx, &dataaccessv1.UpsertMemoryRequest{
			TenantId: tenantID, UserId: "usr_1", Embedding: oneHot(0),
		})
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}
