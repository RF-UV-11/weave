package mongodb

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	databasev1 "weave/core/gen/database/v1"
)

func newULID() string {
	ms := ulid.Timestamp(time.Now())
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ms, entropy).String()
}

func CreateTenant(ctx context.Context, displayName, tenantType string) (*databasev1.Tenant, error) {
	t := &databasev1.Tenant{
		XId:         "tnt_" + newULID(),
		DisplayName: displayName,
		TenantType:  tenantType,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	_, err := Db.Db.Collection(ColNames.Tenants).InsertOne(ctx, t)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func GetTenant(ctx context.Context, id string) (*databasev1.Tenant, error) {
	var t databasev1.Tenant
	err := Db.Db.Collection(ColNames.Tenants).FindOne(ctx, bson.M{"_id": id}).Decode(&t)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func ListTenants(ctx context.Context, limit int32) ([]*databasev1.Tenant, error) {
	if limit <= 0 {
		limit = 50
	}
	findOpts := options.Find().SetLimit(int64(limit))
	cur, err := Db.Db.Collection(ColNames.Tenants).Find(ctx, bson.M{}, findOpts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var tenants []*databasev1.Tenant
	for cur.Next(ctx) {
		var t databasev1.Tenant
		if err := cur.Decode(&t); err != nil {
			return nil, err
		}
		tenants = append(tenants, &t)
	}
	return tenants, cur.Err()
}
