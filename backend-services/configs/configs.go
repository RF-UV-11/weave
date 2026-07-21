// Package configs is the single place backend-services reads its environment.
// Every var lives here, with a default, so `.env.example` and this struct stay
// in sync — see CLAUDE.md.
package configs

import "os"

var Vars struct {
	ServerAddr   string
	MongoConnUri string
	MongoDbName  string
}

func InitializeConfig() {
	Vars.ServerAddr = getenv("BACKEND_SERVICES_ADDR", ":8081")
	Vars.MongoConnUri = getenv("MONGO_URI", "mongodb://localhost:27017")
	Vars.MongoDbName = getenv("MONGO_DB_NAME", "servicesphere")

	if Vars.MongoConnUri == "" {
		panic("MONGO_URI not configured")
	}
	if Vars.MongoDbName == "" {
		panic("MONGO_DB_NAME not configured")
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
