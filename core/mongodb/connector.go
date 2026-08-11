package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	databasev1 "weave/core/gen/database/v1"
)

func CreateConnector(ctx context.Context, tenantID, name, transport, endpoint, credentialRefID string) (*databasev1.Connector, error) {
	c := &databasev1.Connector{
		XId:             "conn_" + newULID(),
		TenantId:        tenantID,
		Name:            name,
		Transport:       transport,
		Endpoint:        endpoint,
		CredentialRefId: credentialRefID,
		Status:          "pending",
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.Connectors).InsertOne(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// GetConnector looks up a connector scoped to tenantID — a connector
// registered by one tenant is never reachable by resolving ID alone
// (docs/architecture/SECURITY.md §2).
func GetConnector(ctx context.Context, tenantID, id string) (*databasev1.Connector, error) {
	var c databasev1.Connector
	err := Db.Db.Collection(ColNames.Connectors).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func ListConnectors(ctx context.Context, tenantID string, limit int32) ([]*databasev1.Connector, error) {
	if limit <= 0 {
		limit = 50
	}
	findOpts := options.Find().SetLimit(int64(limit))
	cur, err := Db.Db.Collection(ColNames.Connectors).Find(ctx, bson.M{"tenant_id": tenantID}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var connectors []*databasev1.Connector
	for cur.Next(ctx) {
		var c databasev1.Connector
		if err := cur.Decode(&c); err != nil {
			return nil, err
		}
		connectors = append(connectors, &c)
	}
	return connectors, cur.Err()
}

func SetConnectorCredentialRef(ctx context.Context, tenantID, id, credentialRefID string) (*databasev1.Connector, error) {
	update := bson.M{"$set": bson.M{"credential_ref_id": credentialRefID}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var c databasev1.Connector
	err := Db.Db.Collection(ColNames.Connectors).
		FindOneAndUpdate(ctx, bson.M{"_id": id, "tenant_id": tenantID}, update, opts).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func UpdateConnectorManifest(ctx context.Context, tenantID, id, manifestJSON, status string) (*databasev1.Connector, error) {
	update := bson.M{"$set": bson.M{
		"capability_manifest":   manifestJSON,
		"status":                status,
		"manifest_refreshed_at": time.Now().UTC().Format(time.RFC3339),
	}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var c databasev1.Connector
	err := Db.Db.Collection(ColNames.Connectors).
		FindOneAndUpdate(ctx, bson.M{"_id": id, "tenant_id": tenantID}, update, opts).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func DeleteConnector(ctx context.Context, tenantID, id string) error {
	_, err := Db.Db.Collection(ColNames.Connectors).DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	return err
}
