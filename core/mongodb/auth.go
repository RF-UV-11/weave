package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"

	databasev1 "weave/core/gen/database/v1"
)

// CreateUser hashes password with bcrypt before it ever reaches Mongo —
// password is never stored or logged in plaintext.
func CreateUser(ctx context.Context, tenantID, email, password string, role databasev1.Role) (*databasev1.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &databasev1.User{
		XId:          "usr_" + newULID(),
		TenantId:     tenantID,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.Users).InsertOne(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByEmail looks up a user scoped to tenantID — the same tenant
// isolation rule as every other lookup in core (docs/architecture/SECURITY.md §2).
func GetUserByEmail(ctx context.Context, tenantID, email string) (*databasev1.User, error) {
	var u databasev1.User
	err := Db.Db.Collection(ColNames.Users).
		FindOne(ctx, bson.M{"tenant_id": tenantID, "email": email}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUser(ctx context.Context, tenantID, id string) (*databasev1.User, error) {
	var u databasev1.User
	err := Db.Db.Collection(ColNames.Users).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// VerifyPassword reports whether password matches the user's stored bcrypt hash.
func VerifyPassword(u *databasev1.User, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.GetPasswordHash()), []byte(password)) == nil
}
