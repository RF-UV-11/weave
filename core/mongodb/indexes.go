package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Mongo is schemaless, so this is our "migrations": an idempotent bootstrap
// that ensures every index exists on startup.
func EnsureIndexes(ctx context.Context) error {
	if _, err := Db.Db.Collection(ColNames.Tenants).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "_id", Value: 1}},
	}); err != nil {
		return err
	}

	// A connector's name is unique per tenant, not globally — two tenants
	// may register a connector with the same name without colliding.
	if _, err := Db.Db.Collection(ColNames.Connectors).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	if _, err := Db.Db.Collection(ColNames.Credentials).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "connector_id", Value: 1}},
	}); err != nil {
		return err
	}

	return nil
}
