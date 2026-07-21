package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ensureIndexes creates every index this service depends on, if missing.
// MongoDB is schemaless, so this — not a migration tool — is the source of
// truth for indexes; it's idempotent and safe to run on every startup. Add
// one block per collection as new mongodb/<collection>.go files are added.
func ensureIndexes(ctx context.Context, db *DbType) error {
	_, err := db.TicketCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// _id is already unique via Mongo's default index; this compound
			// index is for query performance on ListTickets' tenant-scoped,
			// _id-sorted cursor pagination, not uniqueness.
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
