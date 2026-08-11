package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	databasev1 "weave/core/gen/database/v1"
	"weave/core/vault"
)

// CreateCredential seals secret with v and inserts the resulting
// CredentialRef. secret is never persisted or logged in plaintext.
func CreateCredential(ctx context.Context, v *vault.Vault, tenantID, connectorID, secret string) (*databasev1.CredentialRef, error) {
	sealed, err := v.Seal([]byte(secret))
	if err != nil {
		return nil, err
	}

	c := &databasev1.CredentialRef{
		XId:         "cred_" + newULID(),
		TenantId:    tenantID,
		ConnectorId: connectorID,
		Ciphertext:  sealed.Ciphertext,
		Nonce:       sealed.Nonce,
		WrappedDek:  sealed.WrappedDEK,
		DekNonce:    sealed.DEKNonce,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.Credentials).InsertOne(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

func GetCredential(ctx context.Context, tenantID, id string) (*databasev1.CredentialRef, error) {
	var c databasev1.CredentialRef
	err := Db.Db.Collection(ColNames.Credentials).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// OpenCredential decrypts a stored CredentialRef back to its plaintext secret.
func OpenCredential(v *vault.Vault, c *databasev1.CredentialRef) (string, error) {
	plaintext, err := v.Open(&vault.Sealed{
		Ciphertext: c.GetCiphertext(),
		Nonce:      c.GetNonce(),
		WrappedDEK: c.GetWrappedDek(),
		DEKNonce:   c.GetDekNonce(),
	})
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func DeleteCredential(ctx context.Context, tenantID, id string) error {
	_, err := Db.Db.Collection(ColNames.Credentials).DeleteOne(ctx, bson.M{"_id": id, "tenant_id": tenantID})
	return err
}
