// Package migrate is Mongo's stand-in for Alembic-style migrations: MongoDB is
// schemaless, so there's nothing to migrate structurally. Instead this ensures
// every collection's indexes exist — idempotent, safe to run on every startup.
package migrate

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureIndexes creates every index this service group depends on, if missing.
// Add one block per collection as new data-access domains are built (PLAN.md Phase 8).
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	tickets := db.Collection("tickets")
	_, err := tickets.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Every query is tenant-scoped first (CLAUDE.md: multi-tenant by default).
			Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "status", Value: 1}},
		},
		{
			Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "idempotency_key", Value: 1}},
			Options: options.Index().SetSparse(true).SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("ensure tickets indexes: %w", err)
	}
	return nil
}
