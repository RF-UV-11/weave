package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Mongo is schemaless, so this is our "migrations": an idempotent bootstrap
// that ensures every index exists on startup.
func EnsureIndexes(ctx context.Context) error {
	_, err := Db.Db.Collection(ColNames.Tenants).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "_id", Value: 1}},
	})
	return err
}
