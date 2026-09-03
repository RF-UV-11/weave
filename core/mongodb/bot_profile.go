package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	databasev1 "weave/core/gen/database/v1"
)

func CreateBotProfile(ctx context.Context, tenantID, name, persona string, connectorIDs, channels []string, rolesAllowed []databasev1.Role, visibility string, guardrails []string, webSearchEnabled bool) (*databasev1.BotProfile, error) {
	p := &databasev1.BotProfile{
		XId:              "profile_" + newULID(),
		TenantId:         tenantID,
		Name:             name,
		Persona:          persona,
		ConnectorIds:     connectorIDs,
		Channels:         channels,
		RolesAllowed:     rolesAllowed,
		Visibility:       visibility,
		Guardrails:       guardrails,
		WebSearchEnabled: webSearchEnabled,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := Db.Db.Collection(ColNames.BotProfiles).InsertOne(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetActiveBotProfileByChannel returns the first bot profile registered
// under tenantID whose channels include channel. Scoped to tenantID like
// every other lookup in core (docs/architecture/SECURITY.md §2).
func GetActiveBotProfileByChannel(ctx context.Context, tenantID, channel string) (*databasev1.BotProfile, error) {
	var p databasev1.BotProfile
	err := Db.Db.Collection(ColNames.BotProfiles).
		FindOne(ctx, bson.M{"tenant_id": tenantID, "channels": channel}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetBotProfile(ctx context.Context, tenantID, id string) (*databasev1.BotProfile, error) {
	var p databasev1.BotProfile
	err := Db.Db.Collection(ColNames.BotProfiles).
		FindOne(ctx, bson.M{"_id": id, "tenant_id": tenantID}).Decode(&p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListBotProfiles is scoped to tenantID — same isolation rule as every
// other lookup in core (docs/architecture/SECURITY.md §2). Admin UI use
// only; orchestrator resolves the active profile per turn via
// GetActiveBotProfileByChannel, never this.
func ListBotProfiles(ctx context.Context, tenantID string) ([]*databasev1.BotProfile, error) {
	cur, err := Db.Db.Collection(ColNames.BotProfiles).Find(ctx, bson.M{"tenant_id": tenantID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var profiles []*databasev1.BotProfile
	for cur.Next(ctx) {
		var p databasev1.BotProfile
		if err := cur.Decode(&p); err != nil {
			return nil, err
		}
		profiles = append(profiles, &p)
	}
	return profiles, cur.Err()
}
