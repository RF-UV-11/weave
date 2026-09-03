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

	// An email only has to be unique per tenant, not globally — the same
	// person could be a user of two unrelated tenants.
	if _, err := Db.Db.Collection(ColNames.Users).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	// A bot profile's name is unique per tenant (e.g. "external"/"internal").
	if _, err := Db.Db.Collection(ColNames.BotProfiles).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	// GetActiveBotProfileByChannel filters tenant_id + channels membership.
	if _, err := Db.Db.Collection(ColNames.BotProfiles).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "channels", Value: 1}},
	}); err != nil {
		return err
	}

	// An HTTP tool's name is unique per tenant, same reasoning as connectors.
	if _, err := Db.Db.Collection(ColNames.HttpTools).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	// A session's messages are always fetched by (tenant_id, session_id)
	// together, then ordered by creation time.
	if _, err := Db.Db.Collection(ColNames.ChatSessions).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "_id", Value: 1}},
	}); err != nil {
		return err
	}
	if _, err := Db.Db.Collection(ColNames.ChatMessages).Indexes().CreateOne(ctx, mongo.IndexModel{
		// _id (a ULID, so also creation-ordered), not created_at — see
		// mongodb/chat.go's GetSessionMessages for why.
		Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "session_id", Value: 1}, {Key: "_id", Value: 1}},
	}); err != nil {
		return err
	}

	return nil
}
