package qdrant

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"
)

var testClient *Client

const testEmbeddingDim = 8

func TestMain(m *testing.M) {
	addr := os.Getenv("QDRANT_ADDR")
	if addr == "" {
		addr = "localhost:6334"
	}
	c, err := New(addr, testEmbeddingDim)
	if err != nil {
		fmt.Printf("qdrant: skipping integration tests, can't construct client for %s: %v\n", addr, err)
		os.Exit(0)
	}
	// New() doesn't dial eagerly, so probe with a real call before
	// deciding whether Qdrant is actually reachable.
	if _, err := c.Search(context.Background(), "tnt_probe_"+randSuffix(), "usr_probe", make([]float32, testEmbeddingDim), 1); err != nil {
		fmt.Printf("qdrant: skipping integration tests, no Qdrant reachable at %s: %v\n", addr, err)
		os.Exit(0)
	}
	testClient = c
	os.Exit(m.Run())
}

func randSuffix() string {
	return fmt.Sprintf("%d", rand.Int63())
}

// oneHot returns a unit vector with a 1.0 in position i and 0 elsewhere
// — orthogonal vectors (cosine similarity 0) make "near" vs "far"
// unambiguous, unlike scaled-but-parallel vectors which would score
// almost identically under cosine distance regardless of magnitude.
func oneHot(i int) []float32 {
	v := make([]float32, testEmbeddingDim)
	v[i] = 1.0
	return v
}

func TestUpsertAndSearchFindsClosestMatch(t *testing.T) {
	tenantID := "tnt_qdrant_test_" + randSuffix()
	query := oneHot(0)

	nearID, err := testClient.Upsert(t.Context(), tenantID, "usr_1", "user's name is Jordan", query)
	if err != nil {
		t.Fatalf("Upsert near: %v", err)
	}
	if _, err := testClient.Upsert(t.Context(), tenantID, "usr_1", "completely unrelated fact", oneHot(7)); err != nil {
		t.Fatalf("Upsert far: %v", err)
	}

	results, err := testClient.Search(t.Context(), tenantID, "usr_1", query, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].MemoryID != nearID {
		t.Fatalf("expected closest match to be the near memory %q, got %q (text=%q)", nearID, results[0].MemoryID, results[0].Text)
	}
	if results[0].Text != "user's name is Jordan" {
		t.Fatalf("unexpected text: %q", results[0].Text)
	}
}

func TestSearchIsolatedPerUserWithinTenant(t *testing.T) {
	tenantID := "tnt_qdrant_test_" + randSuffix()

	if _, err := testClient.Upsert(t.Context(), tenantID, "usr_a", "user A's secret", oneHot(0)); err != nil {
		t.Fatalf("Upsert for user A: %v", err)
	}

	results, err := testClient.Search(t.Context(), tenantID, "usr_b", oneHot(0), 5)
	if err != nil {
		t.Fatalf("Search as user B: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected user B to see none of user A's memories, got %+v", results)
	}
}

func TestSearchReturnsEmptyForTenantWithNoMemoriesYet(t *testing.T) {
	tenantID := "tnt_qdrant_test_never_written_" + randSuffix()

	results, err := testClient.Search(t.Context(), tenantID, "usr_1", oneHot(0), 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results for a tenant with no collection yet, got %+v", results)
	}
}

func TestUpsertRejectsWrongEmbeddingDimension(t *testing.T) {
	tenantID := "tnt_qdrant_test_" + randSuffix()

	_, err := testClient.Upsert(t.Context(), tenantID, "usr_1", "bad dims", []float32{1.0, 2.0})
	if err == nil {
		t.Fatal("expected an error for an embedding with the wrong dimension")
	}
}
