package configs

import "os"

type Vars struct {
	MongoURI     string
	MongoDBName  string
	GRPCAddr     string
	VaultRootKey string
}

func Load() Vars {
	return Vars{
		MongoURI:     getenv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDBName:  getenv("MONGO_DB_NAME", "weave"),
		GRPCAddr:     getenv("GRPC_ADDR", ":9090"),
		VaultRootKey: getenv("VAULT_ROOT_KEY", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
