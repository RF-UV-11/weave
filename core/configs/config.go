package configs

import (
	"os"
	"strconv"
)

type Vars struct {
	MongoURI              string
	MongoDBName           string
	GRPCAddr              string
	VaultRootKey          string
	JWTSecret             string
	RedisURI              string
	MCPGatewayURL         string
	AllowPrivateEndpoints bool
	QdrantAddr            string
	EmbeddingDim          int
}

func Load() Vars {
	return Vars{
		MongoURI:      getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName:   getenv("MONGO_DB_NAME", "weave"),
		GRPCAddr:      getenv("GRPC_ADDR", ":9090"),
		VaultRootKey:  getenv("VAULT_ROOT_KEY", ""),
		JWTSecret:     getenv("JWT_SECRET", ""),
		RedisURI:      getenv("REDIS_URI", "redis://localhost:6379"),
		MCPGatewayURL: getenv("MCP_GATEWAY_URL", "http://localhost:8766"),
		// Secure by default — must be explicitly opted into for local dev
		// (connectors/reference-mcp, httptest fixtures, etc. run on
		// loopback). Never set this in a real deployment; see
		// core/netguard's doc comment for what it disables.
		AllowPrivateEndpoints: getenv("ALLOW_PRIVATE_ENDPOINTS", "") == "true",
		QdrantAddr:            getenv("QDRANT_ADDR", "localhost:6334"),
		// Must match whatever embedding model orchestrator actually calls
		// (llm/ollama_client.py's EMBED_MODEL) — Qdrant collections are
		// created with a fixed vector size, so a mismatch fails loudly at
		// upsert time rather than silently truncating/padding.
		// nomic-embed-text (the default) is 768-dimensional.
		EmbeddingDim: getenvInt("EMBEDDING_DIM", 768),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
