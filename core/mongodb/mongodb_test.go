package mongodb

import (
	"context"
	"fmt"
	"os"
	"testing"

	"weave/core/vault"
)

// These are integration tests against a real MongoDB (matching core's own
// "no mock DB" design — mongodb/ is the only tier allowed to hold real Db
// state, so faking it here would test nothing meaningful). They connect to
// MONGO_URI (default mongodb://localhost:27017) against a dedicated
// "weave_core_test_mongodb" database and skip entirely if no Mongo is
// reachable, so `go test ./...` still works without infra running.
//
// Each package under core that runs its own Mongo-backed TestMain uses a
// distinct database name (see rpc_services/connector and
// rpc_services/tenant) — `go test ./...` runs packages concurrently, and
// two TestMains sharing one database means one package's teardown Drop()
// can wipe data another package's tests are still using mid-run.
var mongoAvailable bool

func TestMain(m *testing.M) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	if err := InitDatabase(uri, "weave_core_test_mongodb"); err != nil {
		fmt.Printf("mongodb: skipping integration tests, no Mongo reachable at %s: %v\n", uri, err)
		os.Exit(0)
	}
	mongoAvailable = true

	code := m.Run()

	if err := Db.Db.Drop(context.Background()); err != nil {
		fmt.Printf("mongodb: warning: failed to drop test database: %v\n", err)
	}
	os.Exit(code)
}

func testVault(t *testing.T) *vault.Vault {
	t.Helper()
	v, err := vault.New("5eJ9k0h2xW1qzT7yD3mN8pR4sL6vC9bA0fG2hJ5kM8o=")
	if err != nil {
		t.Fatalf("vault.New: %v", err)
	}
	return v
}
