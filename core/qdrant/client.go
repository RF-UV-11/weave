// Package qdrant wraps the Qdrant Go client for long-term/semantic
// memory (docs/architecture/ARCHITECTURE.md §5). core is the only tier
// holding this connection — orchestrator computes embeddings (it holds
// LLM/embedding-model access) and hands the vector to core's
// MemoryService, same "core is the only tier with real infra
// credentials" rule as everywhere else (docs/architecture/SECURITY.md §1).
//
// One collection per tenant (SECURITY.md §2's isolation decision, same
// as RAG): stronger isolation than a shared collection with a filter, at
// the cost of more collections to manage — a deliberate tradeoff given
// what gets embedded (a user's own words) is inherently sensitive.
package qdrant

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/google/uuid"
	qc "github.com/qdrant/go-client/qdrant"
)

type Client struct {
	c            *qc.Client
	embeddingDim uint64
}

func New(addr string, embeddingDim int) (*Client, error) {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return nil, err
	}
	c, err := qc.NewClient(&qc.Config{Host: host, Port: port})
	if err != nil {
		return nil, err
	}
	return &Client{c: c, embeddingDim: uint64(embeddingDim)}, nil
}

func (cl *Client) collectionName(tenantID string) string {
	return "mem_" + tenantID
}

// ensureCollection is called before every write — cheap (a single
// existence check) and means no separate migration/bootstrap step is
// needed when a tenant's first memory is ever written.
func (cl *Client) ensureCollection(ctx context.Context, tenantID string) error {
	name := cl.collectionName(tenantID)
	exists, err := cl.c.CollectionExists(ctx, name)
	if err != nil {
		return fmt.Errorf("qdrant: checking collection %q: %w", name, err)
	}
	if exists {
		return nil
	}
	return cl.c.CreateCollection(ctx, &qc.CreateCollection{
		CollectionName: name,
		VectorsConfig: qc.NewVectorsConfig(&qc.VectorParams{
			Size:     cl.embeddingDim,
			Distance: qc.Distance_Cosine,
		}),
	})
}

// Upsert stores one memory (text + its embedding), scoped to tenantID's
// own collection and tagged with userID in the payload so SearchMemory
// can filter to one user's own memories within a shared-tenant
// collection. Returns the new memory's id.
//
// Qdrant point ids must be a uint64 or a UUID (not an arbitrary string,
// unlike every other _id in this codebase's ULID convention) — a real
// constraint of the store, not a stylistic choice, so this generates a
// UUID rather than reusing newULID()-style ids from mongodb/.
func (cl *Client) Upsert(ctx context.Context, tenantID, userID, text string, embedding []float32) (string, error) {
	if err := cl.ensureCollection(ctx, tenantID); err != nil {
		return "", err
	}
	if len(embedding) != int(cl.embeddingDim) {
		return "", fmt.Errorf("qdrant: embedding has %d dimensions, collection expects %d", len(embedding), cl.embeddingDim)
	}

	id := uuid.NewString()
	_, err := cl.c.Upsert(ctx, &qc.UpsertPoints{
		CollectionName: cl.collectionName(tenantID),
		Points: []*qc.PointStruct{
			{
				Id:      qc.NewID(id),
				Vectors: qc.NewVectors(embedding...),
				Payload: qc.NewValueMap(map[string]any{"user_id": userID, "text": text}),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("qdrant: upsert: %w", err)
	}
	return id, nil
}

type SearchResult struct {
	MemoryID string
	Text     string
	Score    float32
}

// Search returns the topK memories in tenantID's collection closest to
// queryEmbedding, filtered to userID's own memories — one tenant's
// collection can (and will) hold multiple users' memories, and a user's
// long-term memory should never surface another user's facts even
// within the same tenant.
func (cl *Client) Search(ctx context.Context, tenantID, userID string, queryEmbedding []float32, topK int) ([]SearchResult, error) {
	name := cl.collectionName(tenantID)
	exists, err := cl.c.CollectionExists(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("qdrant: checking collection %q: %w", name, err)
	}
	if !exists {
		// No memory has ever been written for this tenant yet — a normal
		// state (e.g. a brand-new tenant's first conversation), not an error.
		return nil, nil
	}

	points, err := cl.c.Query(ctx, &qc.QueryPoints{
		CollectionName: name,
		Query:          qc.NewQuery(queryEmbedding...),
		Filter: &qc.Filter{
			Must: []*qc.Condition{qc.NewMatch("user_id", userID)},
		},
		Limit:       qc.PtrOf(uint64(topK)),
		WithPayload: qc.NewWithPayload(true),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant: query: %w", err)
	}

	results := make([]SearchResult, 0, len(points))
	for _, p := range points {
		text := ""
		if v, ok := p.GetPayload()["text"]; ok {
			text = v.GetStringValue()
		}
		results = append(results, SearchResult{
			MemoryID: p.GetId().GetUuid(),
			Text:     text,
			Score:    p.GetScore(),
		})
	}
	return results, nil
}

func splitHostPort(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("qdrant: invalid address %q: %w", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("qdrant: invalid port in address %q: %w", addr, err)
	}
	return host, port, nil
}
